// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// testdata: root.kicad_sch places power.kicad_sch and sub/io.kicad_sch;
// sub/io.kicad_sch places ../leaf.kicad_sch; extra.kicad_sch is in nobody.
func TestSheetfiles(t *testing.T) {
	b, err := os.ReadFile("testdata/root.kicad_sch")
	if err != nil {
		t.Fatal(err)
	}
	got, err := Sheetfiles(string(b))
	if err != nil || !reflect.DeepEqual(got, []string{"power.kicad_sch", "sub/io.kicad_sch"}) {
		t.Errorf("got %q, %v", got, err)
	}
	for _, bad := range []string{"(kicad_pcb)", "(kicad_sch (sheet (property \"Sheetname\" \"x\")))", "(kicad_sch"} {
		if _, err := Sheetfiles(bad); err == nil {
			t.Errorf("%q: no error", bad)
		}
	}
}

func readFile(p string) (string, error) {
	b, err := os.ReadFile(p)
	return string(b), err
}

func TestProblems(t *testing.T) {
	d := "testdata"
	all := []string{d + "/power.kicad_sch", d + "/sub/io.kicad_sch", d + "/leaf.kicad_sch"}
	for name, c := range map[string]struct {
		given []string
		want  []string
	}{
		"all there":       {all, nil},
		"power missing":   {all[1:], []string{`sheet "power.kicad_sch", which root.kicad_sch references, is not in srcs`}},
		"nested missing":  {all[:2], []string{`sheet "../leaf.kicad_sch", which io.kicad_sch references, is not in srcs`}},
		"one extra":       {append(append([]string{}, all...), d+"/extra.kicad_sch"), []string{`"extra.kicad_sch" is in srcs, but no sheet references it`}},
		"missing a level": {[]string{d + "/power.kicad_sch", d + "/leaf.kicad_sch"}, []string{`sheet "sub/io.kicad_sch", which root.kicad_sch references, is not in srcs`, `"leaf.kicad_sch" is in srcs, but no sheet references it`}},
	} {
		got, err := Problems(d+"/root.kicad_sch", c.given, readFile)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s:\n got %q\nwant %q", name, got, c.want)
		}
	}
}

func TestRun(t *testing.T) {
	d := "testdata"
	stamp := filepath.Join(t.TempDir(), "ok")
	if msg, code := run([]string{"--stamp", stamp, d + "/root.kicad_sch", d + "/power.kicad_sch", d + "/sub/io.kicad_sch", d + "/leaf.kicad_sch"}, ""); code != 0 {
		t.Errorf("clean: %d %s", code, msg)
	}
	if _, err := os.Stat(stamp); err != nil {
		t.Errorf("no stamp: %v", err)
	}
	missing := []string{d + "/root.kicad_sch", d + "/sub/io.kicad_sch", d + "/leaf.kicad_sch"}
	if msg, code := run(missing, ""); code != 1 || !strings.Contains(msg, `"power.kicad_sch"`) {
		t.Errorf("missing: %d %s", code, msg)
	}
	if _, code := run(missing, "missing:power.kicad_sch"); code != 0 {
		t.Error("expect missing:power.kicad_sch: the check did not find it")
	}
	if _, code := run([]string{d + "/root.kicad_sch", d + "/power.kicad_sch", d + "/sub/io.kicad_sch", d + "/leaf.kicad_sch"}, "missing:power.kicad_sch"); code != 1 {
		t.Error("expect missing on a complete schematic must fail")
	}
	if _, code := run([]string{d + "/nope.kicad_sch"}, ""); code != 2 {
		t.Error("an unreadable root must exit 2")
	}
}
