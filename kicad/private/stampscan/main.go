// SPDX-License-Identifier: Apache-2.0

// Command stampscan is the test behind timestamp_scan_test (spec D2): it
// scans files for dates and times, and fails on any that is not the fixed
// epoch.
//
// It runs as a test, with these variables, each a list of runfiles paths
// (rlocationpaths, relative to TEST_SRCDIR), one per line:
//
//   - STAMP_FILES: the files to scan. A directory (a tree artifact) is
//     scanned whole.
//   - STAMP_SOURCES: files whose dates are allowed, such as the schematic
//     sheets whose title block holds the designer's date.
//
// and:
//
//   - STAMP_EPOCH: the epoch, in seconds since 1970 UTC
//     (--@rules_kicad//kicad:source_date_epoch).
//   - STAMP_EXPECT: "clean", or "stamped" for a known-bad file, which must
//     then have at least one date that is not the epoch.
//
// A date is found in these forms, with "-" or "/" between its parts, and
// not next to other digits (so the offsets of a PDF cross-reference table
// are not dates):
//
//   - 2026-10-29, 2026-10-29T08:31:59, 2026-10-29 08:31, each time with an
//     optional zone (Z, +01:00, +0100);
//   - PDF dates: D:20261029083159, with an optional zone (Z, +01'00');
//   - ISO 8601 basic: 20261029T083159, with an optional zone (Z, +0100).
//
// A date is the epoch when its date, and its time if it has one, are the
// epoch's in UTC: no zone, Z, or a zero offset. Any other date fails,
// unless the same text is in one of STAMP_SOURCES.
package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Found is a date in a file.
type Found struct {
	File string // as given, then the path below it for a directory
	Line int
	Text string
}

func (f Found) String() string { return fmt.Sprintf("%s:%d: %s", f.File, f.Line, f.Text) }

// form is one way of writing a date. Its groups are year, month, day,
// hour, minute, second and zone; the time groups may be empty.
type form struct {
	re *regexp.Regexp
	// zero holds the offsets that mean UTC in this form, besides none.
	zero map[string]bool
}

var utc = map[string]bool{"Z": true, "+00:00": true, "-00:00": true, "+0000": true, "-0000": true}

var forms = []form{
	{re: regexp.MustCompile(`(\d{4})[-/](\d{2})[-/](\d{2})(?:[T ](\d{2}):(\d{2})(?::(\d{2}))?(Z|[+-]\d{2}:?\d{2})?)?`), zero: utc},
	{re: regexp.MustCompile(`D:(\d{4})(\d{2})?(\d{2})?(\d{2})?(\d{2})?(\d{2})?(Z|[+-]\d{2}'?\d{2}'?)?`), zero: map[string]bool{"Z": true, "+00'00'": true, "-00'00'": true, "+00'00": true, "-00'00": true, "+0000": true, "-0000": true}},
	{re: regexp.MustCompile(`(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})(Z|[+-]\d{4})?`), zero: utc},
}

func isDigit(b byte) bool { return '0' <= b && b <= '9' }

// date is a date found, with the parts it has.
type date struct {
	start, end         int
	y, mo, d, h, mi, s int
	hasTime            bool
	zone               string
	zero               map[string]bool
}

func num(b []byte, def int) int {
	if len(b) == 0 {
		return def
	}
	n, _ := strconv.Atoi(string(b))
	return n
}

