<!-- SPDX-License-Identifier: Apache-2.0 -->
# Prebuilt KiCad Archive Creation

The KiCad prebuilt archive (`kicad-9.0.9.1-linux-x86_64.tar.xz`, hosted in GitHub
release `prebuilt-kicad-9.0.9.1`) is built from the official KiCad lite AppImage
tarball without running the binary.

## Exact packaging commands

The commands below correspond to the `make-kicad-prebuilt.sh` procedure referenced in
`manifest.json` and `kicad/private/extensions.bzl`:

1. **Download the upstream AppImage tar**:
   ```sh
   curl -fsSLO https://kicad-downloads.s3.cern.ch/appimage/stable/kicad-9.0.9.1-x86_64-lite.AppImage.tar
   echo "401ed5e3e32bcf96af89d9741597670107b6a7c30b550518739b2512e6e92b63  kicad-9.0.9.1-x86_64-lite.AppImage.tar" | sha256sum -c -
   ```

2. **Extract the AppImage from the archive**:
   ```sh
   tar -xf kicad-9.0.9.1-x86_64-lite.AppImage.tar
   ```

3. **Extract the AppImage filesystem without executing it**:
   ```sh
   unsquashfs -d kicad-9.0.9.1-linux-x86_64 kicad-9.0.9.1-x86_64-lite.AppImage
   ```

4. **Prune extraneous host-leaking symlinks**:
   ```sh
   rm -f kicad-9.0.9.1-linux-x86_64/shared/lib/python3.11/sitecustomize.py
   rm -f kicad-9.0.9.1-linux-x86_64/shared/lib/python3.11/config-3.11-x86_64-linux-gnu/libpython3.11.so
   ```

5. **Pack as a reproducible tarball**:
   ```sh
   tar --sort=name --mtime='@0' --owner=0 --group=0 --numeric-owner \
       -cJf kicad-9.0.9.1-linux-x86_64.tar.xz kicad-9.0.9.1-linux-x86_64
   sha256sum kicad-9.0.9.1-linux-x86_64.tar.xz
   # Verifies: 7f570bd2161a2dbdd0dca9ef7ba0ab751d422b379c6209385491ecc047f93bff
   ```

## Verifying and uploading with //tools:mirror

Once the archive is built in `--built_dir`:
```sh
# Dry run: download upstreams and check SHA256 hashes
bazel run //tools:mirror

# Upload (operator only):
bazel run //tools:mirror -- --dry_run=false --built_dir=/path/to/built
```
