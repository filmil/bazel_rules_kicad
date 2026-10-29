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
		"nested missing":  {all[:2], []string{`sheet "leaf.kicad_sch", which sub/io.kicad_sch references, is not in srcs`}},
		"one extra":       {append(append([]string{}, all...), d+"/extra.kicad_sch"), []string{`sheet "extra.kicad_sch" is in srcs, but no sheet references it`}},
		"missing a level": {[]string{d + "/power.kicad_sch", d + "/leaf.kicad_sch"}, []string{`sheet "sub/io.kicad_sch", which root.kicad_sch references, is not in srcs`, `sheet "leaf.kicad_sch" is in srcs, but no sheet references it`}},
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
	if _, code := run(missing, "unused:power.kicad_sch"); code != 1 {
		t.Error("expect unused for a missing sheet must fail")
	}
	if _, code := run(missing, "missing"); code != 2 {
		t.Error("an expectation without a colon must exit 2")
	}
	if _, code := run(missing, "bogus:power.kicad_sch"); code != 2 {
		t.Error("an unknown expectation must exit 2")
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

// A sheet that references its own parent makes a circular hierarchy.
func TestCircular(t *testing.T) {
	files := map[string]string{
		"r/root.kicad_sch":  "(kicad_sch (sheet (property \"Sheetfile\" \"a.kicad_sch\")))",
		"r/a.kicad_sch":     "(kicad_sch (sheet (property \"Sheetfile\" \"sub/b.kicad_sch\")))",
		"r/sub/b.kicad_sch": "(kicad_sch (sheet (property \"Sheetfile\" \"../a.kicad_sch\")))",
	}
	read := func(p string) (string, error) { return files[filepath.ToSlash(p)], nil }
	got, err := Problems("r/root.kicad_sch", []string{"r/a.kicad_sch", "r/sub/b.kicad_sch"}, read)
	want := []string{`sheet "a.kicad_sch", which sub/b.kicad_sch references, is one of its own parents: the hierarchy is circular`}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, %v", got, err)
	}
	// A sheet used twice, not in a cycle, is fine.
	files["r/root.kicad_sch"] = "(kicad_sch (sheet (property \"Sheetfile\" \"sub/b.kicad_sch\")) (sheet (property \"Sheetfile\" \"sub/b.kicad_sch\")))"
	files["r/sub/b.kicad_sch"] = "(kicad_sch)"
	if got, err := Problems("r/root.kicad_sch", []string{"r/sub/b.kicad_sch"}, read); err != nil || len(got) != 0 {
		t.Errorf("reused sheet: %q, %v", got, err)
	}
}

// Cycles are found however the walk first reaches their sheets: a sheet is
// marked seen only when the depth-first walk enters it, so a seen sheet is
// either on the current chain or fully explored.
func TestCircularAfterSharedSheet(t *testing.T) {
	sch := func(refs ...string) string {
		s := "(kicad_sch"
		for _, r := range refs {
			s += " (sheet (property \"Sheetfile\" \"" + r + "\"))"
		}
		return s + ")"
	}
	for name, c := range map[string]struct {
		files map[string]string
		want  string
	}{
		// root -> B -> C, root -> A -> B, C -> A: the cycle A -> B -> C -> A.
		"through a shared sheet": {map[string]string{
			"r/root.kicad_sch": sch("B.kicad_sch", "A.kicad_sch"),
			"r/A.kicad_sch":    sch("B.kicad_sch"),
			"r/B.kicad_sch":    sch("C.kicad_sch"),
			"r/C.kicad_sch":    sch("A.kicad_sch"),
		}, "is one of its own parents"},
		"a sheet that places itself": {map[string]string{
			"r/root.kicad_sch": sch("A.kicad_sch"),
			"r/A.kicad_sch":    sch("A.kicad_sch"),
		}, `sheet "A.kicad_sch", which A.kicad_sch references, is one of its own parents`},
		"the root placed again": {map[string]string{
			"r/root.kicad_sch": sch("A.kicad_sch"),
			"r/A.kicad_sch":    sch("root.kicad_sch"),
		}, `sheet "root.kicad_sch", which A.kicad_sch references, is one of its own parents`},
	} {
		read := func(p string) (string, error) { return c.files[filepath.ToSlash(p)], nil }
		var given []string
		for f := range c.files {
			if f != "r/root.kicad_sch" {
				given = append(given, f)
			}
		}
		got, err := Problems("r/root.kicad_sch", given, read)
		if err != nil || len(got) != 1 || !strings.Contains(got[0], c.want) {
			t.Errorf("%s: got %q, %v", name, got, err)
		}
	}
}
