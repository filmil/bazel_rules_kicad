# SPDX-License-Identifier: Apache-2.0
"""Analysis tests: kicad_library refuses clashing nicknames and empty footprint lists."""

load("@bazel_skylib//lib:unittest.bzl", "analysistest", "asserts")

def _footprint_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, "two footprint libraries are named \"Resistor_SMD\"")
    return analysistest.end(env)

duplicate_footprint_test = analysistest.make(_footprint_impl, expect_failure = True)

def _symbol_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, "two symbol libraries are named \"Device\"")
    return analysistest.end(env)

duplicate_symbol_test = analysistest.make(_symbol_impl, expect_failure = True)

def _no_footprint_impl(ctx):
    env = analysistest.begin(ctx)
    asserts.expect_failure(env, "footprints has no .kicad_mod file")
    return analysistest.end(env)

no_footprint_test = analysistest.make(_no_footprint_impl, expect_failure = True)
