// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A 20 mm by 10 mm sheet like KiCad's: a background rect, a red filled
// square in the left half, a black stroked line, and an invisible text
// element as KiCad adds for searching.
const sheet = `<?xml version="1.0" standalone="no"?>
<svg xmlns="http://www.w3.org/2000/svg" version="1.1" width="20mm" height="10mm" viewBox="0 0 20 10">
<title>SVG Image created as x.svg date 2026/10/29 03:58:50 </title>
<g style="fill:#F5F4EF; stroke:#F5F4EF; stroke-width:0.0001;"><rect x="0" y="0" width="20" height="10" /></g>
<g style="fill:#FF0000; stroke:none;"><path d="M2 2 L8 2 L8 8 L2 8 Z" /></g>
<g style="fill:none; stroke:#000000; stroke-width:0.5;"><path d="M12 5 L18 5" /></g>
<text x="12" y="9" font-size="2" opacity="0" stroke-opacity="0">hidden</text>
</svg>
`

func TestRenderSheet(t *testing.T) {
	img, err := Render(strings.NewReader(sheet), 254) // 10 px per mm
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 200 || b.Dy() != 100 {
		t.Fatalf("size %v, want 200x100", b)
	}
	for _, c := range []struct {
		x, y int
		want color.RGBA
	}{
		{50, 50, color.RGBA{0xff, 0, 0, 0xff}},       // the red square
		{150, 50, color.RGBA{0, 0, 0, 0xff}},         // on the line
		{150, 20, color.RGBA{0xf5, 0xf4, 0xef, 0xff}}, // KiCad's background
		{150, 85, color.RGBA{0xf5, 0xf4, 0xef, 0xff}}, // where the hidden text is
	} {
		if got := img.RGBAAt(c.x, c.y); got != c.want {
			t.Errorf("(%d,%d) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
}

func TestRunDirectoryIsDeterministic(t *testing.T) {
	in := t.TempDir()
	os.WriteFile(filepath.Join(in, "main.svg"), []byte(sheet), 0o644)
	os.WriteFile(filepath.Join(in, "main-Power.svg"), []byte(sheet), 0o644)
	os.WriteFile(filepath.Join(in, "notes.txt"), []byte("not an SVG"), 0o644)
	a, b := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")
	for _, out := range []string{a, b} {
		if err := Run(in, out, 150); err != nil {
			t.Fatal(err)
		}
	}
	entries, _ := os.ReadDir(a)
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if strings.Join(names, " ") != "main-Power.png main.png" {
		t.Errorf("outputs %v", names)
	}
	for _, n := range names {
		x, _ := os.ReadFile(filepath.Join(a, n))
		y, _ := os.ReadFile(filepath.Join(b, n))
		if !bytes.Equal(x, y) {
			t.Errorf("%s differs between two runs", n)
		}
		if _, err := png.Decode(bytes.NewReader(x)); err != nil {
			t.Errorf("%s: %v", n, err)
		}
		// No time or text chunk: only the image's own.
		for _, chunk := range []string{"tIME", "tEXt", "iTXt", "zTXt"} {
			if bytes.Contains(x, []byte(chunk)) {
				t.Errorf("%s has a %s chunk", n, chunk)
			}
		}
	}
	if err := Run(t.TempDir(), filepath.Join(t.TempDir(), "o"), 150); err == nil {
		t.Error("a directory without SVGs must be an error")
	}
}
