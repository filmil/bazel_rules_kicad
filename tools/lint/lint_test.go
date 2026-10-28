// SPDX-License-Identifier: Apache-2.0

package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Each known-bad fixture must give at least one finding of its rule, so the
// lint fails on it (spec T3). The lines are where the finding points.
func TestBadFixturesFail(t *testing.T) {
	cases := []struct {
		file string
		rule string
		line int
	}{
		{"testdata/bad/shell_env.bzl", "shell-env", 5},
		{"testdata/bad/local_tag.BUILD", "forbidden-tag", 7},
		{"testdata/bad/no_sandbox_tag.BUILD", "forbidden-tag", 5},
		{"testdata/bad/network_tag.BUILD", "exception-tag", 4},
		{"testdata/bad/macro_network.bzl", "exception-tag", 4},
		{"testdata/bad/exec_req.bzl", "forbidden-tag", 6},
		{"testdata/bad/action_env.bazelrc", "host-env", 1},
		{"testdata/bad/action_env_home.bazelrc", "host-env", 1},
		{"testdata/bad/workflows/ci.yml", "host-env", 4},
	}
	for _, c := range cases {
		data, err := os.ReadFile(c.file)
		if err != nil {
			t.Fatal(err)
		}
		got := Scan(c.file, "pkg", data, Allowlist{})
		found := false
		for _, f := range got {
			if f.Rule == c.rule && f.Line == c.line {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: want a %s finding at line %d, got %v", c.file, c.rule, c.line, got)
		}
	}
}

// Every file under testdata/bad is covered by a case above, so a new
// fixture without a check cannot pass silently.
func TestEveryBadFixtureIsChecked(t *testing.T) {
	n := 0
	err := filepath.WalkDir("testdata/bad", func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			n++
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if n != 9 {
		t.Errorf("testdata/bad has %d files; update TestBadFixturesFail", n)
	}
}

func TestGoodFixturesPass(t *testing.T) {
	allow := Allowlist{"good:allowed_network": "a person approved it"}
	for _, f := range []string{"testdata/good/clean.BUILD", "testdata/good/clean.bazelrc"} {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		if got := Scan(f, "good", data, allow); len(got) != 0 {
			t.Errorf("%s: want no findings, got %v", f, got)
		}
	}
}

func TestAllowlistIsPerTarget(t *testing.T) {
	data, err := os.ReadFile("testdata/bad/network_tag.BUILD")
	if err != nil {
		t.Fatal(err)
	}
	if got := Scan("x/BUILD.bazel", "x", data, Allowlist{"x:fetches_things": "ok"}); len(got) != 0 {
		t.Errorf("allowlisted target: want no findings, got %v", got)
	}
	if got := Scan("x/BUILD.bazel", "x", data, Allowlist{"x:other": "ok"}); len(got) != 1 {
		t.Errorf("other target allowlisted: want 1 finding, got %v", got)
	}
}

func TestParseAllowlist(t *testing.T) {
	a, err := ParseAllowlist([]byte("# c\n\n//a/b:c  needs the network for X\n"))
	if err != nil {
		t.Fatal(err)
	}
	if a["a/b:c"] != "needs the network for X" {
		t.Errorf("got %v", a)
	}
	for _, bad := range []string{"//a:b\n", "a:b reason\n", "//ab reason\n"} {
		if _, err := ParseAllowlist([]byte(bad)); err == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
}

// Walk scans a tree, skips testdata and Bazel output links, and names files
// relative to the root.
func TestWalk(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("BUILD.bazel", "genrule(name = \"a\", tags = [\"local\"])\n")
	write("integration/BUILD.bazel", "sh_test(name = \"t\", tags = [\"requires-network\"])\n")
	write("testdata/BUILD.bazel", "genrule(name = \"skip\", tags = [\"local\"])\n")
	write(".bazelrc", "build --action_env=HOME\n")
	write("notes.txt", "use_default_shell_env = True\n")

	got, err := Walk(root, Allowlist{})
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, f := range got {
		lines = append(lines, f.String())
	}
	all := strings.Join(lines, "\n")
	for _, want := range []string{
		"BUILD.bazel:1: [forbidden-tag]",
		"integration/BUILD.bazel:1: [exception-tag]",
		".bazelrc:1: [host-env]",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("want %q in:\n%s", want, all)
		}
	}
	if len(got) != 3 {
		t.Errorf("want 3 findings, got:\n%s", all)
	}

	got, err = Walk(root, Allowlist{"integration:t": "approved"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Errorf("with the integration test allowlisted: want 2 findings, got %v", got)
	}
}
