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
//     (--@hdlf_rules_kicad//kicad:source_date_epoch).
//   - STAMP_EXPECT: "clean", or "stamped" for a known-bad file, which must
//     then have at least one date that is not the epoch.
//
// A date is found in these forms, not next to other digits (so the
// offsets of a PDF cross-reference table are not dates):
//
//   - 2026-10-29 or 2026/10/29, with an optional time (T08:31, 08:31:59,
//     08:31:59.123) and zone (Z, +01:00, +0100);
//   - PDF dates: D:20261029, up to D:20261029083159, with an optional
//     zone (Z, +01'00');
//   - ISO 8601 basic: 20261029T0831 or 20261029T083159, with an optional
//     fraction and zone (Z, +0100, +01:00).
//
// A month or day out of range is not a date. A valid date with a time
// that is out of range, or followed by more digits, is a malformed time
// stamp, and never the epoch.
//
// A date is the epoch when every part it has (the date, then the hour,
// minute and second when present) is the epoch's in UTC: no zone, Z, or a
// zero offset, and no fraction but zeros. Any other date fails, unless the
// same text is a date without a time in one of STAMP_SOURCES: a designer
// writes dates, a tool writes times.
package main

import (
	"bytes"
	"fmt"
	"io"
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

// form is one way of writing a date, with the named groups y, mo, d, and
// optionally h, mi, s, frac and zone.
type form struct {
	re *regexp.Regexp
	// zero holds the offsets that mean UTC in this form, besides none.
	zero map[string]bool
}

var utc = map[string]bool{"Z": true, "+00:00": true, "-00:00": true, "+0000": true, "-0000": true}

// The time and zone after an extended date (2026-10-29).
const extendedTime = `(?:[T ](?P<h>\d{2}):(?P<mi>\d{2})(?::(?P<s>\d{2})(?:\.(?P<frac>\d+))?)?(?P<zone>Z|[+-]\d{2}:?\d{2})?)?`

var forms = []form{
	{re: regexp.MustCompile(`(?P<y>\d{4})-(?P<mo>\d{2})-(?P<d>\d{2})` + extendedTime), zero: utc},
	{re: regexp.MustCompile(`(?P<y>\d{4})/(?P<mo>\d{2})/(?P<d>\d{2})` + extendedTime), zero: utc},
	{
		re:   regexp.MustCompile(`D:(?P<y>\d{4})(?P<mo>\d{2})(?P<d>\d{2})(?:(?P<h>\d{2})(?:(?P<mi>\d{2})(?P<s>\d{2})?)?)?(?P<zone>Z|[+-]\d{2}'?\d{2}'?)?`),
		zero: map[string]bool{"Z": true, "+00'00'": true, "-00'00'": true, "+00'00": true, "-00'00": true, "+0000": true, "-0000": true},
	},
	{re: regexp.MustCompile(`(?P<y>\d{4})(?P<mo>\d{2})(?P<d>\d{2})T(?P<h>\d{2})(?P<mi>\d{2})(?P<s>\d{2})?(?:\.(?P<frac>\d+))?(?P<zone>Z|[+-]\d{2}:?\d{2})?`), zero: utc},
}

func isDigit(b byte) bool { return '0' <= b && b <= '9' }

// date is a date found, with the parts it has: -1 for a part it lacks.
type date struct {
	start, end         int
	y, mo, d, h, mi, s int
	frac               string
	zone               string
	zero               map[string]bool
	malformed          bool
}

// dates returns every date in b.
func dates(b []byte) []date {
	var out []date
	for _, f := range forms {
		names := f.re.SubexpNames()
		for _, m := range f.re.FindAllSubmatchIndex(b, -1) {
			start, end := m[0], m[1]
			if start > 0 && isDigit(b[start-1]) {
				continue
			}
			part := map[string]string{}
			for i, n := range names {
				if n != "" && m[2*i] >= 0 {
					part[n] = string(b[m[2*i]:m[2*i+1]])
				}
			}
			get := func(n string) int {
				v, ok := part[n]
				if !ok {
					return -1
				}
				x, _ := strconv.Atoi(v)
				return x
			}
			d := date{
				start: start, end: end,
				y: get("y"), mo: get("mo"), d: get("d"),
				h: get("h"), mi: get("mi"), s: get("s"),
				frac: part["frac"], zone: part["zone"], zero: f.zero,
			}
			if d.mo < 1 || d.mo > 12 || d.d < 1 || d.d > 31 {
				continue
			}
			hasTime := d.h >= 0
			if end < len(b) && isDigit(b[end]) {
				if !hasTime {
					continue // 2026-10-291 is not a date
				}
				d.malformed = true // 2026-10-29 08:31:59+01:000
			}
			if d.h > 23 || d.mi > 59 || d.s > 60 {
				d.malformed = true // 2026-10-29T25:00:00
			}
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].start < out[j].start })
	return out
}

