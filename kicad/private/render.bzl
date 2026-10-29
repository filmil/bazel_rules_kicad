# SPDX-License-Identifier: Apache-2.0
"""kicad_render: a board's top and bottom, rendered in 3D as PNG images."""

load(":design.bzl", "KicadPcbInfo")
load(":normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE", "normalize")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

# The sides rendered, in the order of the outputs.
RENDER_SIDES = ["top", "bottom"]

# kicad-cli 9.0.9 hangs when a side is 34 pixels or less. 64 is
# clear of that.
_MIN_SIZE = 64

def _kicad_render_impl(ctx):
    for dim in ("width", "height"):
        v = getattr(ctx.attr, dim)
        if v < _MIN_SIZE:
            fail("%s: %s is %d; it must be at least %d" % (ctx.label, dim, v, _MIN_SIZE))
    pcb = ctx.attr.pcb[KicadPcbInfo]
    outs = []
    for side in RENDER_SIDES:
        raw = ctx.actions.declare_file("%s.raw/%s.png" % (ctx.label.name, side))

        # Every option is given, so a change in kicad-cli's defaults cannot
        # change the image. The basic quality is the one that gives the
        # same bytes on every run: the high quality samples at random.
        run_tool(
            ctx,
            _KICAD,
            "kicad_cli",
            arguments = [
                "pcb",
                "render",
                "--side",
                side,
                "--width",
                str(ctx.attr.width),
                "--height",
                str(ctx.attr.height),
                "--quality",
                "basic",
                "--background",
                "opaque",
                "--preset",
                "follow_plot_settings",
                "--zoom",
                "1",
                "--output",
                raw.path,
                pcb.board.path,
            ],
            inputs = pcb.files,
            outputs = [raw],
            mnemonic = "KicadRender",
            progress_message = "Rendering the %s of %%{label}" % side,
        )
        out = ctx.actions.declare_file("%s/%s.png" % (ctx.label.name, side))
        normalize(ctx, raw, out)
        outs.append(out)
    return [DefaultInfo(files = depset(outs))]

kicad_render = rule(
    implementation = _kicad_render_impl,
    doc = """A board's top and bottom, rendered in 3D as PNG images.

```starlark
kicad_render(
    name = "render",
    pcb = ":pcb",
)
```

The outputs are `<name>/top.png` and `<name>/bottom.png`: the board seen
from above and from below, in the colors of its stackup, on KiCad's
opaque background. kicad-cli renders them with its raytracer on the CPU,
so no display and no GPU are needed.

KiCad makes the image a little smaller than `width` by `height`: 1600 by
900 gives 1568 by 872. The quality is KiCad's basic one, the only one
that gives the same bytes on every run. The PNGs hold no time stamp and
no text, so the same board gives the same bytes.

The images show no 3D models of the parts: the KiCad toolchain does not
include KiCad's 3D model libraries.

A pull request that changes a board attaches these images (AGENTS.md
rule 5).
""",
    attrs = {
        "pcb": attr.label(
            doc = "The board (`kicad_pcb`).",
            mandatory = True,
            providers = [KicadPcbInfo],
        ),
        "width": attr.int(
            doc = "The image width asked of KiCad, in pixels; at least 64.",
            default = 1600,
        ),
        "height": attr.int(
            doc = "The image height asked of KiCad, in pixels; at least 64.",
            default = 900,
        ),
    },
    toolchains = [_KICAD, NORMALIZE_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
