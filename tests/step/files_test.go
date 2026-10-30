// SPDX-License-Identifier: Apache-2.0

// Package files_test checks the STEP models kicad_step writes for the board
// of //tests/design.
package files_test

import (
	"flag"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	step   = flag.String("step", "", "the output of :step, as a runfiles path")
	copper = flag.String("copper", "", "the output of :copper, as a runfiles path")
)

func read(t *testing.T, rpath string) string {
	t.Helper()
	p, err := runfiles.Rlocation(rpath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var productRe = regexp.MustCompile(`PRODUCT\('([^']*)'`)

// products lists the product names of a model but the first, which is the
// file's own, named after the output.
func products(s string) []string {
	var names []string
	for _, m := range productRe.FindAllStringSubmatch(s, -1) {
		names = append(names, m[1])
	}
	sort.Strings(names)
	var out []string
	for _, n := range names {
		if !strings.HasSuffix(n, " 1") {
			out = append(out, n)
		}
	}
	return out
}

// A STEP file whose header time is the epoch
// (--@hdlf_rules_kicad//kicad:source_date_epoch defaults to 0).
func checkHeader(t *testing.T, name, s string) {
	t.Helper()
	if !strings.HasPrefix(s, "ISO-10303-21;") {
		t.Errorf("%s is not a STEP file", name)
	}
	if !regexp.MustCompile(`FILE_NAME\('[^']*','1970-01-01T00:00:00'`).MatchString(s) {
		t.Errorf("%s: the FILE_NAME time is not at the epoch", name)
	}
	if !strings.Contains(s, "MANIFOLD_SOLID_BREP") {
		t.Errorf("%s holds no solid", name)
	}
}

// The board body only: no copper unless asked for, no parts (issue #43).
func TestBoardBody(t *testing.T) {
	s := read(t, *step)
	checkHeader(t, "step.step", s)
	if got := strings.Join(products(s), " "); got != "main_PCB" {
		t.Errorf("products %q, want main_PCB", got)
	}
}

// copper = True adds the tracks, the pads and the vias (issue #63). The
// board has no zone, so the zones are checked only by their flag, in
// copper_action_test.
func TestCopper(t *testing.T) {
	s := read(t, *copper)
	checkHeader(t, "copper.step", s)
	want := "main_PCB main_copper main_pad main_via"
	if got := strings.Join(products(s), " "); got != want {
		t.Errorf("products %q, want %q", got, want)
	}
}
