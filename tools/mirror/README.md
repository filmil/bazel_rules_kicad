<!-- SPDX-License-Identifier: Apache-2.0 -->
# Prebuilt KiCad Archive Creation

The KiCad prebuilt archive (`kicad-9.0.9.1-linux-x86_64.tar.xz`, sha256
`7f570bd2161a2dbdd0dca9ef7ba0ab751d422b379c6209385491ecc047f93bff`, in the
GitHub release `prebuilt-kicad-9.0.9.1`) is made from KiCad's official
lite AppImage without running anything from it:

```sh
bazel run //tools:kicad_prebuilt -- --out=/tmp/kicad-prebuilt
```

The tool (`tools/kicad_prebuilt`) needs podman on the host. It runs
`build.sh` in Ubuntu 26.04 (linux/amd64), pinned by digest, with every
package from one snapshot of the Ubuntu archive. The snapshot is served
over HTTPS, and the image has no CA certificates, so apt checks the server
against a copy of the host's CA bundle. That copy verifies the connection
and reaches no output. The tool fails unless the archive's sha256 is the
one above. Separate runs give the same bytes, and they match the archive
that was mirrored first. It takes about half an hour, most of it compiling
DwarFS.

`--out` must be missing or empty. It receives the archive, the two inputs
as downloaded (`dwarfs-src.tar.xz`, `appimage.tar`) and `SHA256SUMS`, and
only once the run has worked: a failed run leaves it empty.
`--dry_run` prints the podman command with the script as a here-document,
which a shell runs as it is.

## What the tool does

1. **Downloads the AppImage tarball**,
   `https://kicad-downloads.s3.cern.ch/appimage/stable/kicad-9.0.9.1-x86_64-lite.AppImage.tar`,
   and checks its sha256,
   `401ed5e3e32bcf96af89d9741597670107b6a7c30b550518739b2512e6e92b63`.
2. **Builds `dwarfsextract`** from DwarFS 0.15.10's release source tarball,
   checked by its sha256. The AppImage is a uruntime image whose filesystem
   is DwarFS, not squashfs, so `unsquashfs` cannot read it.
3. **Unpacks the AppImage's filesystem** with `dwarfsextract`, which finds
   the image after the ELF runtime by itself.
4. **Leaves out** `share/kicad/footprints`, `symbols`, `demos` and
   `internat`. The libraries come from their own mirrored archives
   (`prebuilt-kicad-libs-9.0.9`); the rules read neither the demos nor the
   translations. Each must be there, so a KiCad that moved them fails the
   build rather than passing unnoticed.
5. **Packs the archive** the same way every time: POSIX pax, sorted by
   name, every time 0, owner root, no atime or ctime records, and
   `xz -9` in one thread.

The archive keeps two links of the AppImage's Python, which point outside
it: `shared/lib/python3.11/sitecustomize.py` points to
`/etc/python3.11/sitecustomize.py`, and
`config-3.11-x86_64-linux-gnu/libpython3.11.so` points to
`../../x86_64-linux-gnu/libpython3.11.so.1`. An earlier version of this
README said they were removed, but the archive has always had them.
`kicad/private/extensions.bzl` removes them when it fetches the archive
(`remove`). Removing them from the archive itself would change its sha256,
and so the toolchain's pin.

## Verifying and uploading with //tools:mirror

Once the tool has made the archive in `--out`, give that directory as
`--built_dir`:
```sh
# Dry run: download upstreams and check SHA256 hashes
bazel run //tools:mirror

# Upload (operator only):
bazel run //tools:mirror -- --dry_run=false --built_dir=/path/to/built
```
