<!-- SPDX-License-Identifier: Apache-2.0 -->
# The router comparison on hdlf_example_sensor's board

The board of `//e2e/autoroute` is small: three tracks. This package
compares Freerouting and KiCadRoutingTools on a real one: the placed
board of https://github.com/filmil/hdlf_example_sensor (26 footprints, 22
parts and 4 mounting holes; 18 nets; 10 keep-out zones). It has no copper
pours, so KiCadRoutingTools' exact fill, which runs in KiCad's own
Python, is not exercised here either.

The three files are copies of that repository's build outputs, unchanged,
at commit `0a944e6` (main, 2026-11-23):

`:fixtures_test` checks that the copies still have these sha256s.

| File here | Its target there | sha256 |
|---|---|---|
| `sensor.kicad_pcb` | `//pcb:placed_files` (`placed.kicad_pcb`) | `1300850aa979aa9f9b2683427a06a48fdbad86be9aa11f1fa6b892ab01477736` |
| `sensor.kicad_pro` | `//pcb:placed_files` (`placed.kicad_pro`) | `1ca380c225693be96b08735db7b8e454ca0d44a2888a3865b43e9d0d60b547e8` |
| `sensor.net` | `//elec:netlist` (`netlist.net`) | `52ffbad20848262a8f70201be18e84db21b38a26ba245d2aaf50818048122feb` |

Both routers route under the rules the sensor is made to: JLCPCB's
2-layer 1 oz rules from kicad-druid 1.2.0 (MIT,
`//third_party/kicad_druid`), as hdlf_example_sensor uses them. A first
run under this repository's stricter e2e rules (0.25 mm tracks, 0.2 mm
clearance) was no fair comparison: Freerouting left 9 connections
unrouted, and KiCadRoutingTools narrowed 65 of its 242 segments below
0.25 mm. No 3D models are used.

The routing targets are manual, since routing this board takes minutes;
`:placed`, which only stages the board, and `:fixtures_test` are not, so
CI still analyses the package and checks the copies. Run these in
`integration/` (the integration module), on Linux x86_64 (the only host
KiCadRoutingTools' targets support):

```
bazel test //e2e/route_krt_sensor:freerouting_determinism_test //e2e/route_krt_sensor:krt_determinism_test //e2e/route_krt_sensor:krt_board_determinism_test
bazel build //e2e/route_krt_sensor:comparison
```

(A pattern such as `:all` skips manual targets, so they are named.)

Three routers: Freerouting (`freerouting`), KiCadRoutingTools with its
defaults (`krt`), and KiCadRoutingTools with `--escalation board`
(`krt_board`), which narrows a failing net's tracks only down to the
board's own minimums, not the fab's floor.

`comparison.tsv` has one row per router: the DRC's errors and warnings,
the unconnected items, the track length in mm and the vias
(`//krt:compare`). The routing time is not in it, since it differs between
runs; take it from Bazel's execution log (`--execution_log_json_file`,
the `Freerouting` and `KrtRoute` actions). The results are on issue #104.
