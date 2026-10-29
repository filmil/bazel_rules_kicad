# SPDX-License-Identifier: Apache-2.0
"""kicad_drc_report and kicad_drc_test: KiCad's design rules check."""

load(":design.bzl", "KicadPcbInfo")
load(":normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE", "normalize")
load(":report_test.bzl", "forwarded_attrs", "raw_kicad_report_test")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

def _kicad_drc_report_impl(ctx):
    pcb = ctx.attr.pcb[KicadPcbInfo]
    raw = ctx.actions.declare_file(ctx.label.name + ".raw.json")
    args = ["pcb", "drc", "--format", "json", "--severity-error", "--severity-warning"]

    # Parity needs the schematic, which kicad_pcb stages next to the board
    # under the same name. A board that follows a netlist has none; its
    # nets are checked by netlist_parity_test instead.
    if pcb.schematic:
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
    return [DefaultInfo(files = depset([out]))]

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
        other.
      **kwargs: common test attributes, such as `size` and `tags`.
    """
    kicad_drc_report(
        name = name + "_report",
        pcb = pcb,
        testonly = True,
        **forwarded_attrs(kwargs)
    )
    raw_kicad_report_test(
        name = name,
        report = ":" + name + "_report",
        expect = expect,
        **kwargs
    )
