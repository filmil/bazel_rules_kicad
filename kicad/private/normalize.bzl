# SPDX-License-Identifier: Apache-2.0
"""The normalize toolchain: removes KiCad's time stamps from its outputs (spec D2)."""

load("@bazel_skylib//rules:common_settings.bzl", "BuildSettingInfo")
load(":run_tool.bzl", "run_tool")

NORMALIZE_TOOLCHAIN_TYPE = Label("//kicad:normalize_toolchain_type")

NormalizeToolchainInfo = provider(
    doc = "What the normalize toolchain offers beyond the `normalize` tool.",
    fields = {
        "epoch": "int: the time every time stamp becomes, in seconds since 1970 UTC.",
    },
)

def _normalize_toolchain_impl(ctx):
    exe = ctx.attr.normalize[DefaultInfo].files_to_run
    epoch = ctx.attr._epoch[BuildSettingInfo].value
    if epoch < 0:
        fail("--@hdlf_rules_kicad//kicad:source_date_epoch=%d: want 0 or more" % epoch)
    return [platform_common.ToolchainInfo(
        tools = {
            "normalize": exe,
            "svgpng": ctx.attr.svgpng[DefaultInfo].files_to_run,
        },
        # run_tool passes this to the tool.
        env = {"SOURCE_DATE_EPOCH": str(epoch)},
        normalize = NormalizeToolchainInfo(epoch = epoch),
    )]

normalize_toolchain = rule(
    implementation = _normalize_toolchain_impl,
    doc = """The normalize toolchain, for `@hdlf_rules_kicad//kicad:normalize_toolchain_type`.

Its tool, `normalize IN OUT`, copies a file or a directory and replaces
every time stamp KiCad writes (Gerber, drill, Gerber job, PDF, STEP, ERC
and DRC reports) by `--@hdlf_rules_kicad//kicad:source_date_epoch`, default 0.

Its second tool, `svgpng --dpi N IN OUT`, draws the SVG sheets that
kicad-cli plots as PNG images that hold no time stamp (`kicad_sch_render`).
It ignores `SOURCE_DATE_EPOCH`. It is here, not in the helpers toolchain,
because `swap_test` swaps this toolchain for the stub along with KiCad's.
""",
    attrs = {
        "normalize": attr.label(mandatory = True, executable = True, cfg = "exec"),
        "svgpng": attr.label(mandatory = True, executable = True, cfg = "exec"),
        "_epoch": attr.label(default = "//kicad:source_date_epoch"),
    },
)

def normalize(ctx, src, out, mnemonic = "KicadNormalize"):
    """Writes `out`: `src` with its time stamps replaced.

    The rule's `toolchains` must include `NORMALIZE_TOOLCHAIN_TYPE` and
    `RUN_TOOL_TOOLCHAINS`. `src` and `out` are both files, or both
    directories (tree artifacts).

    Args:
      ctx: the rule context.
      src: the `File` that a KiCad action wrote.
      out: the declared output `File`.
      mnemonic: the action's mnemonic.
    """
    run_tool(
        ctx,
        NORMALIZE_TOOLCHAIN_TYPE,
        "normalize",
        arguments = [src.path, out.path],
        inputs = [src],
        outputs = [out],
        mnemonic = mnemonic,
        progress_message = "Normalizing %{output}",
    )
