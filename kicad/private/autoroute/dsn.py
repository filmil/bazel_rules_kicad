# SPDX-License-Identifier: Apache-2.0
"""Exports a board as Specctra DSN for Freerouting, with KiCad's pcbnew.

    python3 dsn.py --board B.kicad_pcb --out B.dsn [--rules R.kicad_dru]

KiCad's own Python runs it (the KiCad toolchain's `kicad_python`): the
DSN exporter is in pcbnew, and kicad-cli has no command for it.

With --rules, every net class's clearance becomes at least the largest
whole-board `clearance` or `edge_clearance` minimum in the rules file, and
its track width at least the largest `track_width` minimum (see
minimums), so that Freerouting, which knows one clearance, keeps the
vendor's. Rules for some nets or areas only, and hole clearances, are not
Freerouting's to keep: the DRC checks them. The board's own rules are not
changed: the routed board carries the ones it came with.

    python3 dsn.py --selftest

checks minimums on fixed inputs.
"""

import argparse
import os
import re
import sys

import pcbnew


_UNITS = {"mm": 1.0, "mil": 0.0254, "in": 25.4, "um": 0.001}

# A rule whose condition names one of these applies to some nets, net
# classes, parts or areas only: its clearance is not the whole board's.
_SCOPED = ("NetClass", "NetName", "Net ==", "Net==", "hasNetclass", "memberOf",
           "Reference", "insideArea", "intersectsArea", "enclosedByArea",
           "insideCourtyard", "intersectsCourtyard", "existsOnLayer", "Layer ==")


def tokens(text):
    """The tokens of a .kicad_dru: "(", ")", atoms and quoted strings (as
    ("str", value)). A # outside a string starts a comment to the line's end."""
    out, i, n = [], 0, len(text)
    while i < n:
        c = text[i]
        if c in " \t\r\n":
            i += 1
        elif c == "#":
            while i < n and text[i] != "\n":
                i += 1
        elif c in "()":
            out.append(c)
            i += 1
        elif c == '"':
            j, buf = i + 1, []
            while j < n and text[j] != '"':
                if text[j] == "\\" and j + 1 < n:
                    j += 1
                buf.append(text[j])
                j += 1
            out.append(("str", "".join(buf)))
            i = j + 1
        else:
            j = i
            while j < n and text[j] not in ' \t\r\n()"':
                j += 1
            out.append(text[i:j])
            i = j
    return out


def parse(text):
    """The .kicad_dru as nested lists."""
    stack, cur = [], []
    for t in tokens(text):
        if t == "(":
            stack.append(cur)
            cur = []
        elif t == ")":
            if not stack:
                raise ValueError("unbalanced )")
            done, cur = cur, stack.pop()
            cur.append(done)
        else:
            cur.append(t[1] if isinstance(t, tuple) else t)
    if stack:
        raise ValueError("unclosed (")
    return cur


def length_mm(atoms):
    """A length such as ["0.2mm"] or ["0.2", "mm"] or ["8mil"], in mm."""
    text = "".join(atoms)
    for unit in sorted(_UNITS, key=len, reverse=True):
        if text.endswith(unit):
            return float(text[: -len(unit)]) * _UNITS[unit]
    return float(text)  # KiCad's default unit is mm


def minimums(text):
    """The largest whole-board copper clearance and the largest minimum
    track width of a .kicad_dru, in mm.

    Counted: the `min` of `clearance` and `edge_clearance` constraints, and
    of `track_width`, in rules without `(severity ignore)` whose condition
    names no net, net class, part, area or layer (those are a part of the
    board's, not the whole board's)."""
    clearance, width = 0.0, 0.0
    for rule in parse(text):
        if not isinstance(rule, list) or not rule or rule[0] != "rule":
            continue
        cond = " ".join(" ".join(x for x in c[1:] if isinstance(x, str))
                        for c in rule if isinstance(c, list) and c and c[0] == "condition")
        if any(k in cond for k in _SCOPED):
            continue
        if any(isinstance(c, list) and c[:2] == ["severity", "ignore"] for c in rule):
            continue
        for c in rule:
            if not (isinstance(c, list) and len(c) >= 2 and c[0] == "constraint"):
                continue
            for limit in c[2:]:
                if isinstance(limit, list) and limit and limit[0] == "min":
                    v = length_mm([x for x in limit[1:] if isinstance(x, str)])
                    if c[1] in ("clearance", "edge_clearance"):
                        clearance = max(clearance, v)
                    elif c[1] == "track_width":
                        width = max(width, v)
    return clearance, width


def _netclasses(board):
    ns = board.GetDesignSettings().m_NetSettings
    classes = [ns.GetDefaultNetclass()]
    for _, nc in sorted(ns.GetNetclasses().items()):
        classes.append(nc)
    return classes


def selftest():
    cases = [
        # opt and max before min; units; a # in a string; a comment.
        ('(rule "a #1" (constraint clearance (opt 0.3mm) (min 0.2mm) (max 1mm)))', (0.2, 0.0)),
        ('(rule "b" (constraint clearance (min 8mil)))', (0.2032, 0.0)),
        ('(rule "c" (constraint track_width (min 0.15 mm)))', (0.0, 0.15)),
        ('# (rule "x" (constraint clearance (min 5mm)))\n(rule "d" (constraint edge_clearance (min 300um)))', (0.3, 0.0)),
        # scoped and ignored rules do not count; type conditions do.
        ('(rule "hv" (condition "A.NetClass == \'HV\'") (constraint clearance (min 2mm)))', (0.0, 0.0)),
        ('(rule "off" (severity ignore) (constraint clearance (min 2mm)))', (0.0, 0.0)),
        ('(rule "t" (condition "A.Type == \'Track\' && A.Net != B.Net") (constraint clearance (min 0.25mm)))', (0.25, 0.0)),
    ]
    bad = 0
    for text, want in cases:
        got = minimums(text)
        if any(abs(g - w) > 1e-9 for g, w in zip(got, want)):
            print("selftest: %s: got %s, want %s" % (text, got, want))
            bad += 1
    if bad:
        sys.exit("selftest: %d case(s) failed" % bad)
    print("selftest: %d cases pass" % len(cases))


def main(argv):
    if argv == ["--selftest"]:
        selftest()
        return
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
    name = os.path.basename(a.out)
    text = re.sub(r'\A\(pcb (?:"[^"]*"|\S+)', lambda _: '(pcb "%s"' % name, text, count=1)
    with open(a.out, "w", encoding="utf-8") as f:
        f.write(text)


if __name__ == "__main__":
    main(sys.argv[1:])
