# SPDX-License-Identifier: Apache-2.0
"""Analysis test: two libraries with one nickname fail with a clear message."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _fails_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, "two footprint libraries are named \"Resistor_SMD\"")
    return analysistest.end(env)

duplicate_nickname_test = analysistest.make(_fails_impl, expect_failure = True)
