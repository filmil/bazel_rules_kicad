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

        # With models, KICAD9_3DMODEL_DIR and its older names point at the
        # staged libraries, and the render reads every model there.
        launcher_args = render.argv[:render.argv.index("--")]
        abs_env = [launcher_args[i + 1] for i in range(len(launcher_args) - 1) if launcher_args[i] == "--env-abs"]
        model_env = [e for e in abs_env if e.startswith("KICAD9_3DMODEL_DIR=")]
        if ctx.attr.models_dir:
            asserts.equals(env, 1, len(model_env), "KICAD9_3DMODEL_DIR: %s" % abs_env)
            if model_env:
                d = model_env[0].split("=", 1)[1]
                names = sorted([e.split("=", 1)[0] for e in abs_env if e.split("=", 1)[1] == d])
                asserts.equals(env, ["KICAD6_3DMODEL_DIR", "KICAD7_3DMODEL_DIR", "KICAD8_3DMODEL_DIR", "KICAD9_3DMODEL_DIR", "KISYS3DMOD"], names)
                asserts.true(env, d.endswith("/" + ctx.attr.models_dir), "the model directory: %s" % d)
                want = d + "/Fake_Lib.3dshapes/part.step"
                asserts.true(env, want in [f.path for f in render.inputs.to_list()], "%s among the inputs" % want)
        else:
            asserts.equals(env, [], model_env, "no KICAD9_3DMODEL_DIR without models")

        # normalize turns the raw image into the output.
        norm = [a for a in normalizes if raw[0] in a.inputs.to_list()]
        asserts.equals(env, 1, len(norm), "KicadNormalize actions that read %s" % raw[0].path)
        if norm:
            asserts.equals(env, [out.path], [f.path for f in norm[0].outputs.to_list()])
        asserts.true(env, out.path.endswith("/%s/%s.png" % (name, side)), "output: %s" % out.path)

    # With models, modelcheck reads the board and the staged libraries in a
    # validation action, as kicad-cli says nothing of a missing model.
    checks = [a for a in actions if a.mnemonic == "KicadRenderModelCheck"]
    if ctx.attr.models_dir:
        asserts.equals(env, 1, len(checks), "KicadRenderModelCheck actions")
        if checks:
            argv = checks[0].argv[checks[0].argv.index("--") + 2:]
            d = argv[argv.index("--models_dir") + 1]
            asserts.true(env, d.endswith("/" + ctx.attr.models_dir), "modelcheck's directory: %s" % d)
            ins = [f.path for f in checks[0].inputs.to_list()]
            asserts.true(env, argv[-1] in ins and argv[-1].endswith(".kicad_pcb"), "modelcheck reads the board: %s" % argv[-1])
            asserts.true(env, d + "/Fake_Lib.3dshapes/part.step" in ins, "modelcheck reads the staged models")
            stamps = [f.path for f in checks[0].outputs.to_list()]
            validation = [f.path for f in analysistest.target_under_test(env)[OutputGroupInfo]._validation.to_list()]
            asserts.true(env, stamps and stamps[0] in validation, "modelcheck's stamp is validated: %s" % validation)
    else:
        asserts.equals(env, [], checks, "no modelcheck without models")
    return analysistest.end(env)

render_action_test = analysistest.make(
    _render_action_impl,
    attrs = {
        "models_dir": attr.string(doc = "The staged model directory's name, when the target has models."),
        "width": attr.int(mandatory = True),
        "height": attr.int(mandatory = True),
    },
)
