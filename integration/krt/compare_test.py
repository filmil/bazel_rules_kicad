# SPDX-License-Identifier: Apache-2.0
"""compare.py on made-up boards and DRC reports."""

import os
import tempfile
import unittest

import compare

BOARD = """(kicad_pcb
\t(segment (start 0 0) (end 3 4) (width 0.2) (layer "F.Cu") (net 1))
\t(arc (start 0 0) (mid 1 1) (end 0 2) (width 0.2) (layer "B.Cu") (net 1))
\t(via (at 3 4) (size 0.6) (drill 0.3) (layers "F.Cu" "B.Cu") (net 1))
\t(via (at 0 2) (size 0.6) (drill 0.3) (layers "F.Cu" "B.Cu") (net 1))
)
"""
REPORT = {
    "violations": [{"severity": "error"}, {"severity": "warning"}, {"severity": "error"}],
    "unconnected_items": [{"severity": "error"}],
    "schematic_parity": [{"severity": "warning"}],
}


class CompareTest(unittest.TestCase):
    def test_board_stats(self):
        length, vias = compare.board_stats(BOARD)
        self.assertAlmostEqual(length, 5.0 + 2.0)
        self.assertEqual(vias, 2)

    def test_drc_counts(self):
        self.assertEqual(compare.drc_counts(REPORT), (3, 2, 1))
        self.assertEqual(compare.drc_counts({}), (0, 0, 0))

    def test_main(self):
        d = tempfile.mkdtemp()
        board, drc, out = (os.path.join(d, n) for n in ("b.kicad_pcb", "r.json", "o.tsv"))
        with open(board, "w") as f:
            f.write(BOARD)
        with open(drc, "w") as f:
            f.write('{"violations": [], "unconnected_items": []}')
        self.assertEqual(compare.main([out, "fr=%s:%s" % (board, drc)]), 0)
        with open(out) as f:
            self.assertEqual(f.read(), "router\tdrc_errors\tdrc_warnings\tunconnected\ttrack_mm\tvias\nfr\t0\t0\t0\t7.0\t2\n")
        self.assertEqual(compare.main([out, "bad"]), 2)


if __name__ == "__main__":
    unittest.main()
