# SPDX-License-Identifier: Apache-2.0
"""Unit tests of staging_path: the two copies never share a path."""

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load(":determinism.bzl", "staging_path")

def _staging_path_test_impl(ctx):
    env = unittest.begin(ctx)
    for tag, short_path, want in [
        ("a", "pkg/out.gbr", "t.a/pkg/out.gbr"),
        ("b", "pkg/out.gbr", "t.b/pkg/out.gbr"),
        # An output of an external repository stays below t.a/ and t.b/.
        ("a", "../repo+/pkg/out.gbr", "t.a/external/repo+/pkg/out.gbr"),
        ("b", "../repo+/pkg/out.gbr", "t.b/external/repo+/pkg/out.gbr"),
        ("a", "../repo+/out.gbr", "t.a/external/repo+/out.gbr"),
    ]:
        got = staging_path("t", tag, short_path)
        asserts.equals(env, want, got, "%s for %s" % (short_path, tag))
        asserts.false(env, "/../" in "/" + got, "%s climbs out: %s" % (short_path, got))
    return unittest.end(env)

staging_path_test = unittest.make(_staging_path_test_impl)
