# SPDX-License-Identifier: Apache-2.0
"""kicad_drill: a board's drill files, Excellon, for a board house."""

load(":design.bzl", "KicadPcbInfo")
load(":normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE", "normalize")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

# The drill map formats whose time stamps normalize replaces, so a map comes
# out the same bytes each time. kicad-cli also writes ps, dxf and svg maps,
# which normalize does not know, so they are not offered.
DRILL_MAP_FORMATS = ["gerberx2", "pdf"]

def _kicad_drill_impl(ctx):
    pcb = ctx.attr.pcb[KicadPcbInfo]
    if ctx.attr.map_format and ctx.attr.map_format not in DRILL_MAP_FORMATS:
        fail("%s: map_format %r: want one of %s, or empty for no map" % (
            ctx.label,
            ctx.attr.map_format,
            ", ".join(DRILL_MAP_FORMATS),
        ))
    raw = ctx.actions.declare_directory(ctx.label.name + ".raw")

    # The drill options are kicad-cli's defaults, given anyway, so a change
    # in its defaults cannot change the files: Excellon in millimetres,
    # decimal coordinates, ovals as alternate slots, the board's origin.
    arguments = [
        "pcb",
        "export",
        "drill",
        "--format",
        "excellon",
        "--drill-origin",
        "absolute",
        "--excellon-units",
        "mm",
        "--excellon-zeros-format",
        "decimal",
        "--excellon-oval-format",
        "alternate",
    ]
    if ctx.attr.separate_th:
        arguments.append("--excellon-separate-th")
    if ctx.attr.map_format:
        arguments += ["--generate-map", "--map-format", ctx.attr.map_format]
    arguments += ["--output", raw.path, pcb.board.path]
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = arguments,
        inputs = pcb.files,
        outputs = [raw],
        mnemonic = "KicadDrill",
        progress_message = "Writing the drill files of %{label}",
    )
    out = ctx.actions.declare_directory(ctx.label.name)
    normalize(ctx, raw, out)
    return [DefaultInfo(files = depset([out]))]

kicad_drill = rule(
    implementation = _kicad_drill_impl,
    doc = """A board's drill files, Excellon, in one directory.

```starlark
kicad_drill(
    name = "drill",
    pcb = ":pcb",
)
```

The output is the directory `<name>`, with `<board>.drl`, every hole of
the board in one Excellon file, in millimetres with decimal coordinates
from the board's origin. With `separate_th = True` there are two files
instead, `<board>-PTH.drl` and `<board>-NPTH.drl`, for a board house that
drills plated and unplated holes apart; kicad-cli writes the NPTH file even
for a board with no unplated holes.

With `map_format` set, each drill file gets a map beside it, a drawing of
its holes by size: `<board>-drl_map.gbr` for `gerberx2`, or
`<board>-drl_map.pdf` for `pdf` (`<board>-PTH-drl_map.gbr` and so on with
`separate_th`).

The time stamps, the date of the drill file's header and the map's, are set
to `--@hdlf_rules_kicad//kicad:source_date_epoch` (default 1970-01-01), so the
same board gives the same bytes.
""",
    attrs = {
        "map_format": attr.string(
            doc = "The drill map's format, `gerberx2` or `pdf`; empty for no map.",
            default = "",
        ),
        "pcb": attr.label(
            doc = "The board (`kicad_pcb`).",
            mandatory = True,
            providers = [KicadPcbInfo],
        ),
        "separate_th": attr.bool(
            doc = "Plated and unplated holes in two files, `-PTH` and `-NPTH`, rather than one.",
            default = False,
        ),
    },
    toolchains = [_KICAD, NORMALIZE_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
