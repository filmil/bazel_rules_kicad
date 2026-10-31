// SPDX-License-Identifier: Apache-2.0

// Command modelcheck fails kicad_render when a part's 3D model is not in
// the models it was given.
//
// Usage:
//
//	modelcheck --stamp STAMP --models_dir DIR BOARD
//
// kicad-cli's `pcb render` draws a part whose model it cannot find without
// the model, prints nothing about it, and exits 0. So this reads the
// board instead: each footprint's (model "...") that is not hidden, under
// one of the 3D model directory variables, must be a file under DIR, where
// kicad_render staged the libraries. Each missing one is named, with its
// part and the @kicad_models target of its library. With none, it writes
// STAMP.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The variables that KiCad footprints name the model directory with
// (stage_models.bzl's MODEL_DIR_VARS).
var modelDir = regexp.MustCompile(`^\$\{(KICAD[0-9]+_3DMODEL_DIR|KISYS3DMOD)\}/(.+)$`)

var library = regexp.MustCompile(`^([^/]+)\.3dshapes/`)

// node is an S-expression: an atom, or a list.
type node struct {
	atom string
	list []*node
	isL  bool
}

func (n *node) head() string {
	if !n.isL || len(n.list) == 0 || n.list[0].isL {
		return ""
	}
	return n.list[0].atom
}

// arg is the i-th atom after the head.
func (n *node) arg(i int) string {
	k := 0
	for _, c := range n.list[1:] {
		if !c.isL {
			if k == i {
				return c.atom
			}
			k++
		}
	}
	return ""
}

func parse(s string) (*node, error) {
	var stack []*node
	var root *node
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == '(':
			n := &node{isL: true}
			if len(stack) > 0 {
				top := stack[len(stack)-1]
				top.list = append(top.list, n)
			} else if root == nil {
				root = n
			}
			stack = append(stack, n)
			i++
		case c == ')':
			if len(stack) == 0 {
				return nil, fmt.Errorf("unbalanced ) at byte %d", i)
			}
			stack = stack[:len(stack)-1]
			i++
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '"':
			var b strings.Builder
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) {
					i++
				}
				b.WriteByte(s[i])
				i++
			}
			if i >= len(s) {
				return nil, fmt.Errorf("unterminated string")
			}
			i++
			if len(stack) == 0 {
				return nil, fmt.Errorf("an atom outside a list")
			}
			top := stack[len(stack)-1]
			top.list = append(top.list, &node{atom: b.String()})
		default:
			j := i
			for j < len(s) && !strings.ContainsRune("() \t\r\n\"", rune(s[j])) {
				j++
			}
			if len(stack) == 0 {
				return nil, fmt.Errorf("an atom outside a list")
			}
			top := stack[len(stack)-1]
			top.list = append(top.list, &node{atom: s[i:j]})
			i = j
		}
	}
	if len(stack) != 0 || root == nil {
		return nil, fmt.Errorf("unbalanced (")
	}
	return root, nil
}

// hidden says whether a model has (hide yes), or a bare hide.
func hidden(m *node) bool {
	for _, c := range m.list {
		if c.atom == "hide" {
			return true
		}
		if c.head() == "hide" && c.arg(0) != "no" {
			return true
		}
	}
	return false
}

// Missing returns one line per shown model of a footprint on the board
// that is not a file under dir, sorted.
func Missing(board, dir string) ([]string, error) {
	root, err := parse(board)
	if err != nil {
		return nil, err
	}
	if root.head() != "kicad_pcb" {
		return nil, fmt.Errorf("not a kicad_pcb")
	}
	var out []string
	for _, fp := range root.list {
		if fp.head() != "footprint" {
			continue
		}
		ref := "a footprint " + fp.arg(0)
		for _, p := range fp.list {
			if p.head() == "property" && p.arg(0) == "Reference" {
				ref = p.arg(1)
			}
		}
		for _, m := range fp.list {
			if m.head() != "model" || hidden(m) {
				continue
			}
			path := m.arg(0)
			v := modelDir.FindStringSubmatch(path)
			if v == nil {
				out = append(out, fmt.Sprintf("%s: model %q is not under ${KICAD9_3DMODEL_DIR}, so no models library can give it", ref, path))
				continue
			}
			rel := v[2]
			if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err == nil {
				continue
			}
			msg := fmt.Sprintf("%s: %s not found", ref, path)
			if l := library.FindStringSubmatch(rel); l != nil {
				msg += fmt.Sprintf("; add @kicad_models//:%s (or your own %s.3dshapes) to models, or check that the library has this file", l[1], l[1])
			}
			out = append(out, msg)
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
	b, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "modelcheck:", err)
		os.Exit(1)
	}
	missing, err := Missing(string(b), *dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "modelcheck: %s: %v\n", flag.Arg(0), err)
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
