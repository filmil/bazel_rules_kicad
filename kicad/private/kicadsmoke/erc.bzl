# SPDX-License-Identifier: Apache-2.0
"""A rule that runs kicad-cli's ERC on one schematic, for the smoke tests."""

load("//kicad/private:run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

def _kicad_erc_report_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".json")
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = ["sch", "erc", "--format", "json", "--output", out.path, ctx.file.schematic.path],
        inputs = [ctx.file.schematic],
        outputs = [out],
        mnemonic = "KicadErc",
    )
    return [DefaultInfo(files = depset([out]))]

kicad_erc_report = rule(
    implementation = _kicad_erc_report_impl,
    doc = "Writes kicad-cli's ERC report of a schematic, in JSON.",
    attrs = {"schematic": attr.label(allow_single_file = [".kicad_sch"], mandatory = True)},
    toolchains = [_KICAD] + RUN_TOOL_TOOLCHAINS,
)