// isEpoch tells whether d is the time t (UTC): every part d has is t's, in
// UTC.
func (d date) isEpoch(t time.Time) bool {
	if d.malformed || d.zone != "" && !d.zero[d.zone] || strings.Trim(d.frac, "0") != "" {
		return false
	}
	for _, p := range [][2]int{
		{d.y, t.Year()}, {d.mo, int(t.Month())}, {d.d, t.Day()},
		{d.h, t.Hour()}, {d.mi, t.Minute()}, {d.s, t.Second()},
	} {
		if p[0] >= 0 && p[0] != p[1] {
			return false
		}
	}
	return true
}

// Allowed returns the text of every date without a time in the given
// files.
func Allowed(paths []string) (map[string]bool, error) {
	allowed := map[string]bool{}
	for _, p := range paths {
		err := walk(p, func(_ string, b []byte) {
			for _, d := range dates(b) {
				if d.h < 0 && !d.malformed {
					allowed[string(b[d.start:d.end])] = true
				}
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

// Check runs the test as the variables of getenv say, and writes what it
// found to w.
func Check(getenv func(string) string, w io.Writer) error {
	root := getenv("TEST_SRCDIR")
	n, err := strconv.ParseInt(getenv("STAMP_EPOCH"), 10, 64)
	if err != nil || n < 0 {
		return fmt.Errorf("STAMP_EPOCH=%q: want seconds since 1970", getenv("STAMP_EPOCH"))
	}
	t := time.Unix(n, 0).UTC()
	expect := getenv("STAMP_EXPECT")
	if expect != "clean" && expect != "stamped" {
		return fmt.Errorf("STAMP_EXPECT=%q: want clean or stamped", expect)
	}
	var sources []string
	for _, s := range lines(getenv("STAMP_SOURCES")) {
		sources = append(sources, filepath.Join(root, s))
	}
	allowed, err := Allowed(sources)
	if err != nil {
		return err
	}
	r, err := Run(root, lines(getenv("STAMP_FILES")), t, allowed)
	if err != nil {
		return err
	}
	if r.Files == 0 {
		return fmt.Errorf("no files to scan")
	}
	fmt.Fprintf(w, "files scanned: %d; dates found: %d; dates allowed by %d sources: %d; epoch: %s\n",
		r.Files, r.Dates, len(sources), len(allowed), t.Format(time.RFC3339))
	for _, b := range r.Bad {
		fmt.Fprintln(w, "not the epoch:", b)
	}
	if expect == "clean" && len(r.Bad) > 0 {
		return fmt.Errorf("%d dates are not the epoch %s", len(r.Bad), t.Format(time.RFC3339))
	}
	if expect == "stamped" && len(r.Bad) == 0 {
		return fmt.Errorf("want a date that is not the epoch (a known-bad file), found none")
	}
	return nil
}

func main() {
	if err := Check(os.Getenv, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "stampscan:", err)
		os.Exit(1)
	}
}
