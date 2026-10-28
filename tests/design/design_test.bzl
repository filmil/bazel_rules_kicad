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
    asserts.equals(env, "main.kicad_sch", info.schematic.root.basename)
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
