// SPDX-License-Identifier: Apache-2.0

// Package lint finds hermeticity escapes in Bazel files.
//
// It reads BUILD files, .bzl files, MODULE.bazel, .bazelrc files and CI
// workflows, and reports each use of a forbidden flag, attribute or tag.
// The rules come from the specification "Hermetic hardware builds with
// Bazel": actions must not see the host PATH or HOME, must not run outside
// the sandbox, and must not use the network unless a person approved an
// exception.
package lint

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Finding is one forbidden use, at a file and line.
type Finding struct {
	File string
	Line int
	Rule string
	Msg  string
}

func (f Finding) String() string {
	return fmt.Sprintf("%s:%d: [%s] %s", f.File, f.Line, f.Rule, f.Msg)
}

// Kind says how a file is read.
type Kind int

const (
	// KindNone is a file the lint does not read.
	KindNone Kind = iota
	// KindStarlark is a BUILD, .bzl or MODULE.bazel file.
	KindStarlark
	// KindRC is a .bazelrc file.
	KindRC
	// KindWorkflow is a CI workflow.
	KindWorkflow
)

// KindOf returns how the lint reads the file at path. Test fixtures use the
// suffixes .BUILD and .bazelrc so that Bazel does not take them for
// packages or settings.
func KindOf(path string) Kind {
	base := filepath.Base(path)
	switch {
	case base == "BUILD" || base == "BUILD.bazel" || base == "MODULE.bazel" ||
		strings.HasSuffix(base, ".BUILD") || strings.HasSuffix(base, ".bzl"):
		return KindStarlark
	case base == ".bazelrc" || strings.HasSuffix(base, ".bazelrc"):
		return KindRC
	case (strings.HasSuffix(base, ".yml") || strings.HasSuffix(base, ".yaml")) &&
		strings.Contains(filepath.ToSlash(path), "workflows/"):
		return KindWorkflow
	}
	return KindNone
}

// ExceptionTags may appear only on targets listed in the allowlist.
var ExceptionTags = []string{"requires-network", "requires-root", "requires-fakeroot"}

// ForbiddenTags may never appear.
var ForbiddenTags = []string{"local", "no-sandbox"}

var (
	// --action_env=PATH, --action_env PATH, --action_env=HOME=/x, and the
	// same for --host_action_env and --test_env.
	reHostEnv  = regexp.MustCompile(`--(?:host_)?(?:action|test)_env[= ](PATH|HOME)\b`)
	reShellEnv = regexp.MustCompile(`use_default_shell_env\s*=\s*True`)
	reName     = regexp.MustCompile(`\bname\s*=\s*"([^"]*)"`)
	// A dict key such as "no-sandbox": "1" in execution_requirements.
	reExecReq = regexp.MustCompile(`["'](local|no-sandbox|requires-network)["']\s*:`)
	reString  = regexp.MustCompile(`"([^"\\]*)"|'([^'\\]*)'`)
)

// Allowlist maps a target, as "path/to/package:name", to the reason a person
// gave for its exception tag.
type Allowlist map[string]string

// ParseAllowlist reads lines of the form "//pkg:name  reason". Blank lines
// and lines starting with # are skipped. A line without a reason is an
// error: the reason is what the person approved.
func ParseAllowlist(data []byte) (Allowlist, error) {
	a := Allowlist{}
	s := bufio.NewScanner(bytes.NewReader(data))
	n := 0
	for s.Scan() {
		n++
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 || !strings.HasPrefix(f[0], "//") || !strings.Contains(f[0], ":") {
			return nil, fmt.Errorf("allowlist line %d: want \"//pkg:name reason\", got %q", n, line)
		}
		a[strings.TrimPrefix(f[0], "//")] = strings.Join(f[1:], " ")
	}
	return a, s.Err()
}

// Scan returns the findings for one file. pkg is the file's Bazel package
// ("" for the root, "integration/foo" inside the integration module), used
// to look targets up in the allowlist.
func Scan(path, pkg string, data []byte, allow Allowlist) []Finding {
	switch KindOf(path) {
	case KindStarlark:
		return scanStarlark(path, pkg, data, allow)
	case KindRC, KindWorkflow:
		return scanFlags(path, data)
	}
	return nil
}

func scanFlags(path string, data []byte) []Finding {
	var out []Finding
	for i, line := range strings.Split(string(data), "\n") {
		code := line
		if j := strings.Index(code, "#"); j >= 0 {
			code = code[:j]
		}
		if m := reHostEnv.FindStringSubmatch(code); m != nil {
			out = append(out, Finding{path, i + 1, "host-env",
				fmt.Sprintf("passes the host %s into actions; tools come from toolchains", m[1])})
		}
	}
	return out
}

