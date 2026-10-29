# SPDX-License-Identifier: Apache-2.0
"""Analysis tests: the drill action and its arguments."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _args(env):
    actions = [a for a in analysistest.target_actions(env) if a.mnemonic == "KicadDrill"]
    asserts.equals(env, 1, len(actions), "KicadDrill actions")
    argv = actions[0].argv
    return argv[argv.index("--") + 2:]

def _value(args, flag):
    return args[args.index(flag) + 1]

def _drill_action_impl(ctx):
    env = analysistest.begin(ctx)
    args = _args(env)
    asserts.equals(env, ["pcb", "export", "drill"], args[:3])

    # kicad-cli's defaults, given anyway.
    asserts.equals(env, "excellon", _value(args, "--format"))
    asserts.equals(env, "absolute", _value(args, "--drill-origin"))
    asserts.equals(env, "mm", _value(args, "--excellon-units"))
    asserts.equals(env, "decimal", _value(args, "--excellon-zeros-format"))
    asserts.equals(env, "alternate", _value(args, "--excellon-oval-format"))

    # No map and one file, unless asked.
    asserts.false(env, "--generate-map" in args, "no map by default")
    asserts.false(env, "--excellon-separate-th" in args, "one file by default")
    asserts.true(env, args[-1].endswith("/pcb_project/main.kicad_pcb"), "the staged board: %s" % args[-1])
    out = analysistest.target_under_test(env)[DefaultInfo].files.to_list()
    asserts.equals(env, 1, len(out), "one output directory")
    asserts.true(env, out[0].is_directory, "the output is a directory")
    return analysistest.end(env)

drill_action_test = analysistest.make(_drill_action_impl)

def _drill_map_action_impl(ctx):
    env = analysistest.begin(ctx)
    args = _args(env)
    asserts.true(env, "--excellon-separate-th" in args, "PTH and NPTH apart")
    asserts.true(env, "--generate-map" in args, "a map")
    asserts.equals(env, "gerberx2", _value(args, "--map-format"))
    return analysistest.end(env)

drill_map_action_test = analysistest.make(_drill_map_action_impl)
