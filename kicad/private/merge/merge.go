// SPDX-License-Identifier: Apache-2.0

// Package merge is a 3-way merge of KiCad's S-expression files (issue #58):
// schematics, boards and symbol libraries. Git merges them line by line,
// which can interleave two elements or leave a list unclosed. This merges
// them element by element.
//
// Each element of the top list has a key that does not depend on what else
// is in the file:
//
//   - its (uuid ...), when it has one: symbols, wires, junctions, labels,
//     sheets, footprints, tracks, vias, zones;
//   - else, for the kinds that repeat, its name or number: (symbol "R")
//     in a library or in lib_symbols, (net 3 "GND"), (property "Name"),
//     (path "/") in sheet_instances;
//   - else its head, for the kinds a file has once: version, paper, setup.
//
// An element of a repeating kind that has none of these (two of the same
// kind with the same name, say) is keyed by its text; if both sides
// change such elements of one kind, that is a conflict. Elements keep
// their exact text, so an unchanged one comes out byte for byte, in KiCad's
// format. A change in white space outside quoted strings is no change.
//
//   - Added on one side: kept, after the last element of its kind.
//   - Deleted on one side and unchanged on the other: deleted.
//   - Changed on one side: the change is taken.
//   - Changed the same way on both sides: taken once.
//   - Changed differently on both sides: a conflict, named by its key.
//
// lib_symbols, sheet_instances and symbol_instances are merged child by
// child, so two sides that each add a library symbol or a sheet do not
// conflict. After the merge, two nets with one name, or two placed symbols
// with one reference and unit, are conflicts: both sides added an R1, say,
// and one must be annotated again.
package merge

import (
	"fmt"
	"sort"
	"strings"

	"github.com/filmil/bazel_rules_kicad/kicad/private/sexpr"
)

// Containers are the uuid-less lists merged child by child.
var Containers = map[string]bool{"lib_symbols": true, "sheet_instances": true, "symbol_instances": true}

// Repeated are the kinds of element, without a uuid, that a list has many
// of, keyed by their first argument.
var Repeated = map[string]bool{"symbol": true, "net": true, "property": true, "path": true, "net_class": true}

type elem struct {
	text, key, head string
	node            *sexpr.Node
	byText          bool // keyed by its text: no stable key
}

type list struct {
	open  string // "(kicad_sch"
	elems []elem
}

// scan splits a list's text into its children. The lists it merges (a
// file's top list, the containers) have only lists after their head.
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
	var children []string
	depth, start, inString := 0, -1, false
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
				return nil, fmt.Errorf("an atom after the head %s, at byte %d", l.open, j)
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
			}
		case depth == 0 && !strings.ContainsRune(" \t\r\n", rune(c)):
			return nil, fmt.Errorf("an atom after the head %s, at byte %d", l.open, j)
		}
	}
	if depth != 0 || inString {
		return nil, fmt.Errorf("unclosed list or string")
	}
	seen := map[string]int{}
	for _, c := range children {
		n, err := sexpr.Parse(c)
		if err != nil {
			return nil, err
		}
		e := elem{text: c, node: n, head: n.Head()}
		switch {
		case uuidOf(n) != "":
			e.key = "uuid:" + uuidOf(n)
		case Repeated[e.head] && n.Arg(0) != "":
			e.key = e.head + "|" + n.Arg(0)
		case Repeated[e.head]:
			e.key, e.byText = e.head+"|text:"+normalize(c), true
		default:
			e.key = e.head
		}
		seen[e.key]++
		l.elems = append(l.elems, e)
	}
	// A key used twice is not stable: key those by text.
	for i := range l.elems {
		e := &l.elems[i]
		if seen[e.key] > 1 && !e.byText {
			e.key, e.byText = e.head+"|text:"+normalize(e.text), true
		}
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

// normalize collapses white space outside quoted strings, so a change in
// layout alone is no change, and one inside a string is.
func normalize(s string) string {
	var b strings.Builder
	inString, space := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inString {
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
			} else if c == '"' {
				inString = false
			}
			continue
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			space = true
			continue
		}
		if space && b.Len() > 0 && c != ')' && !strings.HasSuffix(b.String(), "(") {
			b.WriteByte(' ')
		}
		space = false
		if c == '"' {
			inString = true
		}
		b.WriteByte(c)
	}
	return b.String()
}

