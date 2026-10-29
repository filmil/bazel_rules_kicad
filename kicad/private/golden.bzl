# SPDX-License-Identifier: Apache-2.0
"""golden_test and update_goldens: checked-in golden files (spec rule 4)."""

load(":run_tool.bzl", "HELPERS_TOOLCHAIN_TYPE")

GoldenInfo = provider(
    doc = "A golden file and the built file that must equal it.",
    fields = {
        "built": "File: the built file.",
        "golden": "File: the golden file, a source file of the main repository.",
    },
)

def _rlocation_path(ctx, f):
    """Returns the file's path below the runfiles root."""
    if f.short_path.startswith("../"):
        return f.short_path[3:]
    return ctx.workspace_name + "/" + f.short_path

def _pair_line(ctx, label, info):
    """One line of GOLDEN_PAIRS: test, built file, golden file, its source path."""
    return "\t".join([
        "//%s:%s" % (label.package, label.name),
        _rlocation_path(ctx, info.built),
        _rlocation_path(ctx, info.golden),
        info.golden.short_path,
    ])

def _program(ctx, helpers):
    exe = ctx.actions.declare_file(ctx.label.name)
    ctx.actions.symlink(
        output = exe,
        target_file = helpers.golden.files_to_run.executable,
        is_executable = True,
    )
    return exe

def _golden_test_impl(ctx):
    for attr_name in ("src", "golden"):
        if getattr(ctx.file, attr_name) == None:
            fail("%s: %s gives no file; it must give exactly one" % (ctx.label, attr_name))
    golden = ctx.file.golden
    if not golden.is_source or golden.owner.workspace_name:
        fail("%s: golden must be a checked-in file of this repository; %s is not" % (ctx.label, golden.owner))
    helpers = ctx.toolchains[HELPERS_TOOLCHAIN_TYPE].helpers
    info = GoldenInfo(built = ctx.file.src, golden = golden)
    return [
        info,
        DefaultInfo(
            executable = _program(ctx, helpers),
            runfiles = ctx.runfiles(files = [ctx.file.src, golden]).merge(helpers.golden.default_runfiles),
        ),
        RunEnvironmentInfo(environment = {
            "GOLDEN_MODE": "check",
            "GOLDEN_PAIRS": _pair_line(ctx, ctx.label, info),
        }),
    ]

raw_golden_test = rule(
    implementation = _golden_test_impl,
    test = True,
    doc = "The rule behind golden_test. Use the macro.",
    attrs = {
        "src": attr.label(
            doc = "The built file.",
            allow_single_file = True,
            mandatory = True,
        ),
        "golden": attr.label(
            doc = "The golden file: a checked-in file of this repository.",
            allow_single_file = True,
            mandatory = True,
        ),
    },
    toolchains = [HELPERS_TOOLCHAIN_TYPE],
)

def golden_test(name, src, golden, visibility = None, **kwargs):
    """Checks that a built file equals a checked-in golden file, byte for byte.

    ```starlark
    golden_test(
        name = "bom_test",
        size = "small",
        src = ":bom",
        golden = "expected_bom.csv",
    )
    ```

    When the files differ, the test prints the first line that differs and
    fails. If the change is intended, `bazel run //:update_goldens` (an
    `update_goldens` target that lists this test) writes the built file
    over the golden file.

    The test is visible to the module's root package, where
    `//:update_goldens` lists it, unless `visibility` says otherwise.

    Args:
      name: the test's name.
      src: the built file.
      golden: the golden file, a checked-in file of this repository.
      visibility: the test's visibility; default `["//:__pkg__"]`.
      **kwargs: common test attributes, such as `size` and `tags`.
    """
    raw_golden_test(
        name = name,
        src = src,
        golden = golden,
        visibility = visibility if visibility != None else ["//:__pkg__"],
        **kwargs
    )

def _update_goldens_impl(ctx):
    helpers = ctx.toolchains[HELPERS_TOOLCHAIN_TYPE].helpers
    pairs = []
    files = []
    for t in ctx.attr.tests:
        info = t[GoldenInfo]
        pairs.append(_pair_line(ctx, t.label, info))
        files.append(info.built)
    return [
        DefaultInfo(
            executable = _program(ctx, helpers),
            runfiles = ctx.runfiles(files = files).merge(helpers.golden.default_runfiles),
        ),
        RunEnvironmentInfo(environment = {
            "GOLDEN_MODE": "update",
            "GOLDEN_PAIRS": "\n".join(pairs),
            "GOLDEN_REGISTERED": "\n".join(["//%s:%s" % (t.label.package, t.label.name) for t in ctx.attr.tests]),
        }),
    ]

raw_update_goldens = rule(
    implementation = _update_goldens_impl,
    executable = True,
    doc = "The rule behind update_goldens. Use the macro.",
    attrs = {
        "tests": attr.label_list(
            doc = "Every `golden_test` of the module.",
            providers = [GoldenInfo],
            mandatory = True,
            allow_empty = False,
        ),
    },
    toolchains = [HELPERS_TOOLCHAIN_TYPE],
)

def update_goldens(name, tests, **kwargs):
    """Writes the built files of golden tests over their golden files.

    ```starlark
    update_goldens(
        name = "update_goldens",
        tests = [
            "//e2e/bom:bom_test",
            "//e2e/pos:pos_test",
        ],
    )
    ```

    Run it with `bazel run //:update_goldens` in the module's directory.
    It builds the files the tests compare, then copies each one that
    differs over its golden file and names it. Commit the result with the
    reason for the change.

    It fails, before writing anything, if a `golden_test` in the module's
    BUILD files is missing from `tests`: an update then covers every golden
    file. The scan reads every `golden_test(...)` call written in a BUILD
    file of the module, whatever its layout. A `golden_test` made inside a
    macro of yours is not seen by it, so list that one by hand. The scan
    skips the directories that `.bazelignore` names and those of a nested
    module or repository (with its own `MODULE.bazel`, `REPO.bazel` or
    `WORKSPACE`), which has its own `update_goldens`.

    The target is `testonly`, since it depends on tests; `bazel run` runs
    it all the same.

    Args:
      name: the target's name.
      tests: every `golden_test` of the module.
      **kwargs: common attributes, such as `tags`.
    """
    raw_update_goldens(
        name = name,
        tests = tests,
        testonly = True,
        **kwargs
    )
