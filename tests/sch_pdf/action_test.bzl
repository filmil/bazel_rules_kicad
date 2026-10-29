# SPDX-License-Identifier: Apache-2.0
"""Analysis test: The schematic PDF action and its output."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _sch_pdf_action_impl(ctx):
    env = analysistest.begin(ctx)
    actions = [a for a in analysistest.target_actions(env) if a.mnemonic == "KicadSchPdf"]
    asserts.equals(env, 1, len(actions), "KicadSchPdf actions")
    argv = actions[0].argv
    i = argv.index("--")
    asserts.equals(env, ["sch", "export", "pdf", "--output"], argv[i + 2:i + 6])
    asserts.true(env, argv[-1].endswith("/sch_project/main.kicad_sch"), "the staged root sheet: %s" % argv[-1])
    inputs = [f.basename for f in actions[0].inputs.to_list()]
    for want in ["main.kicad_sch", "power.kicad_sch", "sym-lib-table", "fp-lib-table", "main.kicad_pro"]:
        asserts.true(env, want in inputs, "%s among the inputs" % want)
    files = [f.basename for f in analysistest.target_under_test(env)[DefaultInfo].files.to_list()]
    asserts.equals(env, [ctx.attr.output], files)
    return analysistest.end(env)

sch_pdf_action_test = analysistest.make(
    _sch_pdf_action_impl,
    attrs = {"output": attr.string(mandatory = True)},
)
