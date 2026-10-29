// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"strings"
	"testing"
)

// testdata holds real KiCad 9.0.9 reports: clean_* of the e2e design;
// bad_erc.json of its sheets with no project file (so no library tables:
// 9 lib_symbol_issues and 3 footprint_link_issues warnings); bad_drc.json
// of a test board (a dangling via, a footprint library warning, and a
// footprint with no symbol).
func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParse(t *testing.T) {
	for name, want := range map[string]struct {
		kind  string
		count int
	}{
		"clean_erc.json": {"ERC", 0},
		"clean_drc.json": {"DRC", 0},
		"bad_erc.json":   {"ERC", 12},
		"bad_drc.json":   {"DRC", 3},
	} {
		kind, vs, err := Parse(read(t, name))
		if err != nil || kind != want.kind || len(vs) != want.count {
			t.Errorf("%s: %s, %d violations, %v; want %s, %d", name, kind, len(vs), err, want.kind, want.count)
		}
	}
	_, vs, _ := Parse(read(t, "bad_drc.json"))
	kinds := map[string]bool{}
	for _, v := range vs {
		kinds[v.Kind+" "+v.Type] = true
	}
	for _, k := range []string{"violation via_dangling", "violation lib_footprint_issues", "parity extra_footprint"} {
		if !kinds[k] {
			t.Errorf("bad_drc.json: no %s; have %v", k, kinds)
		}
	}
}

func TestCheck(t *testing.T) {
	for _, c := range []struct {
		file, expect string
		code         int
		says         string
	}{
		{"clean_erc.json", "", 0, "ERC: no violations"},
		{"clean_drc.json", "", 0, "DRC: no violations"},
		{"bad_erc.json", "", 1, "9  warning lib_symbol_issues"},
		{"bad_drc.json", "", 1, "the design must have no violations"},
		// Known bad: passes only with a violation of the expected type.
		{"bad_erc.json", "footprint_link_issues", 0, "as expected"},
		{"bad_drc.json", "extra_footprint", 0, "as expected"},
		{"bad_drc.json", "clearance", 1, "expected a clearance violation"},
		{"clean_erc.json", "pin_not_connected", 1, "expected a pin_not_connected violation"},
	} {
		msg, code := Check(read(t, c.file), c.expect)
		if code != c.code || !strings.Contains(msg, c.says) {
			t.Errorf("%s expect %q: code %d, want %d; message:\n%s", c.file, c.expect, code, c.code, msg)
		}
	}
	for _, bad := range []string{"{", `{"$schema": "https://example.com/x.json"}`} {
		if _, code := Check([]byte(bad), ""); code != 2 {
			t.Errorf("%q: want exit 2", bad)
		}
	}
}

// The summary names each violation's sheet and items, so a failing test
// says what to fix without opening the report.
func TestSummaryDetail(t *testing.T) {
	_, vs, _ := Parse(read(t, "bad_erc.json"))
	s := Summary("ERC", vs)
	for _, want := range []string{"(sheet /)", "- Symbol R1 [R]"} {
		if !strings.Contains(s, want) {
			t.Errorf("summary lacks %q:\n%s", want, s)
		}
	}
}
