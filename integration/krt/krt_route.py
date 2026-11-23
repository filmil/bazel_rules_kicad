# SPDX-License-Identifier: Apache-2.0
"""krt_route --krt_root DIR -- ARGS...: runs KiCadRoutingTools'
py_router/route.py with ARGS, as `python3 py_router/route.py ARGS` would,
from the release's files in DIR.

The release names its Linux router module grid_router-linux-x86_64.so;
build_router.py would copy it to grid_router.so. Here it is loaded under
its module name instead, so nothing is copied and nothing downloaded.
KICAD_CLI and KICAD_PYTHON come from the rule, so route.py finds the
pinned KiCad and looks for no other.

route.py names each item it adds with uuid.uuid4(), a random UUID, so the
same board routed twice differed in those names only (the copper was the
same). Here uuid.uuid4 draws from a generator seeded with the input
board's sha256: the same board gets the same names.
"""

import hashlib
import importlib.machinery
import importlib.util
import os
import random
import runpy
import sys
import uuid


def seed_uuid4(board):
    """Makes uuid.uuid4() a function of board's bytes: version-4 UUIDs
    from a generator seeded with the board's sha256."""
    with open(board, "rb") as f:
        rng = random.Random(hashlib.sha256(f.read()).digest())
    uuid.uuid4 = lambda: uuid.UUID(int=rng.getrandbits(128), version=4)


def main(argv):
    if len(argv) < 3 or argv[0] != "--krt_root" or "--" not in argv:
        sys.stderr.write("usage: krt_route --krt_root DIR -- ARGS...\n")
        return 2
    root = os.path.abspath(argv[1])
    args = argv[argv.index("--") + 1:]
    for var in ("KICAD_CLI", "KICAD_PYTHON"):
        if not os.environ.get(var):
            sys.stderr.write("krt_route: %s is not set\n" % var)
            return 2
    # As rust_alloc.py sets it, before the module (and its mimalloc) loads.
    os.environ.setdefault("MIMALLOC_PURGE_DELAY", "0")
    if not args or not os.path.isfile(args[0]):
        sys.stderr.write("krt_route: the first argument after -- must be the input board\n")
        return 2
    seed_uuid4(args[0])
    so = os.path.join(root, "rust_router", "grid_router-linux-x86_64.so")
    loader = importlib.machinery.ExtensionFileLoader("grid_router", so)
    spec = importlib.util.spec_from_file_location("grid_router", so, loader=loader)
    module = importlib.util.module_from_spec(spec)
    loader.exec_module(module)
    sys.modules["grid_router"] = module
    py_router = os.path.join(root, "py_router")
    sys.path.insert(0, py_router)
    sys.argv = [os.path.join(py_router, "route.py")] + args
    runpy.run_path(sys.argv[0], run_name="__main__")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
