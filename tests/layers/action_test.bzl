# SPDX-License-Identifier: Apache-2.0
"""Analysis test: kicad_layers' actions, their options and its outputs."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _layers_action_impl(ctx):
    env = analysistest.begin(ctx)
    actions = {a.mnemonic: a for a in analysistest.target_actions(env)}
    name = ctx.attr.target_under_test.label.name
    outs = analysistest.target_under_test(env)[DefaultInfo].files.to_list()
    want = [name + ".svg"] + ([name + ".png"] if ctx.attr.png else [])
    asserts.equals(env, want, [f.basename for f in outs], "outputs")
    groups = analysistest.target_under_test(env)[OutputGroupInfo]
    for ext in ["svg", "png"]:
        got = [f.basename for f in getattr(groups, ext).to_list()] if hasattr(groups, ext) else []
        asserts.equals(env, [n for n in want if n.endswith("." + ext)], got, "output group " + ext)
    asserts.equals(env, ctx.attr.png, "KicadLayersPng" in actions, "a KicadLayersPng action")
    svg = actions.get("KicadLayersSvg")
    norm = actions.get("KicadLayersNormalize")
    asserts.true(env, svg != None and norm != None, "KicadLayersSvg and KicadLayersNormalize: %s" % actions.keys())
    if not (svg and norm):
        return analysistest.end(env)
    args = svg.argv[svg.argv.index("--") + 2:]
    raw = svg.outputs.to_list()[0].path
    asserts.equals(env, [
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
        ctx.attr.layers,
        "--output",
        raw,
    ] + ctx.attr.more, args[:-1])
    asserts.true(env, args[-1].endswith("/pcb_project/main.kicad_pcb"), "the staged board: %s" % args[-1])

    # The normalized SVG is the output; the PNG is drawn from it.
    nargs = norm.argv[norm.argv.index("--") + 2:]
    asserts.equals(env, [raw, outs[0].path], nargs)
    png = actions.get("KicadLayersPng")
    if png:
        pargs = png.argv[png.argv.index("--") + 2:]
        asserts.equals(env, ["--dpi", str(ctx.attr.dpi), outs[0].path, outs[1].path], pargs)
    return analysistest.end(env)

layers_action_test = analysistest.make(
    _layers_action_impl,
    attrs = {
        "layers": attr.string(mandatory = True, doc = "The --layers value, joined."),
        "more": attr.string_list(doc = "The options after --layers: --mirror, then `options`."),
        "png": attr.bool(),
        "dpi": attr.int(),
    },
)
