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
//  1. Each entity gets a content hash: its text, with each reference
//     replaced by the hash of the entity it names, and the number of
//     entities that refer to it.
//  2. The entities that nothing refers to are the roots. They are sorted
//     by hash.
//  3. A depth-first walk from the roots, in that order, visits each
//     entity's references in the order they appear in its text, and
//     numbers each entity from 1 the first time it reaches it.
//
// Whitespace outside strings is dropped (the writer's line breaks depend
// on the width of the old numbers), and each entity is written on a line
// of its own. The header and the trailer are kept as they are.
//
// The result is checked: it must hold the same multiset of entity hashes
// as the input, so the renumbering cannot change what the file says.
func CanonicalSTEP(b []byte) ([]byte, error) {
	start := bytes.Index(b, []byte("\nDATA;"))
	if start < 0 {
		return nil, fmt.Errorf("step: no DATA section")
	}
	start += len("\nDATA;")
	end := bytes.Index(b[start:], []byte("ENDSEC;"))
	if end < 0 {
		return nil, fmt.Errorf("step: the DATA section has no ENDSEC")
	}
	end += start
	ents, order, err := parseEntities(b[start:end])
	if err != nil {
		return nil, err
	}
	hashes, err := entityHashes(ents)
	if err != nil {
		return nil, err
	}
	number := canonicalNumbers(ents, order, hashes)

	var out bytes.Buffer
	out.Write(b[:start])
	out.WriteByte('\n')
	byNumber := make([]int, len(number))
	for old, n := range number {
		byNumber[n-1] = old
	}
	for i, old := range byNumber {
		fmt.Fprintf(&out, "#%d = ", i+1)
		writeRenumbered(&out, ents[old].body, number)
		out.WriteString(";\n")
	}
	out.Write(b[end:])

	// The check: the same entities, whatever their numbers.
	ents2, _, err := parseEntities(out.Bytes()[start : bytes.Index(out.Bytes()[start:], []byte("ENDSEC;"))+start])
	if err != nil {
		return nil, fmt.Errorf("step: the renumbered file does not parse: %w", err)
	}
	hashes2, err := entityHashes(ents2)
	if err != nil {
		return nil, err
	}
	if !sameMultiset(hashes, hashes2) {
		return nil, fmt.Errorf("step: renumbering changed the entities")
	}
	return out.Bytes(), nil
}

