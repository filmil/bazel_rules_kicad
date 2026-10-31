// SPDX-License-Identifier: Apache-2.0

package sexpr

import "testing"

func TestParse(t *testing.T) {
	n, err := Parse("\n(kicad_pcb (footprint \"R:R\" (property \"Reference\" \"R\\\"1\") hide) (x))\n")
	if err != nil {
		t.Fatal(err)
	}
	if n.Head() != "kicad_pcb" || len(n.List) != 3 {
		t.Fatalf("root %q with %d items", n.Head(), len(n.List))
	}
	fp := n.List[1]
	if fp.Head() != "footprint" || fp.Arg(0) != "R:R" || fp.Arg(1) != "hide" || fp.Arg(2) != "" {
		t.Errorf("footprint %q args %q %q %q", fp.Head(), fp.Arg(0), fp.Arg(1), fp.Arg(2))
	}
	if p := fp.List[2]; p.Arg(1) != `R"1` {
		t.Errorf("escaped quote: %q", p.Arg(1))
	}
	if (&Node{Atom: "x"}).Head() != "" || (&Node{Atom: "x"}).Arg(0) != "" {
		t.Error("an atom has a head or args")
	}
}

func TestParseErrors(t *testing.T) {
	for _, s := range []string{"", "(a", "(a))", "(a) (b)", `(a "b`, ")", "(a \"b\\"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q): no error", s)
		}
	}
}
