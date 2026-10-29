# SPDX-License-Identifier: Apache-2.0
"""Analysis test: the schematic render's two actions and its output."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _sch_render_action_impl(ctx):
    env = analysistest.begin(ctx)
    actions = {a.mnemonic: a for a in analysistest.target_actions(env)}
    svg = actions.get("KicadSchSvg")
    png = actions.get("KicadSchPng")
    asserts.true(env, svg != None and png != None, "a KicadSchSvg and a KicadSchPng action: %s" % actions.keys())
    if svg and png:
        args = svg.argv[svg.argv.index("--") + 2:]
        asserts.equals(env, ["sch", "export", "svg", "--output"], args[:4])
        asserts.true(env, args[-1].endswith("/sch_project/main.kicad_sch"), "the staged root: %s" % args[-1])
        pargs = png.argv[png.argv.index("--") + 2:]
        out_dir = analysistest.target_under_test(env)[DefaultInfo].files.to_list()[0]
        asserts.equals(env, ["--dpi", str(ctx.attr.dpi), args[4], out_dir.path], pargs)
        asserts.equals(env, [out_dir.path], [f.path for f in png.outputs.to_list()])

        # The PNG action reads the SVG directory and nothing of KiCad.
        asserts.equals(env, [args[4]], [f.path for f in png.inputs.to_list() if f.is_directory])
    out = analysistest.target_under_test(env)[DefaultInfo].files.to_list()
    asserts.equals(env, 1, len(out), "one output")
    asserts.true(env, out[0].is_directory, "a directory of PNG files")
    return analysistest.end(env)

sch_render_action_test = analysistest.make(
    _sch_render_action_impl,
    attrs = {"dpi": attr.int(mandatory = True)},
)
