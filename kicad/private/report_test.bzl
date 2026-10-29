# SPDX-License-Identifier: Apache-2.0
"""The test behind kicad_erc_test and kicad_drc_test."""

load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE")

_FORWARDED = [
    "applicable_licenses",
    "compatible_with",
    "exec_compatible_with",
    "exec_properties",
    "features",
    "tags",
    "target_compatible_with",
    "visibility",
]

def forwarded_attrs(kwargs):
    """Returns the test's attributes that its report target also takes.

    They decide where and whether a target builds.

    Args:
      kwargs: the test macro's keyword arguments.

    Returns:
      A dict of those attributes.
    """
    return {k: kwargs[k] for k in _FORWARDED if k in kwargs}

def _rlocation_path(ctx, f):
    """Returns the file's path below TEST_SRCDIR.

    That is its repository's directory, then the path in the repository.
    """
    if f.short_path.startswith("../"):
        return f.short_path[3:]
    return ctx.workspace_name + "/" + f.short_path

def _report_test_impl(ctx):
    helpers = ctx.toolchains[HELPERS_TOOLCHAIN_TYPE].helpers
    exe = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(
        output = exe,
        target_file = helpers.reportcheck.files_to_run.executable,
        is_executable = True,
    )
    return [
        DefaultInfo(
            executable = exe,
            runfiles = ctx.runfiles(files = [ctx.file.report]).merge(helpers.reportcheck.default_runfiles),
        ),
        RunEnvironmentInfo(environment = {
            "REPORT": _rlocation_path(ctx, ctx.file.report),
            "REPORT_EXPECT": ctx.attr.expect,
        }),
    ]

raw_kicad_report_test = rule(
    implementation = _report_test_impl,
    test = True,
    doc = "Checks a KiCad ERC or DRC report. Use kicad_erc_test or kicad_drc_test.",
    attrs = {
        "report": attr.label(
            doc = "The report, in JSON.",
            allow_single_file = [".json"],
            mandatory = True,
        ),
        "expect": attr.string(
            doc = "Empty: no violations allowed. Else a violation type the report must have.",
        ),
    },
    toolchains = [HELPERS_TOOLCHAIN_TYPE],
)