func same(a, b string) bool { return normalize(a) == normalize(b) }

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
	out, conflicts, err := mergeList(base, ours, theirs, "", 0)
	if err != nil {
		return "", nil, err
	}
	if len(conflicts) == 0 {
		conflicts = checkMerged(out)
	}
	if len(conflicts) > 0 {
		return ours, conflicts, nil
	}
	// Keep ours' leading and trailing white space (a final newline).
	lead := ours[:len(ours)-len(strings.TrimLeft(ours, " \t\r\n"))]
	trail := ours[len(strings.TrimRight(ours, " \t\r\n")):]
	return lead + out + trail, nil, nil
}

func mergeList(base, ours, theirs, where string, depth int) (string, []string, error) {
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
		return "", []string{fmt.Sprintf("%sthe files are of different kinds (%s, %s, %s)", where, b.open, a.open, t.open)}, nil
	}
	bi, ai, ti := index(b), index(a), index(t)
	var conflicts []string
	conflict := func(key, why string) {
		conflicts = append(conflicts, fmt.Sprintf("%s%s: %s", where, key, why))
	}
	// Elements keyed by text: a kind both sides changed is a conflict.
	changedByText := func(l *list, other map[string]int) map[string]bool {
		m := map[string]bool{}
		for _, e := range l.elems {
			if _, ok := other[e.key]; e.byText && !ok {
				m[e.head] = true
			}
		}
		for _, e := range b.elems {
			if _, ok := index(l)[e.key]; e.byText && !ok {
				m[e.head] = true
			}
		}
		return m
	}
	ca, ct := changedByText(a, bi), changedByText(t, bi)
	for h := range ca {
		if ct[h] {
			conflict(h, "elements of this kind without a name or uuid changed on both sides")
		}
	}
	var out []elem
	for _, e := range a.elems {
		bk, inB := bi[e.key]
		tk, inT := ti[e.key]
		switch {
		case inB && !inT:
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
				m, c, err := mergeList(bt, e.text, tt, where+e.key+"/", depth+1)
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
		// groups them; failing that, after the nearest element before it
		// in theirs that the result has; else last.
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
	// KiCad's layout: one child per line, a tab deeper than the list.
	if len(out) == 0 {
		return a.open + ")", conflicts, nil
	}
	var s strings.Builder
	s.WriteString(a.open)
	in := "\n" + strings.Repeat("\t", depth+1)
	for _, e := range out {
		s.WriteString(in)
		s.WriteString(e.text)
	}
	s.WriteString("\n" + strings.Repeat("\t", depth) + ")")
	return s.String(), conflicts, nil
}

// checkMerged finds what no single element shows: two nets of a board with
// one name, two placed symbols of a schematic with one reference and unit.
func checkMerged(text string) []string {
	n, err := sexpr.Parse(text)
	if err != nil {
		return []string{"the merge does not parse: " + err.Error()}
	}
	var out []string
	nets := map[string][]string{}
	refs := map[string][]string{}
	kind := n.Head()
	for _, e := range n.List[1:] {
		switch {
		case kind == "kicad_pcb" && e.Head() == "net":
			if name := e.Arg(1); name != "" {
				nets[name] = append(nets[name], e.Arg(0))
			}
		case kind == "kicad_sch" && e.Head() == "symbol":
			// A placed symbol (a library's symbols have references too,
			// "R" for all resistors, so only a schematic's count).
			ref, unit := "", "1"
			for _, c := range e.List {
				switch {
				case c.Head() == "property" && c.Arg(0) == "Reference":
					ref = c.Arg(1)
				case c.Head() == "unit":
					unit = c.Arg(0)
				}
			}
			if ref != "" && !strings.HasSuffix(ref, "?") {
				refs[ref+" unit "+unit] = append(refs[ref+" unit "+unit], uuidOf(e))
			}
		}
	}
	for _, m := range []struct {
		what string
		m    map[string][]string
	}{{"net name %q is used by nets %s", nets}, {"reference %s is used by symbols %s; annotate one again", refs}} {
		var keys []string
		for k, v := range m.m {
			if len(v) > 1 {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			out = append(out, fmt.Sprintf(m.what, k, strings.Join(m.m[k], ", ")))
		}
	}
	return out
}
