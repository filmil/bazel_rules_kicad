// SPDX-License-Identifier: Apache-2.0

package merge

import (
	"os"
	"strings"
	"testing"

	"github.com/filmil/bazel_rules_kicad/kicad/private/sexpr"
)

func read(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func clean(t *testing.T, base, ours, theirs string) string {
	t.Helper()
	out, conflicts, err := Merge(base, ours, theirs)
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) > 0 {
		t.Fatalf("conflicts: %v", conflicts)
	}
	if _, err := sexpr.Parse(out); err != nil {
		t.Fatalf("the merge does not parse: %v", err)
	}
	return out
}

// heads counts the top list's children by head, and lib_symbols' by name.
func heads(t *testing.T, text string) (map[string]int, []string) {
	t.Helper()
	n, err := sexpr.Parse(text)
	if err != nil {
		t.Fatal(err)
	}
	c := map[string]int{}
	var libs []string
	for _, e := range n.List[1:] {
		c[e.Head()]++
		if e.Head() == "lib_symbols" {
			for _, s := range e.List[1:] {
				libs = append(libs, s.Arg(0))
			}
		}
	}
	return c, libs
}

// Two branches that each add a library symbol, a symbol and a wire.
func TestBothAdd(t *testing.T) {
	base, ours, theirs := read(t, "base.kicad_sch"), read(t, "ours.kicad_sch"), read(t, "theirs.kicad_sch")
	out := clean(t, base, ours, theirs)
	c, libs := heads(t, out)
	if c["symbol"] != 2 || c["wire"] != 2 {
		t.Errorf("got %d symbols and %d wires; want 2 and 2", c["symbol"], c["wire"])
	}
	if strings.Join(libs, " ") != "Device:R Device:C" {
		t.Errorf("lib_symbols has %v; want Device:R and Device:C", libs)
	}
	// Wires stay with wires, symbols with symbols, as KiCad writes them.
	if i, j := strings.Index(out, `(uuid "4d65726b-0000-4000-8000-0000000000b1")`), strings.Index(out, `(lib_id "Device:R")`); i < 0 || j < 0 || i > j {
		t.Errorf("theirs' wire is not with the wires")
	}
	// The other way round gives the same elements.
	c2, libs2 := heads(t, clean(t, base, theirs, ours))
	if c2["symbol"] != 2 || c2["wire"] != 2 || len(libs2) != 2 {
		t.Errorf("theirs into ours: %v %v", c2, libs2)
	}
}

// A change on one side comes out as that side's text, byte for byte.
func TestOneSide(t *testing.T) {
	base, ours := read(t, "base.kicad_sch"), read(t, "ours.kicad_sch")
	if out := clean(t, base, ours, base); out != ours {
		t.Errorf("ours alone: the merge differs from ours")
	}
	if out := clean(t, base, base, ours); out != ours {
		t.Errorf("theirs alone: the merge differs from theirs")
	}
	if out := clean(t, base, base, base); out != base {
		t.Errorf("no change: the merge differs from base")
	}
}

// The same change on both sides is clean, and taken once.
func TestSameChange(t *testing.T) {
	base, ours := read(t, "base.kicad_sch"), read(t, "ours.kicad_sch")
	if out := clean(t, base, ours, ours); out != ours {
		t.Errorf("the merge of one change made twice differs from it")
	}
	// White space alone is no change.
	spaced := strings.Replace(ours, "(at 101.6 101.6 0)", "(at  101.6 101.6 0)", 1)
	if out := clean(t, ours, ours, spaced); out == "" {
		t.Errorf("no output")
	}
}

func moved(text, to string) string {
	return strings.Replace(text, "\t\t(at 101.6 101.6 0)", "\t\t(at "+to+" 0)", 1)
}

