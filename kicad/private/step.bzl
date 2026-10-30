# SPDX-License-Identifier: Apache-2.0
"""kicad_step: a board as a STEP model, for mechanical design."""

load(":design.bzl", "KicadPcbInfo")
load(":normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE", "normalize")
load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

# The names KiCad's footprints use for the 3D model directory, by version.
_MODEL_DIR_VARS = [
    "KICAD6_3DMODEL_DIR",
    "KICAD7_3DMODEL_DIR",
    "KICAD8_3DMODEL_DIR",
    "KICAD9_3DMODEL_DIR",
    "KISYS3DMOD",
]

def _package_relative(f):
    """The path of `f` inside its package: "<lib>.3dshapes/x.step".

    A library of @kicad_models has its files at the top of its repository;
    a library of one's own may sit in any package.
    """
    parts = f.short_path.split("/")
    if parts[0] == "..":
        parts = parts[2:]
    rel = "/".join(parts)
    pkg = f.owner.package
    if pkg and rel.startswith(pkg + "/"):
        rel = rel[len(pkg) + 1:]
    return rel

def _stage_models(ctx):
    """Links the model libraries into one directory, as KiCad expects them.

    Returns the directory's path, relative to the execution root, and the
    links. Each library repository holds <lib>.3dshapes/ and LICENSE.md; the
    directory gets every <lib>.3dshapes/ side by side.
    """
    root = ctx.label.name + "_models"
    links = []
    rels = []
    seen = {}
    for f in ctx.files.models:
        rel = _package_relative(f)
        if rel == "LICENSE.md":
            continue
        segs = rel.split("/")
        if len(segs) < 2 or not segs[0].endswith(".3dshapes"):
            fail("%s: %s is not in a <name>.3dshapes directory; give targets of @kicad_models, or files under <name>.3dshapes/ of a library of your own" % (ctx.label, f.short_path))
        if rel in seen:
            fail("%s: %s comes from two models targets" % (ctx.label, rel))
        seen[rel] = True
        link = ctx.actions.declare_file(root + "/" + rel)
        ctx.actions.symlink(output = link, target_file = f)
        links.append(link)
        rels.append(rel)
    if not links:
        fail("%s: models has no model file" % ctx.label)

    # The directory that holds the links: a link's path less its <rel>.
    return links[0].path[:-(len(rels[0]) + 1)], links

def _kicad_step_impl(ctx):
    pcb = ctx.attr.pcb[KicadPcbInfo]
    raw = ctx.actions.declare_file(ctx.label.name + ".raw.step")

    args = ["pcb", "export", "step"]
    inputs = [pcb.files]
    path_env = {}
    log = None
    if ctx.attr.models:
        # KiCad's footprints name their models as
        # ${KICAD9_3DMODEL_DIR}/<lib>.3dshapes/<model>; footprints from
        # older KiCad versions use the older names.
        models_dir, links = _stage_models(ctx)
        for name in _MODEL_DIR_VARS:
            path_env[name] = models_dir

        # The links, and what they point at.
        inputs.append(depset(links + ctx.files.models))

        # kicad-cli leaves out a part whose model it cannot find, says so
        # on its standard output, and exits 0; stepcheck fails on that.
        log = ctx.actions.declare_file(ctx.label.name + ".step.log")
    else:
        # Without models, kicad-cli would leave each part out with a warning
        # and exit 0; say so instead.
        args.append("--no-components")

    # Copper is optional. kicad-cli 9.0.9 writes its shells, and the parts',
    # in a different order on every run; the normalizer renumbers them
    # (issue #63).
    if ctx.attr.copper:
        args += ["--include-tracks", "--include-pads", "--include-zones"]
    args += ["--output", raw.path, pcb.board.path]
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = args,
        inputs = depset(transitive = inputs),
        outputs = [raw],
        path_env = path_env,
        stdout = log,
        mnemonic = "KicadStep",
        progress_message = "Exporting %{label} to STEP",
    )
    out = ctx.actions.declare_file(ctx.label.name + ".step")
    normalize(ctx, raw, out)
    if log == None:
        return [DefaultInfo(files = depset([out]))]

    # A validation action: Bazel runs it with every build of the target.
    stamp = ctx.actions.declare_file(ctx.label.name + ".stepcheck")
    run_tool(
        ctx,
        HELPERS_TOOLCHAIN_TYPE,
        "stepcheck",
        arguments = ["--stamp", stamp.path, log.path],
        inputs = [log],
        outputs = [stamp],
        mnemonic = "KicadStepCheck",
        progress_message = "Checking that every part of %{label} has its 3D model",
    )
    return [
        DefaultInfo(files = depset([out])),
        OutputGroupInfo(_validation = depset([stamp])),
    ]

kicad_step = rule(
    implementation = _kicad_step_impl,
    doc = """A board as a STEP model: `<name>.step`.

```starlark
kicad_step(
    name = "step",
    pcb = ":pcb",
)
```

The model holds the board body: its outline, thickness and holes, as
`<board>_PCB`. With `copper = True` it also holds the copper: tracks and
zones as `<board>_copper`, pads as `<board>_pad`, and vias as
`<board>_via`. With `models`, it also holds the parts whose footprints
name a model in those libraries:

```starlark
kicad_step(
    name = "step",
    models = [
        "@kicad_models//:LED_SMD",
        "@kicad_models//:Resistor_SMD",
    ],
    pcb = ":pcb",
)
```

Only the libraries listed are fetched (over BitTorrent; see
`@hdlf_rules_kicad//kicad:models.bzl`). With `models`, the build fails when a
part's model is not in them, and names the library to add. kicad-cli
writes the shells of copper and parts in a different order on every run;
the normalizer renumbers them canonically (issue #63), so the bytes are
stable.

The time stamp in the header is set to
`--@hdlf_rules_kicad//kicad:source_date_epoch` (default 1970-01-01), so the
same board gives the same bytes on the same platform. Across platforms
the geometry may differ in the last digits; rules_brep compares models
by tolerance.
""",
    attrs = {
        "copper": attr.bool(
            doc = "Export the copper too: tracks, vias, pads and zones.",
        ),
        "pcb": attr.label(
            doc = "The board (`kicad_pcb`).",
            mandatory = True,
            providers = [KicadPcbInfo],
        ),
        "models": attr.label_list(
            doc = "3D model libraries: targets of `@kicad_models` (one per library), or " +
                  "files under `<name>.3dshapes/` of a library of your own. " +
                  "Without them the model has no parts.",
            allow_files = True,
        ),
    },
    toolchains = [_KICAD, NORMALIZE_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
