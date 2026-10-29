<!-- SPDX-License-Identifier: Apache-2.0 -->
# Fixtures of timestamp_scan_test

* `raw.gtl` is the head of a Gerber file as KiCad 9.0.9 writes it, before
  the normalizer: it has the time of the export twice.
  It is the first 12 lines of
  `kicad/private/normalize/testdata/a/fixture-F_Cu.gtl`, then `M02*`.
* `titled.kicad_sch` is a schematic sheet whose title block has a date,
  as a designer writes it.
* `titled.net` is the part of a netlist where KiCad copies that date,
  next to its own time stamp at the epoch.
