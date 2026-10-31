# SPDX-License-Identifier: Apache-2.0
"""kicad_step: a board as a STEP model, for mechanical design."""

load(":design.bzl", "KicadPcbInfo")
load(":normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE", "normalize")
load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE", "RUN_TOOL_TOOLCHAINS", "run_tool")
load(":stage_models.bzl", "MODEL_DIR_VARS", "stage_models")

_KICAD = Label("//kicad:toolchain_type")

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
        models_dir, links = stage_models(ctx)
        for name in MODEL_DIR_VARS:
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
                  "files under `<name>.3dshapes/` of a library of your own, with " +
                  "`<name>.3dshapes/` at the top of its Bazel package. " +
                  "Without them the model has no parts.",
            allow_files = True,
        ),
    },
    toolchains = [_KICAD, NORMALIZE_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
