# SPDX-License-Identifier: Apache-2.0
"""Analysis test: the render actions of each side and the outputs."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

# The outputs a user relies on: one image per side.
_SIDES = ["top", "bottom"]

def _render_action_impl(ctx):
    env = analysistest.begin(ctx)
    actions = analysistest.target_actions(env)
    renders = [a for a in actions if a.mnemonic == "KicadRender"]
    normalizes = [a for a in actions if a.mnemonic == "KicadNormalize"]
    outs = analysistest.target_under_test(env)[DefaultInfo].files.to_list()
    asserts.equals(env, len(_SIDES), len(renders), "KicadRender actions")
    asserts.equals(env, len(_SIDES), len(outs), "outputs")
    name = ctx.attr.target_under_test.label.name
    for side in _SIDES:
        mine = [a for a in renders if a.argv[a.argv.index("--side") + 1] == side]
        asserts.equals(env, 1, len(mine), "KicadRender actions for %s" % side)
        out = [f for f in outs if f.path.endswith("/%s/%s.png" % (name, side))]
        asserts.equals(env, 1, len(out), "outputs for %s" % side)
        if len(mine) != 1 or len(out) != 1:
            continue
        render, out = mine[0], out[0]
        args = render.argv[render.argv.index("--") + 2:]
        raw = render.outputs.to_list()
        asserts.equals(env, 1, len(raw), "one raw output")
        board = args[-1]
        asserts.true(env, board.endswith("/pcb_project/main.kicad_pcb"), "the staged board: %s" % board)
        asserts.equals(env, [
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
            raw[0].path,
            board,
        ], args)
        asserts.true(env, raw[0].path.endswith("/%s.raw/%s.png" % (name, side)), "raw: %s" % raw[0].path)

        # normalize turns the raw image into the output.
        norm = [a for a in normalizes if raw[0] in a.inputs.to_list()]
        asserts.equals(env, 1, len(norm), "KicadNormalize actions that read %s" % raw[0].path)
        if norm:
            asserts.equals(env, [out.path], [f.path for f in norm[0].outputs.to_list()])
        asserts.true(env, out.path.endswith("/%s/%s.png" % (name, side)), "output: %s" % out.path)
    return analysistest.end(env)

render_action_test = analysistest.make(
    _render_action_impl,
    attrs = {
        "width": attr.int(mandatory = True),
        "height": attr.int(mandatory = True),
    },
)
