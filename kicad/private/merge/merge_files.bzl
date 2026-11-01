# SPDX-License-Identifier: Apache-2.0
"""merge_files: runs kicad_merge in an action, for its tests.

The merged schematic then goes through kicad_schematic and kicad_erc_test
like any other: the acceptance test of issue #58 is that KiCad loads the
merge, with ERC as clean as before.
"""

load("//kicad/private:run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE", "RUN_TOOL_TOOLCHAINS", "run_tool")

def _merge_files_impl(ctx):
    out = ctx.actions.declare_file(ctx.attr.out)

    # An action's inputs are read-only, so --out takes the result in place
    # of ours.
    run_tool(
        ctx,
        HELPERS_TOOLCHAIN_TYPE,
        "kicad_merge",
        arguments = ["--out", out.path, ctx.file.base.path, ctx.file.ours.path, ctx.file.theirs.path],
        inputs = [ctx.file.base, ctx.file.ours, ctx.file.theirs],
        outputs = [out],
        mnemonic = "KicadMerge",
        progress_message = "Merging %{label}",
    )
    return [DefaultInfo(files = depset([out]))]

merge_files = rule(
    implementation = _merge_files_impl,
    doc = "The 3-way merge of base, ours and theirs, by kicad_merge, as `out`.",
    attrs = {
        "base": attr.label(mandatory = True, allow_single_file = True),
        "ours": attr.label(mandatory = True, allow_single_file = True),
        "theirs": attr.label(mandatory = True, allow_single_file = True),
        "out": attr.string(mandatory = True),
    },
    toolchains = RUN_TOOL_TOOLCHAINS,
)
