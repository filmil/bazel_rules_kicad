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

```starlark
bazel_dep(name = "hdlf_rules_kicad", version = "0.1.0")
```

`.bazelrc`, while the module is only in the filmil registry:

```
common --registry=https://raw.githubusercontent.com/filmil/bazel-registry/main
common --registry=https://bcr.bazel.build
```

The rules are documented in [docs/defs.md](docs/defs.md).

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
Prebuilt tools and libraries that the build downloads are listed here, with
their versions, sources and licenses, as they are added.

* KiCad's 3D models (kicad-packages3D 9.0.9,
  https://gitlab.com/kicad/libraries/kicad-packages3D): CC-BY-SA 4.0 with
  the KiCad libraries exception. Each library's archive carries the
  upstream `LICENSE.md`, and so does the directory it is served from.
