# SPDX-License-Identifier: Apache-2.0
"""kicad_step: a board as a STEP model, for mechanical design."""

load(":design.bzl", "KicadPcbInfo")
load(":normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE", "normalize")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

def _kicad_step_impl(ctx):
    pcb = ctx.attr.pcb[KicadPcbInfo]
    raw = ctx.actions.declare_file(ctx.label.name + ".raw.step")

    # No parts: their 3D models are not in the build yet (issue #43). Without
    # this flag kicad-cli leaves each part out with a warning and exits 0.
    # No copper either: kicad-cli 9.0.9 orders the copper shells differently
    # on every run (issue #63).
    args = ["pcb", "export", "step", "--no-components", "--output", raw.path, pcb.board.path]
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = args,
        inputs = pcb.files,
        outputs = [raw],
        mnemonic = "KicadStep",
        progress_message = "Exporting %{label} to STEP",
    )
    out = ctx.actions.declare_file(ctx.label.name + ".step")
    normalize(ctx, raw, out)
    return [DefaultInfo(files = depset([out]))]

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
`<board>_PCB`. It holds no parts yet: their 3D models come with issue
#43. It holds no copper: kicad-cli writes tracks, pads and zones in a
different order on every run (issue #63).

The time stamp in the header is set to
`--@rules_kicad//kicad:source_date_epoch` (default 1970-01-01), so the
same board gives the same bytes on the same platform. Across platforms
the geometry may differ in the last digits; rules_brep compares models
by tolerance.
""",
    attrs = {
        "pcb": attr.label(
            doc = "The board (`kicad_pcb`).",
            mandatory = True,
            providers = [KicadPcbInfo],
        ),
    },
    toolchains = [_KICAD, NORMALIZE_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
