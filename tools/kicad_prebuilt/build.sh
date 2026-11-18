#!/bin/bash
# SPDX-License-Identifier: Apache-2.0
#
# Makes the KiCad prebuilt archive from KiCad's lite AppImage, without
# running anything from it (issue #100). It runs inside the pinned Ubuntu
# container that `bazel run //tools:kicad_prebuilt` starts, never on the
# host. The Go tool embeds this file, mounts the output directory at /out
# and the host's CA bundle at /host-ca.crt (read-only), and passes the
# pinned inputs as environment variables:
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
# /out receives the archive, the two inputs as downloaded, and SHA256SUMS,
# and only once everything has worked: a failed run leaves it empty.
set -euo pipefail
JOBS=${JOBS:-4}
export SOURCE_DATE_EPOCH=0
mkdir -p /work

# Every package from one snapshot of the archive, so the versions are fixed.
# The snapshot is served over HTTPS only, and the image has no CA
# certificates: apt checks the server against the host's CA bundle, which
# only verifies the connection. apt still checks every package against the
# archive's signed index, and nothing comes from the live archive.
rm -f /etc/apt/sources.list /etc/apt/sources.list.d/*
cat > /etc/apt/sources.list.d/snapshot.sources <<SRC
Types: deb
URIs: https://snapshot.ubuntu.com/ubuntu/$APT_SNAPSHOT/
Suites: resolute resolute-updates resolute-security
Components: main universe
Signed-By: /usr/share/keyrings/ubuntu-archive-keyring.gpg
SRC
APT=(-o Acquire::https::CaInfo=/host-ca.crt -o Acquire::Retries=3)
apt-get "${APT[@]}" update -q
DEBIAN_FRONTEND=noninteractive apt-get "${APT[@]}" install -y -q --no-install-recommends \
  ca-certificates curl xz-utils \
  g++-15 cmake ninja-build pkg-config \
  libacl1-dev libarchive-dev libboost-chrono-dev libboost-context-dev \
  libboost-filesystem-dev libboost-process-dev libboost-program-options-dev \
  libboost-regex-dev libboost-thread-dev libbrotli-dev libflac++-dev \
  libfmt-dev libjemalloc-dev liblz4-dev liblzma-dev libmagic-dev \
  libparallel-hashmap-dev librange-v3-dev libssl-dev libutfcpp-dev \
  libxxhash-dev libzstd-dev nlohmann-json3-dev

fetch() { # URL SHA256 FILE
  curl -fsSL --retry 5 --retry-all-errors -o "$3" "$1"
  echo "$2  $3" | sha256sum -c -
}

# dwarfsextract, from the release source tarball. CMake must not fetch
# anything: a system library too old for DwarFS stops the build.
fetch "$DWARFS_URL" "$DWARFS_SHA256" /work/dwarfs-src.tar.xz
mkdir -p /src && tar -xJf /work/dwarfs-src.tar.xz -C /src --strip-components=1
CC=gcc-15 CXX=g++-15 cmake -S /src -B /build -G Ninja \
  -DCMAKE_BUILD_TYPE=Release \
  -DFETCHCONTENT_FULLY_DISCONNECTED=ON \
  -DWITH_LIBDWARFS=ON -DWITH_TOOLS=ON -DWITH_FUSE_DRIVER=OFF \
  -DWITH_TESTS=OFF -DWITH_MAN_OPTION=OFF -DWITH_DESKTOP_INTEGRATION=OFF
cmake --build /build -j"$JOBS" --target dwarfsextract
X=/build/dwarfsextract
[[ -x $X ]] || { echo "no dwarfsextract at $X" >&2; exit 1; }

# The AppImage: the tarball holds it. Its DwarFS image starts right after
# the ELF runtime; dwarfsextract finds it with --image-offset=auto.
fetch "$APPIMAGE_URL" "$APPIMAGE_SHA256" /work/appimage.tar
tar -xf /work/appimage.tar -C /work "$APPIMAGE_NAME"
mkdir -p "/work/$PREBUILT"
"$X" --input="/work/$APPIMAGE_NAME" --image-offset=auto --output="/work/$PREBUILT"

# What the prebuilt leaves out: KiCad's footprint, symbol, demo and
# translation files. The libraries come from their own mirrored archives
# (prebuilt-kicad-libs-9.0.9); the rest the rules never read. Each must be
# there: a KiCad that moved them would otherwise pass unnoticed.
for d in footprints symbols demos internat; do
  [[ -d /work/$PREBUILT/share/kicad/$d ]] || { echo "no share/kicad/$d in the AppImage" >&2; exit 1; }
  rm -rf "/work/$PREBUILT/share/kicad/$d"
done
# The archive's top directory is 0775, as in the pinned archive.
chmod 0775 "/work/$PREBUILT"

# The archive: POSIX pax, sorted, every time 0, owner root, no atime or
# ctime records, xz -9 in one thread. These settings reproduce the pinned
# archive byte for byte.
(cd /work && tar --format=posix --pax-option=delete=atime,delete=ctime --sort=name \
  --mtime='@0' --owner=0 --group=0 --numeric-owner -cf - "$PREBUILT") |
  xz -9 -T1 > "/work/$PREBUILT.tar.xz"
tar --version | sed -n 1p
xz --version | sed -n 1p

# Only now into /out, so a failed run leaves it empty.
(cd /work && sha256sum "$PREBUILT.tar.xz" dwarfs-src.tar.xz appimage.tar > SHA256SUMS)
cp /work/"$PREBUILT.tar.xz" /work/dwarfs-src.tar.xz /work/appimage.tar /work/SHA256SUMS /out/
cat /out/SHA256SUMS
