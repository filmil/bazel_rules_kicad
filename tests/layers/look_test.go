// SPDX-License-Identifier: Apache-2.0

// Package look_test checks real kicad_layers outputs.
package look_test

import (
	"bytes"
	"flag"
	"image/png"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	svgPath  = flag.String("svg", "", "a kicad_layers SVG, as a runfiles path")
	pngPaths = flag.String("png", "", "a kicad_layers target's outputs (SVG and PNG), as runfiles paths")
)

func read(t *testing.T, rpath string) []byte {
	t.Helper()
	p, err := runfiles.Rlocation(rpath)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The plot's date is the epoch, and no other date is left.
func TestSVGHasNoDate(t *testing.T) {
	b := read(t, *svgPath)
	if !bytes.Contains(b, []byte("date 1970/01/01 00:00:00")) {
		t.Errorf("the <title> does not hold the epoch: %.300s", b)
	}
	if m := regexp.MustCompile(`\d{4}/\d\d/\d\d \d\d:\d\d:\d\d`).FindAll(b, -1); len(m) != 1 {
		t.Errorf("%d dates, want 1 (the epoch): %q", len(m), m)
	}
}

// The board of //tests/design is 20 mm by 12 mm; the plot is cut to it,
// so at 300 dpi the PNG is about 236 by 142 pixels.
func TestPNGIsTheBoardsSize(t *testing.T) {
	var pngPath string
	for _, p := range strings.Fields(*pngPaths) {
		if strings.HasSuffix(p, ".png") {
			pngPath = p
		}
	}
	if pngPath == "" {
		t.Fatalf("no PNG in %q", *pngPaths)
	}
	img, err := png.Decode(bytes.NewReader(read(t, pngPath)))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() < 230 || b.Dx() > 250 || b.Dy() < 136 || b.Dy() > 156 {
		t.Errorf("size %dx%d, want about 236x142 (20x12 mm at 300 dpi)", b.Dx(), b.Dy())
	}
}
