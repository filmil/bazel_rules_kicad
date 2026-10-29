// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"sort"
	"strconv"
)

// CanonicalSTEP renumbers the entities of the DATA section of a STEP file
// (ISO 10303-21) in an order that depends only on their content (issue
// #63).
//
// kicad-cli writes the same entities on every run, but numbers and orders
// them differently as soon as a model has copper or parts. The order is
// made canonical so:
//
//  1. Each entity gets a content key: its text, with each reference
//     replaced by the key of the entity it names, and the number of
//     entities that refer to it.
//  2. The entities that nothing refers to are the roots. They are taken
//     in key order. Among roots with the same key, the one whose entities
//     would be written as the smallest text next is taken first, so the
//     choice does not depend on the input's order.
//  3. A depth-first walk from each root visits an entity's references in
//     the order they appear in its text, and numbers each entity from 1
//     the first time it reaches it.
//
// Comments are dropped. Whitespace outside strings is dropped too, except
// a single space between two word characters; the writer's line breaks
// depend on the width of the old numbers. Lines are wrapped after a comma
// at 80 columns. The header and the trailer are kept as they are.
//
// The result is checked: it must hold the same multiset of entity keys as
// the input, so the renumbering cannot change what the file says.
func CanonicalSTEP(b []byte) ([]byte, error) {
	start, end, err := dataSection(b)
	if err != nil {
		return nil, err
	}
	ents, order, err := parseEntities(b[start:end])
	if err != nil {
		return nil, err
	}
	keys, err := entityKeys(ents)
	if err != nil {
		return nil, err
	}
	number := canonicalNumbers(ents, order, keys)

	var out bytes.Buffer
	out.Write(b[:start])
	out.WriteByte('\n')
	byNumber := make([]int, len(number))
	for old, n := range number {
		byNumber[n-1] = old
	}
	for i, old := range byNumber {
		writeWrapped(&out, fmt.Sprintf("#%d = ", i+1), renumbered(ents[old].body, number))
	}
	out.Write(b[end:])

	// The check: the same entities, whatever their numbers.
	s2, e2, err := dataSection(out.Bytes())
	if err != nil {
		return nil, fmt.Errorf("step: the renumbered file: %w", err)
	}
	ents2, _, err := parseEntities(out.Bytes()[s2:e2])
	if err != nil {
		return nil, fmt.Errorf("step: the renumbered file does not parse: %w", err)
	}
	keys2, err := entityKeys(ents2)
	if err != nil {
		return nil, err
	}
	if !sameMultiset(keys, keys2) {
		return nil, fmt.Errorf("step: renumbering changed the entities")
	}
	return out.Bytes(), nil
}

// dataSection returns where the DATA section's instances start (just after
// "DATA;") and end (at its "ENDSEC;"). Both are found as keywords, outside
// strings and comments.
func dataSection(b []byte) (int, int, error) {
	start := -1
	for i := 0; i < len(b); i++ {
		switch {
		case b[i] == '\'':
			i = stringEnd(b, i)
		case b[i] == '/' && i+1 < len(b) && b[i+1] == '*':
			j := bytes.Index(b[i+2:], []byte("*/"))
			if j < 0 {
				return 0, 0, fmt.Errorf("step: an unclosed comment")
			}
			i += j + 3
		case isWord(b[i]) && (i == 0 || !isWord(b[i-1])):
			j := i
			for j < len(b) && isWord(b[j]) {
				j++
			}
			word := string(b[i:j])
			k := j
			for k < len(b) && isSpace(b[k]) {
				k++
			}
			if k < len(b) && b[k] == ';' {
				switch {
				case word == "DATA" && start < 0:
					start = k + 1
				case word == "ENDSEC" && start >= 0:
					return start, i, nil
				}
			}
			i = j - 1
		}
	}
	if start < 0 {
		return 0, 0, fmt.Errorf("step: no DATA section")
	}
	return 0, 0, fmt.Errorf("step: the DATA section has no ENDSEC")
}

// stringEnd returns the index of the quote that closes the string opened
// at i (two quotes in a row stand for one quote inside it), or len(b)-1
// when it is not closed.
func stringEnd(b []byte, i int) int {
	for j := i + 1; j < len(b); j++ {
		if b[j] == '\'' {
			if j+1 < len(b) && b[j+1] == '\'' {
				j++
				continue
			}
			return j
		}
	}
	return len(b) - 1
}

// entity is one instance of the DATA section.
type entity struct {
	// body is the text after "#N =" without comments, without whitespace
	// outside strings (but for one space between two word characters),
	// and without the final ";".
	body []byte
	// refs are the entities it names, in the order of its text.
	refs []int
}

