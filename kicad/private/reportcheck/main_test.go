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
// footprint with no symbol); unconnected_drc.json of the e2e board with
// one track removed (one unconnected item).
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
		"unconnected_drc.json": {"DRC", 1},
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
	_, uvs, _ := Parse(read(t, "unconnected_drc.json"))
	if len(uvs) != 1 || uvs[0].Kind != "unconnected" || uvs[0].Severity != "error" || uvs[0].Type != "unconnected_items" {
		t.Errorf("unconnected_drc.json: %+v", uvs)
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
		// Known bad: passes only with violations of the expected type and
		// of no other.
		{"unconnected_drc.json", "unconnected_items", 0, "found only unconnected_items violations, as expected"},
		{"bad_erc.json", "footprint_link_issues", 1, "also found"},
		{"bad_drc.json", "extra_footprint", 1, "also found"},
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

// Entries the design marks as excluded do not count.
func TestExclusionsDoNotCount(t *testing.T) {
	r := `{"$schema": "https://schemas.kicad.org/drc.v1.json", "violations": [
		{"severity": "exclusion", "type": "clearance", "description": "x", "items": []},
		{"severity": "ignore", "type": "clearance", "description": "x", "items": []}]}`
	if msg, code := Check([]byte(r), ""); code != 0 {
		t.Errorf("code %d: %s", code, msg)
	}
}

// Run reads the report by its runfiles path, keeps a copy, and checks it.
func TestRun(t *testing.T) {
	src, out := t.TempDir(), t.TempDir()
	os.MkdirAll(src+"/_main/pkg", 0o755)
	os.WriteFile(src+"/_main/pkg/erc.json", read(t, "clean_erc.json"), 0o644)
	if msg, code := Run(src, "_main/pkg/erc.json", out, ""); code != 0 {
		t.Errorf("clean: %d %s", code, msg)
	}
	if b, err := os.ReadFile(out + "/erc.json"); err != nil || len(b) == 0 {
		t.Errorf("no copy in the outputs directory: %v", err)
	}
	if _, code := Run(src, "_main/pkg/missing.json", "", ""); code != 2 {
		t.Error("a missing report must exit 2")
	}
	if _, code := Run("", "x", "", ""); code != 2 {
		t.Error("no TEST_SRCDIR must exit 2")
	}
}

// testdata/canary_*.json are real DRC reports of the canary board, run with
// a rule file and the canary appended (kicad/private/drucanary): loaded
// with a good file, dropped with a file missing a closing parenthesis.
func TestCanary(t *testing.T) {
	if ok, err := CanaryFound(read(t, "canary_loaded.json")); !ok || err != nil {
		t.Errorf("loaded: %v %v", ok, err)
	}
	if ok, err := CanaryFound(read(t, "canary_dropped.json")); ok || err != nil {
		t.Errorf("dropped: %v %v", ok, err)
	}
	if _, err := CanaryFound(read(t, "clean_erc.json")); err == nil {
		t.Error("an ERC report as the canary must be an error")
	}
	clean := read(t, "clean_drc.json")
	for _, c := range []struct {
		canary []byte
		expect string
		code   int
		says   string
	}{
		{nil, "", 0, "no violations"},
		{read(t, "canary_loaded.json"), "", 0, "no violations"},
		{read(t, "canary_dropped.json"), "", 1, "design_rules_not_loaded"},
		{read(t, "canary_dropped.json"), "design_rules_not_loaded", 0, "as expected"},
		{read(t, "canary_loaded.json"), "design_rules_not_loaded", 1, "expected a design_rules_not_loaded violation"},
	} {
		msg, code := Check(clean, c.expect, c.canary)
		if code != c.code || !strings.Contains(msg, c.says) {
			t.Errorf("expect %q: code %d, want %d:\n%s", c.expect, code, c.code, msg)
		}
	}
}
