# SPDX-License-Identifier: Apache-2.0
"""timestamp_scan_test: no date or time in the outputs but the epoch (spec D2)."""

load("@bazel_skylib//rules:common_settings.bzl", "BuildSettingInfo")
load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE")

def _rlocation_path(ctx, f):
    """Returns the file's path below TEST_SRCDIR."""
    if f.short_path.startswith("../"):
        return f.short_path[3:]
    return ctx.workspace_name + "/" + f.short_path

def _timestamp_scan_test_impl(ctx):
    helpers = ctx.toolchains[HELPERS_TOOLCHAIN_TYPE].helpers
    if not ctx.files.srcs:
        fail("%s: srcs has no files to scan" % ctx.label)
    epoch = ctx.attr._epoch[BuildSettingInfo].value
    if epoch < 0:
        fail("--@rules_kicad//kicad:source_date_epoch=%d: want 0 or more" % epoch)
    exe = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(
        output = exe,
        target_file = helpers.stampscan.files_to_run.executable,
        is_executable = True,
    )
    files = ctx.files.srcs + ctx.files.sources
    return [
        DefaultInfo(
            executable = exe,
            runfiles = ctx.runfiles(files = files).merge(helpers.stampscan.default_runfiles),
        ),
        RunEnvironmentInfo(environment = {
            # One path per line: a path may hold spaces.
            "STAMP_FILES": "\n".join([_rlocation_path(ctx, f) for f in ctx.files.srcs]),
            "STAMP_SOURCES": "\n".join([_rlocation_path(ctx, f) for f in ctx.files.sources]),
            "STAMP_EPOCH": str(epoch),
            "STAMP_EXPECT": ctx.attr.expect,
        }),
    ]

timestamp_scan_test = rule(
    implementation = _timestamp_scan_test_impl,
    test = True,
    doc = """Checks that outputs hold no date or time but the epoch (spec D2).

```starlark
timestamp_scan_test(
    name = "timestamp_scan_test",
    srcs = [
        ":gerbers",
        ":netlist",
        ":sch_pdf",
    ],
    sources = [":sch"],
)
```

The test scans every file of `srcs`, and every file in a directory
output, for dates and times: `2026-10-29` or `2026/10/29` with an
optional time and zone (`2026-10-29T08:31:59.5+01:00`), PDF dates
(`D:20261029083159`) and ISO 8601 basic (`20261029T083159Z`). A number
next to other digits is not a date, so a PDF's cross-reference offsets
do not count.

Each date must be `--@rules_kicad//kicad:source_date_epoch` (default
1970-01-01T00:00:00 UTC) in every part it has: the date, then the hour,
minute and second, with no zone, `Z` or a zero offset. A time out of
range never passes. A date without a time that is also in one of
`sources` passes as well: KiCad copies the date of a title block, which
the designer wrote, into the netlist. A tool's time stamp has a time, so
`sources` never lets one through.

The test prints every date that fails, with its file and line.
""",
    attrs = {
        "srcs": attr.label_list(
            doc = "The outputs to scan: files, or rules whose outputs are files or directories.",
            allow_files = True,
            mandatory = True,
        ),
        "sources": attr.label_list(
            doc = "Design sources whose dates are allowed, such as the schematic " +
                  "(`kicad_schematic`) whose title blocks have a date.",
            allow_files = True,
        ),
        "expect": attr.string(
            doc = "\"clean\", or \"stamped\" for this rule's own known-bad test: " +
                  "srcs must then have a date that is not the epoch.",
            default = "clean",
            values = ["clean", "stamped"],
        ),
        "_epoch": attr.label(default = "//kicad:source_date_epoch"),
    },
    toolchains = [HELPERS_TOOLCHAIN_TYPE],
)
