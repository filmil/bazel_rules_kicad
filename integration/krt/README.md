<!-- SPDX-License-Identifier: Apache-2.0 -->
# The KiCadRoutingTools trial

Issue https://github.com/filmil/bazel_rules_kicad/issues/104 asks whether
KiCadRoutingTools (https://github.com/drandyhaas/KiCadRoutingTools, MIT)
can route boards here, beside or instead of Freerouting. This package is
the trial. It is kept in the integration module, so users of
hdlf_rules_kicad get none of it until the trial succeeds and the rule
moves into the module.

- `krt.bzl`: `kicad_route_krt`, a board routed by KiCadRoutingTools'
  `py_router/route.py`, and its toolchain. The action runs through
  `run_tool` with the KiCad toolchain's environment, fonts and private
  home, `KICAD_CLI` and `KICAD_PYTHON` set to the pinned KiCad's
  `kicad-cli` and Python, and no network.
- `krt_route.py`: runs `route.py` from the release's files. It loads the
  release's `grid_router-linux-x86_64.so` under its module name (the
  release's `build_router.py` would copy it to `grid_router.so`), and seeds
  `uuid.uuid4` with the input board's sha256. `route.py` names each item
  it adds with a random UUID, so without this, two routings of one board
  differed in those names, though their copper was the same.
- `requirements_lock.txt`: numpy 2.5.5, scipy 1.18.1 and shapely 2.2.0,
  each pinned to its CPython 3.14 manylinux x86_64 wheel. These are what
  `route.py` needs (its `startup_checks.ROUTING_PACKAGES`).

The tests are in `//e2e/route_krt`: the board `//e2e/autoroute` routes
with Freerouting, routed with KiCadRoutingTools, checked for determinism
and by the same DRC.
