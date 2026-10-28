# SPDX-License-Identifier: Apache-2.0
"""Public API of rules_kicad.

Load every rule from this file. For now it holds only the API version:

```starlark
load("@rules_kicad//kicad:defs.bzl", "RULES_KICAD_API_VERSION")
```

The rules arrive in milestones M1 and M2 (see the issues of
https://github.com/filmil/bazel_rules_kicad). This file is the only public
entry point; everything under `//kicad/private` may change without notice.
"""

# The release this file belongs to. It lets a consumer check, in a test,
# that the module it resolved is the one it expects.
RULES_KICAD_API_VERSION = "0"
