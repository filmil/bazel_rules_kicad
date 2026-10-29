// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"
)

func step(data string) []byte {
	return []byte("ISO-10303-21;\nHEADER;\nFILE_NAME('x.step','2026-10-29T08:00:00',(''),(''),'','','');\nENDSEC;\nDATA;\n" +
		data + "ENDSEC;\nEND-ISO-10303-21;\n")
}

// The same entities, numbered and ordered differently, and wrapped at
// different places, give the same bytes.
func TestCanonicalSTEPOrder(t *testing.T) {
	a := step(`#1 = SHAPE('a',(#2,#3));
#2 = POINT('',(0.,0.,0.));
#3 = LINE('l',#2,
  #4);
#4 = DIRECTION('',(1.,0.,0.));
#5 = STYLE('color',(#6),#3);
#6 = COLOUR('red');
`)
	b := step(`#10 = COLOUR('red');
#11 = DIRECTION('',(1.,0.,0.));
#12 = STYLE('color',(#10),#14);
#13 = POINT('',(0.,0.,0.));
#14 = LINE('l',#13,#11);
#15 = SHAPE('a',(#13,
    #14));
`)
	ca, err := CanonicalSTEP(a)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := CanonicalSTEP(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ca, cb) {
		t.Errorf("not the same:\n%s\n---\n%s", ca, cb)
	}
	// Canonical twice is canonical once.
	if cc, err := CanonicalSTEP(ca); err != nil || !bytes.Equal(cc, ca) {
		t.Errorf("not idempotent: %v\n%s", err, cc)
	}
	// The header and the trailer are kept.
	if !bytes.HasPrefix(ca, a[:bytes.Index(a, []byte("DATA;"))]) || !bytes.HasSuffix(ca, []byte("ENDSEC;\nEND-ISO-10303-21;\n")) {
		t.Errorf("header or trailer changed:\n%s", ca)
	}
}

// Strings are copied as they are, with their spaces, '#', ';' and ”.
func TestCanonicalSTEPStrings(t *testing.T) {
	in := step("#7 = NOTE('a #1; b  ''c''',#8);\n#8 = NOTE('x');\n")
	out, err := CanonicalSTEP(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "#1 = NOTE('a #1; b  ''c''',#2);\n#2 = NOTE('x');\n") {
		t.Errorf("got:\n%s", out)
	}
}

