// SPDX-License-Identifier: Apache-2.0

// Command modelcheck fails kicad_render when a part's 3D model is not in
// the build.
//
// Usage:
//
//	modelcheck --stamp STAMP --models_dir DIR BOARD
//
// kicad-cli's `pcb render` draws a part whose model it cannot find without
// the model, prints nothing about it, and exits 0. So this reads the
// board instead. Each footprint's shown (model "...") must be a file:
//
//   - under ${KICAD6_3DMODEL_DIR} to ${KICAD9_3DMODEL_DIR} or
//     ${KISYS3DMOD}, the variables kicad_render sets: in DIR, where it
//     staged the libraries;
//   - under ${KIPRJMOD}, or a relative path: next to the board, where
//     kicad_pcb staged the project.
//
// Any other path (absolute, or under another variable) cannot come from
// the build, and is named too. Each missing model is named with its part,
// and the @kicad_models target of its library when it has one. With none,
// it writes STAMP.
package main

import (
	"flag"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/filmil/bazel_rules_kicad/kicad/private/sexpr"
)

// The variables kicad_render sets (stage_models.bzl's MODEL_DIR_VARS).
var modelDir = regexp.MustCompile(`^\$\{(KICAD[6-9]_3DMODEL_DIR|KISYS3DMOD)\}/(.+)$`)

var projectDir = regexp.MustCompile(`^\$\{KIPRJMOD\}/(.+)$`)

var library = regexp.MustCompile(`^([^/]+)\.3dshapes/`)

// hidden says whether a model has (hide yes), or a bare hide.
func hidden(m *sexpr.Node) bool {
	for _, c := range m.List[1:] {
		if !c.IsList && c.Atom == "hide" {
			return true
		}
		if c.Head() == "hide" && c.Arg(0) != "no" {
			return true
		}
	}
	return false
}

// reference is a footprint's reference: a Reference property (KiCad 8 and
// later), or an fp_text reference (KiCad 6 and 7).
func reference(fp *sexpr.Node) string {
	for _, p := range fp.List[1:] {
		if p.Head() == "property" && p.Arg(0) == "Reference" {
			return p.Arg(1)
		}
		if p.Head() == "fp_text" && p.Arg(0) == "reference" {
			return p.Arg(1)
		}
	}
	return "the footprint " + fp.Arg(0)
}

// isFile says whether p is a regular file, after links.
func isFile(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Mode().IsRegular()
}

// within joins rel to dir, or fails when rel is absolute or leaves dir.
func within(dir, rel string) (string, bool) {
	if path.IsAbs(rel) || filepath.IsAbs(rel) {
		return "", false
	}
	for _, seg := range strings.Split(rel, "/") {
		if seg == ".." {
			return "", false
		}
	}
	return filepath.Join(dir, filepath.FromSlash(rel)), true
}

// check returns why a model path is missing, or "".
func check(p, modelsDir, boardDir string) string {
	if m := modelDir.FindStringSubmatch(p); m != nil {
		f, ok := within(modelsDir, m[2])
		if !ok {
			return fmt.Sprintf("model %q leaves the model directory", p)
		}
		if isFile(f) {
			return ""
		}
		msg := fmt.Sprintf("%s not found", p)
		if l := library.FindStringSubmatch(m[2]); l != nil {
			msg += fmt.Sprintf("; add @kicad_models//:%s (or your own %s.3dshapes) to models, or check that the library has this file", l[1], l[1])
		} else {
			msg += "; a model library is a <name>.3dshapes directory"
		}
		return msg
	}
	rel := p
	if m := projectDir.FindStringSubmatch(p); m != nil {
		rel = m[1]
	} else if strings.HasPrefix(p, "$") || path.IsAbs(p) || filepath.IsAbs(p) {
		return fmt.Sprintf("model %q is under neither ${KICAD9_3DMODEL_DIR} nor the project, so the build cannot give it", p)
	}
	f, ok := within(boardDir, rel)
	if !ok {
		return fmt.Sprintf("model %q leaves the project directory", p)
	}
	if isFile(f) {
		return ""
	}
	return fmt.Sprintf("%s not found next to the board; models of the project are not staged, so put the model in a library under models", p)
}

// Missing returns one line per shown model of a footprint on the board
// that the build does not give, sorted.
func Missing(board, modelsDir, boardDir string) ([]string, error) {
	root, err := sexpr.Parse(board)
	if err != nil {
		return nil, err
	}
	if root.Head() != "kicad_pcb" {
		return nil, fmt.Errorf("not a kicad_pcb")
	}
	var out []string
	for _, fp := range root.List[1:] {
		if fp.Head() != "footprint" {
			continue
		}
		for _, m := range fp.List[1:] {
			if m.Head() != "model" || hidden(m) {
				continue
			}
			if why := check(m.Arg(0), modelsDir, boardDir); why != "" {
				out = append(out, reference(fp)+": "+why)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

func main() {
	stamp := flag.String("stamp", "", "the file to write when no model is missing")
	dir := flag.String("models_dir", "", "where kicad_render staged the model libraries")
	flag.Parse()
	if flag.NArg() != 1 || *stamp == "" || *dir == "" {
		fmt.Fprintln(os.Stderr, "usage: modelcheck --stamp STAMP --models_dir DIR BOARD")
		os.Exit(2)
	}
	board := flag.Arg(0)
	b, err := os.ReadFile(board)
	if err != nil {
		fmt.Fprintln(os.Stderr, "modelcheck:", err)
		os.Exit(1)
	}
	missing, err := Missing(string(b), *dir, filepath.Dir(board))
	if err != nil {
		fmt.Fprintf(os.Stderr, "modelcheck: %s: %v\n", board, err)
		os.Exit(1)
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "the render would lack %d 3D model(s):\n", len(missing))
		for _, m := range missing {
			fmt.Fprintln(os.Stderr, "  "+m)
		}
		os.Exit(1)
	}
	if err := os.WriteFile(*stamp, nil, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "modelcheck:", err)
		os.Exit(1)
	}
}
