# SPDX-License-Identifier: Apache-2.0
"""Imports Freerouting's session (the routed copper) into a board.

    python3 ses.py --board B.kicad_pcb --ses B.ses --out R.kicad_pcb \\
        --report R.md [--allow_unrouted]

KiCad's own Python runs it. The importer gives each new track and via a
new UUID; the generator is seeded first, from a hash of the board and the
session, so the same inputs give the same UUIDs and other inputs others. Unless --allow_unrouted, a connection left unrouted fails.

Freerouting writes a net name bare when KiCad's export quoted it only
for being unusual, such as IN|SIDE, and KiCad's session reader then
rejects the whole file. Every bare net name is quoted before the import.

The report is Markdown, for a person: per net, the track length on each
copper layer, the tracks and the vias, and the connections left unrouted.
"""

import argparse
import hashlib
import os
import re
import sys
import tempfile

import pcbnew


def quote_nets(text):
    """The session with each bare net name, as in (net IN|SIDE, quoted."""
    return re.sub(r'\(net ([^\s"()]+)', lambda m: '(net "%s"' % m.group(1), text)


def report(board, name, left):
    """The routed board, per net, as a Markdown table."""
    nets = {}
    layers = set()
    for t in board.GetTracks():
        net = t.GetNetname() or "(no net)"
        n = nets.setdefault(net, {"len": {}, "tracks": 0, "vias": 0})
        if t.GetClass() == "PCB_VIA":
            n["vias"] += 1
            continue
        layer = board.GetLayerName(t.GetLayer())
        layers.add(layer)
        n["len"][layer] = n["len"].get(layer, 0.0) + pcbnew.ToMM(t.GetLength())
        n["tracks"] += 1
    layers = sorted(layers, key=lambda l: (l != "F.Cu", l != "B.Cu", l))
    out = ["# Routing of %s" % name, ""]
    out.append("Routed by Freerouting; %d connection(s) left unrouted." % left)
    out.append("")
    head = ["Net"] + ["%s (mm)" % l for l in layers] + ["Total (mm)", "Tracks", "Vias"]
    out.append("| " + " | ".join(head) + " |")
    out.append("|" + "|".join(["---"] + ["---:"] * (len(head) - 1)) + "|")
    total = {"len": {}, "tracks": 0, "vias": 0}
    for net in sorted(nets):
        n = nets[net]
        row = [net.replace("|", "\\|")]
        for l in layers:
            v = n["len"].get(l, 0.0)
            total["len"][l] = total["len"].get(l, 0.0) + v
            row.append("%.2f" % v)
        row += ["%.2f" % sum(n["len"].values()), str(n["tracks"]), str(n["vias"])]
        total["tracks"] += n["tracks"]
        total["vias"] += n["vias"]
        out.append("| " + " | ".join(row) + " |")
    row = ["**All**"] + ["%.2f" % total["len"].get(l, 0.0) for l in layers]
    row += ["%.2f" % sum(total["len"].values()), str(total["tracks"]), str(total["vias"])]
    out.append("| " + " | ".join(row) + " |")
    return "\n".join(out) + "\n"


def main(argv):
    p = argparse.ArgumentParser(prog="ses")
    p.add_argument("--board", required=True)
    p.add_argument("--ses", required=True)
    p.add_argument("--out", required=True)
    p.add_argument("--report", required=True)
    p.add_argument("--allow_unrouted", action="store_true")
    a = p.parse_args(argv)
    h = hashlib.sha256()
    for path in (a.board, a.ses):
        with open(path, "rb") as f:
            h.update(f.read())
    pcbnew.KIID.SeedGenerator(int.from_bytes(h.digest()[:4], "big"))
    board = pcbnew.LoadBoard(a.board)
    with open(a.ses, encoding="utf-8") as f:
        session = quote_nets(f.read())
    fd, quoted = tempfile.mkstemp(suffix=".ses")
    with os.fdopen(fd, "w", encoding="utf-8") as f:
        f.write(session)
    if not pcbnew.ImportSpecctraSES(board, quoted):
        sys.exit("ses: KiCad could not import %s" % a.ses)
    board.BuildConnectivity()
    left = board.GetConnectivity().GetUnconnectedCount(False)
    with open(a.report, "w", encoding="utf-8") as f:
        f.write(report(board, os.path.basename(a.out), left))
    if left and not a.allow_unrouted:
        sys.exit("ses: Freerouting left %d connection(s) unrouted" % left)
    if not pcbnew.SaveBoard(a.out, board):
        sys.exit("ses: KiCad could not write %s" % a.out)


if __name__ == "__main__":
    main(sys.argv[1:])
