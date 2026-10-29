# SPDX-License-Identifier: Apache-2.0
"""Analysis test: The DRC report action's flags and inputs."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _drc_action_impl(ctx):
    env = analysistest.begin(ctx)
    actions = [a for a in analysistest.target_actions(env) if a.mnemonic == "KicadDrc"]
    asserts.equals(env, 1, len(actions), "KicadDrc actions")
    argv = actions[0].argv
    asserts.equals(env, ctx.attr.parity, "--schematic-parity" in argv, "--schematic-parity in %s" % argv)
    inputs = [f.basename for f in actions[0].inputs.to_list()]
    asserts.equals(env, ctx.attr.rules, "main.kicad_dru" in inputs, "the staged rule file among the inputs")
    return analysistest.end(env)

drc_action_test = analysistest.make(
    _drc_action_impl,
    attrs = {
        "parity": attr.bool(mandatory = True),
        "rules": attr.bool(mandatory = True),
    },
)
