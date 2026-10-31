// SPDX-License-Identifier: Apache-2.0

// Package sexpr parses the S-expressions KiCad writes: its schematics and
// boards. sheetcheck and modelcheck share it.
package sexpr

import (
	"fmt"
	"strings"
)

// Node is an S-expression: an atom (a symbol or a string) or a list.
type Node struct {
	Atom   string
	List   []*Node
	IsList bool
}

// Parse reads one S-expression, which must be all of text but for white
// space around it.
func Parse(text string) (*Node, error) {
	p := &parser{s: text}
	p.space()
	n, err := p.node()
	if err != nil {
		return nil, err
	}
	p.space()
	if p.i != len(p.s) {
		return nil, fmt.Errorf("text after the top list at byte %d", p.i)
	}
	return n, nil
}

type parser struct {
	s string
	i int
}

func (p *parser) space() {
	for p.i < len(p.s) && strings.ContainsRune(" \t\r\n", rune(p.s[p.i])) {
		p.i++
	}
}

func (p *parser) node() (*Node, error) {
	if p.i >= len(p.s) {
		return nil, fmt.Errorf("unexpected end")
	}
	switch c := p.s[p.i]; {
	case c == '(':
		p.i++
		n := &Node{IsList: true}
		for {
			p.space()
			if p.i >= len(p.s) {
				return nil, fmt.Errorf("unclosed list")
			}
			if p.s[p.i] == ')' {
				p.i++
				return n, nil
			}
			c, err := p.node()
			if err != nil {
				return nil, err
			}
			n.List = append(n.List, c)
		}
	case c == ')':
		return nil, fmt.Errorf("unexpected ) at byte %d", p.i)
	case c == '"':
		p.i++
		var b strings.Builder
		for {
			if p.i >= len(p.s) {
				return nil, fmt.Errorf("unclosed string")
			}
			c := p.s[p.i]
			p.i++
			if c == '"' {
				return &Node{Atom: b.String()}, nil
			}
			if c == '\\' {
				if p.i >= len(p.s) {
					return nil, fmt.Errorf("unclosed string")
				}
				c = p.s[p.i]
				p.i++
			}
			b.WriteByte(c)
		}
	default:
		start := p.i
		for p.i < len(p.s) && !strings.ContainsRune(" \t\r\n()\"", rune(p.s[p.i])) {
			p.i++
		}
		return &Node{Atom: p.s[start:p.i]}, nil
	}
}

// Head is a list's first atom: "footprint" for (footprint "R" ...), or
// "" for an atom or a list that starts with a list.
func (n *Node) Head() string {
	if !n.IsList || len(n.List) == 0 || n.List[0].IsList {
		return ""
	}
	return n.List[0].Atom
}

// Arg is a list's i-th atom after its head, or "".
func (n *Node) Arg(i int) string {
	if !n.IsList || len(n.List) == 0 {
		return ""
	}
	k := 0
	for _, c := range n.List[1:] {
		if !c.IsList {
			if k == i {
				return c.Atom
			}
			k++
		}
	}
	return ""
}
