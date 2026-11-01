# SPDX-License-Identifier: Apache-2.0
"""Exports a board as Specctra DSN for Freerouting, with KiCad's pcbnew.

    python3 dsn.py --board B.kicad_pcb --out B.dsn [--rules R.kicad_dru]

KiCad's own Python runs it (the KiCad toolchain's `kicad_python`): the
DSN exporter is in pcbnew, and kicad-cli has no command for it.

With --rules, every net class's clearance becomes at least the largest
`clearance` or `edge_clearance` minimum in the rules file, and its track
width at least the largest `track_width` minimum, so that Freerouting,
which knows one clearance, keeps the vendor's. Hole clearances are not
Freerouting's to keep: a plated hole's copper ring keeps tracks farther
off than they ask, and the DRC checks them.
"""

import argparse
import os
import re
import sys

import pcbnew


def _strip_comments(text):
    out = []
    for line in text.splitlines():
        i = line.find("#")
        out.append(line if i < 0 else line[:i])
    return "\n".join(out)


def minimums(text):
    """The largest copper clearance and the largest minimum track width of
    a .kicad_dru, in mm. Commented lines (# ...) do not count."""
    text = _strip_comments(text)
    clearance, width = 0.0, 0.0
    for kind, value in re.findall(r"\(constraint\s+(\w+)\s+\(min\s+([0-9.]+)mm\)", text):
        v = float(value)
        if kind in ("clearance", "edge_clearance"):
            clearance = max(clearance, v)
        elif kind == "track_width":
            width = max(width, v)
    return clearance, width


def _netclasses(board):
    ns = board.GetDesignSettings().m_NetSettings
    classes = [ns.GetDefaultNetclass()]
    for _, nc in sorted(ns.GetNetclasses().items()):
        classes.append(nc)
    return classes


def main(argv):
    p = argparse.ArgumentParser(prog="dsn")
    p.add_argument("--board", required=True)
    p.add_argument("--out", required=True)
    p.add_argument("--rules", default="")
    a = p.parse_args(argv)
    board = pcbnew.LoadBoard(a.board)
    if a.rules:
        clearance, width = minimums(open(a.rules, encoding="utf-8").read())
        for nc in _netclasses(board):
            if clearance > 0 and pcbnew.ToMM(nc.GetClearance()) < clearance:
                nc.SetClearance(pcbnew.FromMM(clearance))
            if width > 0 and pcbnew.ToMM(nc.GetTrackWidth()) < width:
                nc.SetTrackWidth(pcbnew.FromMM(width))
        print("dsn: clearance %.3f mm, track width at least %.3f mm" % (clearance, width))
    if not pcbnew.ExportSpecctraDSN(board, a.out):
        sys.exit("dsn: KiCad could not export %s" % a.board)
    # KiCad names the design by the path it was given; the base name keeps
    # the file the same wherever Bazel puts it.
    text = open(a.out, encoding="utf-8").read()
    text = re.sub(r'\A\(pcb "[^"]*"', '(pcb "%s"' % os.path.basename(a.out), text, count=1)
    with open(a.out, "w", encoding="utf-8") as f:
        f.write(text)


if __name__ == "__main__":
    main(sys.argv[1:])
