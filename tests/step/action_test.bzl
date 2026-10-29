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
    asserts.equals(env, ["pcb", "export", "step", "--no-components", "--output"], args[:-2])
    raw = args[-2]
    asserts.true(env, args[-1].endswith("/pcb_project/main.kicad_pcb"), "the staged board: %s" % args[-1])
    asserts.equals(env, [raw], [f.path for f in actions[0].outputs.to_list()])

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
        "output": attr.string(mandatory = True),
    },
)
