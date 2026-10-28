# SPDX-License-Identifier: Apache-2.0
"""Analysis test: off Linux x86_64, analysis fails with a clear message."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _unsupported_fails_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, "rules_kicad v1 supports only Linux x86_64 hosts")
    return analysistest.end(env)

unsupported_fails_test = analysistest.make(
    _unsupported_fails_impl,
    expect_failure = True,
)
