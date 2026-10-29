# SPDX-License-Identifier: Apache-2.0
"""kicad_sch_render: a schematic's sheets as PNG images."""

load(":design.bzl", "KicadSchematicInfo")
load(":normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

def _kicad_sch_render_impl(ctx):
    if ctx.attr.dpi <= 0:
        fail("%s: dpi must be positive" % ctx.label)
    sch = ctx.attr.schematic[KicadSchematicInfo]
    svgs = ctx.actions.declare_directory(ctx.label.name + ".svg")
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = ["sch", "export", "svg", "--output", svgs.path, sch.root.path],
        inputs = sch.files,
        outputs = [svgs],
        mnemonic = "KicadSchSvg",
        progress_message = "Plotting the sheets of %{label} to SVG",
    )

    # The SVGs hold the time of the export in their title; the PNGs drawn
    # from them hold pixels only.
    out = ctx.actions.declare_directory(ctx.label.name)
    run_tool(
        ctx,
        NORMALIZE_TOOLCHAIN_TYPE,
        "svgpng",
        arguments = ["--dpi", str(ctx.attr.dpi), svgs.path, out.path],
        inputs = [svgs],
        outputs = [out],
        mnemonic = "KicadSchPng",
        progress_message = "Drawing the sheets of %{label} as PNG",
    )
    return [DefaultInfo(files = depset([out]))]

kicad_sch_render = rule(
    implementation = _kicad_sch_render_impl,
    doc = """A schematic's sheets as PNG images, one per sheet.

```starlark
kicad_sch_render(
    name = "sch_render",
    schematic = ":sch",
)
```

The output is the directory `<name>`, with `<root>.png` for the root
sheet and `<root>-<sheet name>.png` for each other sheet, in KiCad's
colors, with the drawing sheet and title block. kicad-cli plots each
sheet to SVG; a Go program built from source (oksvg and rasterx) draws
them. KiCad draws all text as strokes, so no font is read. The PNGs hold
no time stamp, so the same schematic gives the same bytes.

A pull request that changes a schematic attaches these images (AGENTS.md
rule 5).
""",
    attrs = {
        "schematic": attr.label(
            doc = "The schematic (`kicad_schematic`).",
            mandatory = True,
            providers = [KicadSchematicInfo],
        ),
        "dpi": attr.int(
            doc = "Dots per inch. 150 makes an A4 sheet about 1750 by 1240 pixels.",
            default = 150,
        ),
    },
    toolchains = [_KICAD, NORMALIZE_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
