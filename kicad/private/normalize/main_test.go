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
	for _, bad := range []string{"x", "-1", "1.5"} {
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
