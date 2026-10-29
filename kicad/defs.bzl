# SPDX-License-Identifier: Apache-2.0
"""Public API of rules_kicad.

Load every rule from this file:

```starlark
load("@rules_kicad//kicad:defs.bzl", "determinism_test", "swap_test", "tool_toolchain")
```

The rules arrive in milestones M1 and M2 (see the issues of
https://github.com/filmil/bazel_rules_kicad). This file is the only public
entry point; everything under `//kicad/private` may change without notice.
"""

load("//kicad/private:design.bzl", _KicadPcbInfo = "KicadPcbInfo", _KicadSchematicInfo = "KicadSchematicInfo", _kicad_pcb = "kicad_pcb", _kicad_schematic = "kicad_schematic")
load("//kicad/private:determinism.bzl", _determinism_test = "determinism_test")
load("//kicad/private:erc.bzl", _kicad_erc_report = "kicad_erc_report", _kicad_erc_test = "kicad_erc_test")
load("//kicad/private:kicad_toolchain.bzl", _KicadToolchainInfo = "KicadToolchainInfo", _kicad_toolchain = "kicad_toolchain")
load("//kicad/private:library.bzl", _KicadLibraryInfo = "KicadLibraryInfo", _kicad_lib_tables = "kicad_lib_tables", _kicad_library = "kicad_library", _lib_table_test = "lib_table_test")
load("//kicad/private:normalize.bzl", _NormalizeToolchainInfo = "NormalizeToolchainInfo")
load("//kicad/private:swap_test.bzl", _swap_test = "swap_test")
load("//kicad/private:toolchains.bzl", _tool_toolchain = "tool_toolchain")

determinism_test = _determinism_test
swap_test = _swap_test
tool_toolchain = _tool_toolchain
kicad_toolchain = _kicad_toolchain
KicadToolchainInfo = _KicadToolchainInfo
kicad_library = _kicad_library
kicad_lib_tables = _kicad_lib_tables
lib_table_test = _lib_table_test
KicadLibraryInfo = _KicadLibraryInfo
NormalizeToolchainInfo = _NormalizeToolchainInfo
kicad_schematic = _kicad_schematic
kicad_erc_report = _kicad_erc_report
kicad_erc_test = _kicad_erc_test
kicad_pcb = _kicad_pcb
KicadSchematicInfo = _KicadSchematicInfo
KicadPcbInfo = _KicadPcbInfo

# The release this file belongs to. It lets a consumer check, in a test,
# that the module it resolved is the one it expects.
RULES_KICAD_API_VERSION = "0"
