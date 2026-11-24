# SPDX-License-Identifier: Apache-2.0
"""The copied board, project and netlist have the sha256s README.md
records: they are hdlf_example_sensor's files, unchanged."""

import hashlib
import os
import re
import unittest


class FixturesTest(unittest.TestCase):
    def test_sha256s(self):
        readme = os.environ["README"]
        with open(readme, encoding="utf-8") as f:
            rows = re.findall(r"^\| `(sensor\.[a-z_]+)` \| .* \| `([0-9a-f]{64})` \|$", f.read(), re.M)
        self.assertEqual(sorted(n for n, _ in rows), ["sensor.kicad_pcb", "sensor.kicad_pro", "sensor.net"])
        for name, want in rows:
            with open(os.path.join(os.path.dirname(readme), name), "rb") as f:
                self.assertEqual(hashlib.sha256(f.read()).hexdigest(), want, name)


if __name__ == "__main__":
    unittest.main()
