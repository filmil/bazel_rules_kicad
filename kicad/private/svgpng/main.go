// SPDX-License-Identifier: Apache-2.0

// Command svgpng draws the SVGs that KiCad wrote as PNGs (issue #21).
//
// Usage:
//
//	svgpng --dpi N IN OUT
//
// IN is an SVG file and OUT the PNG to write, or IN is a directory and
// OUT a directory that gets <name>.png for each <name>.svg in IN.
//
// KiCad draws every glyph of its text as paths, and adds each text again
// as an invisible <text> element so the SVG can be searched. The
// rasterizer (oksvg) draws paths and shapes only, so no font is needed and
// none of the host's is read. The PNG holds pixels only: no time stamp and
// no text chunks, so the same SVG gives the same bytes.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

// invisibleText matches a <text> element whose opening tag makes it
// invisible, as KiCad writes each text for searching: opacity="0".
var invisibleText = regexp.MustCompile(`(?s)<text\b[^>]*\bopacity="0"[^>]*>.*?</text>`)

// Render draws an SVG at dpi dots per inch, on white. KiCad's SVGs are
// sized in millimeters, one unit of the view box per millimeter.
//
// It drops KiCad's invisible <text> elements and parses the rest strictly:
// an element oksvg cannot draw, such as visible text, fails the run
// instead of leaving it out of the image.
func Render(svg io.Reader, dpi float64) (*image.RGBA, error) {
	src, err := io.ReadAll(svg)
	if err != nil {
		return nil, err
	}
	src = invisibleText.ReplaceAll(src, nil)
	icon, err := oksvg.ReadIconStream(bytes.NewReader(src), oksvg.StrictErrorMode)
	if err != nil {
		return nil, err
	}
	w := int(icon.ViewBox.W*dpi/25.4 + 0.5)
	h := int(icon.ViewBox.H*dpi/25.4 + 0.5)
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("empty view box %v", icon.ViewBox)
	}
	icon.SetTarget(0, 0, float64(w), float64(h))

	// oksvg scales the geometry to the target but not the line widths and
	// dashes, which stay in SVG units: a 0.15 mm wire would be 0.15 pixel
	// wide.
	scale := dpi / 25.4
	for i := range icon.SVGPaths {
		p := &icon.SVGPaths[i]
		p.LineWidth *= scale
		p.DashOffset *= scale
		dash := make([]float64, len(p.Dash))
		for j, d := range p.Dash {
			dash[j] = d * scale
		}
		p.Dash = dash
	}
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	icon.Draw(rasterx.NewDasher(w, h, rasterx.NewScannerGV(w, h, img, img.Bounds())), 1)
	return img, nil
}

func convert(in, out string, dpi float64) error {
	f, err := os.Open(in)
	if err != nil {
		return err
	}
	defer f.Close()
	img, err := Render(f, dpi)
	if err != nil {
		return fmt.Errorf("%s: %w", in, err)
	}
	o, err := os.Create(out)
	if err != nil {
		return err
	}
	if err := png.Encode(o, img); err != nil {
		o.Close()
		return err
	}
	return o.Close()
}

// Run converts a file, or each .svg of a directory into another directory.
func Run(in, out string, dpi float64) error {
	fi, err := os.Stat(in)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return convert(in, out, dpi)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(in)
	if err != nil {
		return err
	}
	var svgs []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".svg") {
			svgs = append(svgs, e.Name())
		}
	}
	if len(svgs) == 0 {
		return fmt.Errorf("%s: no .svg file", in)
	}
	sort.Strings(svgs)
	for _, n := range svgs {
		if err := convert(filepath.Join(in, n), filepath.Join(out, strings.TrimSuffix(n, ".svg")+".png"), dpi); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	dpi := flag.Float64("dpi", 150, "dots per inch")
	flag.Parse()
	if flag.NArg() != 2 || *dpi <= 0 {
		fmt.Fprintln(os.Stderr, "usage: svgpng --dpi N IN OUT")
		os.Exit(2)
	}
	if err := Run(flag.Arg(0), flag.Arg(1), *dpi); err != nil {
		fmt.Fprintln(os.Stderr, "svgpng:", err)
		os.Exit(1)
	}
}
