// SPDX-License-Identifier: Apache-2.0

// Command normalize removes the time stamps that KiCad writes into its
// outputs, so that the same inputs give the same bytes (spec D2).
//
// Usage:
//
//	normalize IN OUT
//
// IN is a file or a directory (a tree artifact, such as a set of Gerber
// files). OUT gets the same file or tree, with every time stamp replaced by
// the time in SOURCE_DATE_EPOCH (seconds since 1970, UTC; 0 when unset).
//
// The format of each file comes from its content, not its name, since KiCad
// names Gerber files .gtl, .gbs, .gm1 and more:
//
//   - Gerber X2: %TF.CreationDate,...*%, G04 #@! TF.CreationDate,...* and
//     the "G04 Created by KiCad (...) date ...*" comment;
//   - Excellon drill: the "; DRILL file {...} date ..." comment and
//     ; #@! TF.CreationDate,...;
//   - Gerber job (.gbrjob): "CreationDate" of the header;
//   - PDF: /CreationDate and /ModDate, with a value of the same length, so
//     the cross-reference offsets stay right;
//   - STEP: the time stamp of FILE_NAME;
//   - KiCad ERC and DRC JSON reports: the top-level "date".
//
// A file in none of these formats is copied unchanged. A file in one of
// them whose time stamp has a form this program does not know is an error,
// so a new KiCad version cannot slip a time stamp through.
package main

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

// Format is a kind of file the normalizer knows.
type Format string

const (
	Unknown Format = "unknown"
	Gerber  Format = "gerber"
	Drill   Format = "drill"
	Job     Format = "gerber-job"
	PDF     Format = "pdf"
	STEP    Format = "step"
	Report  Format = "kicad-report"
)

// Detect returns the format of a file from its first bytes.
func Detect(b []byte) Format {
	head := b
	if len(head) > 4096 {
		head = head[:4096]
	}
	switch {
	case bytes.HasPrefix(b, []byte("%PDF-")):
		return PDF
	case bytes.HasPrefix(b, []byte("ISO-10303-21;")):
		return STEP
	case bytes.HasPrefix(b, []byte("M48")):
		return Drill
	case bytes.HasPrefix(b, []byte("%TF.")) || bytes.HasPrefix(b, []byte("G04 ")) || bytes.HasPrefix(b, []byte("%FS")):
		return Gerber
	case bytes.HasPrefix(bytes.TrimSpace(head), []byte("{")) && bytes.Contains(head, []byte(`"GenerationSoftware"`)) && bytes.Contains(head, []byte(`"Header"`)):
		return Job
	case bytes.HasPrefix(bytes.TrimSpace(head), []byte("{")) && regexp.MustCompile(`"\$schema":\s*"https://schemas\.kicad\.org/(drc|erc)\.`).Match(head):
		return Report
	}
	return Unknown
}

// rule replaces the time stamp in each match of re: group 1 is the part
// before it, group 2 the time stamp, group 3 the part after it.
type rule struct {
	re     *regexp.Regexp
	stamp  func(t time.Time, old []byte) ([]byte, error)
	needed bool // the format always has it; a file without it is an error
}

func layout(l string) func(time.Time, []byte) ([]byte, error) {
	return func(t time.Time, _ []byte) ([]byte, error) { return []byte(t.Format(l)), nil }
}

// pdfDate keeps the length of the old value: PDF readers find objects by
// byte offset, so the file must not grow or shrink.
func pdfDate(t time.Time, old []byte) ([]byte, error) {
	base := t.Format("D:20060102150405")
	switch {
	case len(old) == len(base):
		return []byte(base), nil
	case len(old) == len(base)+1 && old[len(old)-1] == 'Z':
		return []byte(base + "Z"), nil
	case len(old) == len(base)+7: // D:YYYYMMDDHHmmSS+HH'mm'
		return []byte(base + "+00'00'"), nil
	case len(old) == len(base)+6: // D:YYYYMMDDHHmmSS+HH'mm
		return []byte(base + "+00'00"), nil
	}
	return nil, fmt.Errorf("a PDF date of an unknown form: %q", old)
}

