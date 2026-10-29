# SPDX-License-Identifier: Apache-2.0
"""Analysis test: The BOM action's options and output."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _bom_action_impl(ctx):
    env = analysistest.begin(ctx)
    actions = [a for a in analysistest.target_actions(env) if a.mnemonic == "KicadBom"]
    asserts.equals(env, 1, len(actions), "KicadBom actions")
    argv = actions[0].argv
    args = argv[argv.index("--") + 2:]
    asserts.equals(env, ["sch", "export", "bom"], args[:3])

    # Every option kicad-cli has a default for is given.
    for flag, value in [
        ("--fields", "Reference,Value,Footprint,${QUANTITY},${DNP}"),
        ("--labels", "Refs,Value,Footprint,Qty,DNP"),
        ("--group-by", "Value,Footprint,${DNP}"),
        ("--sort-field", "Reference"),
        ("--field-delimiter", ","),
        ("--string-delimiter", "\""),
        ("--ref-delimiter", ","),
        ("--ref-range-delimiter", ""),
    ]:
        asserts.true(env, flag in args, "%s is given" % flag)
        if flag in args:
            asserts.equals(env, value, args[args.index(flag) + 1], flag)

    # --sort-asc is a flag with no value: kicad-cli crashes on "true".
    asserts.true(env, "--sort-asc" in args, "--sort-asc is given")
    asserts.false(env, "true" in args or "false" in args, "no value for --sort-asc")
    asserts.false(env, "--exclude-dnp" in args, "--exclude-dnp only when asked for")
    asserts.equals(env, "--output", args[-3])
    asserts.true(env, args[-2].endswith("/bom.csv"), "the output: %s" % args[-2])
    asserts.true(env, args[-1].endswith("/sch_project/main.kicad_sch"), "the staged root: %s" % args[-1])
    files = [f.basename for f in analysistest.target_under_test(env)[DefaultInfo].files.to_list()]
    asserts.equals(env, ["bom.csv"], files)
    return analysistest.end(env)

bom_action_test = analysistest.make(_bom_action_impl)
