# The e2e design

A small design that the rules of rules_kicad run on, laid out as a user
writes one.

* `main.kicad_sch`, the root sheet: a two-pin power connector J1 with
  `+3V3` and `GND`, a power flag on each net, and the sheet "Power".
* `power.kicad_sch`: a 1k resistor R1 and an LED D1 in series from `+3V3`
  to `GND`.
* `main.kicad_pcb`: the board, 22 mm by 12 mm, with J1 (2.54 mm header),
  R1 and D1 (0603) and their three nets routed on the top copper.
* `main.kicad_pro`: the project file, with KiCad 9's default rules.

All symbols and footprints come from the KiCad 9.0.9 libraries
(`@kicad_libs//:standard`).

On 2026-10-29, KiCad 9.0.9 found the design clean when staged the way
`kicad_schematic` and `kicad_pcb` stage it:

* ERC with every severity: 0 violations;
* DRC with every severity and schematic parity: 0 violations, 0 unconnected
  items, 0 parity issues.

`tools/` holds the scripts that wrote the files: `mksch.py` writes both
sheets from the library symbols, with fixed UUIDs; `mkpcb.py` writes the
board with KiCad's own Python (`pcbnew`). Edit the design in KiCad from now
on; the scripts are a record of where it came from.
