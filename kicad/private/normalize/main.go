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
//   - STEP: the time stamp of FILE_NAME, and the numbering of the
//     entities, made canonical (step.go, issue #63);
//   - KiCad ERC and DRC JSON reports: the top-level "date";
//   - KiCad netlists (s-expression): the date of the (design ...) header,
//     and the paths of its source and of each library, which name the
//     action's directory, cut to the file name;
//   - PNG: the chunks of text, time and Exif data are dropped (png.go).
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
	Netlist Format = "kicad-netlist"
)

var netlistRe = regexp.MustCompile(`^\(export\s+\(version\s+"[^"]*"\)`)

var reportSchemaRe = regexp.MustCompile(`"\$schema":\s*"https://schemas\.kicad\.org/(drc|erc)\.`)

// Detect returns the format of a file from its first bytes. A UTF-8 byte
// order mark and leading white space are skipped.
func Detect(b []byte) Format {
	head := b
	if len(head) > 4096 {
		head = head[:4096]
	}
	head = bytes.TrimLeft(bytes.TrimPrefix(head, []byte("\xef\xbb\xbf")), " \t\r\n")
	switch {
	case bytes.HasPrefix(b, pngSignature):
		return PNG
	case bytes.HasPrefix(head, []byte("%PDF-")):
		return PDF
	case bytes.HasPrefix(head, []byte("ISO-10303-21;")):
		return STEP
	case bytes.HasPrefix(head, []byte("M48")):
		return Drill
	case bytes.HasPrefix(head, []byte("%TF.")) || bytes.HasPrefix(head, []byte("G04 ")) || bytes.HasPrefix(head, []byte("%FS")):
		return Gerber
	case bytes.HasPrefix(head, []byte("{")) && bytes.Contains(head, []byte(`"GenerationSoftware"`)) && bytes.Contains(head, []byte(`"Header"`)):
		return Job
	case bytes.HasPrefix(head, []byte("{")) && reportSchemaRe.Match(head):
		return Report
	case netlistRe.Match(head):
		return Netlist
	}
	return Unknown
}

// rule replaces the time stamp in each match of re: group 1 is the part
// before it, group 2 the time stamp, group 3 the part after it.
type rule struct {
	re     *regexp.Regexp
	stamp  func(t time.Time, old []byte) ([]byte, error)
	needed bool // the format always has it; a file without it is an error
	path   bool // group 2 is a path, cut to its file name; not a time stamp
}

// base cuts a path to its last element. Paths are POSIX (v1 runs on Linux
// only); a backslash is an escape in KiCad's strings, not a separator.
func base(_ time.Time, old []byte) ([]byte, error) {
	if i := bytes.LastIndexByte(old, '/'); i >= 0 {
		return old[i+1:], nil
	}
	return old, nil
}

// needAny lists the formats where at least one rule must match: KiCad
// always writes a time stamp there, but which forms depends on options
// (Gerber X2 attributes can be off).
var needAny = map[Format]bool{Gerber: true}

func layout(l string) func(time.Time, []byte) ([]byte, error) {
	return func(t time.Time, _ []byte) ([]byte, error) { return []byte(t.Format(l)), nil }
}

// pdfDate keeps the length of the old value: PDF readers find objects by
// byte offset, so the file must not grow or shrink.
func pdfDate(t time.Time, old []byte) ([]byte, error) {
	base := t.Format("D:20060102150405")
	if len(base) != 16 {
		return nil, fmt.Errorf("the year of %s does not fit a PDF date", t)
	}
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
		{re: regexp.MustCompile(`(/CreationDate\s*\()([^)]*)(\))`), stamp: pdfDate, needed: true},
		{re: regexp.MustCompile(`(/ModDate\s*\()([^)]*)(\))`), stamp: pdfDate},
	},
	STEP: {
		{re: regexp.MustCompile(`(FILE_NAME\(\s*'(?:[^']|'')*'\s*,\s*')([^']*)(')`), stamp: layout("2006-01-02T15:04:05"), needed: true},
	},
	Report: {
		// The top-level key only: KiCad indents by 4, so keys of nested
		// objects start at 8 or more.
		{re: regexp.MustCompile(`(?m)(^    "date":\s*")([^"]*)(")`), stamp: layout("2006-01-02T15:04:05+0000"), needed: true},
	},
	Netlist: {
		// The (design ...) header: (source "...") then (date "..."). The
		// title blocks have dates of their own, the designer's; they stay.
		// Quoted strings may hold backslash escapes.
		{re: regexp.MustCompile(`(\(design\s+\(source\s+")((?:[^"\\]|\\.)*)(")`), stamp: base, needed: true, path: true},
		{re: regexp.MustCompile(`(\(design\s+\(source\s+"(?:[^"\\]|\\.)*"\)\s+\(date\s+")([^"]*)(")`), stamp: layout("2006-01-02T15:04:05+0000"), needed: true},
		// Only a library's path, not text elsewhere that looks like one.
		{re: regexp.MustCompile(`(\(library\s+\(logical\s+"(?:[^"\\]|\\.)*"\)\s+\(uri\s+")((?:[^"\\]|\\.)*)(")`), stamp: base, path: true},
	},
}

// stampRe is what every time stamp KiCad writes looks like: a year, then
// digits and separators. A match whose time stamp is not of this form is
// left for an error rather than replaced blindly.
var stampRe = regexp.MustCompile(`^(D:)?\d{4}-?\d{2}-?\d{2}[T ]?\d{2}:?\d{2}:?\d{2}([+-]\d{2}'?:?\d{2}'?|Z)?$`)

// Normalize returns b with its time stamps replaced by t.
func Normalize(b []byte, t time.Time) ([]byte, Format, error) {
	f := Detect(b)
	if f == PNG {
		out, err := StripPNG(b)
		return out, f, err
	}
	out := b
	anyFound := false
	for _, r := range rules[f] {
		found := false
		var err error
		out = r.re.ReplaceAllFunc(out, func(m []byte) []byte {
			sub := r.re.FindSubmatch(m)
			found = true
			if !r.path && !stampRe.Match(sub[2]) {
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
		anyFound = anyFound || found
	}
	if needAny[f] && !anyFound {
		return nil, f, fmt.Errorf("%s: no time stamp of a form this program knows", f)
	}
	if f == STEP {
		c, err := CanonicalSTEP(out)
		if err != nil {
			return nil, f, err
		}
		out = c
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
	// Up to the end of year 9999: every format here has a 4-digit year.
	if err != nil || n < 0 || n >= 253402300800 {
		return time.Time{}, fmt.Errorf("SOURCE_DATE_EPOCH=%q: want seconds since 1970, before the year 10000", v)
	}
	return time.Unix(n, 0).UTC(), nil
}

func normalizeFile(in, out string, t time.Time) error {
	fi, err := os.Stat(in)
	if err != nil {
		return err
	}
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
	// The mode of the input, so an executable stays one.
	return os.WriteFile(out, n, fi.Mode().Perm()|0o200)
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
