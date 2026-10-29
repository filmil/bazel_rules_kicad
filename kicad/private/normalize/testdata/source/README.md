# How the normalizer's fixtures were made

`../a` and `../b` hold real KiCad 9.0.9 outputs: the same exports of the same
board and schematic, made about 30 seconds apart on eph4, a host in CET.
Every file in `a` differs from its twin in `b` in its time stamps only.
They are test inputs, so they are checked in, unlike other KiCad outputs.

The sources are here:

* `fixture.kicad_pcb`: a 20 mm by 12 mm board with two plated holes, two SMD
  pads, a track, a via and silkscreen text, made once with the `pcbnew`
  module of the prebuilt KiCad's own Python.
* `fixture.kicad_sch`: an empty schematic, written by hand.

The exports ran with kicad-cli twice, into `a` and `b`: `pcb export
gerbers`, `pcb export drill --generate-map --map-format gerberx2`, `pcb export
pdf --layers F.Cu,Edge.Cuts`, `pcb export step`, `pcb drc --format json`,
`sch erc --format json` and `sch export pdf`. Of the outputs, the tests use
one copper layer, the job file, the drill file and its map, both PDFs, the
STEP file and both reports. The one-off scripts that did this are not kept.

`a/copper.step` and `b/copper.step` are two runs, on another day and
host, of `kicad-cli pcb export step --no-components --include-tracks
--include-pads --include-zones` on `fixture.kicad_pcb`. kicad-cli numbers
and orders the entities of the copper differently on every run (issue
#63), so besides their time stamps the two files differ in most lines; the
normalizer must bring them to the same bytes.

`expected_epoch_86400.drl` (in `..`) is `a/fixture.drl` with its two dates
edited by hand to 1970-01-02, so that the normalizer is not its own check.

`a/e2e.net` and `b/e2e.net` are two netlist exports of the e2e schematic
(`integration/e2e`), a few seconds apart, by `kicad-cli sch export
netlist`. Their paths, which named a scratch directory, were replaced by
two made-up action directories (`/sandbox/one/...`, `/sandbox/two/...`):
a netlist names its schematic and its libraries by absolute path, and the
normalizer must cut those to file names.
