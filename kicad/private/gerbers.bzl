# SPDX-License-Identifier: Apache-2.0
"""kicad_gerbers: a board's Gerber files, for a board house."""

load(":design.bzl", "KicadPcbInfo")
load(":normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE", "normalize")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

# Every copper layer KiCad has: it skips the ones a board does not, so a
# board gets exactly its own copper layers, however many.
COPPER_LAYERS = ["F.Cu"] + ["In%d.Cu" % i for i in range(1, 31)] + ["B.Cu"]

# The layers a board house needs besides copper.
FAB_LAYERS = [
    "F.Paste",
    "B.Paste",
    "F.Silkscreen",
    "B.Silkscreen",
    "F.Mask",
    "B.Mask",
    "Edge.Cuts",
]

DEFAULT_GERBER_LAYERS = COPPER_LAYERS + FAB_LAYERS

def _kicad_gerbers_impl(ctx):
    pcb = ctx.attr.pcb[KicadPcbInfo]
    if not ctx.attr.layers:
        fail("%s: layers is empty" % ctx.label)
    for layer in ctx.attr.layers:
        if not layer or "," in layer:
            fail("%s: layer %r is empty or has a comma" % (ctx.label, layer))
    raw = ctx.actions.declare_directory(ctx.label.name + ".raw")

    # kicad-cli's defaults: Gerber X2 with netlist attributes, Protel file
    # extensions, absolute coordinates. It has flags only to turn these off,
    # so they cannot be given; tests/gerbers checks them in the files. The
    # precision has a flag, so it is given.
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = [
            "pcb",
            "export",
            "gerbers",
            "--layers",
            ",".join(ctx.attr.layers),
            "--precision",
            "6",
            "--output",
            raw.path,
            pcb.board.path,
        ],
        inputs = pcb.files,
        outputs = [raw],
        mnemonic = "KicadGerbers",
        progress_message = "Plotting the Gerber files of %{label}",
    )
    out = ctx.actions.declare_directory(ctx.label.name)
    normalize(ctx, raw, out)
    return [DefaultInfo(files = depset([out]))]

kicad_gerbers = rule(
    implementation = _kicad_gerbers_impl,
    doc = """A board's Gerber files, and its Gerber job file, in one directory.

```starlark
kicad_gerbers(
    name = "gerbers",
    pcb = ":pcb",
)
```

The output is the directory `<name>`, with one file per layer the board
has (`<board>-F_Cu.gtl`, `<board>-Edge_Cuts.gm1`, ...) and
`<board>-job.gbrjob`. The files are Gerber X2, with netlist attributes, 6
decimals and Protel file extensions.

The default `layers` lists every copper layer KiCad has; kicad-cli skips
the ones a board does not have, so a board gets all of its copper layers,
however many. Besides copper: paste, silkscreen and mask on both sides,
and the board outline. The time stamps are set to
`--@rules_kicad//kicad:source_date_epoch` (default 1970-01-01), so the
same board gives the same bytes.
""",
    attrs = {
        "pcb": attr.label(
            doc = "The board (`kicad_pcb`).",
            mandatory = True,
            providers = [KicadPcbInfo],
        ),
        "layers": attr.string_list(
            doc = "The layers to plot, by KiCad's names (F.Cu, In1.Cu, Edge.Cuts, ...). " +
                  "Layers the board does not have are skipped. The default is " +
                  "`DEFAULT_GERBER_LAYERS` from `@rules_kicad//kicad:defs.bzl`; to add a " +
                  "layer, write `layers = DEFAULT_GERBER_LAYERS + [\"F.Fab\"]`.",
            default = DEFAULT_GERBER_LAYERS,
        ),
    },
    toolchains = [_KICAD, NORMALIZE_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
