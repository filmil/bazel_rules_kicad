# SPDX-License-Identifier: Apache-2.0
"""Public API of rules_kicad.

Load every rule from this file:

```starlark
load("@rules_kicad//kicad:defs.bzl", "swap_test", "tool_toolchain")
```

The rules arrive in milestones M1 and M2 (see the issues of
https://github.com/filmil/bazel_rules_kicad). This file is the only public
entry point; everything under `//kicad/private` may change without notice.
"""

load("//kicad/private:swap_test.bzl", _swap_test = "swap_test")
load("//kicad/private:toolchains.bzl", _tool_toolchain = "tool_toolchain")

swap_test = _swap_test
tool_toolchain = _tool_toolchain

# The release this file belongs to. It lets a consumer check, in a test,
# that the module it resolved is the one it expects.
RULES_KICAD_API_VERSION = "0"
