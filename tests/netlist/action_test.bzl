# SPDX-License-Identifier: Apache-2.0
"""Analysis test: The netlist action and its output."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _netlist_action_impl(ctx):
    env = analysistest.begin(ctx)
    actions = [a for a in analysistest.target_actions(env) if a.mnemonic == "KicadNetlist"]
    asserts.equals(env, 1, len(actions), "KicadNetlist actions")
    argv = actions[0].argv
    i = argv.index("--")
    asserts.equals(env, ["sch", "export", "netlist", "--format", "kicadsexpr", "--output"], argv[i + 2:i + 8])
    asserts.true(env, argv[-1].endswith("/sch_project/main.kicad_sch"), "the staged root sheet: %s" % argv[-1])
    files = [f.basename for f in analysistest.target_under_test(env)[DefaultInfo].files.to_list()]
    asserts.equals(env, [ctx.attr.output], files)
    return analysistest.end(env)

netlist_action_test = analysistest.make(
    _netlist_action_impl,
    attrs = {"output": attr.string(mandatory = True)},
)
