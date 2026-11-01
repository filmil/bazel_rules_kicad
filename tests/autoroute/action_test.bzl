# SPDX-License-Identifier: Apache-2.0
"""Analysis tests: kicad_autoroute's three actions and their options."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _actions(env):
    by = {}
    for a in analysistest.target_actions(env):
        by.setdefault(a.mnemonic, []).append(a)
    for m in ["KicadDsn", "Freerouting", "KicadSes"]:
        asserts.equals(env, 1, len(by.get(m, [])), "%s actions" % m)
    return by

def _tool_args(action):
    argv = action.argv
    return argv[argv.index("--") + 2:]

def _autoroute_action_impl(ctx):
    env = analysistest.begin(ctx)
    by = _actions(env)
    dsn = _tool_args(by["KicadDsn"][0])
    asserts.equals(env, "-I", dsn[0], "isolated Python")
    asserts.true(env, dsn[1].endswith("/autoroute/dsn.py"), "the export script: %s" % dsn[1])
    asserts.true(env, "--rules" in dsn, "the design rules go to the export")
    if "--rules" in dsn:
        asserts.true(env, dsn[dsn.index("--rules") + 1].endswith("placed_project/board.kicad_dru"), "the staged rules")

    fr = by["Freerouting"][0]
    args = _tool_args(fr)
    for flag in [
        "-Djava.awt.headless=true",
        "--gui.enabled=false",
        "--profile.allow_telemetry=false",
        "--usage_and_diagnostic_data.disable_analytics=true",
        "--logging.file.enabled=false",
    ]:
        asserts.true(env, flag in args, "Freerouting gets %s" % flag)
    asserts.true(env, "-mt" in args and args[args.index("-mt") + 1] == "1", "one thread")
    asserts.false(env, "-mp" in args, "-mp only when asked for")
    asserts.true(env, args[args.index("-jar") + 1].endswith("freerouting.jar"), "the jar")

    ses = _tool_args(by["KicadSes"][0])
    asserts.equals(env, "-I", ses[0], "isolated Python")
    asserts.true(env, ses[1].endswith("/autoroute/ses.py"), "the import script: %s" % ses[1])
    asserts.false(env, "--allow_unrouted" in ses, "unrouted fails by default")
    files = [f.basename for f in analysistest.target_under_test(env)[DefaultInfo].files.to_list()]
    asserts.equals(env, ["routed.kicad_pcb"], files)
    return analysistest.end(env)

autoroute_action_test = analysistest.make(_autoroute_action_impl)

def _autoroute_options_action_impl(ctx):
    env = analysistest.begin(ctx)
    by = _actions(env)
    args = _tool_args(by["Freerouting"][0])
    asserts.true(env, "-mp" in args and args[args.index("-mp") + 1] == "7", "-mp 7")
    asserts.true(env, "--allow_unrouted" in _tool_args(by["KicadSes"][0]), "--allow_unrouted")
    return analysistest.end(env)

autoroute_options_action_test = analysistest.make(_autoroute_options_action_impl)

def _autoroute_no_rules_action_impl(ctx):
    env = analysistest.begin(ctx)
    by = _actions(env)
    asserts.false(env, "--rules" in _tool_args(by["KicadDsn"][0]), "no --rules without design rules")
    return analysistest.end(env)

autoroute_no_rules_action_test = analysistest.make(_autoroute_no_rules_action_impl)
