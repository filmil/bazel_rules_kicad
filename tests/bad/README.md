# Known-bad designs

Spec T3: a check that cannot fail proves nothing. Each directory here is
the e2e design (`integration/e2e`) with one fault, and its test passes only
if KiCad 9.0.9 reports that fault and no other.

| Directory | Fault | Test | KiCad reports |
|---|---|---|---|
| `erc/` | `power.kicad_sch` lacks the wire from R1 to D1 | `:erc_test` | 2 × `pin_not_connected` (R1 pin 2, D1 pin A) |
| `drc/` | `main.kicad_dru` asks for 1.5 mm between nets | `:drc_test` | 6 × `clearance` |
| `parity/` | `main.kicad_pcb` lacks D1 and its two tracks | `:parity_test` | 1 × `missing_footprint` (D1) |

The other files are copies of the e2e design. A broken rule file, which
KiCad ignores without a word, is in `tests/drc_rules`.
