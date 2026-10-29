# SPDX-License-Identifier: Apache-2.0
"""kicad_drc_report and kicad_drc_test: KiCad's design rules check."""

load(":design.bzl", "KicadPcbInfo")
load(":normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE", "normalize")
load(":report_test.bzl", "forwarded_attrs", "raw_kicad_report_test")
load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

def _kicad_drc_report_impl(ctx):
    pcb = ctx.attr.pcb[KicadPcbInfo]
    raw = ctx.actions.declare_file(ctx.label.name + ".raw.json")
    args = ["pcb", "drc", "--format", "json", "--severity-error", "--severity-warning"]

    # Parity needs the schematic, which kicad_pcb stages next to the board
    # under the same name. A board that follows a netlist has none; its
    # nets are checked by netlist_parity_test instead.
    if pcb.schematic != None:
        args.append("--schematic-parity")

    # No --exit-code-violations: the report is the output, and the test
    # decides. The design rules, if any, are <board>.kicad_dru in the
    # project directory, where KiCad reads them.
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = args + ["--output", raw.path, pcb.board.path],
        inputs = pcb.files,
        outputs = [raw],
        mnemonic = "KicadDrc",
        progress_message = "Checking the design rules of %{label}",
    )
    out = ctx.actions.declare_file(ctx.label.name + ".json")
    normalize(ctx, raw, out)
    if pcb.design_rules == None:
        return [DefaultInfo(files = depset([out])), OutputGroupInfo(canary = depset())]
    return [
        DefaultInfo(files = depset([out])),
        OutputGroupInfo(canary = depset([_canary(ctx, pcb.design_rules)])),
    ]

def _canary(ctx, rules):
    """DRC with the design rules and a canary rule, on the canary board.

    KiCad drops a rule file it cannot read and runs only its built-in
    checks (kicad#25848). The canary rule, last in a copy of the file, must show
    up in this report; the test fails if it does not (issue #17).
    """
    dir = ctx.label.name + "_canary"
    helpers = ctx.toolchains[HELPERS_TOOLCHAIN_TYPE].helpers
    board = ctx.actions.declare_file(dir + "/canary.kicad_pcb")
    ctx.actions.symlink(output = board, target_file = helpers.canary_board)
    dru = ctx.actions.declare_file(dir + "/canary.kicad_dru")
    run_tool(
        ctx,
        HELPERS_TOOLCHAIN_TYPE,
        "drucanary",
        arguments = [rules.path, dru.path],
        inputs = [rules],
        outputs = [dru],
        mnemonic = "KicadDruCanary",
    )

    # A project, so KiCad reads the empty tables here and nothing else.
    project = ctx.actions.declare_file(dir + "/canary.kicad_pro")
    ctx.actions.write(project, "{}\n")
    sym = ctx.actions.declare_file(dir + "/sym-lib-table")
    ctx.actions.write(sym, "(sym_lib_table\n  (version 7)\n)\n")
    fp = ctx.actions.declare_file(dir + "/fp-lib-table")
    ctx.actions.write(fp, "(fp_lib_table\n  (version 7)\n)\n")
    raw = ctx.actions.declare_file(dir + "/canary.raw.json")
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = ["pcb", "drc", "--format", "json", "--severity-error", "--severity-warning", "--output", raw.path, board.path],
        inputs = [board, dru, project, sym, fp],
        outputs = [raw],
        mnemonic = "KicadDrcCanary",
        progress_message = "Checking that KiCad reads the design rules of %{label}",
    )
    out = ctx.actions.declare_file(dir + "/canary.json")
    normalize(ctx, raw, out)
    return out

kicad_drc_report = rule(
    implementation = _kicad_drc_report_impl,
    doc = """KiCad's DRC report of a board, in JSON.

kicad-cli runs on the board's staged project (`kicad_pcb`), so it finds
footprints only through the project's library tables, and reads the
design rules (`design_rules`) where KiCad expects them, next to the board.
When the board follows a schematic, the report also has the schematic
parity check: footprints and nets that differ from the schematic. It lists
every violation of severity error or warning, and every unconnected item;
which checks have which severity is the design's own setting. The report's
date is removed (spec D2), so the same board gives the same bytes.

KiCad drops a design rules file it cannot read, without a word: its
built-in checks still run, but none of the file's rules do, and DRC still
passes (kicad#25848). So with `design_rules`, the rule also
runs DRC on a canary board, with a copy of the rule file that has a canary
rule appended; that report is the output group `canary`, and
`kicad_drc_test` fails unless it has the canary's violation.
""",
    attrs = {
        "pcb": attr.label(
            doc = "The board (`kicad_pcb`).",
            mandatory = True,
            providers = [KicadPcbInfo],
        ),
    },
    toolchains = [_KICAD, NORMALIZE_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)

def kicad_drc_test(name, pcb, expect = "", **kwargs):
    """Fails if KiCad's design rules check finds a violation.

    The check covers the board's design rules, unconnected items and, when
    the board follows a schematic, schematic parity. It makes
    `<name>_report`, the DRC report (`kicad_drc_report`), and a test that
    prints a summary of it and keeps it among the test's outputs.

    ```starlark
    kicad_drc_test(
        name = "drc_test",
        pcb = ":pcb",
    )
    ```

    Args:
      name: the test's name.
      pcb: the board (`kicad_pcb`).
      expect: empty for a board that must be clean. For a known-bad board
        (spec T3), a violation type, such as `clearance`: the test then
        passes only if the report has violations of that type and of no
        other. `design_rules_not_loaded` is the type of a rule file that
        KiCad did not read.
      **kwargs: common test attributes, such as `size` and `tags`.
    """
    common = forwarded_attrs(kwargs)
    kicad_drc_report(
        name = name + "_report",
        pcb = pcb,
        testonly = True,
        **common
    )

    # The canary report, when the board has design rules (empty otherwise).
    native.filegroup(
        name = name + "_canary",
        srcs = [":" + name + "_report"],
        output_group = "canary",
        testonly = True,
        **common
    )
    raw_kicad_report_test(
        name = name,
        report = ":" + name + "_report",
        canary = ":" + name + "_canary",
        expect = expect,
        **kwargs
    )
