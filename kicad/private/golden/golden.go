// SPDX-License-Identifier: Apache-2.0

// Package golden checks built files against checked-in golden files, and
// rewrites the golden files from the build (spec rule 4).
package golden

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Pair is one golden file and the built file it must equal.
type Pair struct {
	// Test is the label of the golden_test, such as //tests/bom:default_test.
	Test string
	// Built is the runfiles path of the built file.
	Built string
	// Golden is the runfiles path of the golden file.
	Golden string
	// Source is the golden file's path in the workspace, such as
	// tests/bom/expected_default.csv.
	Source string
}

// ParsePairs reads pairs written one per line, their fields separated by
// tabs, in the order of the Pair fields.
func ParsePairs(s string) ([]Pair, error) {
	var out []Pair
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 4 || f[0] == "" || f[1] == "" || f[2] == "" || f[3] == "" {
			return nil, fmt.Errorf("golden pair %q: want 4 fields separated by tabs", line)
		}
		out = append(out, Pair{Test: f[0], Built: f[1], Golden: f[2], Source: f[3]})
	}
	return out, nil
}

// FirstDifference returns the first line, counted from 1, where a and b
// differ, and the two lines; 0 when they are equal.
func FirstDifference(a, b []byte) (int, string, string) {
	if bytes.Equal(a, b) {
		return 0, "", ""
	}
	la := strings.SplitAfter(string(a), "\n")
	lb := strings.SplitAfter(string(b), "\n")
	for i := 0; ; i++ {
		var x, y string
		if i < len(la) {
			x = la[i]
		}
		if i < len(lb) {
			y = lb[i]
		}
		if x != y {
			return i + 1, x, y
		}
	}
}

// Check compares the built file with the golden file of p. resolve turns a
// runfiles path into a path to read.
func Check(p Pair, resolve func(string) (string, error), w io.Writer) error {
	built, err := readRunfile(resolve, p.Built)
	if err != nil {
		return err
	}
	want, err := readRunfile(resolve, p.Golden)
	if err != nil {
		return err
	}
	line, got, exp := FirstDifference(built, want)
	if line == 0 {
		fmt.Fprintf(w, "%s matches %s\n", p.Built, p.Source)
		return nil
	}
	fmt.Fprintf(w, "%s differs from the golden file %s, first at line %d:\n", p.Built, p.Source, line)
	// Quoted as they are, newline included: the files may differ only in it.
	fmt.Fprintf(w, "  built:  %q\n  golden: %q\n", got, exp)
	fmt.Fprintf(w, "If the change is intended, update the golden files with the module's update_goldens\n"+
		"target (bazel run //:update_goldens), and give the reason in the commit message.\n")
	return fmt.Errorf("%s: the built file differs from %s", p.Test, p.Source)
}

// Update writes the built file of each pair over its golden file in the
// workspace at root. It writes only the files that differ, and reports
// what it did.
func Update(pairs []Pair, root string, resolve func(string) (string, error), w io.Writer) (int, error) {
	changed := 0
	for _, p := range pairs {
		built, err := readRunfile(resolve, p.Built)
		if err != nil {
			return changed, err
		}
		dst := filepath.Join(root, filepath.FromSlash(p.Source))
		old, err := os.ReadFile(dst)
		if err != nil {
			return changed, fmt.Errorf("%s: the golden file: %w", p.Test, err)
		}
		if bytes.Equal(old, built) {
			fmt.Fprintf(w, "unchanged %s\n", p.Source)
			continue
		}
		if err := replace(dst, built); err != nil {
			return changed, fmt.Errorf("%s: %w", p.Test, err)
		}
		changed++
		fmt.Fprintf(w, "updated   %s (from %s)\n", p.Source, p.Test)
	}
	return changed, nil
}

// replace writes b over the file dst through a new file in the same
// directory, renamed over it: an interrupted run leaves dst whole. The new
// file gets dst's mode, so a read-only golden file stays read-only.
func replace(dst string, b []byte) error {
	fi, err := os.Stat(dst)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(dst), "."+filepath.Base(dst)+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // gone after the rename; cleans up on an error
	if _, err := f.Write(b); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp, fi.Mode().Perm()); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func readRunfile(resolve func(string) (string, error), rpath string) ([]byte, error) {
	p, err := resolve(rpath)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rpath, err)
	}
	return os.ReadFile(p)
}

var callRe = regexp.MustCompile(`(^|[^\w.])golden_test\s*\(`)

var nameRe = regexp.MustCompile(`(^|[(,\s])name\s*=\s*(?:"([^"]*)"|'([^']*)')`)

