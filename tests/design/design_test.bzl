# SPDX-License-Identifier: Apache-2.0
"""Analysis tests (spec T2) of KicadSchematicInfo and KicadPcbInfo."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("//kicad:defs.bzl", "KicadPcbInfo", "KicadSchematicInfo")

def _rel(f, info):
    return f.path[len(info.project_dir) + 1:]

def _schematic_impl(ctx):
    env = analysistest.begin(ctx)
    info = analysistest.target_under_test(env)[KicadSchematicInfo]
    asserts.true(env, info.project_dir.endswith("/tests/design/sch_project"), info.project_dir)
    asserts.equals(env, "main.kicad_sch", _rel(info.root, info))
    asserts.equals(env, ["sheets/power.kicad_sch"], [_rel(s, info) for s in info.sheets])
    asserts.equals(env, "main.kicad_pro", _rel(info.project, info))
    names = sorted([_rel(f, info) for f in info.files.to_list() if f.path.startswith(info.project_dir + "/")])
    asserts.equals(env, ["fp-lib-table", "main.kicad_pro", "main.kicad_sch", "sheets/power.kicad_sch", "sym-lib-table"], names)
    asserts.true(env, "My_Parts" in info.library.symbols, "the library's symbols")
    asserts.true(env, any([f.basename == "My_Parts.kicad_sym" for f in info.files.to_list()]), "the library's files")

    # The sheet check runs with every build of the target.
    validation = analysistest.target_under_test(env)[OutputGroupInfo]._validation.to_list()
    asserts.equals(env, ["sch.sheetcheck"], [f.basename for f in validation])
    return analysistest.end(env)

schematic_info_test = analysistest.make(_schematic_impl)

def _pcb_impl(ctx):
    env = analysistest.begin(ctx)
    info = analysistest.target_under_test(env)[KicadPcbInfo]
    asserts.true(env, info.project_dir.endswith("/tests/design/pcb_project"), info.project_dir)

    # Named after the schematic's root, so KiCad finds the schematic.
    asserts.equals(env, "main.kicad_pcb", _rel(info.board, info))
    asserts.equals(env, "main.kicad_dru", _rel(info.design_rules, info))
    asserts.equals(env, None, info.netlist)
    asserts.equals(env, "main.kicad_sch", _rel(info.schematic.root, info))
    asserts.equals(env, ["sheets/power.kicad_sch"], [_rel(s, info) for s in info.schematic.sheets])
    asserts.equals(env, info.project_dir, info.schematic.project_dir)
    asserts.equals(env, info.project, info.schematic.project)
    names = sorted([_rel(f, info) for f in info.files.to_list() if f.path.startswith(info.project_dir + "/")])
    asserts.equals(env, ["fp-lib-table", "main.kicad_dru", "main.kicad_pcb", "main.kicad_pro", "main.kicad_sch", "sheets/power.kicad_sch", "sym-lib-table"], names)

    # The schematic's libraries and the board's own.
    asserts.true(env, "My_Parts" in info.library.footprints, "the schematic's footprints")
    asserts.true(env, "Fixture2" in info.library.symbols, "the board's own library")
    return analysistest.end(env)

pcb_info_test = analysistest.make(_pcb_impl)

def _netlist_impl(ctx):
    env = analysistest.begin(ctx)
    info = analysistest.target_under_test(env)[KicadPcbInfo]
    asserts.equals(env, None, info.schematic)
    asserts.equals(env, "board.kicad_pcb", info.board.basename)
    asserts.equals(env, "board.net", info.netlist.basename)
    asserts.equals(env, None, info.design_rules)
    asserts.equals(env, "board.kicad_pro", info.project.basename)
    return analysistest.end(env)

netlist_pcb_info_test = analysistest.make(_netlist_impl)

def _fails_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, ctx.attr.message)
    return analysistest.end(env)

fails_test = analysistest.make(
    _fails_impl,
    expect_failure = True,
    attrs = {"message": attr.string(mandatory = True)},
)

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
