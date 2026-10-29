// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

var epoch0 = time.Unix(0, 0).UTC()

func texts(f []Found) []string {
	var out []string
	for _, x := range f {
		out = append(out, x.Text)
	}
	return out
}

// Each form KiCad writes, as the normalizer leaves it at the epoch.
func TestEpochPasses(t *testing.T) {
	for _, s := range []string{
		"%TF.CreationDate,1970-01-01T00:00:00+00:00*%",
		"G04 Created by KiCad (PCBNEW 9.0.9.1) date 1970-01-01 00:00:00*",
		"; DRILL file {KiCad 9.0.9.1} date 1970-01-01T00:00:00+0000",
		`"CreationDate": "1970-01-01T00:00:00+00:00"`,
		"/CreationDate (D:19700101000000)",
		"/ModDate (D:19700101000000+00'00')",
		"FILE_NAME('board.step','1970-01-01T00:00:00',('Pcbnew'),",
		`    (date "1970-01-01T00:00:00+0000")`,
		"stamp 19700101T000000Z",
		"day 1970/01/01",
	} {
		bad, n := Scan("f", []byte(s), epoch0, nil)
		if n != 1 || len(bad) != 0 {
			t.Errorf("%q: %d dates, bad %v; want 1 date at the epoch", s, n, texts(bad))
		}
	}
}

func TestOtherDatesFail(t *testing.T) {
	for _, s := range []string{
		"%TF.CreationDate,2026-10-28T23:39:14+01:00*%",
		"G04 Created by KiCad (PCBNEW 9.0.9.1) date 2026-10-28 23:39:14*",
		"/CreationDate (D:20261029083154)",
		"(date \"2026-10-29\")",
		"1970-01-01T00:00:01Z",      // one second off
		"1970-01-01T00:00:00+01:00", // the epoch's wall time, in another zone
		"D:19700101000000+01'00'",
		"19700102T000000",
		"2026/10/29 08:31",
	} {
		bad, _ := Scan("f", []byte(s), epoch0, nil)
		if len(bad) != 1 {
			t.Errorf("%q: bad %v, want one", s, texts(bad))
		}
	}
}

// Numbers that only look like dates are not dates.
func TestNotDates(t *testing.T) {
	for _, s := range []string{
		"0000042762 00000 n \n0000042679 00000 n", // PDF cross-reference
		"%TF.ProjectId,main,6d61696e-2e6b-6963-6164-5f7063620000,1*%",
		"X120260102Y34*",        // Gerber coordinates
		"12026-10-29",           // a digit before
		"2026-10-291",           // a digit after
		"2026-13-01 2026-00-10", // no such month
		"2026-10-32",            // no such day
	} {
		if bad, n := Scan("f", []byte(s), epoch0, nil); n != 0 {
			t.Errorf("%q: %d dates %v, want none", s, n, texts(bad))
		}
	}
}

func TestLineNumber(t *testing.T) {
	bad, _ := Scan("f.gtl", []byte("a\nb\nc 2026-10-28T23:39:14+01:00\n"), epoch0, nil)
	if len(bad) != 1 || bad[0].String() != "f.gtl:3: 2026-10-28T23:39:14+01:00" {
		t.Errorf("got %v", bad)
	}
}

// A date the designer wrote in a source passes; the same date with a time,
// which only a tool writes, does not.
func TestAllowedFromSources(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "main.kicad_sch")
	if err := os.WriteFile(src, []byte("(title_block\n\t\t(date \"2026-10-29\")\n)"), 0o644); err != nil {
		t.Fatal(err)
	}
	allowed, err := Allowed([]string{src})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(allowed, map[string]bool{"2026-10-29": true}) {
		t.Fatalf("allowed %v", allowed)
	}
	bad, n := Scan("net", []byte(`(date "2026-10-29") (date "2026-10-29T08:32:01+0000")`), epoch0, allowed)
	if n != 2 || !reflect.DeepEqual(texts(bad), []string{"2026-10-29T08:32:01+0000"}) {
		t.Errorf("%d dates, bad %v", n, texts(bad))
	}
}

func TestEpochFlag(t *testing.T) {
	day := time.Unix(86400, 0).UTC()
	if bad, _ := Scan("f", []byte("1970-01-02T00:00:00Z D:19700102000000"), day, nil); len(bad) != 0 {
		t.Errorf("bad %v at epoch 86400", texts(bad))
	}
	if bad, _ := Scan("f", []byte("1970-01-01T00:00:00Z"), day, nil); len(bad) != 1 {
		t.Errorf("epoch 0 passed at epoch 86400")
	}
}

// A directory (a tree artifact) is scanned whole; names are relative to
// the root.
func TestRunDirectory(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"out/gerbers/a.gtl": "date 1970-01-01 00:00:00*",
		"out/gerbers/b.gbl": "date 2026-10-28 23:39:14*",
		"out/report.json":   `"date": "1970-01-01T00:00:00+0000"`,
	} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r, err := Run(root, []string{"out/gerbers", "out/report.json"}, epoch0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Files != 3 || r.Dates != 3 || len(r.Bad) != 1 || r.Bad[0].String() != "out/gerbers/b.gbl:1: 2026-10-28 23:39:14" {
		t.Errorf("result %+v", r)
	}
}
