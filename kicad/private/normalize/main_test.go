// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testdata/a and testdata/b are the same exports of the same board and
// schematic (testdata/source), made by KiCad 9.0.9 about 30 s apart on a
// host in CET, so every time stamp differs.
var formats = map[string]Format{
	"fixture-F_Cu.gtl":    Gerber,
	"fixture-drl_map.gbr": Gerber,
	"fixture.drl":         Drill,
	"fixture-job.gbrjob":  Job,
	"board.pdf":           PDF,
	"sch.pdf":             PDF,
	"board.step":          STEP,
	"drc.json":            Report,
	"erc.json":            Report,
	"e2e.net":             Netlist,
}

func read(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFixturesNormalizeToTheSameBytes(t *testing.T) {
	epoch := time.Unix(0, 0).UTC()
	for name, want := range formats {
		a, b := read(t, filepath.Join("testdata/a", name)), read(t, filepath.Join("testdata/b", name))
		if bytes.Equal(a, b) {
			t.Fatalf("%s: the fixtures are identical; they must differ in their time stamps", name)
		}
		na, fa, err := Normalize(a, epoch)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		nb, _, err := Normalize(b, epoch)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if fa != want {
			t.Errorf("%s: detected %s, want %s", name, fa, want)
		}
		if !bytes.Equal(na, nb) {
			t.Errorf("%s: the two runs still differ after normalizing", name)
		}
		// No trace of the day the fixtures were made.
		for _, day := range []string{"2026-10-28", "20261028"} {
			if bytes.Contains(na, []byte(day)) {
				t.Errorf("%s: still contains %s", name, day)
			}
		}
		if want == PDF && len(na) != len(a) {
			t.Errorf("%s: length %d, want %d", name, len(na), len(a))
		}
	}
}

func TestEpochValueInEachFormat(t *testing.T) {
	day2 := time.Unix(86400+3661, 0).UTC() // 1970-01-02 01:01:01
	for name, want := range map[string][]string{
		"fixture-F_Cu.gtl":   {"%TF.CreationDate,1970-01-02T01:01:01+00:00*%", "date 1970-01-02 01:01:01*"},
		"fixture.drl":        {"; DRILL file {KiCad 9.0.9.1} date 1970-01-02T01:01:01+0000\n", "; #@! TF.CreationDate,1970-01-02T01:01:01+00:00\n"},
		"fixture-job.gbrjob": {`"CreationDate": "1970-01-02T01:01:01+00:00"`},
		"board.pdf":          {"/CreationDate (D:19700102010101)"},
		"board.step":         {"FILE_NAME('board.step','1970-01-02T01:01:01',"},
		"drc.json":           {`"date": "1970-01-02T01:01:01+0000",`},
		"erc.json":           {`"date": "1970-01-02T01:01:01+0000",`},
		"sch.pdf":            {"/CreationDate (D:19700102010101)"},
		"fixture-drl_map.gbr": {"%TF.CreationDate,1970-01-02T01:01:01+00:00*%"},
	} {
		n, _, err := Normalize(read(t, filepath.Join("testdata/a", name)), day2)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, w := range want {
			if !bytes.Contains(n, []byte(w)) {
				t.Errorf("%s: no %q", name, w)
			}
		}
	}
}

func TestPDFDateKeepsItsLength(t *testing.T) {
	ep := time.Unix(0, 0).UTC()
	for _, old := range []string{"D:20261028233917", "D:20261028233917Z", "D:20261028233917+01'00'", "D:20261028233917-05'30"} {
		n, err := pdfDate(ep, []byte(old))
		if err != nil || len(n) != len(old) {
			t.Errorf("%q: got %q, %v", old, n, err)
		}
	}
	if _, err := pdfDate(ep, []byte("D:2026")); err == nil {
		t.Error("a short PDF date must be an error")
	}
}

func TestUnknownStampFormsFail(t *testing.T) {
	ep := time.Unix(0, 0).UTC()
	drl := string(read(t, "testdata/a/fixture.drl"))
	for name, text := range map[string]string{
		"drill date in words": strings.Replace(drl, "2026-10-28T23:39:16+0100", "yesterday", 1),
		"drill without date":  strings.Replace(drl, "; DRILL file", "; drill file", 1),
		"step without stamp":  "ISO-10303-21;\nHEADER;\nENDSEC;\n",
		"report without date": "{\n    \"$schema\": \"https://schemas.kicad.org/drc.v1.json\",\n    \"source\": \"x\"\n}\n",
		"job without date":    "{\n  \"Header\": {\n    \"GenerationSoftware\": {}\n  }\n}\n",
		"gerber without date": "%FSLAX46Y46*%\nG04 Gerber Fmt 4.6*\n%MOMM*%\nM02*\n",
		"gerber date in words": "%TF.CreationDate,yesterday*%\n%FSLAX46Y46*%\nM02*\n",
		"pdf without date":    "%PDF-1.5\n1 0 obj\n<< /Producer (KiCad PDF) >>\nendobj\n%%EOF\n",
		"pdf date in words":   "%PDF-1.5\n1 0 obj\n<< /CreationDate (yesterday) >>\nendobj\n%%EOF\n",
	} {
		if _, _, err := Normalize([]byte(text), ep); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}

func TestUnknownFormatPassesThrough(t *testing.T) {
	in := []byte("Ref,Val,Package\nR1,10k,R_0603\n")
	out, f, err := Normalize(in, time.Unix(0, 0).UTC())
	if err != nil || f != Unknown || !bytes.Equal(in, out) {
		t.Errorf("got %q %s %v", out, f, err)
	}
}

func TestEpoch(t *testing.T) {
	for v, want := range map[string]int64{"": 0, "0": 0, "1700000000": 1700000000} {
		got, err := Epoch(v)
		if err != nil || got.Unix() != want || got.Location() != time.UTC {
			t.Errorf("%q: %v %v", v, got, err)
		}
	}
	for _, bad := range []string{"x", "-1", "1.5", "253402300800"} {
		if _, err := Epoch(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

// A directory (a tree artifact) is normalized file by file, sub-directories
// included, and its other files are copied.
func TestRunDirectory(t *testing.T) {
	in, out := t.TempDir(), filepath.Join(t.TempDir(), "out")
	os.MkdirAll(filepath.Join(in, "sub"), 0o755)
	os.WriteFile(filepath.Join(in, "sub", "F_Cu.gtl"), read(t, "testdata/a/fixture-F_Cu.gtl"), 0o644)
	os.WriteFile(filepath.Join(in, "notes.txt"), []byte("hello\n"), 0o644)
	if err := Run(in, out, time.Unix(0, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if b := read(t, filepath.Join(out, "sub", "F_Cu.gtl")); bytes.Contains(b, []byte("2026")) {
		t.Error("the Gerber file in the tree kept its date")
	}
	if b := read(t, filepath.Join(out, "notes.txt")); string(b) != "hello\n" {
		t.Errorf("the other file became %q", b)
	}
	// A file in a known format with a bad stamp fails the whole run.
	os.WriteFile(filepath.Join(in, "bad.drl"), []byte("M48\n; DRILL file {KiCad} date soon\n"), 0o644)
	if err := Run(in, filepath.Join(t.TempDir(), "o2"), time.Unix(0, 0).UTC()); err == nil {
		t.Error("a bad stamp in the tree did not fail the run")
	}
}

// The X2 attribute in a G04 comment, which the fixtures do not use (KiCad
// writes it so when Gerber X2 is off but its attributes are kept as
// comments).
func TestGerberX2InComment(t *testing.T) {
	in := "G04 #@! TF.CreationDate,2026-10-28T23:39:14+01:00*\n%FSLAX46Y46*%\nM02*\n"
	n, f, err := Normalize([]byte(in), time.Unix(0, 0).UTC())
	if err != nil || f != Gerber || !bytes.Contains(n, []byte("G04 #@! TF.CreationDate,1970-01-01T00:00:00+00:00*")) {
		t.Errorf("got %q %s %v", n, f, err)
	}
}

// A byte order mark or blank lines before the content do not hide a format.
func TestDetectSkipsBOMAndSpace(t *testing.T) {
	drl := read(t, "testdata/a/fixture.drl")
	for _, prefix := range []string{"\xef\xbb\xbf", "\n\n", "  \r\n"} {
		if f := Detect(append([]byte(prefix), drl...)); f != Drill {
			t.Errorf("%q: detected %s", prefix, f)
		}
	}
}

// A key named date inside a nested object is not the report's date.
func TestReportNestedDateUntouched(t *testing.T) {
	in := "{\n    \"$schema\": \"https://schemas.kicad.org/drc.v1.json\",\n    \"date\": \"2026-10-28T23:39:20+0100\",\n    \"items\": [\n        {\n            \"date\": \"keep\"\n        }\n    ]\n}\n"
	n, _, err := Normalize([]byte(in), time.Unix(0, 0).UTC())
	if err != nil || !bytes.Contains(n, []byte(`"date": "keep"`)) || bytes.Contains(n, []byte("2026")) {
		t.Errorf("got %q %v", n, err)
	}
}

// The mode of a file survives: an executable stays executable.
func TestRunKeepsMode(t *testing.T) {
	dir := t.TempDir()
	in, out := filepath.Join(dir, "tool"), filepath.Join(dir, "out", "tool")
	os.WriteFile(in, []byte("#!/x\n"), 0o755)
	if err := Run(in, out, time.Unix(0, 0).UTC()); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(out); err != nil || fi.Mode().Perm()&0o111 == 0 {
		t.Errorf("mode %v, %v", fi.Mode(), err)
	}
}

// The netlist fixtures are two exports of the e2e schematic, their paths
// rewritten to two made-up action directories: the design's source and the
// libraries' paths name the directory, which differs per configuration.
func TestNetlist(t *testing.T) {
	n, f, err := Normalize(read(t, "testdata/a/e2e.net"), time.Unix(86400, 0).UTC())
	if err != nil || f != Netlist {
		t.Fatalf("%s %v", f, err)
	}
	for _, want := range []string{
		`(source "main.kicad_sch")
    (date "1970-01-02T00:00:00+0000")`,
		`(uri "Device.kicad_sym")`,
		`(uri "Connector_Generic.kicad_sym")`,
		// The title block's date is the designer's, and stays.
		`(date "2026-10-29")`,
	} {
		if !bytes.Contains(n, []byte(want)) {
			t.Errorf("no %q", want)
		}
	}
	if bytes.Contains(n, []byte("/sandbox/")) {
		t.Error("a path to the action directory is left")
	}
	if _, _, err := Normalize([]byte("(export (version \"E\")\n  (components))\n"), time.Unix(0, 0)); err == nil {
		t.Error("a netlist without a design header must be an error")
	}
	bad := "(export (version \"E\")\n  (design\n    (source \"/a/main.kicad_sch\")\n    (date \"yesterday\")))\n"
	if _, _, err := Normalize([]byte(bad), time.Unix(0, 0)); err == nil {
		t.Error("a design date of an unknown form must be an error")
	}
	// Text that only looks like a library path stays, and an escaped quote
	// in a path does not end it early.
	odd := `(export (version "E")
  (design
    (source "/a/b \"q\"/main.kicad_sch")
    (date "2026-10-29T02:37:56+0000"))
  (components (comp (ref "R1") (description "see (uri \"x/y\")")))
  (libraries (library (logical "Device")
      (uri "/a/libs/Device.kicad_sym"))))
`
	n2, _, err := Normalize([]byte(odd), time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`(source "main.kicad_sch")`, `see (uri \"x/y\")`, `(uri "Device.kicad_sym")`} {
		if !bytes.Contains(n2, []byte(want)) {
			t.Errorf("no %q in:\n%s", want, n2)
		}
	}
}
