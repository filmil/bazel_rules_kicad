# SPDX-License-Identifier: Apache-2.0
"""compare OUT NAME=BOARD:DRC_JSON...: one TSV row per routed board, for
the router comparison of issue #104: the router's name, the DRC's errors
and warnings (violations, unconnected items included), the unconnected
items alone, the copper track length in mm (segments and arcs, arcs by
their chord), and the vias. Wall time is not here: it is not the same on
two runs, so it is taken from Bazel's execution log instead."""

import json
import math
import re
import sys

HEADER = ["router", "drc_errors", "drc_warnings", "unconnected", "track_mm", "vias"]
_N = r"(-?[\d.]+)"
SEGMENT = re.compile(r"\((?:segment|arc)\s+\(start %s %s\)(?:\s+\(mid [^)]*\))?\s+\(end %s %s\)" % (_N, _N, _N, _N))
VIA = re.compile(r"^\s*\(via\b", re.M)


def board_stats(text):
    """(track length in mm, vias) of a .kicad_pcb's copper."""
    length = sum(math.hypot(float(c) - float(a), float(d) - float(b)) for a, b, c, d in SEGMENT.findall(text))
    return length, len(VIA.findall(text))


def drc_counts(report):
    """(errors, warnings, unconnected) of a kicad-cli DRC JSON report."""
    items = report.get("violations", []) + report.get("unconnected_items", []) + report.get("schematic_parity", [])
    errors = sum(1 for v in items if v.get("severity") == "error")
    warnings = sum(1 for v in items if v.get("severity") == "warning")
    return errors, warnings, len(report.get("unconnected_items", []))


def row(name, board_text, report):
    length, vias = board_stats(board_text)
    e, w, u = drc_counts(report)
    return [name, str(e), str(w), str(u), "%.1f" % length, str(vias)]


def main(argv):
    if len(argv) < 2:
        sys.stderr.write(__doc__.splitlines()[0] + "\n")
        return 2
    rows = ["\t".join(HEADER)]
    for spec in argv[1:]:
        name, _, paths = spec.partition("=")
        board, _, drc = paths.partition(":")
        if not (name and board and drc):
            sys.stderr.write("compare: %r is not NAME=BOARD:DRC_JSON\n" % spec)
            return 2
        with open(board, encoding="utf-8") as f:
            text = f.read()
        with open(drc, encoding="utf-8") as f:
            report = json.load(f)
        rows.append("\t".join(row(name, text, report)))
    with open(argv[0], "w", encoding="utf-8") as f:
        f.write("\n".join(rows) + "\n")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
