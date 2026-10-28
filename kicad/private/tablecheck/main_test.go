// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckURI(t *testing.T) {
	for uri, bad := range map[string]bool{
		"${KIPRJMOD}/../../external/libs/symbols/Device.kicad_sym": false,
		"${KIPRJMOD}/lib/My.pretty":                                false,
		"/usr/share/kicad/symbols/Device.kicad_sym":                true,
		"~/kicad/Device.kicad_sym":                                 true,
		"${KICAD9_SYMBOL_DIR}/Device.kicad_sym":                    true,
		"${KIPRJMOD}/${HOME}/x.kicad_sym":                          true,
		"${KIPRJMOD}/${KIPRJMOD}/x.kicad_sym":                      true,
		"${KIPRJMOD}/$(KIPRJMOD)/x.kicad_sym":                      true,
		"${KIPRJMOD}/${HOME/x.kicad_sym":                           true,
		"${KIPRJMOD}/":                                             true,
		"${KIPRJMOD}//etc/x.kicad_sym":                             true,
		"$(KIPRJMOD)/x.kicad_sym":                                  true,
		"lib/Device.kicad_sym":                                     true,
		"C:/kicad/Device.kicad_sym":                                true,
	} {
		if got := CheckURI(uri) != ""; got != bad {
			t.Errorf("%q: bad = %v, want %v (%s)", uri, got, bad, CheckURI(uri))
		}
	}
}

const good = `(sym_lib_table
  (version 7)
  (lib (name "Device")(type "KiCad")(uri "${KIPRJMOD}/../ext/Device.kicad_sym")(options "")(descr ""))
)
`

func TestCheckTable(t *testing.T) {
	if ps := CheckTable("good", good); len(ps) != 0 {
		t.Errorf("good table: %v", ps)
	}
	if ps := CheckTable("empty", "(fp_lib_table\n  (version 7)\n)\n"); len(ps) != 0 {
		t.Errorf("empty table: %v", ps)
	}
	bad := strings.Replace(good, "${KIPRJMOD}/../ext", "/home/u/kicad", 1)
	if ps := CheckTable("bad", bad); len(ps) != 1 || ps[0].URI != "/home/u/kicad/Device.kicad_sym" {
		t.Errorf("bad table: %v", ps)
	}
	// Whitespace other than a space, and a descr that mentions (lib.
	odd := "(sym_lib_table\n\t(version 7)\n\t(lib\n\t\t(name \"A\")(uri \"/abs/A.kicad_sym\")(descr \"see (lib x)\"))\n)\n"
	if ps := CheckTable("odd", odd); len(ps) != 1 || ps[0].URI != "/abs/A.kicad_sym" {
		t.Errorf("odd table: %v", ps)
	}
	// Escaped quotes and backslashes are read as KiCad reads them.
	esc := `(fp_lib_table (lib (name "a\"b")(uri "${KIPRJMOD}/x\\y.pretty")))`
	if ps := CheckTable("escaped", esc); len(ps) != 0 {
		t.Errorf("escaped table: %v", ps)
	}
	for name, text := range map[string]string{
		"junk":       "hello",
		"unclosed":   "(sym_lib_table (lib (uri \"${KIPRJMOD}/a\")",
		"no uri":     "(sym_lib_table (lib (name \"A\")))",
		"two uris":   "(sym_lib_table (lib (uri \"${KIPRJMOD}/a\")(uri \"${KIPRJMOD}/b\")))",
		"list uri":   "(sym_lib_table (lib (uri (x))))",
		"other kind": "(kicad_sch (version 1))",
		"trailing":   good + ")",
	} {
		if ps := CheckTable(name, text); len(ps) == 0 {
			t.Errorf("%s: no problem found", name)
		}
	}
}

func TestRunExpect(t *testing.T) {
	dir := t.TempDir()
	g, b := filepath.Join(dir, "g"), filepath.Join(dir, "b")
	os.WriteFile(g, []byte(good), 0o644)
	os.WriteFile(b, []byte(strings.Replace(good, "${KIPRJMOD}", "${KICAD9_SYMBOL_DIR}", 1)), 0o644)
	for _, c := range []struct {
		files  []string
		expect string
		code   int
	}{
		{[]string{g}, "clean", 0},
		{[]string{g, b}, "clean", 1},
		{[]string{g}, "host", 1},
		{[]string{g, b}, "host", 0},
		{[]string{filepath.Join(dir, "missing")}, "clean", 2},
		{[]string{g}, "bogus", 2},
	} {
		if _, code := run(c.files, c.expect); code != c.code {
			t.Errorf("%v %s: code %d, want %d", c.files, c.expect, code, c.code)
		}
	}
}