// instanceNumber reads "#N" at b[i]: N must be a positive number with no
// leading zero. It returns N and the index after it.
func instanceNumber(b []byte, i int) (int, int, error) {
	j := i + 1
	for j < len(b) && isDigit(b[j]) {
		j++
	}
	digits := string(b[i+1 : j])
	if digits == "" || digits[0] == '0' {
		return 0, 0, fmt.Errorf("step: %q is not an instance name (#1 or more, no leading zero)", clip(b[i:]))
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0, 0, fmt.Errorf("step: %q is not an instance name", clip(b[i:]))
	}
	return n, j, nil
}

// parseEntities reads "#N = body;" instances. It returns them by number,
// and the numbers in the order of the file.
func parseEntities(data []byte) (map[int]*entity, []int, error) {
	ents := map[int]*entity{}
	var order []int
	i := 0
	for {
		// Skip whitespace and comments up to the next instance.
		for i < len(data) && isSpace(data[i]) {
			i++
		}
		if i < len(data)-1 && data[i] == '/' && data[i+1] == '*' {
			j := bytes.Index(data[i+2:], []byte("*/"))
			if j < 0 {
				return nil, nil, fmt.Errorf("step: an unclosed comment")
			}
			i += j + 4
			continue
		}
		if i >= len(data) {
			return ents, order, nil
		}
		if data[i] != '#' {
			return nil, nil, fmt.Errorf("step: want an instance '#N =', found %q", clip(data[i:]))
		}
		n, j, err := instanceNumber(data, i)
		if err != nil {
			return nil, nil, err
		}
		for j < len(data) && isSpace(data[j]) {
			j++
		}
		if j >= len(data) || data[j] != '=' {
			return nil, nil, fmt.Errorf("step: #%d has no '='", n)
		}
		e := &entity{}
		k := j + 1
		closed := false
		pendingSpace := false // whitespace or a comment since the last byte
		emit := func(p []byte) {
			if pendingSpace && len(e.body) > 0 && isWord(e.body[len(e.body)-1]) && isWord(p[0]) {
				e.body = append(e.body, ' ')
			}
			pendingSpace = false
			e.body = append(e.body, p...)
		}
		for k < len(data) && !closed {
			c := data[k]
			switch {
			case c == '\'':
				m := stringEnd(data, k)
				if data[m] != '\'' || m == k {
					return nil, nil, fmt.Errorf("step: #%d has an unclosed string", n)
				}
				emit(data[k : m+1])
				k = m + 1
			case c == '/' && k+1 < len(data) && data[k+1] == '*':
				m := bytes.Index(data[k+2:], []byte("*/"))
				if m < 0 {
					return nil, nil, fmt.Errorf("step: #%d has an unclosed comment", n)
				}
				k += m + 4
				pendingSpace = true
			case c == '#':
				r, m, err := instanceNumber(data, k)
				if err != nil {
					return nil, nil, fmt.Errorf("step: #%d: %w", n, err)
				}
				e.refs = append(e.refs, r)
				emit(data[k:m])
				k = m
			case c == ';':
				closed = true
				k++
			case isSpace(c):
				pendingSpace = true
				k++
			default:
				emit(data[k : k+1])
				k++
			}
		}
		if !closed {
			return nil, nil, fmt.Errorf("step: #%d has no ';'", n)
		}
		if _, dup := ents[n]; dup {
			return nil, nil, fmt.Errorf("step: #%d is defined twice", n)
		}
		ents[n] = e
		order = append(order, n)
		i = k
	}
}

// entityKeys returns each entity's content key: its body with each
// reference replaced by the referred entity's key, and its number of
// referrers. A reference to an undefined entity, or a cycle, is an error.
func entityKeys(ents map[int]*entity) (map[int]string, error) {
	referrers := map[int]int{}
	for n, e := range ents {
		for _, r := range e.refs {
			if _, ok := ents[r]; !ok {
				return nil, fmt.Errorf("step: #%d refers to #%d, which is not defined", n, r)
			}
			referrers[r]++
		}
	}
	keys := map[int]string{}
	visiting := map[int]bool{}
	var key func(n int) (string, error)
	key = func(n int) (string, error) {
		if k, ok := keys[n]; ok {
			return k, nil
		}
		if visiting[n] {
			return "", fmt.Errorf("step: a reference cycle through #%d", n)
		}
		visiting[n] = true
		var buf bytes.Buffer
		var err error
		eachPart(ents[n].body, func(text []byte, ref int) {
			if err != nil {
				return
			}
			if ref == 0 {
				buf.Write(text)
				return
			}
			var k string
			k, err = key(ref)
			buf.WriteString("@" + k)
		})
		if err != nil {
			return "", err
		}
		fmt.Fprintf(&buf, "|%d", referrers[n])
		sum := sha256.Sum256(buf.Bytes())
		k := fmt.Sprintf("%x", sum[:12])
		keys[n] = k
		visiting[n] = false
		return k, nil
	}
	for n := range ents {
		if _, err := key(n); err != nil {
			return nil, err
		}
	}
	return keys, nil
}

