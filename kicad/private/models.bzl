# SPDX-License-Identifier: Apache-2.0
"""The `models` module extension: KiCad's 3D model libraries (issue #43)."""

load("@rules_bittorrent//:repo.bzl", "bittorrent_archive")
load(":models_pins.bzl", "MODELS", "MODELS_BASE_URL")

# Each 3D model library's repository: the archive holds LICENSE.md and
# <name>.3dshapes/.
_MODELS_BUILD = """\
# The KiCad 3D model library {name}, as published by //tools:models3d.
filegroup(
    name = "files",
    srcs = glob(["{name}.3dshapes/**"]) + ["LICENSE.md"],
    visibility = ["//visibility:public"],
)
"""

def _kicad_models_hub_impl(rctx):
    lines = [
        "# One target per KiCad 3D model library. Each is an alias into the",
        "# library's own repository, which Bazel fetches over BitTorrent only",
        "# when a build uses the target.",
    ]
    for name in rctx.attr.libraries:
        lines.append('alias(name = "%s", actual = "@kicad_models_%s//:files", visibility = ["//visibility:public"])' % (name, name))
    rctx.file("BUILD.bazel", "\n".join(lines) + "\n")

_kicad_models_hub = repository_rule(
    implementation = _kicad_models_hub_impl,
    attrs = {"libraries": attr.string_list(mandatory = True)},
)

def _models_impl(mctx):
    for name in sorted(MODELS.keys()):
        archive = name + ".3dshapes.tar.gz"
        bittorrent_archive(
            name = "kicad_models_" + name,
            uri = MODELS_BASE_URL + archive + ".torrent",
            file = archive,
            integrity = MODELS[name],
            build_file_content = _MODELS_BUILD.format(name = name),
            # At most 15 minutes in all, not rules_bittorrent's 200000 s: the
            # largest library takes seconds from the web seed. A rain that
            # hangs after a complete download costs one attempt (2 minutes);
            # rules_bittorrent 2.1.1 then checks the file and goes on
            # (https://github.com/filmil/bazel_rules_bt/issues/49).
            timeout = 900,
        )
    _kicad_models_hub(name = "kicad_models", libraries = sorted(MODELS.keys()))
    return mctx.extension_metadata(reproducible = True)

models = module_extension(
    implementation = _models_impl,
    doc = """KiCad's 3D model libraries, for `kicad_step(models = ...)`.

```starlark
models = use_extension("@hdlf_rules_kicad//kicad:models.bzl", "models")
use_repo(models, "kicad_models")
```

`@kicad_models` has one target per library, such as
`@kicad_models//:Resistor_SMD`. Each library is a repository of its own,
fetched over BitTorrent with rules_bittorrent from a `.torrent` whose web
seed is bt.hdlfactory.com, and only when a build uses it. The archives
are pinned by sha256 in `kicad/private/models_pins.bzl`, which
`bazel run //tools:models3d` writes.
""",
)
