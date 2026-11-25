// SPDX-License-Identifier: Apache-2.0

// Package look_test checks real kicad_layers outputs.
package look_test

import (
	"bytes"
	"flag"
	"image/color"
	"image/png"
	"os"
	"regexp"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	fCuSVG = flag.String("f_cu_svg", "", ":f_cu's SVG, as a runfiles path")
	bCuSVG = flag.String("b_cu_svg", "", ":b_cu's SVG (mirrored, black and white), as a runfiles path")
	bCuPNG = flag.String("b_cu_png", "", ":b_cu's PNG (300 dpi), as a runfiles path")
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

var dateRe = regexp.MustCompile(`\d{4}/\d\d/\d\d \d\d:\d\d:\d\d`)

// Each plot's date is the epoch, and no other date is left.
func TestSVGsHaveNoDate(t *testing.T) {
	for name, p := range map[string]string{"f_cu": *fCuSVG, "b_cu": *bCuSVG} {
		b := read(t, p)
		if !bytes.Contains(b, []byte("date 1970/01/01 00:00:00")) {
			t.Errorf("%s: the <title> does not hold the epoch: %.300s", name, b)
		}
		if m := dateRe.FindAll(b, -1); len(m) != 1 {
			t.Errorf("%s: %d dates, want 1 (the epoch): %q", name, len(m), m)
		}
	}
}

var colorRe = regexp.MustCompile(`#[0-9A-Fa-f]{6}`)

// options reach kicad-cli: :b_cu is --black-and-white, so it has black and
// white only, while :f_cu has the theme's copper color.
func TestOptionsTakeEffect(t *testing.T) {
	for _, c := range colorRe.FindAll(read(t, *bCuSVG), -1) {
		if s := string(bytes.ToUpper(c)); s != "#000000" && s != "#FFFFFF" {
			t.Errorf("b_cu is --black-and-white, but has the color %s", s)
			break
		}
	}
	colored := false
	for _, c := range colorRe.FindAll(read(t, *fCuSVG), -1) {
		if s := string(bytes.ToUpper(c)); s != "#000000" && s != "#FFFFFF" {
			colored = true
		}
	}
	if !colored {
		t.Error("f_cu has no color but black and white; want the theme's copper color")
	}
}

// The board of //tests/design is 20 mm by 12 mm; the plot is cut to it,
// so at 300 dpi the PNG is about 236 by 142 pixels. Its pads and outline
// are drawn: it is not all one color.
func TestPNGIsTheBoard(t *testing.T) {
	img, err := png.Decode(bytes.NewReader(read(t, *bCuPNG)))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	if b.Dx() < 230 || b.Dx() > 250 || b.Dy() < 136 || b.Dy() > 156 {
		t.Errorf("size %dx%d, want about 236x142 (20x12 mm at 300 dpi)", b.Dx(), b.Dy())
	}
	first := color.NRGBAModel.Convert(img.At(b.Min.X, b.Min.Y))
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if color.NRGBAModel.Convert(img.At(x, y)) != first {
				return
			}
		}
	}
	t.Error("the PNG is one color: nothing is drawn")
}