// TestNames returns the names of the golden_test calls in a BUILD file:
// for each call, the name attribute within its parentheses, wherever it
// is. Comments are dropped and strings are skipped when the parentheses
// are matched.
func TestNames(b []byte) []string {
	var names []string
	for _, m := range callRe.FindAllIndex(b, -1) {
		body := b[m[1]:]
		var args []byte // the call's arguments, without comments
		depth, closed := 1, false
		for i := 0; i < len(body) && !closed; i++ {
			switch c := body[i]; c {
			case '"', '\'':
				// Copy the string literal whole, with its escapes.
				j := i + 1
				for ; j < len(body) && body[j] != c; j++ {
					if body[j] == '\\' {
						j++
					}
				}
				if j >= len(body) {
					j = len(body) - 1
				}
				args = append(args, body[i:j+1]...)
				i = j
			case '#':
				for i < len(body) && body[i] != '\n' {
					i++
				}
				args = append(args, '\n')
			case '(', '[', '{':
				depth++
				args = append(args, c)
			case ')', ']', '}':
				depth--
				if depth == 0 {
					closed = true
				} else {
					args = append(args, c)
				}
			default:
				args = append(args, c)
			}
		}
		if !closed {
			continue
		}
		if n := nameRe.FindSubmatch(args); n != nil {
			names = append(names, string(n[2])+string(n[3]))
		}
	}
	return names
}

// nestedRepo names the files that make a directory the root of another
// module or repository.
var nestedRepo = []string{"MODULE.bazel", "REPO.bazel", "WORKSPACE", "WORKSPACE.bazel"}

// Unregistered returns the golden_test targets in the workspace at root
// that are not in registered, as labels. It reads the BUILD files of the
// module at root, and not those of a module or repository nested in it
// (such as integration/), of a directory .bazelignore names, or behind a
// symbolic link (such as the bazel-* output links).
func Unregistered(root string, registered []string) ([]string, error) {
	root, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	ignored := map[string]bool{}
	if b, err := os.ReadFile(filepath.Join(root, ".bazelignore")); err == nil {
		for _, l := range Lines(string(b)) {
			if !strings.HasPrefix(l, "#") {
				ignored[filepath.Clean(l)] = true
			}
		}
	}
	known := map[string]bool{}
	for _, l := range registered {
		known[l] = true
	}
	var missing []string
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if d.IsDir() {
			if p == root {
				return nil
			}
			if strings.HasPrefix(d.Name(), ".") || ignored[rel] {
				return filepath.SkipDir
			}
			for _, f := range nestedRepo {
				if exists(filepath.Join(p, f)) {
					return filepath.SkipDir
				}
			}
			return nil
		}
		// WalkDir does not follow symbolic links, so the bazel-* links,
		// which are not directories to it, are never entered.
		if d.Name() != "BUILD.bazel" && d.Name() != "BUILD" {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(filepath.Dir(rel))
		if pkg == "." {
			pkg = ""
		}
		for _, name := range TestNames(b) {
			if l := "//" + pkg + ":" + name; !known[l] {
				missing = append(missing, l)
			}
		}
		return nil
	})
	sort.Strings(missing)
	return missing, err
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Lines splits a list given one item per line.
func Lines(s string) []string {
	var out []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		if t := strings.TrimSpace(sc.Text()); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// Main runs the program as getenv says (GOLDEN_MODE, GOLDEN_PAIRS,
// GOLDEN_REGISTERED and BUILD_WORKSPACE_DIRECTORY), resolving runfiles
// paths with resolve and reporting to w.
func Main(getenv func(string) string, resolve func(string) (string, error), w io.Writer) error {
	pairs, err := ParsePairs(getenv("GOLDEN_PAIRS"))
	if err != nil {
		return err
	}
	if len(pairs) == 0 {
		return fmt.Errorf("GOLDEN_PAIRS is empty")
	}
	switch mode := getenv("GOLDEN_MODE"); mode {
	case "check":
		var failed []string
		for _, p := range pairs {
			if err := Check(p, resolve, w); err != nil {
				failed = append(failed, err.Error())
			}
		}
		if len(failed) > 0 {
			return fmt.Errorf("%s", strings.Join(failed, "\n"))
		}
		return nil
	case "update":
		root := getenv("BUILD_WORKSPACE_DIRECTORY")
		if root == "" {
			return fmt.Errorf("BUILD_WORKSPACE_DIRECTORY is not set: run this with `bazel run`")
		}
		missing, err := Unregistered(root, Lines(getenv("GOLDEN_REGISTERED")))
		if err != nil {
			return err
		}
		if len(missing) > 0 {
			return fmt.Errorf("these golden_test targets are not in the tests of this update_goldens target; "+
				"add them, so that their golden files are updated too:\n  %s", strings.Join(missing, "\n  "))
		}
		n, err := Update(pairs, root, resolve, w)
		if err != nil {
			return err
		}
		fmt.Fprintf(w, "%d of %d golden files updated. Give the reason in the commit message.\n", n, len(pairs))
		return nil
	default:
		return fmt.Errorf("GOLDEN_MODE=%q: want check or update", mode)
	}
}
