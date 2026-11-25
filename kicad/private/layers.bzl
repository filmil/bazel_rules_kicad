# SPDX-License-Identifier: Apache-2.0
"""kicad_layers: layers of a board plotted flat, as SVG and optionally PNG."""

load(":design.bzl", "KicadPcbInfo")
load(":normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE", "normalize")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

# The options this rule gives kicad-cli itself; `options` may not repeat
# or override them.
_FIXED_OPTIONS = [
    "--output",
    "--layers",
    "--mirror",
    "--mode-single",
    "--mode-multi",
    "--page-size-mode",
    "--exclude-drawing-sheet",
    "--drill-shape-opt",
]

def _kicad_layers_impl(ctx):
    for opt in ctx.attr.options:
        # Short options can bundle letters (-bm) or attach a value
        # (-ox.svg), so only long ones are taken.
        if not opt.startswith("--") or opt == "--":
            fail("%s: options may hold long options only (--name or --name=value), not %s" % (ctx.label, opt))
        if opt.split("=")[0] in _FIXED_OPTIONS:
            fail("%s: options may not hold %s; the rule sets it" % (ctx.label, opt))
    if ctx.attr.png_dpi < 0:
        fail("%s: png_dpi is %d; it must be positive, or 0 for no PNG" % (ctx.label, ctx.attr.png_dpi))
    pcb = ctx.attr.pcb[KicadPcbInfo]
    raw = ctx.actions.declare_file(ctx.label.name + ".raw/" + ctx.label.name + ".svg")
    svg = ctx.actions.declare_file(ctx.label.name + ".svg")

    # Every option that shapes the plot is given, so a change in
    # kicad-cli's defaults cannot change it: one file, the board's area
    # only, no drawing sheet, drill holes at their real size.
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = [
            "pcb",
            "export",
            "svg",
            "--mode-single",
            "--page-size-mode",
            "2",
            "--exclude-drawing-sheet",
            "--drill-shape-opt",
            "2",
            "--layers",
            ",".join(ctx.attr.layers),
            "--output",
            raw.path,
        ] + (["--mirror"] if ctx.attr.mirror else []) + ctx.attr.options + [pcb.board.path],
        inputs = pcb.files,
        outputs = [raw],
        mnemonic = "KicadLayersSvg",
        progress_message = "Plotting %s of %%{label}" % ", ".join(ctx.attr.layers),
    )

    # kicad-cli writes the time of the plot into the SVG's <title>.
    normalize(ctx, raw, svg, mnemonic = "KicadLayersNormalize")
    outs = [svg]
    groups = {"svg": depset([svg])}
    if ctx.attr.png_dpi:
        png = ctx.actions.declare_file(ctx.label.name + ".png")
        run_tool(
            ctx,
            NORMALIZE_TOOLCHAIN_TYPE,
            "svgpng",
            arguments = ["--dpi", str(ctx.attr.png_dpi), svg.path, png.path],
            inputs = [svg],
            outputs = [png],
            mnemonic = "KicadLayersPng",
            progress_message = "Drawing %{output}",
        )
        outs.append(png)
        groups["png"] = depset([png])
    return [DefaultInfo(files = depset(outs)), OutputGroupInfo(**groups)]

kicad_layers = rule(
    implementation = _kicad_layers_impl,
    doc = """Layers of a board plotted flat, as one SVG, and optionally a PNG.

```starlark
kicad_layers(
    name = "b_cu",
    pcb = ":pcb",
    layers = ["B.Cu", "Edge.Cuts"],
    mirror = True,
    png_dpi = 600,
)
```

The outputs are `<name>.svg` and, when `png_dpi` is set, `<name>.png`;
the output groups `svg` and `png` hold one each, for a rule that takes
a single file (`filegroup(srcs = [":b_cu"], output_group = "png")`).
They are drawn by `kicad-cli pcb export svg` with every option that
shapes the plot given: all layers in one image, cut to the board's area,
with no drawing sheet, and drill holes at their real size. The colors are
those of KiCad's default theme unless `options` says otherwise.

kicad-cli writes the time of the plot into the SVG's `<title>`; the
normalize toolchain sets it to `--@hdlf_rules_kicad//kicad:source_date_epoch`,
so the same board gives the same bytes. The PNG is drawn from the SVG by
svgpng, as `kicad_sch_render` draws its sheets. KiCad writes text as
strokes, so no font is read.
""",
    attrs = {
        "pcb": attr.label(
            doc = "The board (`kicad_pcb`).",
            mandatory = True,
            providers = [KicadPcbInfo],
        ),
        "layers": attr.string_list(
            doc = "The layers, by KiCad's untranslated names, such as `F.Cu`, " +
                  "`F.Silkscreen` and `Edge.Cuts`. All are drawn in one image.",
            mandatory = True,
            allow_empty = False,
        ),
        "mirror": attr.bool(
            doc = "Mirror the plot, as the board's bottom is seen from below.",
            default = False,
        ),
        "options": attr.string_list(
            doc = "More `kicad-cli pcb export svg` options, such as " +
                  "`--black-and-white` or `--sketch-pads-on-fab-layers`, in " +
                  "their long forms. Not the ones the rule sets: the output, the layers, " +
                  "`--mirror`, the mode, the page size, the drawing sheet " +
                  "and the drill shape.",
        ),
        "png_dpi": attr.int(
            doc = "Also draw `<name>.png`, at this many dots per inch: 600 " +
                  "makes a 50 mm board about 1180 pixels wide. 0, the " +
                  "default, draws no PNG.",
            default = 0,
        ),
    },
    toolchains = [_KICAD, NORMALIZE_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
