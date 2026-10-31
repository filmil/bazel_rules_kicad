// SPDX-License-Identifier: Apache-2.0

// Package merge is a 3-way merge of KiCad's S-expression files (issue #58):
// schematics, boards and symbol libraries. Git merges them line by line,
// which can interleave two elements or leave a list unclosed. This merges
// them element by element.
//
// The elements of the top list are keyed: by their (uuid ...) when they
// have one (symbols, wires, junctions, labels, sheets, footprints, tracks,
// vias, zones), else by their head and first argument when the head
// repeats ((net 3 "GND"), a symbol in lib_symbols), else by their head
// alone (version, paper, setup). An element keeps its exact text, so an
// unchanged one comes out byte for byte as it went in, in KiCad's own
// format.
//
//   - Added on one side: kept, after the last element of its kind.
//   - Deleted on one side and unchanged on the other: deleted.
//   - Changed on one side: the change is taken.
//   - Changed the same way on both sides: taken once.
//   - Changed differently on both sides: a conflict, named by its key;
//     but lib_symbols is merged symbol by symbol, so two sides that each
//     add a library symbol do not conflict.
//
// Changes in white space alone are not changes.
package merge

import (
	"fmt"
	"strings"

	"github.com/filmil/bazel_rules_kicad/kicad/private/sexpr"
)

// Containers are the uuid-less lists merged child by child.
var Containers = map[string]bool{"lib_symbols": true}

// elem is one element of a list: its text, as it stands in the file, and
// its key.
type elem struct {
	text string
	key  string
	head string
	node *sexpr.Node
}

// list is a parsed list: its head line, the separator before each child,
// its children and its closing text.
type list struct {
	open  string // "(kicad_sch"
	sep   string // "\n\t"
	close string // "\n)"
	elems []elem
}

// scan splits a list's text into its children, keeping each one's text.
// The children of the lists it merges (a file's top list, lib_symbols)
// are lists; anything else after the head is an error.
func scan(text string) (*list, error) {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "(") || !strings.HasSuffix(t, ")") {
		return nil, fmt.Errorf("not a list")
	}
	l := &list{}
	i := 1
	for i < len(t) && !strings.ContainsRune(" \t\r\n()\"", rune(t[i])) {
		i++
	}
	l.open = t[:i]
	// The head's own atoms, if any, stay with it: (symbol "R" ...) in a
	// library file is not merged here, only the top list and lib_symbols.
	var children, seps []string
	depth, start, gap, inString := 0, -1, i, false
	for j := i; j < len(t)-1; j++ {
		c := t[j]
		switch {
		case inString:
			if c == '\\' {
				j++
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			if depth == 0 {
				return nil, fmt.Errorf("an atom outside a list at byte %d", j)
			}
			inString = true
		case c == '(':
			if depth == 0 {
				start = j
			}
			depth++
		case c == ')':
			depth--
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced ) at byte %d", j)
			}
			if depth == 0 {
				children = append(children, t[start:j+1])
				seps = append(seps, t[gap:start])
				gap = j + 1
			}
		case depth == 0 && !strings.ContainsRune(" \t\r\n", rune(c)):
			return nil, fmt.Errorf("an atom outside a list at byte %d", j)
		}
	}
	if depth != 0 || inString {
		return nil, fmt.Errorf("unclosed list or string")
	}
	l.close = t[gap:]
	l.sep = "\n\t"
	for _, s := range seps {
		if strings.Contains(s, "\n") {
			l.sep = s
			break
		}
	}
	counts := map[string]int{}
	for _, c := range children {
		n, err := sexpr.Parse(c)
		if err != nil {
			return nil, err
		}
		e := elem{text: c, node: n, head: n.Head()}
		counts[e.head]++
		l.elems = append(l.elems, e)
	}
	seen := map[string]int{}
	for i := range l.elems {
		e := &l.elems[i]
		switch {
		case uuidOf(e.node) != "":
			e.key = "uuid:" + uuidOf(e.node)
		case counts[e.head] > 1 || Containers[l.open[1:]]:
			e.key = e.head + "|" + e.node.Arg(0)
		default:
			e.key = e.head
		}
		// Keys are unique within a list: the repeats are numbered.
		base := e.key
		if n := seen[base]; n > 0 {
			e.key = fmt.Sprintf("%s#%d", base, n)
		}
		seen[base]++
	}
	return l, nil
}

