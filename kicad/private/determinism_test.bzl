# SPDX-License-Identifier: Apache-2.0
"""Unit tests of staging_path: the two copies never share a path."""

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load(":determinism.bzl", "staging_path")

_CASES = [
    ("pkg/out.gbr", "main/pkg/out.gbr"),
    # The root package of the main repository.
    ("out.gbr", "main/out.gbr"),
    # A main-repository path that starts with external/ ...
    ("external/repo+/out.gbr", "main/external/repo+/out.gbr"),
    # ... stays apart from an external repository's output.
    ("../repo+/out.gbr", "external/repo+/out.gbr"),
    ("../repo+/pkg/out.gbr", "external/repo+/pkg/out.gbr"),
]

def _staging_path_test_impl(ctx):
    env = unittest.begin(ctx)
    for tag in ["a", "b"]:
        seen = {}
        for short_path, want in _CASES:
            got = staging_path("t", tag, short_path)
            asserts.equals(env, "t.%s/%s" % (tag, want), got, "%s for %s" % (short_path, tag))
            asserts.false(env, ".." in got.split("/"), "%s climbs out: %s" % (short_path, got))
            asserts.true(env, got.startswith("t.%s/" % tag), "%s leaves t.%s/: %s" % (short_path, tag, got))
            asserts.false(env, got in seen, "%s and %s share %s" % (short_path, seen.get(got), got))
            seen[got] = short_path
    for short_path, _ in _CASES:
        asserts.true(
            env,
            staging_path("t", "a", short_path) != staging_path("t", "b", short_path),
            "a and b share a path for %s" % short_path,
        )
    return unittest.end(env)

staging_path_test = unittest.make(_staging_path_test_impl)
