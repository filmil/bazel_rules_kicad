# SPDX-License-Identifier: Apache-2.0
"""krt_route --krt_root DIR -- BOARD [OUT] [OPTIONS...]: runs
KiCadRoutingTools' py_router/route.py on BOARD, from the release's files in
DIR, as `python3 py_router/route.py BOARD [OUT] [OPTIONS...]` would.

- The release names its Linux router module grid_router-linux-x86_64.so,
  and build_router.py would copy it to grid_router.so. Here a symlink
  grid_router.so, in a directory of its own put first on PYTHONPATH, makes
  it importable by route.py and by any Python process route.py starts.
- route.py names each item it adds with uuid.uuid4(), a random UUID, so the
  same board routed twice differed in those names only (its copper was the
  same). Here uuid.uuid4, and the random module, draw from generators
  seeded with BOARD's sha256: the same board gets the same names.
- KICAD_CLI and KICAD_PYTHON come from the rule, so route.py finds the
  pinned KiCad and looks for no other.
"""

import hashlib
import os
import random
import runpy
import sys
import tempfile
import uuid

USAGE = "usage: krt_route --krt_root DIR -- BOARD [OUT] [OPTIONS...]\n"


def parse(argv):
    """(root, route.py's arguments), or an error message."""
    if len(argv) < 4 or argv[0] != "--krt_root" or argv[2] != "--":
        return None, USAGE
    root, args = os.path.abspath(argv[1]), argv[3:]
    if not os.path.isfile(args[0]):
        return None, "krt_route: BOARD %s is not a file\n" % args[0]
    for var in ("KICAD_CLI", "KICAD_PYTHON"):
        if not os.environ.get(var):
            return None, "krt_route: %s is not set\n" % var
    return (root, args), None


def seed(board):
    """Makes uuid.uuid4() and the random module functions of the board's
    bytes: version-4 UUIDs, and random's numbers, from generators seeded
    with its sha256."""
    with open(board, "rb") as f:
        digest = hashlib.sha256(f.read()).digest()
    rng = random.Random(digest)
    uuid.uuid4 = lambda: uuid.UUID(int=rng.getrandbits(128), version=4)
    random.seed(digest)


def module_dir(root):
    """A new directory with grid_router.so, a symlink to the release's
    module, first on sys.path and on PYTHONPATH for child processes."""
    d = tempfile.mkdtemp(prefix="krt_grid_router_")
    os.symlink(os.path.join(root, "rust_router", "grid_router-linux-x86_64.so"), os.path.join(d, "grid_router.so"))
    sys.path.insert(0, d)
    os.environ["PYTHONPATH"] = os.pathsep.join([d] + [p for p in os.environ.get("PYTHONPATH", "").split(os.pathsep) if p])
    return d


def main(argv):
    parsed, err = parse(argv)
    if err:
        sys.stderr.write(err)
        return 2
    root, args = parsed
    sys.dont_write_bytecode = True
    # As rust_alloc.py sets it, before the module (and its mimalloc) loads.
    os.environ.setdefault("MIMALLOC_PURGE_DELAY", "0")
    seed(args[0])
    module_dir(root)
    py_router = os.path.join(root, "py_router")
    sys.path.insert(0, py_router)
    sys.argv = [os.path.join(py_router, "route.py")] + args
    runpy.run_path(sys.argv[0], run_name="__main__")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
