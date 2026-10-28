# SPDX-License-Identifier: Apache-2.0
"""A rule that runs `kicad-cli version`, for the toolchain's smoke test."""

load("//kicad/private:run_tool.bzl", "RUN_TOOL_TOOLCHAINS", "run_tool")

_KICAD = Label("//kicad:toolchain_type")

def _kicad_version_impl(ctx):
    out = ctx.actions.declare_file(ctx.label.name + ".txt")
    run_tool(
        ctx,
        _KICAD,
        "kicad_cli",
        arguments = ["version"],
        stdout = out,
        mnemonic = "KicadVersion",
    )
    return [DefaultInfo(files = depset([out]))]

kicad_version = rule(
    implementation = _kicad_version_impl,
    doc = "Writes what `kicad-cli version` prints.",
    toolchains = [_KICAD] + RUN_TOOL_TOOLCHAINS,
)
