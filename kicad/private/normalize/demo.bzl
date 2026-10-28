# SPDX-License-Identifier: Apache-2.0
"""A rule that normalizes one file, for the normalize toolchain's own tests."""

load("//kicad/private:normalize.bzl", "NORMALIZE_TOOLCHAIN_TYPE", "normalize")
load("//kicad/private:run_tool.bzl", "RUN_TOOL_TOOLCHAINS")

_EPOCH = str(Label("//kicad:source_date_epoch"))

def _epoch_transition_impl(settings, attr):
    if attr.epoch < 0:
        return {_EPOCH: settings[_EPOCH]}
    return {_EPOCH: attr.epoch}

_epoch_transition = transition(
    implementation = _epoch_transition_impl,
    inputs = [_EPOCH],
    outputs = [_EPOCH],
)

def _normalized_impl(ctx):
    if ctx.file.src.is_directory:
        out = ctx.actions.declare_directory(ctx.label.name)
    else:
        out = ctx.actions.declare_file(ctx.label.name + "/" + ctx.file.src.basename)
    normalize(ctx, ctx.file.src, out)
    return [DefaultInfo(files = depset([out]))]

normalized = rule(
    implementation = _normalized_impl,
    doc = "Normalizes src. With epoch >= 0, builds with --source_date_epoch=epoch.",
    attrs = {
        "src": attr.label(allow_single_file = True, mandatory = True),
        "epoch": attr.int(default = -1),
    },
    cfg = _epoch_transition,
    toolchains = [NORMALIZE_TOOLCHAIN_TYPE] + RUN_TOOL_TOOLCHAINS,
)
