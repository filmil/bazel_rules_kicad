# SPDX-License-Identifier: Apache-2.0
"""Module extensions of hdlf_rules_kicad.

```starlark
kicad = use_extension("@hdlf_rules_kicad//kicad:extensions.bzl", "kicad")
kicad.toolchain(version = "9.0.9")
```
"""

load("//kicad/private:extensions.bzl", _kicad = "kicad")

kicad = _kicad
