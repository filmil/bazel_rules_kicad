# SPDX-License-Identifier: Apache-2.0
"""Analysis test: the STEP export action, its flags and its output."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _step_action_impl(ctx):
    env = analysistest.begin(ctx)
    all_actions = analysistest.target_actions(env)
    actions = [a for a in all_actions if a.mnemonic == "KicadStep"]
    asserts.equals(env, 1, len(actions), "KicadStep actions")
    argv = actions[0].argv
    args = argv[argv.index("--") + 2:]
    flags = ([] if ctx.attr.models_dir else ["--no-components"]) + ctx.attr.flags
    asserts.equals(env, ["pcb", "export", "step"] + flags + ["--output"], args[:-2])

    # With models, KICAD9_3DMODEL_DIR points at the staged libraries, and
    # the action reads every model there.
    launcher_args = argv[:argv.index("--")]
    abs_env = [launcher_args[i + 1] for i in range(len(launcher_args) - 1) if launcher_args[i] == "--env-abs"]
    model_env = [e for e in abs_env if e.startswith("KICAD9_3DMODEL_DIR=")]
    if ctx.attr.models_dir:
        asserts.equals(env, 1, len(model_env), "KICAD9_3DMODEL_DIR: %s" % abs_env)

        # The older names point at the same directory.
        names = sorted([e.split("=", 1)[0] for e in abs_env if e.split("=", 1)[1] == (model_env[0].split("=", 1)[1] if model_env else "")])
        asserts.equals(env, ["KICAD6_3DMODEL_DIR", "KICAD7_3DMODEL_DIR", "KICAD8_3DMODEL_DIR", "KICAD9_3DMODEL_DIR", "KISYS3DMOD"], names)
        if model_env:
            d = model_env[0].split("=", 1)[1]
            asserts.true(env, d.endswith("/" + ctx.attr.models_dir), "the model directory: %s" % d)
            want = d + "/Fake_Lib.3dshapes/part.step"
            asserts.true(env, want in [f.path for f in actions[0].inputs.to_list()], "%s among the inputs" % want)
    else:
        asserts.equals(env, [], model_env, "no KICAD9_3DMODEL_DIR without models")
    raw = args[-2]
    asserts.true(env, args[-1].endswith("/pcb_project/main.kicad_pcb"), "the staged board: %s" % args[-1])
    outs = [f.path for f in actions[0].outputs.to_list()]
    if ctx.attr.models_dir:
        # With models, kicad-cli's log goes to stepcheck, a validation
        # action, which fails when a part has no model.
        log = [o for o in outs if o.endswith(".step.log")]
        asserts.equals(env, [raw] + log, outs)
        asserts.equals(env, 1, len(log), "the log among %s" % outs)
        check = [a for a in all_actions if a.mnemonic == "KicadStepCheck"]
        asserts.equals(env, 1, len(check), "KicadStepCheck actions")
        if check and log:
            asserts.true(env, log[0] in [f.path for f in check[0].inputs.to_list()], "stepcheck reads the log")
        validation = analysistest.target_under_test(env)[OutputGroupInfo]._validation.to_list()
        stamps = [f.path for f in check[0].outputs.to_list()] if check else []
        asserts.true(env, stamps and stamps[0] in [f.path for f in validation], "stepcheck's stamp is validated: %s" % validation)
    else:
        asserts.equals(env, [raw], outs)
        asserts.equals(env, [], [a for a in all_actions if a.mnemonic == "KicadStepCheck"], "no stepcheck without models")

    out = analysistest.target_under_test(env)[DefaultInfo].files.to_list()
    asserts.equals(env, [ctx.attr.output], [f.basename for f in out])
    norm = [a for a in all_actions if a.mnemonic == "KicadNormalize"]
    asserts.equals(env, 1, len(norm), "KicadNormalize actions")
    asserts.true(env, raw in [f.path for f in norm[0].inputs.to_list()], "normalize reads %s" % raw)
    asserts.equals(env, [out[0].path], [f.path for f in norm[0].outputs.to_list()])
    return analysistest.end(env)

step_action_test = analysistest.make(
    _step_action_impl,
    attrs = {
        "flags": attr.string_list(doc = "The flags after --no-components (or after `step`, with models)."),
        "models_dir": attr.string(doc = "The staged model directory's name, when the target has models."),
        "output": attr.string(mandatory = True),
    },
)
