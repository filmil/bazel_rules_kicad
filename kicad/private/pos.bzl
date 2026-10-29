# SPDX-License-Identifier: Apache-2.0
"""kicad_pos: a board's footprint positions, as CSV, for a pick-and-place machine."""

load(":design.bzl", "KicadPcbInfo")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

def _kicad_pos_impl(ctx):
    pcb = ctx.attr.pcb[KicadPcbInfo]
    base = ctx.label.name[:-len(".csv")] if ctx.label.name.endswith(".csv") else ctx.label.name
    out = ctx.actions.declare_file(base + ".csv")

    # The format, the side and the units are given, so a change in
    # kicad-cli's defaults (ASCII, inches) cannot change the file.
    args = [
        "pcb",
        "export",
        "pos",
        "--format",
        "csv",
        "--side",
        "both",
        "--units",
        "mm",
    ]
    if ctx.attr.smd_only:
        args.append("--smd-only")
    if ctx.attr.exclude_th:
        args.append("--exclude-fp-th")
    if ctx.attr.exclude_dnp:
        args.append("--exclude-dnp")
    if ctx.attr.bottom_negate_x:
        args.append("--bottom-negate-x")
    if ctx.attr.use_drill_file_origin:
        args.append("--use-drill-file-origin")
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = args + ["--output", out.path, pcb.board.path],
        inputs = pcb.files,
        outputs = [out],
        mnemonic = "KicadPos",
        progress_message = "Writing the footprint positions of %{label}",
    )
    return [DefaultInfo(files = depset([out]))]

kicad_pos = rule(
    implementation = _kicad_pos_impl,
    doc = """A board's footprint positions, as CSV, for a pick-and-place machine.

```starlark
kicad_pos(
    name = "pos",
    pcb = ":pcb",
)
```

The output is `<name>.csv` (or `<name>` when the name already ends in
`.csv`), with the parts of both sides in one file. The first row is
`Ref,Val,Package,PosX,PosY,Rot,Side`; each other row is one footprint,
with its position in millimetres, its rotation in degrees, and its side,
`top` or `bottom`. Positions are measured from the top-left corner of
KiCad's page, with Y growing upwards, so every part on the page has a
negative Y. With `use_drill_file_origin`, they are measured from the
board's drill and place file origin instead, which a pick-and-place machine
usually wants. The output has no time stamp.
""",
    attrs = {
        "pcb": attr.label(
            doc = "The board (`kicad_pcb`).",
            mandatory = True,
            providers = [KicadPcbInfo],
        ),
        "smd_only": attr.bool(
            doc = "List only footprints marked SMD.",
            default = False,
        ),
        "exclude_th": attr.bool(
            doc = "Leave out footprints that have a through-hole pad.",
            default = False,
        ),
        "exclude_dnp": attr.bool(
            doc = "Leave out parts marked \"do not populate\".",
            default = False,
        ),
        "bottom_negate_x": attr.bool(
            doc = "Negate the X of parts on the bottom side, as seen from below.",
            default = False,
        ),
        "use_drill_file_origin": attr.bool(
            doc = "Measure positions from the board's drill and place file origin, " +
                  "not from the top-left corner of the page.",
            default = False,
        ),
    },
    toolchains = [_KICAD] + RUN_TOOL_TOOLCHAINS,
)
