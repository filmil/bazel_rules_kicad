# How the normalizer's fixtures were made

`../a` and `../b` hold real KiCad 9.0.9 outputs: the same exports of the same
board and schematic, made about 30 seconds apart on eph4, a host in CET.
Every file in `a` differs from its twin in `b` in its time stamps only.
They are test inputs, so they are checked in, unlike other KiCad outputs.

The sources are here:

* `fixture.kicad_pcb`: a 20 mm by 12 mm board with two plated holes, two SMD
  pads, a track, a via and silkscreen text. `mkboard.py` made it, with the
  `pcbnew` module of the prebuilt KiCad's own Python:
  `<tree>/bin/python3.11 -I mkboard.py fixture.kicad_pcb`.
* `fixture.kicad_sch`: an empty schematic, written by hand.

`export.sh <tree> <this dir> <out>` runs the exports: Gerber, drill with a
Gerber map, PDF of the board and of the schematic, STEP, and the DRC and ERC
reports in JSON. It ran twice, into `a` and `b`; of the outputs, the tests
use one copper layer, the job file, the drill file and its map, both PDFs,
the STEP file and both reports.

`expected_epoch_86400.drl` (in `..`) is `a/fixture.drl` with its two dates
edited by hand to 1970-01-02, so that the normalizer is not its own check.