// One element moved differently on both sides is a conflict, named by
// its uuid; ours comes back unchanged.
func TestConflict(t *testing.T) {
	ours := read(t, "ours.kicad_sch")
	a, b := moved(ours, "102.87 101.6"), moved(ours, "104.14 101.6")
	if a == ours || b == ours {
		t.Fatal("the test did not move the symbol")
	}
	out, conflicts, err := Merge(ours, a, b)
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 1 || !strings.Contains(conflicts[0], "uuid:4d65726b-0000-4000-8000-0000000000a2") ||
		!strings.Contains(conflicts[0], "changed differently") {
		t.Errorf("conflicts %v; want one, on the symbol's uuid", conflicts)
	}
	if out != a {
		t.Errorf("with a conflict, the result is not ours unchanged")
	}
	// Moved on one side and deleted on the other.
	deleted := strings.Replace(ours, symbolText(t, ours), "", 1)
	_, conflicts, _ = Merge(ours, a, deleted)
	if len(conflicts) != 1 || !strings.Contains(conflicts[0], "changed in ours, deleted in theirs") {
		t.Errorf("changed vs deleted: %v", conflicts)
	}
	_, conflicts, _ = Merge(ours, deleted, a)
	if len(conflicts) != 1 || !strings.Contains(conflicts[0], "deleted in ours, changed in theirs") {
		t.Errorf("deleted vs changed: %v", conflicts)
	}
	// Deleted on one side and unchanged on the other: deleted.
	if out := clean(t, ours, deleted, ours); strings.Contains(out, `(lib_id "Device:R")`) {
		t.Errorf("a symbol deleted on one side is still there")
	}
}

// symbolText is the placed symbol's text, with the line before it.
func symbolText(t *testing.T, text string) string {
	i := strings.Index(text, "\n\t(symbol\n")
	j := strings.Index(text[i+1:], "\n\t)\n")
	if i < 0 || j < 0 {
		t.Fatal("no symbol")
	}
	return text[i : i+1+j+3]
}

// The e2e schematic, as KiCad wrote it: a wire on one side, a junction on
// the other.
func TestRealFile(t *testing.T) {
	base := read(t, "e2e_main.kicad_sch")
	wire := "\t(wire\n\t\t(pts\n\t\t\t(xy 40 40) (xy 50 40)\n\t\t)\n\t\t(stroke\n\t\t\t(width 0)\n\t\t\t(type default)\n\t\t)\n\t\t(uuid \"4d65726b-0000-4000-8000-0000000000c1\")\n\t)\n"
	junction := "\t(junction\n\t\t(at 40 40)\n\t\t(diameter 0)\n\t\t(color 0 0 0 0)\n\t\t(uuid \"4d65726b-0000-4000-8000-0000000000c2\")\n\t)\n"
	at := strings.Index(base, "\t(sheet_instances")
	ours := base[:at] + wire + base[at:]
	theirs := base[:at] + junction + base[at:]
	if out := clean(t, base, base, base); out != base {
		t.Errorf("the real file does not come back byte for byte")
	}
	out := clean(t, base, ours, theirs)
	for _, u := range []string{"0000000000c1", "0000000000c2"} {
		if !strings.Contains(out, u) {
			t.Errorf("the merge lost %s", u)
		}
	}
	c, _ := heads(t, out)
	if c["wire"] != 5 || c["junction"] != 3 {
		t.Errorf("got %d wires and %d junctions; want 5 and 3", c["wire"], c["junction"])
	}
}

// A board's nets have no uuid: they are keyed by number.
func TestBoardNets(t *testing.T) {
	base := "(kicad_pcb\n\t(version 20241229)\n\t(net 0 \"\")\n\t(net 1 \"GND\")\n)\n"
	ours := "(kicad_pcb\n\t(version 20241229)\n\t(net 0 \"\")\n\t(net 1 \"GND\")\n\t(net 2 \"VCC\")\n)\n"
	theirs := "(kicad_pcb\n\t(version 20241229)\n\t(net 0 \"\")\n\t(net 1 \"GND\")\n\t(net 3 \"SDA\")\n)\n"
	out := clean(t, base, ours, theirs)
	if !strings.Contains(out, `(net 2 "VCC")`) || !strings.Contains(out, `(net 3 "SDA")`) {
		t.Errorf("nets lost:\n%s", out)
	}
	// Both sides add net 2 with different names: a conflict.
	other := strings.Replace(ours, "VCC", "3V3", 1)
	if _, c, _ := Merge(base, ours, other); len(c) != 1 || !strings.Contains(c[0], "net|2") {
		t.Errorf("net 2 added differently: %v", c)
	}
}

func TestBadInput(t *testing.T) {
	good := read(t, "base.kicad_sch")
	for _, bad := range []string{"", "(kicad_sch", "(kicad_sch (version 1)", "(kicad_sch \"x\")", "(kicad_sch (a \"unclosed))"} {
		if _, _, err := Merge(good, bad, good); err == nil {
			t.Errorf("Merge with ours %q gave no error", bad)
		}
	}
	// Different kinds of file.
	if _, c, err := Merge(good, "(kicad_pcb\n)", good); err == nil && len(c) == 0 {
		t.Errorf("a board merged into a schematic without a conflict")
	}
}
