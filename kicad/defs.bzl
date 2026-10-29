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

load("//kicad/private:bom.bzl", _kicad_bom = "kicad_bom")
load("//kicad/private:design.bzl", _KicadPcbInfo = "KicadPcbInfo", _KicadSchematicInfo = "KicadSchematicInfo", _kicad_pcb = "kicad_pcb", _kicad_schematic = "kicad_schematic")
load("//kicad/private:determinism.bzl", _determinism_test = "determinism_test")
load("//kicad/private:drc.bzl", _kicad_drc_report = "kicad_drc_report", _kicad_drc_test = "kicad_drc_test")
load("//kicad/private:drill.bzl", _DRILL_MAP_FORMATS = "DRILL_MAP_FORMATS", _kicad_drill = "kicad_drill")
load("//kicad/private:erc.bzl", _kicad_erc_report = "kicad_erc_report", _kicad_erc_test = "kicad_erc_test")
load("//kicad/private:gerbers.bzl", _DEFAULT_GERBER_LAYERS = "DEFAULT_GERBER_LAYERS", _kicad_gerbers = "kicad_gerbers")
load("//kicad/private:golden.bzl", _GoldenInfo = "GoldenInfo", _golden_test = "golden_test", _update_goldens = "update_goldens")
load("//kicad/private:kicad_toolchain.bzl", _KicadToolchainInfo = "KicadToolchainInfo", _kicad_toolchain = "kicad_toolchain")
load("//kicad/private:library.bzl", _KicadLibraryInfo = "KicadLibraryInfo", _kicad_lib_tables = "kicad_lib_tables", _kicad_library = "kicad_library", _lib_table_test = "lib_table_test")
load("//kicad/private:netlist.bzl", _kicad_netlist = "kicad_netlist")
load("//kicad/private:normalize.bzl", _NormalizeToolchainInfo = "NormalizeToolchainInfo")
load("//kicad/private:pos.bzl", _kicad_pos = "kicad_pos")
load("//kicad/private:render.bzl", _kicad_render = "kicad_render")
load("//kicad/private:sch_pdf.bzl", _kicad_sch_pdf = "kicad_sch_pdf")
load("//kicad/private:sch_render.bzl", _kicad_sch_render = "kicad_sch_render")
load("//kicad/private:step.bzl", _kicad_step = "kicad_step")
load("//kicad/private:swap_test.bzl", _swap_test = "swap_test")
load("//kicad/private:timestamp_scan.bzl", _timestamp_scan_test = "timestamp_scan_test")
load("//kicad/private:toolchains.bzl", _tool_toolchain = "tool_toolchain")

DEFAULT_GERBER_LAYERS = _DEFAULT_GERBER_LAYERS
DRILL_MAP_FORMATS = _DRILL_MAP_FORMATS
GoldenInfo = _GoldenInfo
KicadLibraryInfo = _KicadLibraryInfo
KicadPcbInfo = _KicadPcbInfo
KicadSchematicInfo = _KicadSchematicInfo
KicadToolchainInfo = _KicadToolchainInfo
NormalizeToolchainInfo = _NormalizeToolchainInfo
determinism_test = _determinism_test
golden_test = _golden_test
kicad_bom = _kicad_bom
kicad_drc_report = _kicad_drc_report
kicad_drc_test = _kicad_drc_test
kicad_drill = _kicad_drill
kicad_erc_report = _kicad_erc_report
kicad_erc_test = _kicad_erc_test
kicad_gerbers = _kicad_gerbers
kicad_lib_tables = _kicad_lib_tables
kicad_library = _kicad_library
kicad_netlist = _kicad_netlist
kicad_pcb = _kicad_pcb
kicad_pos = _kicad_pos
kicad_render = _kicad_render
kicad_sch_pdf = _kicad_sch_pdf
kicad_sch_render = _kicad_sch_render
kicad_schematic = _kicad_schematic
kicad_step = _kicad_step
kicad_toolchain = _kicad_toolchain
lib_table_test = _lib_table_test
swap_test = _swap_test
timestamp_scan_test = _timestamp_scan_test
tool_toolchain = _tool_toolchain
update_goldens = _update_goldens

# The release this file belongs to. It lets a consumer check, in a test,
# that the module it resolved is the one it expects.
RULES_KICAD_API_VERSION = "0"