var rules = map[Format][]rule{
	Gerber: {
		{re: regexp.MustCompile(`(%TF\.CreationDate,)([^*]*)(\*%)`), stamp: layout("2006-01-02T15:04:05+00:00")},
		{re: regexp.MustCompile(`(G04 #@! TF\.CreationDate,)([^*]*)(\*)`), stamp: layout("2006-01-02T15:04:05+00:00")},
		{re: regexp.MustCompile(`(G04 Created by KiCad \([^)]*\) date )([^*]*)(\*)`), stamp: layout("2006-01-02 15:04:05")},
	},
	Drill: {
		{re: regexp.MustCompile(`(; DRILL file \{[^}]*\} date )([^\r\n]*)(\r?\n)`), stamp: layout("2006-01-02T15:04:05+0000"), needed: true},
		{re: regexp.MustCompile(`(; #@! TF\.CreationDate,)([^\r\n]*)(\r?\n)`), stamp: layout("2006-01-02T15:04:05+00:00")},
	},
	Job: {
		{re: regexp.MustCompile(`("CreationDate":\s*")([^"]*)(")`), stamp: layout("2006-01-02T15:04:05+00:00"), needed: true},
	},
	PDF: {
		{re: regexp.MustCompile(`(/CreationDate\s*\()([^)]*)(\))`), stamp: pdfDate},
		{re: regexp.MustCompile(`(/ModDate\s*\()([^)]*)(\))`), stamp: pdfDate},
	},
	STEP: {
		{re: regexp.MustCompile(`(FILE_NAME\(\s*'(?:[^']|'')*'\s*,\s*')([^']*)(')`), stamp: layout("2006-01-02T15:04:05"), needed: true},
	},
	Report: {
		// The top-level key only: KiCad writes it at an indent of 4.
		{re: regexp.MustCompile(`(\n    "date":\s*")([^"]*)(")`), stamp: layout("2006-01-02T15:04:05+0000"), needed: true},
	},
}

// stampRe is what every time stamp KiCad writes looks like: a year, then
// digits and separators. A match whose time stamp is not of this form is
// left for an error rather than replaced blindly.
var stampRe = regexp.MustCompile(`^(D:)?\d{4}-?\d{2}-?\d{2}[T ]?\d{2}:?\d{2}:?\d{2}([+-]\d{2}'?:?\d{2}'?|Z)?$`)

// Normalize returns b with its time stamps replaced by t.
func Normalize(b []byte, t time.Time) ([]byte, Format, error) {
	f := Detect(b)
	out := b
	for _, r := range rules[f] {
		found := false
		var err error
		out = r.re.ReplaceAllFunc(out, func(m []byte) []byte {
			sub := r.re.FindSubmatch(m)
			found = true
			if !stampRe.Match(sub[2]) {
				err = fmt.Errorf("%s: %q is not a time stamp this program knows", f, sub[2])
				return m
			}
			s, e := r.stamp(t, sub[2])
			if e != nil {
				err = e
				return m
			}
			return bytes.Join([][]byte{sub[1], s, sub[3]}, nil)
		})
		if err != nil {
			return nil, f, err
		}
		if r.needed && !found {
			return nil, f, fmt.Errorf("%s: no time stamp matches %s", f, r.re)
		}
	}
	if f == PDF && len(out) != len(b) {
		return nil, f, fmt.Errorf("pdf: the length changed from %d to %d", len(b), len(out))
	}
	return out, f, nil
}

// Epoch reads SOURCE_DATE_EPOCH: seconds since 1970, UTC. Unset is 0.
func Epoch(v string) (time.Time, error) {
	if v == "" {
		return time.Unix(0, 0).UTC(), nil
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n < 0 {
		return time.Time{}, fmt.Errorf("SOURCE_DATE_EPOCH=%q: want seconds since 1970", v)
	}
	return time.Unix(n, 0).UTC(), nil
}

func normalizeFile(in, out string, t time.Time) error {
	b, err := os.ReadFile(in)
	if err != nil {
		return err
	}
	n, _, err := Normalize(b, t)
	if err != nil {
		return fmt.Errorf("%s: %w", in, err)
	}
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		return err
	}
	return os.WriteFile(out, n, 0o644)
}

// Run normalizes in (a file or a directory) into out.
func Run(in, out string, t time.Time) error {
	fi, err := os.Stat(in)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return normalizeFile(in, out, t)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	return filepath.WalkDir(in, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(in, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(out, rel), 0o755)
		}
		return normalizeFile(p, filepath.Join(out, rel), t)
	})
}

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: normalize IN OUT")
		os.Exit(2)
	}
	t, err := Epoch(os.Getenv("SOURCE_DATE_EPOCH"))
	if err == nil {
		err = Run(os.Args[1], os.Args[2], t)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "normalize:", err)
		os.Exit(1)
	}
}
