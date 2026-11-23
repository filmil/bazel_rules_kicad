# SPDX-License-Identifier: Apache-2.0
"""krt_route's argument parsing, its seeding and its module directory, on
made-up input (the routing itself is tested by //e2e/route_krt)."""

import os
import random
import sys
import tempfile
import unittest
import uuid
from unittest import mock

import krt_route


class KrtRouteTest(unittest.TestCase):
    def setUp(self):
        self.dir = tempfile.mkdtemp()
        self.board = os.path.join(self.dir, "b.kicad_pcb")
        with open(self.board, "w") as f:
            f.write("(kicad_pcb)\n")
        self.env = mock.patch.dict(os.environ, {"KICAD_CLI": "/k/cli", "KICAD_PYTHON": "/k/py"})
        self.env.start()
        self.addCleanup(self.env.stop)

    def test_parse(self):
        (root, args), err = krt_route.parse(["--krt_root", self.dir, "--", self.board, "out", "--nets", "A"])
        self.assertIsNone(err)
        self.assertEqual((root, args), (os.path.abspath(self.dir), [self.board, "out", "--nets", "A"]))

    def test_parse_errors(self):
        for argv in (
            [],
            ["--krt_root", self.dir, self.board],  # no --
            ["--krt_root", "--", self.board],  # no DIR
            ["--krt_root", self.dir, "x", "--", self.board],  # something before --
            ["--krt_root", self.dir, "--"],  # no BOARD
            ["--krt_root", self.dir, "--", os.path.join(self.dir, "none")],  # BOARD missing
        ):
            parsed, err = krt_route.parse(argv)
            self.assertIsNone(parsed, argv)
            self.assertTrue(err, argv)
        with mock.patch.dict(os.environ, {"KICAD_PYTHON": ""}):
            self.assertIn("KICAD_PYTHON", krt_route.parse(["--krt_root", self.dir, "--", self.board])[1])

    def test_seed(self):
        krt_route.seed(self.board)
        a = [uuid.uuid4() for _ in range(3)] + [random.random()]
        krt_route.seed(self.board)
        b = [uuid.uuid4() for _ in range(3)] + [random.random()]
        self.assertEqual(a, b)
        self.assertTrue(all(u.version == 4 and u.variant == uuid.RFC_4122 for u in a[:3]))
        self.assertEqual(len(set(a[:3])), 3)
        other = os.path.join(self.dir, "c.kicad_pcb")
        with open(other, "w") as f:
            f.write("(kicad_pcb (version 1))\n")
        krt_route.seed(other)
        self.assertNotEqual(uuid.uuid4(), a[0])

    def test_module_dir(self):
        os.makedirs(os.path.join(self.dir, "rust_router"))
        so = os.path.join(self.dir, "rust_router", "grid_router-linux-x86_64.so")
        open(so, "w").close()
        with mock.patch.dict(os.environ, {"PYTHONPATH": "/x"}), mock.patch.object(sys, "path", list(sys.path)):
            d = krt_route.module_dir(self.dir)
            self.assertEqual(os.readlink(os.path.join(d, "grid_router.so")), so)
            self.assertEqual(sys.path[0], d)
            self.assertEqual(os.environ["PYTHONPATH"], d + os.pathsep + "/x")


if __name__ == "__main__":
    unittest.main()