// A complex instance loses the spaces between its records only.
func TestCanonicalSTEPComplexInstance(t *testing.T) {
	in := step("#3 = ( GEOMETRIC_REPRESENTATION_CONTEXT(3)\n  GLOBAL_UNIT_ASSIGNED_CONTEXT((#4)) REPRESENTATION_CONTEXT('Context #1',\n  '3D') );\n#4 = ( LENGTH_UNIT() NAMED_UNIT(*) SI_UNIT(.MILLI.,.METRE.) );\n")
	out, err := CanonicalSTEP(in)
	if err != nil {
		t.Fatal(err)
	}
	want := "#1 = (GEOMETRIC_REPRESENTATION_CONTEXT(3)GLOBAL_UNIT_ASSIGNED_CONTEXT((#2))REPRESENTATION_CONTEXT('Context #1',\n" +
		"  '3D'));\n" +
		"#2 = (LENGTH_UNIT()NAMED_UNIT(*)SI_UNIT(.MILLI.,.METRE.));\n"
	if !strings.Contains(string(out), want) {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestCanonicalSTEPErrors(t *testing.T) {
	for name, in := range map[string][]byte{
		"no DATA":          []byte("ISO-10303-21;\nHEADER;\nENDSEC;\nEND-ISO-10303-21;\n"),
		"undefined":        step("#1 = A(#2);\n"),
		"twice":            step("#1 = A();\n#1 = B();\n"),
		"cycle":            step("#1 = A(#2);\n#2 = B(#1);\n"),
		"no semicolon":     step("#1 = A()\n"),
		"unclosed string":  step("#1 = A('x);\n"),
		"not an instance":  step("FOO;\n"),
		"no ENDSEC":        []byte("ISO-10303-21;\nHEADER;\nENDSEC;\nDATA;\n#1 = A();\nEND-ISO-10303-21;\n"),
		"no =":             step("#1 A();\n"),
		"bad reference":    step("#1 = A(#);\n"),
		"instance #0":      step("#0 = A();\n"),
		"leading zero":     step("#01 = A();\n#1 = B(#01);\n"),
		"unclosed comment": step("#1 = A(/* x);\n"),
	} {
		if _, err := CanonicalSTEP(in); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

// Two real runs of kicad-cli with copper (testdata/a and b, issue #63)
// differ in most lines; normalized, they are the same, and they hold the
// same entities as before.
func TestCopperRunsAgree(t *testing.T) {
	a := read(t, "testdata/a/copper.step")
	b := read(t, "testdata/b/copper.step")
	if bytes.Equal(a, b) {
		t.Fatal("the fixtures are the same: they no longer show issue #63")
	}
	ep := time.Unix(0, 0).UTC()
	na, _, err := Normalize(a, ep)
	if err != nil {
		t.Fatal(err)
	}
	nb, _, err := Normalize(b, ep)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(na, nb) {
		t.Error("the two copper runs normalize to different bytes")
	}
	if n := bytes.Count(na, []byte("\n#")); n < 1000 {
		t.Errorf("%d entities: the model lost its copper", n)
	}
}

// Comments are dropped, wherever they are; one between two words keeps
// them apart.
func TestCanonicalSTEPComments(t *testing.T) {
	a := step("/* lead */\n#1 = A(/* a #9; 'x' */#2,.T./**/.F.);\n#2 = B('/* in a string */');\n")
	b := step("#5 = B('/* in a string */');\n#4 = A(#5,.T. .F.);\n")
	ca, err := CanonicalSTEP(a)
	if err != nil {
		t.Fatal(err)
	}
	cb, err := CanonicalSTEP(b)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ca, cb) || !strings.Contains(string(ca), "#1 = A(#2,.T. .F.);\n#2 = B('/* in a string */');\n") {
		t.Errorf("got:\n%s\n---\n%s", ca, cb)
	}
}

// ENDSEC inside a string does not end the DATA section.
func TestCanonicalSTEPEndsecInString(t *testing.T) {
	out, err := CanonicalSTEP(step("#1 = NOTE('ENDSEC;',#2);\n#2 = NOTE('x');\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "#1 = NOTE('ENDSEC;',#2);\n#2 = NOTE('x');\nENDSEC;\n") {
		t.Errorf("got:\n%s", out)
	}
}

// Long instances are wrapped after a comma at 80 columns, the same way
// whatever the input's wrapping.
func TestCanonicalSTEPWrap(t *testing.T) {
	var refs, defs []string
	for i := 2; i < 40; i++ {
		refs = append(refs, fmt.Sprintf("#%d", i))
		defs = append(defs, fmt.Sprintf("#%d = P(%d.);\n", i, i))
	}
	out, err := CanonicalSTEP(step("#1 = LIST((" + strings.Join(refs, ",") + "));\n" + strings.Join(defs, "")))
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(string(out), "\n") {
		if len(l) > 80 {
			t.Errorf("a line of %d columns: %q", len(l), l)
		}
	}
	if !strings.Contains(string(out), "\n  #") {
		t.Errorf("not wrapped:\n%s", out)
	}
}

// Roots with the same key come out the same whatever their order in the
// input (the tie-break). The two R share S, and each names a P of its own
// with the same content, so both R have the same key.
func TestCanonicalSTEPTiedRoots(t *testing.T) {
	orders := []string{
		"#1 = R(#3,#4);\n#2 = R(#3,#5);\n#3 = S('s');\n#4 = P('p');\n#5 = P('p');\n",
		"#1 = R(#3,#5);\n#2 = R(#3,#4);\n#3 = S('s');\n#4 = P('p');\n#5 = P('p');\n",
		"#7 = P('p');\n#8 = R(#9,#7);\n#9 = S('s');\n#6 = P('p');\n#3 = R(#9,#6);\n",
	}
	// The case is a tie: both roots have the same key.
	ents, order, err := parseEntities([]byte(orders[0]))
	if err != nil {
		t.Fatal(err)
	}
	keys, err := entityKeys(ents)
	if err != nil {
		t.Fatal(err)
	}
	if keys[order[0]] != keys[order[1]] {
		t.Fatal("the two R do not tie: the test does not test the tie-break")
	}
	var first []byte
	for i, o := range orders {
		out, err := CanonicalSTEP(step(o))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = out
		} else if !bytes.Equal(out, first) {
			t.Errorf("order %d:\n%s\nwant:\n%s", i, out, first)
		}
	}
}
