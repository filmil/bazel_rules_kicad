#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
#
# Makes the KiCad prebuilt archive from KiCad's lite AppImage, without
# running anything from it (issue #100). It runs inside the pinned Ubuntu
# container that `bazel run //tools:kicad_prebuilt` starts, never on the
# host, with the output directory mounted at /out. The Go tool embeds this
# file and passes the pinned inputs as environment variables:
#
#   APT_SNAPSHOT        the snapshot.ubuntu.com time of every package;
#   DWARFS_URL, DWARFS_SHA256
#                       DwarFS's release source tarball;
#   APPIMAGE_URL, APPIMAGE_SHA256, APPIMAGE_NAME
#                       KiCad's AppImage tarball, and the AppImage in it;
#   PREBUILT            the archive's name, without .tar.xz;
#   JOBS                parallel compile jobs.
#
# The AppImage is a uruntime image whose filesystem is DwarFS, not
# squashfs, so unsquashfs cannot read it. This builds DwarFS's
# dwarfsextract from source and extracts the image with it.
#
# /out receives the archive, the two inputs as downloaded, and SHA256SUMS.
set -euo pipefail
JOBS=${JOBS:-4}
export SOURCE_DATE_EPOCH=0

# Every package from one snapshot of the archive, so the versions are fixed.
# The snapshot is served over HTTPS only, and the image has no CA
# certificates: so ca-certificates comes first, from the image's own
# archive. It holds only root certificates and puts nothing into the
# output; every other package comes from the snapshot.
apt-get update -q
DEBIAN_FRONTEND=noninteractive apt-get install -y -q --no-install-recommends ca-certificates
rm -f /etc/apt/sources.list /etc/apt/sources.list.d/*
cat > /etc/apt/sources.list.d/snapshot.sources <<SRC
Types: deb
URIs: https://snapshot.ubuntu.com/ubuntu/$APT_SNAPSHOT/
Suites: resolute resolute-updates resolute-security
Components: main universe
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg
SRC
apt-get update -q
DEBIAN_FRONTEND=noninteractive apt-get install -y -q --no-install-recommends \
  curl xz-utils \
  g++-15 cmake ninja-build pkg-config \
  libacl1-dev libarchive-dev libboost-chrono-dev libboost-context-dev \
  libboost-filesystem-dev libboost-process-dev libboost-program-options-dev \
  libboost-regex-dev libboost-thread-dev libbrotli-dev libflac++-dev \
  libfmt-dev libjemalloc-dev liblz4-dev liblzma-dev libmagic-dev \
  libparallel-hashmap-dev librange-v3-dev libssl-dev libutfcpp-dev \
  libxxhash-dev libzstd-dev nlohmann-json3-dev

fetch() { # URL SHA256 FILE
  curl -fsSL -o "$3" "$1"
  echo "$2  $3" | sha256sum -c -
}

# dwarfsextract, from the release source tarball. CMake must not fetch
# anything: a system library too old for DwarFS stops the build.
fetch "$DWARFS_URL" "$DWARFS_SHA256" /out/dwarfs-src.tar.xz
mkdir -p /src && tar -xJf /out/dwarfs-src.tar.xz -C /src --strip-components=1
CC=gcc-15 CXX=g++-15 cmake -S /src -B /build -G Ninja \
  -DCMAKE_BUILD_TYPE=Release \
  -DFETCHCONTENT_FULLY_DISCONNECTED=ON \
  -DWITH_LIBDWARFS=ON -DWITH_TOOLS=ON -DWITH_FUSE_DRIVER=OFF \
  -DWITH_TESTS=OFF -DWITH_MAN_OPTION=OFF -DWITH_DESKTOP_INTEGRATION=OFF
cmake --build /build -j"$JOBS" --target dwarfsextract
X=$(find /build -name dwarfsextract -type f -perm -u+x | head -1)
"$X" --help | head -1

# The AppImage: the tarball holds it. Its DwarFS image starts right after
# the ELF runtime; dwarfsextract finds it with --image-offset=auto.
fetch "$APPIMAGE_URL" "$APPIMAGE_SHA256" /out/appimage.tar
mkdir -p /work && tar -xf /out/appimage.tar -C /work "$APPIMAGE_NAME"
mkdir -p "/work/$PREBUILT"
"$X" --input="/work/$APPIMAGE_NAME" --image-offset=auto --output="/work/$PREBUILT"

# What the prebuilt leaves out: KiCad's footprint, symbol, demo and
# translation files. The libraries come from their own mirrored archives
# (prebuilt-kicad-libs-9.0.9); the rest the rules never read.
for d in footprints symbols demos internat; do
  rm -rf "/work/$PREBUILT/share/kicad/$d"
done
# The archive's top directory is 0775, as in the pinned archive.
chmod 0775 "/work/$PREBUILT"

# The archive: POSIX pax, sorted, every time 0, owner root, no atime or
# ctime records, xz -9 in one thread. These settings reproduce the pinned
# archive byte for byte.
cd /work
tar --format=posix --pax-option=delete=atime,delete=ctime --sort=name \
  --mtime='@0' --owner=0 --group=0 --numeric-owner -cf - "$PREBUILT" |
  xz -9 -T1 > "/out/$PREBUILT.tar.xz"
tar --version | head -1
xz --version | head -1
cd /out && sha256sum "$PREBUILT.tar.xz" dwarfs-src.tar.xz appimage.tar > SHA256SUMS
cat SHA256SUMS