func scanStarlark(path, pkg string, data []byte, allow Allowlist) []Finding {
	clean := stripComments(string(data))
	var out []Finding
	lines := strings.Split(clean, "\n")
	for i, line := range lines {
		if reShellEnv.MatchString(line) {
			out = append(out, Finding{path, i + 1, "shell-env",
				"use_default_shell_env = True gives actions the host PATH"})
		}
		if m := reHostEnv.FindStringSubmatch(line); m != nil {
			out = append(out, Finding{path, i + 1, "host-env",
				fmt.Sprintf("passes the host %s into actions; tools come from toolchains", m[1])})
		}
	}
	for i, line := range lines {
		if m := reExecReq.FindStringSubmatch(line); m != nil {
			out = append(out, Finding{path, i + 1, "forbidden-tag",
				fmt.Sprintf("execution requirement %q runs the action outside the sandbox or with the network", m[1])})
		}
	}
	names := callNames(lines)
	for _, t := range tagsIn(lines, 0, len(lines)-1) {
		for _, bad := range ForbiddenTags {
			if t.value == bad {
				out = append(out, Finding{path, t.line, "forbidden-tag",
					fmt.Sprintf("tag %q runs the action outside the sandbox", bad)})
			}
		}
		for _, ex := range ExceptionTags {
			if t.value != ex {
				continue
			}
			// A tag inside a named top-level call belongs to that target. A
			// tag anywhere else (a macro body in a .bzl file) is allowlisted
			// by the file itself, as "//pkg:file.bzl".
			label := pkg + ":" + filepath.Base(path)
			if n, ok := names[t.line]; ok && n != "" {
				label = pkg + ":" + n
			}
			if _, ok := allow[label]; !ok {
				out = append(out, Finding{path, t.line, "exception-tag",
					fmt.Sprintf("tag %q on //%s needs a line in tools/lint/hermetic_exceptions.txt, approved by a person", ex, label)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Line < out[j].Line })
	return out
}

// stripComments blanks out # comments outside strings, keeping line breaks
// so line numbers stay right.
func stripComments(s string) string {
	var b strings.Builder
	var quote byte
	inComment := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '\n':
			inComment = false
			b.WriteByte(c)
		case inComment:
			b.WriteByte(' ')
		case quote != 0:
			b.WriteByte(c)
			if c == '\\' && i+1 < len(s) {
				i++
				b.WriteByte(s[i])
			} else if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
			b.WriteByte(c)
		case c == '#':
			inComment = true
			b.WriteByte(' ')
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

type tag struct {
	value string
	line  int
}

// callNames maps each line inside a top-level call (a rule or macro use in
// a BUILD file) to the call's name attribute. It is not a Starlark parser:
// it matches brackets outside strings, which is enough for BUILD files.
// Lines inside a def or outside any call are not in the map.
func callNames(lines []string) map[int]string {
	out := map[int]string{}
	for i := 0; i < len(lines); i++ {
		l := lines[i]
		if l == "" || l[0] == ' ' || l[0] == '\t' || strings.HasPrefix(l, "def ") || !strings.Contains(l, "(") {
			continue
		}
		depth, end := 0, i
		for j := i; j < len(lines); j++ {
			depth += bracketDelta(lines[j])
			end = j
			if depth <= 0 {
				break
			}
		}
		name := ""
		if m := reName.FindStringSubmatch(strings.Join(lines[i:end+1], "\n")); m != nil {
			name = m[1]
		}
		for j := i; j <= end; j++ {
			out[j+1] = name
		}
		i = end
	}
	return out
}

func bracketDelta(line string) int {
	d := 0
	var quote byte
	for i := 0; i < len(line); i++ {
		c := line[i]
		if quote != 0 {
			if c == '\\' {
				i++
			} else if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '(', '[', '{':
			d++
		case ')', ']', '}':
			d--
		}
	}
	return d
}

var reTagsStart = regexp.MustCompile(`\btags\s*=\s*\[`)

// tagsIn returns the strings inside every `tags = [...]` list between lines
// from and to.
func tagsIn(lines []string, from, to int) []tag {
	var out []tag
	for i := from; i <= to; i++ {
		loc := reTagsStart.FindStringIndex(lines[i])
		if loc == nil {
			continue
		}
		depth := 0
		for j := i; j <= to; j++ {
			text := lines[j]
			if j == i {
				text = text[loc[1]-1:]
			}
			for _, m := range reString.FindAllStringSubmatch(text, -1) {
				v := m[1]
				if v == "" {
					v = m[2]
				}
				out = append(out, tag{v, j + 1})
			}
			depth += bracketDelta(text)
			if depth <= 0 {
				i = j
				break
			}
		}
	}
	return out
}

// Walk scans every file under root that the lint reads, and returns the
// findings with paths relative to root. Directories named testdata, .git,
// and Bazel's output symlinks are skipped. Packages inside the integration
// module are looked up in the allowlist as "integration/...".
func Walk(root string, allow Allowlist) ([]Finding, error) {
	var out []Finding
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != root && (name == ".git" || name == "testdata" || strings.HasPrefix(name, "bazel-")) {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Type()&os.ModeSymlink != 0 || KindOf(p) == KindNone {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		pkg := filepath.ToSlash(filepath.Dir(rel))
		if pkg == "." {
			pkg = ""
		}
		out = append(out, Scan(filepath.ToSlash(rel), pkg, data, allow)...)
		return nil
	})
	return out, err
}
