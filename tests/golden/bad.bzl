# SPDX-License-Identifier: Apache-2.0
"""A golden_test whose golden file is built, which must fail analysis.

It is a macro so that update_goldens's scan of BUILD files does not ask
for it to be registered: it is a known-bad target, not a golden file.
"""

load("//kicad:defs.bzl", "golden_test")

def generated_golden_test(name, src, golden):
    """A golden_test with a generated golden file; tagged manual."""
    golden_test(
        name = name,
        src = src,
        golden = golden,
        tags = ["manual"],
    )