// eachPart calls f on the parts of a body in order: a run of text (with
// ref 0), or a reference (its text and its number).
func eachPart(body []byte, f func(text []byte, ref int)) {
	s := 0
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '\'':
			i = stringEnd(body, i)
		case '#':
			if i > s {
				f(body[s:i], 0)
			}
			j := i + 1
			for j < len(body) && isDigit(body[j]) {
				j++
			}
			r, _ := strconv.Atoi(string(body[i+1 : j]))
			f(body[i:j], r)
			s = j
			i = j - 1
		}
	}
	if s < len(body) {
		f(body[s:], 0)
	}
}

// canonicalNumbers numbers the entities: roots in key order, each with a
// depth-first walk. Roots with the same key are taken in the order of the
// text they would write next, smallest first.
func canonicalNumbers(ents map[int]*entity, order []int, keys map[int]string) map[int]int {
	referred := map[int]bool{}
	for _, e := range ents {
		for _, r := range e.refs {
			referred[r] = true
		}
	}
	var roots []int
	for _, n := range order {
		if !referred[n] {
			roots = append(roots, n)
		}
	}
	sort.SliceStable(roots, func(i, j int) bool { return keys[roots[i]] < keys[roots[j]] })

	number := map[int]int{}
	// walk numbers the entities reachable from n that have no number, in
	// depth-first order, into into (n's new numbers start at next).
	var walk func(n int, into map[int]int, next *int, seq *[]int)
	walk = func(n int, into map[int]int, next *int, seq *[]int) {
		if _, ok := number[n]; ok {
			return
		}
		if _, ok := into[n]; ok {
			return
		}
		into[n] = *next
		*next++
		*seq = append(*seq, n)
		for _, r := range ents[n].refs {
			walk(r, into, next, seq)
		}
	}
	// preview is the text that walking from root would write now.
	preview := func(root int) string {
		into := map[int]int{}
		next := len(number) + 1
		var seq []int
		walk(root, into, &next, &seq)
		lookup := func(old int) int {
			if v, ok := number[old]; ok {
				return v
			}
			return into[old]
		}
		var buf bytes.Buffer
		for _, n := range seq {
			buf.Write(renumberedBy(ents[n].body, lookup))
			buf.WriteByte(';')
		}
		return buf.String()
	}
	commit := func(root int) {
		next := len(number) + 1
		var seq []int
		walk(root, number, &next, &seq)
	}
	for i := 0; i < len(roots); {
		j := i + 1
		for j < len(roots) && keys[roots[j]] == keys[roots[i]] {
			j++
		}
		group := append([]int(nil), roots[i:j]...)
		for len(group) > 0 {
			best, bestText := 0, preview(group[0])
			for g := 1; g < len(group); g++ {
				if t := preview(group[g]); t < bestText {
					best, bestText = g, t
				}
			}
			commit(group[best])
			group = append(group[:best], group[best+1:]...)
		}
		i = j
	}
	return number
}

// renumbered returns body with each reference replaced by its new number.
func renumbered(body []byte, number map[int]int) []byte {
	return renumberedBy(body, func(old int) int { return number[old] })
}

// renumberedBy is renumbered with the new numbers given by a function.
func renumberedBy(body []byte, number func(int) int) []byte {
	var out bytes.Buffer
	eachPart(body, func(text []byte, ref int) {
		if ref == 0 {
			out.Write(text)
			return
		}
		fmt.Fprintf(&out, "#%d", number(ref))
	})
	return out.Bytes()
}

// writeWrapped writes head, body and ";" as one instance, broken after a
// comma (outside strings) when a line would pass 80 columns. Continuation
// lines are indented by two spaces.
func writeWrapped(out *bytes.Buffer, head string, body []byte) {
	const width = 80
	line := len(head)
	out.WriteString(head)
	// Chunks end just after a comma outside strings.
	s := 0
	first := true
	flush := func(chunk []byte) {
		if line+len(chunk) > width && !first {
			out.WriteString("\n  ")
			line = 2
		}
		out.Write(chunk)
		line += len(chunk)
		first = false
	}
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '\'':
			i = stringEnd(body, i)
		case ',':
			flush(body[s : i+1])
			s = i + 1
		}
	}
	flush(append(append([]byte(nil), body[s:]...), ';'))
	out.WriteByte('\n')
}

func sameMultiset(a, b map[int]string) bool {
	if len(a) != len(b) {
		return false
	}
	count := map[string]int{}
	for _, k := range a {
		count[k]++
	}
	for _, k := range b {
		count[k]--
	}
	for _, c := range count {
		if c != 0 {
			return false
		}
	}
	return true
}

func isSpace(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

// isWord tells whether c can be part of a keyword, a number or an
// enumeration, which a space must keep apart from a neighbour of the same
// kind.
func isWord(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || isDigit(c) || c == '_' || c == '.' || c == '!' || c == '-' || c == '+'
}

func clip(b []byte) []byte {
	if len(b) > 40 {
		return b[:40]
	}
	return b
}
