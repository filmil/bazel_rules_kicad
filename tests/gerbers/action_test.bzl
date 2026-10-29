# SPDX-License-Identifier: Apache-2.0
"""Analysis test: the Gerber plot action and its output."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("//kicad:defs.bzl", "DEFAULT_GERBER_LAYERS")

def _gerbers_action_impl(ctx):
    env = analysistest.begin(ctx)
    actions = [a for a in analysistest.target_actions(env) if a.mnemonic == "KicadGerbers"]
    asserts.equals(env, 1, len(actions), "KicadGerbers actions")
    argv = actions[0].argv
    args = argv[argv.index("--") + 2:]
    asserts.equals(env, ["pcb", "export", "gerbers"], args[:3])
    layers = args[args.index("--layers") + 1].split(",")
    asserts.equals(env, DEFAULT_GERBER_LAYERS, layers)

    # Every copper layer KiCad has, then the fab layers.
    asserts.equals(env, ["F.Cu", "In1.Cu"], layers[:2])
    asserts.true(env, "In30.Cu" in layers and "B.Cu" in layers and "Edge.Cuts" in layers, "copper and outline")
    asserts.equals(env, "6", args[args.index("--precision") + 1])
    asserts.true(env, args[-1].endswith("/pcb_project/main.kicad_pcb"), "the staged board: %s" % args[-1])
    out = analysistest.target_under_test(env)[DefaultInfo].files.to_list()
    asserts.equals(env, 1, len(out), "one output")
    asserts.true(env, out[0].is_directory, "a directory of Gerber files")
    return analysistest.end(env)

gerbers_action_test = analysistest.make(_gerbers_action_impl)
