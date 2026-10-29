// SPDX-License-Identifier: Apache-2.0

// Package look_test checks the pixels of a real kicad_sch_render output.
package look_test

import (
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

var dir = flag.String("dir", "", "the kicad_sch_render output directory, as a runfiles path")

// The root sheet of //tests/design is A4: at 150 dpi, 1754 by 1240 pixels.
// Its drawing-sheet frame starts 10 mm from the left edge (pixel 59), drawn
// 0.15 mm wide like a wire: about one pixel, and dark. Had svgpng not scaled
// line widths, it would come out 0.15 pixel wide, near the background color.
func TestFrameIsDrawn(t *testing.T) {
	f, err := os.Open(filepath.Join(os.Getenv("TEST_SRCDIR"), *dir, "main.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b != image.Rect(0, 0, 1754, 1240) {
		t.Fatalf("size %v, want 1754x1240 (A4 at 150 dpi)", b)
	}
	// The darkest pixel across the row, since the line is anti-aliased.
	darkest := 3 * 0xffff
	for x := 52; x <= 66; x++ {
		r, g, b, _ := img.At(x, 500).RGBA()
		if v := int(r + g + b); v < darkest {
			darkest = v
		}
	}
	if darkest > 3*0xffff*40/100 {
		t.Errorf("the frame near (59,500) is too light (%d of %d): not drawn", darkest, 3*0xffff)
	}
}