func uuidOf(n *sexpr.Node) string {
	for _, c := range n.List {
		if c.Head() == "uuid" {
			return c.Arg(0)
		}
	}
	return ""
}

// same says whether two texts differ only in white space.
func same(a, b string) bool {
	return strings.Join(strings.Fields(a), " ") == strings.Join(strings.Fields(b), " ")
}

func index(l *list) map[string]int {
	m := map[string]int{}
	for i, e := range l.elems {
		m[e.key] = i
	}
	return m
}

// Merge merges ours and theirs, both edits of base. It returns the merged
// text and the conflicts; with conflicts, the text is ours unchanged.
func Merge(base, ours, theirs string) (string, []string, error) {
	out, conflicts, err := mergeList(base, ours, theirs, "")
	if err != nil {
		return "", nil, err
	}
	if len(conflicts) > 0 {
		return ours, conflicts, nil
	}
	// Keep ours' leading and trailing white space (a final newline).
	lead := ours[:len(ours)-len(strings.TrimLeft(ours, " \t\r\n"))]
	trail := ours[len(strings.TrimRight(ours, " \t\r\n")):]
	return lead + out + trail, nil, nil
}

func mergeList(base, ours, theirs, where string) (string, []string, error) {
	b, err := scan(base)
	if err != nil {
		return "", nil, fmt.Errorf("base%s: %v", where, err)
	}
	a, err := scan(ours)
	if err != nil {
		return "", nil, fmt.Errorf("ours%s: %v", where, err)
	}
	t, err := scan(theirs)
	if err != nil {
		return "", nil, fmt.Errorf("theirs%s: %v", where, err)
	}
	if a.open != t.open || a.open != b.open {
		return "", []string{fmt.Sprintf("%s: the files are of different kinds (%s, %s, %s)", where, b.open, a.open, t.open)}, nil
	}
	bi, ai, ti := index(b), index(a), index(t)
	var conflicts []string
	conflict := func(key, why string) {
		conflicts = append(conflicts, fmt.Sprintf("%s%s: %s", where, key, why))
	}
	var out []elem
	for _, e := range a.elems {
		bk, inB := bi[e.key]
		tk, inT := ti[e.key]
		switch {
		case inB && !inT:
			// Theirs deleted it.
			if !same(e.text, b.elems[bk].text) {
				conflict(e.key, "changed in ours, deleted in theirs")
				out = append(out, e)
			}
		case inB && inT:
			bt, tt := b.elems[bk].text, t.elems[tk].text
			switch {
			case same(e.text, bt):
				out = append(out, t.elems[tk])
			case same(tt, bt), same(e.text, tt):
				out = append(out, e)
			case Containers[e.head]:
				m, c, err := mergeList(bt, e.text, tt, where+e.key+"/")
				if err != nil {
					return "", nil, err
				}
				conflicts = append(conflicts, c...)
				out = append(out, elem{text: m, key: e.key, head: e.head})
			default:
				conflict(e.key, "changed differently in ours and theirs")
				out = append(out, e)
			}
		case !inB && inT:
			if !same(e.text, t.elems[tk].text) {
				conflict(e.key, "added differently in ours and theirs")
			}
			out = append(out, e)
		default:
			out = append(out, e) // added in ours
		}
	}
	// What theirs added, or what ours deleted that theirs kept.
	for ti0, e := range t.elems {
		if _, inA := ai[e.key]; inA {
			continue
		}
		if bk, inB := bi[e.key]; inB {
			if !same(e.text, b.elems[bk].text) {
				conflict(e.key, "deleted in ours, changed in theirs")
			}
			continue
		}
		// Added in theirs: after the last element of its kind, as KiCad
		// groups them (wires, then symbols); failing that, after the
		// nearest element before it in theirs that the result has; else
		// last.
		pos := -1
		for i, o := range out {
			if o.head == e.head {
				pos = i + 1
			}
		}
		for k := ti0 - 1; k >= 0 && pos < 0; k-- {
			for i, o := range out {
				if o.key == t.elems[k].key {
					pos = i + 1
					break
				}
			}
		}
		if pos < 0 {
			pos = len(out)
		}
		out = append(out[:pos], append([]elem{e}, out[pos:]...)...)
	}
	var s strings.Builder
	s.WriteString(a.open)
	for _, e := range out {
		s.WriteString(a.sep)
		s.WriteString(e.text)
	}
	s.WriteString(a.close)
	return s.String(), conflicts, nil
}
