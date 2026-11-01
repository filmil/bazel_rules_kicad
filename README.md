<!-- SPDX-License-Identifier: Apache-2.0 -->
[![Test](https://github.com/filmil/bazel_rules_kicad/actions/workflows/test.yml/badge.svg?branch=main)](https://github.com/filmil/bazel_rules_kicad/actions/workflows/test.yml)
[![Tag and Release](https://github.com/filmil/bazel_rules_kicad/actions/workflows/tag-and-release.yml/badge.svg)](https://github.com/filmil/bazel_rules_kicad/releases)
[![Publish to my Bazel registry](https://github.com/filmil/bazel_rules_kicad/actions/workflows/publish.yml/badge.svg)](https://github.com/filmil/bazel_rules_kicad/actions/workflows/publish.yml)
[![Publish on Bazel Central Registry](https://github.com/filmil/bazel_rules_kicad/actions/workflows/publish-bcr.yml/badge.svg)](https://github.com/filmil/bazel_rules_kicad/actions/workflows/publish-bcr.yml)

# hdlf_rules_kicad

Hermetic Bazel rules for KiCad schematics and boards.
The build checks a design (ERC, DRC) and makes its outputs (PDF, netlist,
BOM, Gerbers, drill, position, STEP, schematic and board renders).
Every tool comes from a Bazel toolchain, so nothing needs to be installed
on the machine, and two builds give byte-identical outputs.

This repository is new. The rules arrive in milestones; see the
[issues](https://github.com/filmil/bazel_rules_kicad/issues) and the
[milestones](https://github.com/filmil/bazel_rules_kicad/milestones).
Only Linux x86_64 hosts are supported.

## Using it

In `MODULE.bazel`:

```starlark
bazel_dep(name = "hdlf_rules_kicad", version = "0.1.0")

# KiCad's symbol and footprint libraries, for designs that use them.
kicad = use_extension("@hdlf_rules_kicad//kicad:extensions.bzl", "kicad")
use_repo(kicad, "kicad_libs")
```

`.bazelrc`, while the module is only in the filmil registry:

```
common --registry=https://raw.githubusercontent.com/filmil/bazel-registry/main
common --registry=https://bcr.bazel.build
```

In a `BUILD.bazel` next to the design's files, a schematic and its board,
checked, and made into the files a fab house takes:

```starlark
load(
    "@hdlf_rules_kicad//kicad:defs.bzl",
    "kicad_bom",
    "kicad_drc_test",
    "kicad_drill",
    "kicad_erc_test",
    "kicad_gerbers",
    "kicad_pcb",
    "kicad_pos",
    "kicad_render",
    "kicad_sch_pdf",
    "kicad_schematic",
)

kicad_schematic(
    name = "sch",
    srcs = ["power.kicad_sch"],  # the other sheets
    project = "main.kicad_pro",
    root = "main.kicad_sch",
    deps = ["@kicad_libs//:standard"],
)

kicad_erc_test(
    name = "erc_test",
    schematic = ":sch",
)

kicad_pcb(
    name = "pcb",
    board = "main.kicad_pcb",
    schematic = ":sch",
)

kicad_drc_test(
    name = "drc_test",
    pcb = ":pcb",
)

kicad_sch_pdf(
    name = "sch_pdf",
    schematic = ":sch",
)

kicad_bom(
    name = "bom",
    schematic = ":sch",
)

kicad_gerbers(
    name = "gerbers",
    pcb = ":pcb",
)

kicad_drill(
    name = "drill",
    pcb = ":pcb",
)

kicad_pos(
    name = "pos",
    pcb = ":pcb",
)

kicad_render(
    name = "render",
    pcb = ":pcb",
)
```

The board takes the schematic's libraries; `kicad_pcb(deps = [...])`
adds any the schematic does not bring.
`bazel test //...` runs ERC and DRC; `bazel build //...` makes the PDF,
BOM, Gerbers, drill and position files, and the renders. The DRC test
also checks that the board follows the schematic (schematic parity).
`integration/e2e` is a complete example, built as a user of the module
builds it.

The rules are documented in [docs/defs.md](docs/defs.md), and the module
extensions (the KiCad version, the libraries and the 3D models) in
[docs/extensions.md](docs/extensions.md).

### 3D models

`kicad_step(models = [...])` puts the parts on the board's STEP model,
and `kicad_render(models = [...])` on its images. Either fails the build
when a part's model is not in the libraries given, and names the
library to add. Without `models`, neither shows any part, and neither
checks. KiCad's 3D model libraries come from their own module
extension:

```starlark
models = use_extension("@hdlf_rules_kicad//kicad:models.bzl", "models")
use_repo(models, "kicad_models")
```

Each library, such as `@kicad_models//:Resistor_SMD`, is fetched over
BitTorrent (rules_bittorrent) from a `.torrent` whose web seed is
https://bt.hdlfactory.com/, and only when a build uses it. With no peers
online, the web seed serves the whole file.

### Merging KiCad files in git

Git merges `.kicad_sch`, `.kicad_pcb` and `.kicad_sym` files line by line.
KiCad's elements span many lines, so a line merge can interleave two of
them or leave a list unclosed, and KiCad no longer loads the file.
`kicad_merge` merges them element by element instead:

* Each top-level element is keyed by its `uuid`: symbols, wires,
  junctions, labels, sheets, footprints, tracks, vias and zones.
  Kinds without a uuid are keyed by name or number: `lib_symbols` is
  merged symbol by symbol, `sheet_instances` path by path, a board's nets
  by number, a library's symbols by name.
* An element added on one side is kept. One deleted on one side and
  unchanged on the other is deleted. One changed on one side takes the
  change.
* An element changed differently on both sides is a conflict. The driver
  names it by its key, leaves your side as it was, and git marks the file
  conflicted.
* Two placed symbols with one reference (both branches added an R1), or
  two nets with one name, are a conflict too: annotate one again.
* Unchanged elements come out byte for byte, so the result diffs well.

To install it for a repository, build the driver, then tell git when to
use it:

```sh
bazel build @hdlf_rules_kicad//kicad:kicad_merge
mkdir -p "$HOME/.local/bin"
cp "$(bazel cquery --output=files @hdlf_rules_kicad//kicad:kicad_merge)" "$HOME/.local/bin/kicad_merge"

git config merge.kicad.name "KiCad S-expression merge"
git config merge.kicad.driver "$HOME/.local/bin/kicad_merge %O %A %B %P"
```

```
# .gitattributes
*.kicad_sch merge=kicad
*.kicad_pcb merge=kicad
*.kicad_sym merge=kicad
```

## Building

```sh
bazel build //...
bazel test //...
cd integration && bazel test //...
```

After changing a `.bzl` file, regenerate the documentation with
`bazel run //docs:update_docs`.
Format with `bazel run //tools:buildifier`.

## Releasing

The operator runs the `Tag and Release` workflow
(`.github/workflows/tag-and-release.yml`, `workflow_dispatch`).
It picks the next version from the conventional commits since the last
tag, makes the tag, the release and its source archive, and opens one
pull request on the filmil registry and one on the Bazel Central Registry.
It needs the repository secret `BCR_PUBLISH_TOKEN`.

None of the modules published here are published with any sort of guarantee
whatsoever. Use at your own risk.

See the overall description at the [project page][pp].

[pp]: https://hdlfactory.com/post/2025/09/29/getting-ready-for-the-brave-new-bazel-modules-world/

## Licenses

This repository is under the Apache License 2.0 (`LICENSE`).
The prebuilt tools and libraries that the build downloads, with their
versions, sources and licenses:

* KiCad 9.0.9 (https://gitlab.com/kicad/code/kicad), the toolchain:
  GPL-3.0-or-later (https://www.kicad.org/about/licenses/). It is the
  official KiCad 9.0.9 lite AppImage for Linux x86_64, revision 1
  (https://kicad-downloads.s3.cern.ch/appimage/stable/kicad-9.0.9.1-x86_64-lite.AppImage.tar),
  unpacked without running it, pruned, and repacked as a tarball. This
  repository's release `prebuilt-kicad-9.0.9.1` hosts the tarball; its
  notes give the AppImage's sha256. The AppImage bundles the libraries KiCad needs
  (wxWidgets 3.3, Python 3.11 and others), each under its own license.
* KiCad's symbol and footprint libraries 9.0.9
  (https://gitlab.com/kicad/libraries/kicad-symbols,
  https://gitlab.com/kicad/libraries/kicad-footprints), as `@kicad_libs`:
  CC-BY-SA 4.0 with the KiCad libraries exception. Each archive carries
  the upstream `LICENSE.md`. This repository's release
  `prebuilt-kicad-libs-9.0.9` mirrors GitLab's archives.
* DejaVu fonts 2.37 (https://github.com/dejavu-fonts/dejavu-fonts), for
  KiCad's outline text: the Bitstream Vera fonts license, with the
  DejaVu changes in the public domain (the archive's `LICENSE`).
* KiCad's 3D models (kicad-packages3D 9.0.9,
  https://gitlab.com/kicad/libraries/kicad-packages3D): CC-BY-SA 4.0 with
  the KiCad libraries exception. Each library's archive carries the
  upstream `LICENSE.md`, and so does the directory it is served from.
  Fetched only when a build names a library in `models` of
  `kicad_step` or `kicad_render`.
* rain 2.2.1 (https://github.com/cenkalti/rain), the BitTorrent client
  that rules_bittorrent 2.1.1 runs to fetch the 3D models: MIT. Fetched
  only with the 3D models.
* The Go SDK (go1.26.7 with rules_go 0.64.2, https://go.dev): BSD-3-Clause.
  rules_go downloads it to build this module's helpers from source.
