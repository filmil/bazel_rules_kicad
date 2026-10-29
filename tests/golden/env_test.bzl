# SPDX-License-Identifier: Apache-2.0
"""Analysis test: what update_goldens hands its program."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

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
