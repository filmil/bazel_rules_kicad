# SPDX-License-Identifier: Apache-2.0
"""Analysis tests: what golden_test and update_goldens hand their program."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")
load("//kicad:defs.bzl", "GoldenInfo")

def _update_env_impl(ctx):
    env = analysistest.begin(ctx)
    target = analysistest.target_under_test(env)
    run_env = target[RunEnvironmentInfo].environment
    asserts.equals(env, "update", run_env["GOLDEN_MODE"])
    asserts.equals(env, "//tests/golden:hello_test", run_env["GOLDEN_REGISTERED"])
    asserts.equals(env, [
        "//tests/golden:hello_test",
        "_main/tests/golden/hello.txt",
        "_main/tests/golden/expected_hello.txt",
        "tests/golden/expected_hello.txt",
    ], run_env["GOLDEN_PAIRS"].split("\t"))
    return analysistest.end(env)

update_env_test = analysistest.make(_update_env_impl)

def _golden_test_env_impl(ctx):
    env = analysistest.begin(ctx)
    target = analysistest.target_under_test(env)
    run_env = target[RunEnvironmentInfo].environment
    asserts.equals(env, "check", run_env["GOLDEN_MODE"])
    asserts.equals(env, [
        "//tests/golden:hello_test",
        "_main/tests/golden/hello.txt",
        "_main/tests/golden/expected_hello.txt",
        "tests/golden/expected_hello.txt",
    ], run_env["GOLDEN_PAIRS"].split("\t"))
    info = target[GoldenInfo]
    asserts.equals(env, "tests/golden/hello.txt", info.built.short_path)
    asserts.equals(env, "tests/golden/expected_hello.txt", info.golden.short_path)
    asserts.true(env, info.golden.is_source, "the golden file is a source file")

    # Both files are in the test's runfiles.
    runfiles = [f.short_path for f in target[DefaultInfo].default_runfiles.files.to_list()]
    for want in ["tests/golden/hello.txt", "tests/golden/expected_hello.txt"]:
        asserts.true(env, want in runfiles, "%s in the runfiles: %s" % (want, runfiles))
    return analysistest.end(env)

golden_test_env_test = analysistest.make(_golden_test_env_impl)
