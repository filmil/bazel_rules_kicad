// SPDX-License-Identifier: Apache-2.0

// Package files_test checks the PNG images kicad_render writes for the
// board of //tests/design.
package files_test

import (
	"bytes"
	"encoding/binary"
	"flag"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	render = flag.String("render", "", "the outputs of :render (1600 by 900), as runfiles paths separated by spaces")
	small  = flag.String("small", "", "the outputs of :render_small (320 by 180), as runfiles paths separated by spaces")
)

// outputs returns the paths of a target's outputs, by base name.
func outputs(t *testing.T, rpaths string) map[string]string {
	t.Helper()
	m := map[string]string{}
	for _, r := range strings.Fields(rpaths) {
		p, err := runfiles.Rlocation(r)
		if err != nil {
			t.Fatal(err)
		}
		m[filepath.Base(p)] = p
	}
	return m
}

func decode(t *testing.T, p string) ([]byte, image.Image) {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("%s: %v", p, err)
	}
	return b, img
}

// chunks returns the types of the PNG chunks of b, in order.
func chunks(t *testing.T, b []byte) []string {
	t.Helper()
	var kinds []string
	for rest := b[8:]; len(rest) >= 12; {
		n := binary.BigEndian.Uint32(rest[:4])
		kinds = append(kinds, string(rest[4:8]))
		rest = rest[12+n:]
	}
	return kinds
}

func TestTwoSides(t *testing.T) {
	for _, set := range []string{*render, *small} {
		got := outputs(t, set)
		if len(got) != 2 || got["top.png"] == "" || got["bottom.png"] == "" {
			t.Errorf("outputs %v, want top.png and bottom.png", got)
		}
	}
}

// KiCad makes the image a little smaller than asked: 1600 by 900 gives
// 1568 by 872. The given size reaches kicad-cli.
func TestSize(t *testing.T) {
	for _, c := range []struct {
		set     string
		w, h    int
		minimum int
	}{
		{*render, 1600, 900, 32},
		{*small, 320, 180, 32},
	} {
		for name, p := range outputs(t, c.set) {
			_, img := decode(t, p)
			b := img.Bounds()
			if b.Dx() > c.w || b.Dx() < c.w-c.minimum || b.Dy() > c.h || b.Dy() < c.h-c.minimum {
				t.Errorf("%s: %dx%d, want a little under %dx%d", name, b.Dx(), b.Dy(), c.w, c.h)
			}
		}
	}
	for name, p := range outputs(t, *render) {
		if _, img := decode(t, p); img.Bounds() != image.Rect(0, 0, 1568, 872) {
			t.Errorf("%s: %v, want 1568x872", name, img.Bounds())
		}
	}
}

// No chunk holds text, a time or Exif data.
func TestNoMetadata(t *testing.T) {
	for name, p := range outputs(t, *render) {
		b, _ := decode(t, p)
		for _, k := range chunks(t, b) {
			switch k {
			case "tEXt", "zTXt", "iTXt", "tIME", "eXIf":
				t.Errorf("%s: has a %s chunk", name, k)
			}
		}
	}
}

// The middle half of each image is the board. The top has white
// silkscreen (the reference J1 and the text FIXTURE); the bottom has none.
// The corners are KiCad's opaque background, lighter at the top.
func TestPixels(t *testing.T) {
	out := outputs(t, *render)
	for _, c := range []struct {
		name       string
		silkscreen bool
	}{
		{"top.png", true},
		{"bottom.png", false},
	} {
		_, img := decode(t, out[c.name])
		b := img.Bounds()
		white, green := 0, 0
		for y := b.Dy() / 4; y < b.Dy()*3/4; y++ {
			for x := b.Dx() / 4; x < b.Dx()*3/4; x++ {
				r, g, bl, _ := img.At(x, y).RGBA()
				if r>>8 > 0xe8 && g>>8 > 0xe8 && bl>>8 > 0xe8 {
					white++
				}
				if g > r && g > bl {
					green++
				}
			}
		}
		area := b.Dx() * b.Dy() / 4
		if green < area/2 {
			t.Errorf("%s: %d green pixels of %d in the middle: no solder mask", c.name, green, area)
		}
		if c.silkscreen && white < 1000 {
			t.Errorf("%s: %d white pixels in the middle: no silkscreen", c.name, white)
		}
		if !c.silkscreen && white != 0 {
			t.Errorf("%s: %d white pixels in the middle, want none", c.name, white)
		}
		_, _, _, a := img.At(2, 2).RGBA()
		if a != 0xffff {
			t.Errorf("%s: the corner is not opaque (alpha %d)", c.name, a)
		}
		top := luma(img.At(2, 2))
		bottom := luma(img.At(2, b.Dy()-3))
		if top <= bottom {
			t.Errorf("%s: the background is not lighter at the top (%d, %d)", c.name, top, bottom)
		}
	}
}

func luma(c interface{ RGBA() (r, g, b, a uint32) }) uint32 {
	r, g, b, _ := c.RGBA()
	return r + g + b
}
