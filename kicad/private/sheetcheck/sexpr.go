// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"
)

// node is an S-expression: an atom (a symbol or a string) or a list.
type node struct {
	atom   string
	list   []*node
	isList bool
}

func parse(text string) (*node, error) {
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

func (p *parser) node() (*node, error) {
	if p.i >= len(p.s) {
		return nil, fmt.Errorf("unexpected end")
	}
	switch c := p.s[p.i]; {
	case c == '(':
		p.i++
		n := &node{isList: true}
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
			n.list = append(n.list, c)
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
				return &node{atom: b.String()}, nil
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
		return &node{atom: p.s[start:p.i]}, nil
	}
}

func (n *node) head() string {
	if !n.isList || len(n.list) == 0 || n.list[0].isList {
		return ""
	}
	return n.list[0].atom
}
