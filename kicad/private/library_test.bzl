# SPDX-License-Identifier: Apache-2.0
"""Unit tests of relative_to: the ../ steps reach the execution root."""

load("@bazel_skylib//lib:unittest.bzl", "asserts", "unittest")
load(":design.bzl", "relative_path")
load(":library.bzl", "relative_to")

def _relative_to_test_impl(ctx):
    env = unittest.begin(ctx)
    for project_dir, path, want in [
        ("bazel-out/k8-fastbuild/bin/pkg/t", "external/libs/S.kicad_sym", "${KIPRJMOD}/../../../../../external/libs/S.kicad_sym"),
        ("bazel-out/k8-fastbuild/bin/t", "pkg/My.pretty", "${KIPRJMOD}/../../../../pkg/My.pretty"),
        # Repeated and trailing slashes, and "." parts, are not steps.
        ("a//b/./c/", "x.kicad_sym", "${KIPRJMOD}/../../../x.kicad_sym"),
        # The project at the execution root itself.
        ("", "x.kicad_sym", "${KIPRJMOD}/x.kicad_sym"),
        (".", "x.kicad_sym", "${KIPRJMOD}/x.kicad_sym"),
        # A sibling-layout external path: up to the root, then out of it.
        ("bazel-out/k8-fastbuild/bin", "../libs/S.kicad_sym", "${KIPRJMOD}/../../../../libs/S.kicad_sym"),
    ]:
        asserts.equals(env, want, relative_to(path, project_dir), "%s from %s" % (path, project_dir))
    return unittest.end(env)

relative_to_test = unittest.make(_relative_to_test_impl)

def _relative_path_test_impl(ctx):
    env = unittest.begin(ctx)
    for short_path, base, want in [
        ("pkg/main.kicad_sch", "pkg", "main.kicad_sch"),
        ("pkg/sheets/a.kicad_sch", "pkg", "sheets/a.kicad_sch"),
        ("pkgx/a.kicad_sch", "pkg", None),
        ("other/a.kicad_sch", "pkg", None),
        # The root package.
        ("a.kicad_sch", "", "a.kicad_sch"),
        ("sub/a.kicad_sch", "", "sub/a.kicad_sch"),
        ("../repo/a.kicad_sch", "", None),
        # An external repository.
        ("../repo/pkg/a.kicad_sch", "../repo/pkg", "a.kicad_sch"),
    ]:
        asserts.equals(env, want, relative_path(short_path, base), "%s below %r" % (short_path, base))
    return unittest.end(env)

relative_path_test = unittest.make(_relative_path_test_impl)
