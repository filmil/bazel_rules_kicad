# SPDX-License-Identifier: Apache-2.0
"""kicad_erc_report and kicad_erc_test: KiCad's electrical rules check."""

load(":design.bzl", "KicadSchematicInfo")
load(":normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE", "normalize")
load(":report_test.bzl", "raw_kicad_report_test")
load(":run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

def _kicad_erc_report_impl(ctx):
    sch = ctx.attr.schematic[KicadSchematicInfo]
    raw = ctx.actions.declare_file(ctx.label.name + ".raw.json")

    # No --exit-code-violations: the report is the output, and the test
    # decides. kicad-cli exits non-zero only when it cannot check.
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = ["sch", "erc", "--format", "json", "--output", raw.path, sch.root.path],
        inputs = sch.files,
        outputs = [raw],
        mnemonic = "KicadErc",
        progress_message = "Checking the electrical rules of %{label}",
    )
    out = ctx.actions.declare_file(ctx.label.name + ".json")
    normalize(ctx, raw, out)
    return [DefaultInfo(files = depset([out]))]

kicad_erc_report = rule(
    implementation = _kicad_erc_report_impl,
    doc = """KiCad's ERC report of a schematic, in JSON.

kicad-cli runs on the schematic's staged project, so it finds symbols and
footprints only through the project's library tables. The report keeps
errors and warnings; its date is removed (spec D2), so the same schematic
gives the same bytes.
""",
    attrs = {
        "schematic": attr.label(
            doc = "The schematic (`kicad_schematic`).",
            mandatory = True,
            providers = [KicadSchematicInfo],
        ),
    },
    toolchains = [_KICAD, NORMALIZE_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)

def kicad_erc_test(name, schematic, expect = "", **kwargs):
    """Fails if KiCad's electrical rules check finds a violation.

    It makes `<name>_report`, the ERC report (`kicad_erc_report`), and a
    test that prints a summary of it and keeps it among the test's outputs
    (`bazel-testlogs/<package>/<name>/test.outputs/`).

    ```starlark
    kicad_erc_test(
        name = "erc_test",
        schematic = ":sch",
    )
    ```

    Args:
      name: the test's name.
      schematic: the schematic (`kicad_schematic`).
      expect: empty for a schematic that must be clean. For a known-bad
        schematic (spec T3), a violation type, such as
        `pin_not_connected`: the test then passes only if the report has a
        violation of that type.
      **kwargs: common test attributes, such as `size` and `tags`.
    """
    kicad_erc_report(
        name = name + "_report",
        schematic = schematic,
        tags = kwargs.get("tags"),
        visibility = kwargs.get("visibility"),
        testonly = True,
    )
    raw_kicad_report_test(
        name = name,
        report = ":" + name + "_report",
        expect = expect,
        **kwargs
    )