// dates returns every date in b. A match next to another digit, or with a
// month, day or time out of range, is not a date.
func dates(b []byte) []date {
	var out []date
	for _, f := range forms {
		for _, m := range f.re.FindAllSubmatchIndex(b, -1) {
			start, end := m[0], m[1]
			if start > 0 && isDigit(b[start-1]) || end < len(b) && isDigit(b[end]) {
				continue
			}
			g := func(i int) []byte {
				if m[2*i] < 0 {
					return nil
				}
				return b[m[2*i]:m[2*i+1]]
			}
			d := date{
				start: start, end: end,
				y: num(g(1), 0), mo: num(g(2), 1), d: num(g(3), 1),
				h: num(g(4), 0), mi: num(g(5), 0), s: num(g(6), 0),
				hasTime: g(4) != nil,
				zone:    string(g(7)),
				zero:    f.zero,
			}
			if d.mo < 1 || d.mo > 12 || d.d < 1 || d.d > 31 || d.h > 23 || d.mi > 59 || d.s > 60 {
				continue
			}
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out
}

// isEpoch tells whether d is the time t (UTC): its date, and its time if
// it has one, in UTC.
func (d date) isEpoch(t time.Time) bool {
	if d.zone != "" && !d.zero[d.zone] {
		return false
	}
	if d.y != t.Year() || d.mo != int(t.Month()) || d.d != t.Day() {
		return false
	}
	return !d.hasTime || d.h == t.Hour() && d.mi == t.Minute() && d.s == t.Second()
}

// Allowed returns the text of every date in the given files.
func Allowed(paths []string) (map[string]bool, error) {
	allowed := map[string]bool{}
	for _, p := range paths {
		err := walk(p, func(_ string, b []byte) {
			for _, d := range dates(b) {
				allowed[string(b[d.start:d.end])] = true
			}
		})
		if err != nil {
			return nil, err
		}
	}
	return allowed, nil
}

// Scan returns the dates in b that are not t and not allowed, and how many
// dates it found in all.
func Scan(name string, b []byte, t time.Time, allowed map[string]bool) (bad []Found, n int) {
	for _, d := range dates(b) {
		n++
		text := string(b[d.start:d.end])
		if d.isEpoch(t) || allowed[text] {
			continue
		}
		bad = append(bad, Found{File: name, Line: 1 + bytes.Count(b[:d.start], []byte("\n")), Text: text})
	}
	return bad, n
}

// walk calls f with each file at p: p itself, or every file below it.
func walk(p string, f func(name string, b []byte)) error {
	return filepath.WalkDir(p, func(q string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		b, err := os.ReadFile(q)
		if err != nil {
			return err
		}
		f(q, b)
		return nil
	})
}

// Result is what a scan found.
type Result struct {
	Files int     // files scanned
	Dates int     // dates found, the epoch's included
	Bad   []Found // dates that are not the epoch and not allowed
}

// Run scans the files at paths. Names in the result are relative to root.
func Run(root string, paths []string, t time.Time, allowed map[string]bool) (Result, error) {
	var r Result
	for _, p := range paths {
		err := walk(filepath.Join(root, p), func(name string, b []byte) {
			rel, err := filepath.Rel(root, name)
			if err != nil {
				rel = name
			}
			bad, n := Scan(rel, b, t, allowed)
			r.Files++
			r.Dates += n
			r.Bad = append(r.Bad, bad...)
		})
		if err != nil {
			return r, err
		}
	}
	return r, nil
}

func lines(v string) []string {
	var out []string
	for _, l := range strings.Split(v, "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

func check() error {
	root := os.Getenv("TEST_SRCDIR")
	n, err := strconv.ParseInt(os.Getenv("STAMP_EPOCH"), 10, 64)
	if err != nil || n < 0 {
		return fmt.Errorf("STAMP_EPOCH=%q: want seconds since 1970", os.Getenv("STAMP_EPOCH"))
	}
	t := time.Unix(n, 0).UTC()
	var sources []string
	for _, s := range lines(os.Getenv("STAMP_SOURCES")) {
		sources = append(sources, filepath.Join(root, s))
	}
	allowed, err := Allowed(sources)
	if err != nil {
		return err
	}
	r, err := Run(root, lines(os.Getenv("STAMP_FILES")), t, allowed)
	if err != nil {
		return err
	}
	if r.Files == 0 {
		return fmt.Errorf("no files to scan")
	}
	fmt.Printf("files scanned: %d; dates found: %d; dates allowed by %d sources: %d; epoch: %s\n",
		r.Files, r.Dates, len(sources), len(allowed), t.Format(time.RFC3339))
	for _, b := range r.Bad {
		fmt.Println("not the epoch:", b)
	}
	switch expect := os.Getenv("STAMP_EXPECT"); expect {
	case "clean":
		if len(r.Bad) > 0 {
			return fmt.Errorf("%d dates are not the epoch %s", len(r.Bad), t.Format(time.RFC3339))
		}
	case "stamped":
		if len(r.Bad) == 0 {
			return fmt.Errorf("want a date that is not the epoch (a known-bad file), found none")
		}
	default:
		return fmt.Errorf("STAMP_EXPECT=%q: want clean or stamped", expect)
	}
	return nil
}

func main() {
	if err := check(); err != nil {
		fmt.Fprintln(os.Stderr, "stampscan:", err)
		os.Exit(1)
	}
}
