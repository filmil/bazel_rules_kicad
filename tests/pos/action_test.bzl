# SPDX-License-Identifier: Apache-2.0
"""Analysis tests: the position file action's options and output."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

_FLAGS = [
    "--smd-only",
    "--exclude-fp-th",
    "--exclude-dnp",
    "--bottom-negate-x",
    "--use-drill-file-origin",
]

def _args(env):
    actions = [a for a in analysistest.target_actions(env) if a.mnemonic == "KicadPos"]
    asserts.equals(env, 1, len(actions), "KicadPos actions")
    argv = actions[0].argv
    return argv[argv.index("--") + 2:]

def _pos_action_impl(ctx):
    env = analysistest.begin(ctx)
    args = _args(env)
    asserts.equals(env, ["pcb", "export", "pos"], args[:3])

    # The options kicad-cli has a default for are given.
    for flag, value in [
        ("--format", "csv"),
        ("--side", "both"),
        ("--units", "mm"),
    ]:
        asserts.true(env, flag in args, "%s is given" % flag)
        if flag in args:
            asserts.equals(env, value, args[args.index(flag) + 1], flag)
    for flag in _FLAGS:
        asserts.false(env, flag in args, "%s only when asked for" % flag)
    asserts.equals(env, "--output", args[-3])
    asserts.true(env, args[-2].endswith("/pos.csv"), "the output: %s" % args[-2])
    asserts.true(env, args[-1].endswith("/pcb_project/main.kicad_pcb"), "the staged board: %s" % args[-1])
    files = [f.basename for f in analysistest.target_under_test(env)[DefaultInfo].files.to_list()]
    asserts.equals(env, ["pos.csv"], files)
    return analysistest.end(env)

pos_action_test = analysistest.make(_pos_action_impl)

def _pos_flags_action_impl(ctx):
    env = analysistest.begin(ctx)
    args = _args(env)
    for flag in _FLAGS:
        asserts.true(env, flag in args, "%s is given" % flag)

    # The name already ends in .csv: no second .csv.
    asserts.equals(env, "--output", args[-3])
    asserts.true(env, args[-2].endswith("/flags.csv"), "the output: %s" % args[-2])
    files = [f.basename for f in analysistest.target_under_test(env)[DefaultInfo].files.to_list()]
    asserts.equals(env, ["flags.csv"], files)
    return analysistest.end(env)

pos_flags_action_test = analysistest.make(_pos_flags_action_impl)