// entity is one instance of the DATA section.
type entity struct {
	// body is the text after "#N =", without whitespace outside strings
	// and without the final ";".
	body []byte
	// refs are the entities it names, in the order of its text.
	refs []int
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
		j := i + 1
		for j < len(data) && isDigit(data[j]) {
			j++
		}
		n, err := strconv.Atoi(string(data[i+1 : j]))
		if err != nil {
			return nil, nil, fmt.Errorf("step: a bad instance number at %q", clip(data[i:]))
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
		for k < len(data) && !closed {
			c := data[k]
			switch {
			case c == '\'':
				// A string, with '' for a quote: copied as it is.
				s := k
				for k++; k < len(data); k++ {
					if data[k] == '\'' {
						if k+1 < len(data) && data[k+1] == '\'' {
							k++
							continue
						}
						break
					}
				}
				if k >= len(data) {
					return nil, nil, fmt.Errorf("step: #%d has an unclosed string", n)
				}
				e.body = append(e.body, data[s:k+1]...)
				k++
			case c == '#':
				m := k + 1
				for m < len(data) && isDigit(data[m]) {
					m++
				}
				r, err := strconv.Atoi(string(data[k+1 : m]))
				if err != nil {
					return nil, nil, fmt.Errorf("step: #%d has a bad reference", n)
				}
				e.refs = append(e.refs, r)
				e.body = append(e.body, data[k:m]...)
				k = m
			case c == ';':
				closed = true
				k++
			case isSpace(c):
				k++
			default:
				e.body = append(e.body, c)
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

// entityHashes returns each entity's content hash: its body with each
// reference replaced by the referred entity's hash, and its number of
// referrers. A reference to an undefined entity, or a cycle, is an error.
func entityHashes(ents map[int]*entity) (map[int]string, error) {
	referrers := map[int]int{}
	for n, e := range ents {
		for _, r := range e.refs {
			if _, ok := ents[r]; !ok {
				return nil, fmt.Errorf("step: #%d refers to #%d, which is not defined", n, r)
			}
			referrers[r]++
		}
	}
	hashes := map[int]string{}
	visiting := map[int]bool{}
	var hash func(n int) (string, error)
	hash = func(n int) (string, error) {
		if h, ok := hashes[n]; ok {
			return h, nil
		}
		if visiting[n] {
			return "", fmt.Errorf("step: a reference cycle through #%d", n)
		}
		visiting[n] = true
		e := ents[n]
		var buf bytes.Buffer
		ri := 0
		body := e.body
		for i := 0; i < len(body); i++ {
			switch c := body[i]; {
			case c == '\'':
				j := i + 1
				for ; j < len(body); j++ {
					if body[j] == '\'' {
						if j+1 < len(body) && body[j+1] == '\'' {
							j++
							continue
						}
						break
					}
				}
				buf.Write(body[i : j+1])
				i = j
			case c == '#':
				j := i + 1
				for j < len(body) && isDigit(body[j]) {
					j++
				}
				h, err := hash(e.refs[ri])
				if err != nil {
					return "", err
				}
				ri++
				buf.WriteString("@" + h)
				i = j - 1
			default:
				buf.WriteByte(c)
			}
		}
		fmt.Fprintf(&buf, "|%d", referrers[n])
		sum := sha256.Sum256(buf.Bytes())
		h := fmt.Sprintf("%x", sum[:12])
		hashes[n] = h
		visiting[n] = false
		return h, nil
	}
	for n := range ents {
		if _, err := hash(n); err != nil {
			return nil, err
		}
	}
	return hashes, nil
}

// canonicalNumbers numbers the entities by a depth-first walk from the
// roots sorted by hash. Roots with the same hash keep their file order:
// their subgraphs, and so the output, are the same either way.
func canonicalNumbers(ents map[int]*entity, order []int, hashes map[int]string) map[int]int {
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
	sort.SliceStable(roots, func(i, j int) bool { return hashes[roots[i]] < hashes[roots[j]] })
	number := map[int]int{}
	var visit func(n int)
	visit = func(n int) {
		if _, ok := number[n]; ok {
			return
		}
		number[n] = len(number) + 1
		for _, r := range ents[n].refs {
			visit(r)
		}
	}
	for _, r := range roots {
		visit(r)
	}
	return number
}

// writeRenumbered writes body with each reference replaced by its new
// number.
func writeRenumbered(out *bytes.Buffer, body []byte, number map[int]int) {
	for i := 0; i < len(body); i++ {
		switch c := body[i]; {
		case c == '\'':
			j := i + 1
			for ; j < len(body); j++ {
				if body[j] == '\'' {
					if j+1 < len(body) && body[j+1] == '\'' {
						j++
						continue
					}
					break
				}
			}
			out.Write(body[i : j+1])
			i = j
		case c == '#':
			j := i + 1
			for j < len(body) && isDigit(body[j]) {
				j++
			}
			old, _ := strconv.Atoi(string(body[i+1 : j]))
			fmt.Fprintf(out, "#%d", number[old])
			i = j - 1
		default:
			out.WriteByte(c)
		}
	}
}

func sameMultiset(a, b map[int]string) bool {
	if len(a) != len(b) {
		return false
	}
	count := map[string]int{}
	for _, h := range a {
		count[h]++
	}
	for _, h := range b {
		count[h]--
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

func clip(b []byte) []byte {
	if len(b) > 40 {
		return b[:40]
	}
	return b
}
