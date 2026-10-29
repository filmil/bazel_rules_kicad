// SPDX-License-Identifier: Apache-2.0

package golden

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// workspace writes files under a new directory and returns it. Each file
// name is a path relative to the directory.
func workspace(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// under resolves runfiles paths against a directory.
func under(root string) func(string) (string, error) {
	return func(rpath string) (string, error) { return filepath.Join(root, filepath.FromSlash(rpath)), nil }
}

func TestParsePairs(t *testing.T) {
	got, err := ParsePairs("//a:t\t_main/a/out.csv\t_main/a/expected.csv\ta/expected.csv\n\n")
	if err != nil {
		t.Fatal(err)
	}
	want := []Pair{{Test: "//a:t", Built: "_main/a/out.csv", Golden: "_main/a/expected.csv", Source: "a/expected.csv"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v", got)
	}
	for _, bad := range []string{"//a:t\tx\ty", "//a:t\tx\t\tz"} {
		if _, err := ParsePairs(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestFirstDifference(t *testing.T) {
	for _, c := range []struct {
		a, b string
		line int
	}{
		{"x\ny\n", "x\ny\n", 0},
		{"x\ny\n", "x\nz\n", 2},
		{"x\n", "x\ny\n", 2},  // the golden file is longer
		{"x\ny", "x\ny\n", 2}, // only the last newline differs
	} {
		if line, _, _ := FirstDifference([]byte(c.a), []byte(c.b)); line != c.line {
			t.Errorf("%q vs %q: line %d, want %d", c.a, c.b, line, c.line)
		}
	}
}

func TestCheck(t *testing.T) {
	rf := workspace(t, map[string]string{
		"_main/p/out.csv":      "a,b\n1,2\n",
		"_main/p/expected.csv": "a,b\n1,2\n",
		"_main/p/stale.csv":    "a,b\n1,3\n",
	})
	good := Pair{Test: "//p:ok_test", Built: "_main/p/out.csv", Golden: "_main/p/expected.csv", Source: "p/expected.csv"}
	if err := Check(good, under(rf), &bytes.Buffer{}); err != nil {
		t.Errorf("equal files: %v", err)
	}
	var out bytes.Buffer
	stale := Pair{Test: "//p:stale_test", Built: "_main/p/out.csv", Golden: "_main/p/stale.csv", Source: "p/stale.csv"}
	err := Check(stale, under(rf), &out)
	if err == nil {
		t.Fatal("a stale golden file passed")
	}
	for _, want := range []string{"first at line 2", `"1,2\n"`, `"1,3\n"`, "bazel run //:update_goldens"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, out.String())
		}
	}
}

// A stale golden file in a copy of a workspace: Update rewrites it from
// the build, leaves the up-to-date one alone, and a check then passes.
func TestUpdateStaleGolden(t *testing.T) {
	rf := workspace(t, map[string]string{
		"_main/bom/out.csv": "Ref,Value\nR1,10k\n",
		"_main/pos/out.csv": "Ref,X\nR1,1.0\n",
	})
	ws := workspace(t, map[string]string{
		"bom/expected.csv": "Ref,Value\nR1,4k7\n", // stale
		"pos/expected.csv": "Ref,X\nR1,1.0\n",     // up to date
	})
	if err := os.Chmod(filepath.Join(ws, "bom/expected.csv"), 0o640); err != nil {
		t.Fatal(err)
	}
	pairs := []Pair{
		{Test: "//bom:bom_test", Built: "_main/bom/out.csv", Golden: "_main/bom/expected.csv", Source: "bom/expected.csv"},
		{Test: "//pos:pos_test", Built: "_main/pos/out.csv", Golden: "_main/pos/expected.csv", Source: "pos/expected.csv"},
	}
	var out bytes.Buffer
	n, err := Update(pairs, ws, under(rf), &out)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("%d files updated, want 1:\n%s", n, out.String())
	}
	b, _ := os.ReadFile(filepath.Join(ws, "bom/expected.csv"))
	if string(b) != "Ref,Value\nR1,10k\n" {
		t.Errorf("bom golden is %q", b)
	}
	if fi, _ := os.Stat(filepath.Join(ws, "bom/expected.csv")); fi.Mode().Perm() != 0o640 {
		t.Errorf("mode %v, want the golden file's own 0640", fi.Mode().Perm())
	}
	if !strings.Contains(out.String(), "updated   bom/expected.csv") || !strings.Contains(out.String(), "unchanged pos/expected.csv") {
		t.Errorf("output:\n%s", out.String())
	}
	// The updated workspace now checks clean: the golden is read from it.
	check := Pair{Test: "//bom:bom_test", Built: "_main/bom/out.csv", Golden: "bom/expected.csv", Source: "bom/expected.csv"}
	resolve := func(p string) (string, error) {
		if strings.HasPrefix(p, "_main/") {
			return filepath.Join(rf, p), nil
		}
		return filepath.Join(ws, p), nil
	}
	if err := Check(check, resolve, &bytes.Buffer{}); err != nil {
		t.Errorf("after the update: %v", err)
	}
}

// A golden file that is not there is an error, not a new file: the golden
// file and its test come into the tree together.
func TestUpdateMissingGolden(t *testing.T) {
	rf := workspace(t, map[string]string{"_main/a/out.csv": "x\n"})
	ws := workspace(t, map[string]string{"a/BUILD.bazel": ""})
	pairs := []Pair{{Test: "//a:t", Built: "_main/a/out.csv", Golden: "_main/a/expected.csv", Source: "a/expected.csv"}}
	if _, err := Update(pairs, ws, under(rf), &bytes.Buffer{}); err == nil {
		t.Error("a missing golden file was written")
	}
}

func TestUnregistered(t *testing.T) {
	ws := workspace(t, map[string]string{
		"BUILD.bazel": "golden_test(\n    name = \"root_test\",\n)\n",
		"tests/bom/BUILD.bazel": "golden_test(\n    name = \"default_test\",\n    size = \"small\",\n)\n\n" +
			"golden_test(\n    name = \"custom_test\",\n)\n",
		"tests/bom/notes.txt": "golden_test(name = \"no\")\n",
		// A nested module or repository has its own update_goldens.
		"integration/MODULE.bazel":        "",
		"integration/e2e/bom/BUILD.bazel": "golden_test(name = \"no\")\n",
		"vendor/REPO.bazel":               "",
		"vendor/BUILD.bazel":              "golden_test(name = \"no\")\n",
		// .bazelignore hides a directory from Bazel, and from the scan.
		".bazelignore":                "# comment\nscratch\n",
		"scratch/BUILD.bazel":         "golden_test(name = \"no\")\n",
		"elsewhere/out/x/BUILD.bazel": "golden_test(name = \"no\")\n",
	})
	// The bazel-* output links are symbolic links, which the scan does not
	// follow.
	if err := os.Symlink(filepath.Join(ws, "elsewhere/out"), filepath.Join(ws, "bazel-out")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".bazelignore"), []byte("# comment\nscratch\nelsewhere\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	missing, err := Unregistered(ws, []string{"//tests/bom:default_test"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"//:root_test", "//tests/bom:custom_test"}; !reflect.DeepEqual(missing, want) {
		t.Errorf("missing %v, want %v", missing, want)
	}
	// BUILD_WORKSPACE_DIRECTORY may itself be a symbolic link.
	link := filepath.Join(t.TempDir(), "ws")
	if err := os.Symlink(ws, link); err != nil {
		t.Fatal(err)
	}
	if got, err := Unregistered(link, []string{"//tests/bom:default_test"}); err != nil || len(got) != 2 {
		t.Errorf("through a link: %v, %v", got, err)
	}
}

func TestTestNames(t *testing.T) {
	src := `
golden_test(name = "one_line")

golden_test(
    # a comment that says name = "not_this"
    size = "small",
    src = ":x",   # ) a parenthesis in a comment
    golden = "a(b).csv",
    name = "name_last",
)

    golden_test(
        name = "indented",
    )

my_golden_test(name = "other_macro")
native.golden_test(name = "not_a_call_of_ours")
golden_test(name = 'single_quotes')
`
	if got, want := TestNames([]byte(src)), []string{"one_line", "name_last", "indented", "single_quotes"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// Main, as the program runs it under bazel test and bazel run.
func TestMain_(t *testing.T) {
	rf := workspace(t, map[string]string{
		"_main/a/out.csv":      "new\n",
		"_main/a/expected.csv": "old\n",
	})
	ws := workspace(t, map[string]string{
		"a/expected.csv": "old\n",
		"a/BUILD.bazel":  "golden_test(\n    name = \"t\",\n)\n",
	})
	pairs := "//a:t\t_main/a/out.csv\t_main/a/expected.csv\ta/expected.csv"
	for _, c := range []struct {
		name string
		env  map[string]string
		err  string // a part of the error; empty for none
	}{
		{"check fails", map[string]string{"GOLDEN_MODE": "check", "GOLDEN_PAIRS": pairs}, "differs from a/expected.csv"},
		{"no pairs", map[string]string{"GOLDEN_MODE": "check"}, "GOLDEN_PAIRS is empty"},
		{"bad mode", map[string]string{"GOLDEN_MODE": "fix", "GOLDEN_PAIRS": pairs}, "want check or update"},
		{"update outside bazel run", map[string]string{"GOLDEN_MODE": "update", "GOLDEN_PAIRS": pairs}, "BUILD_WORKSPACE_DIRECTORY"},
		{"update, test not listed", map[string]string{"GOLDEN_MODE": "update", "GOLDEN_PAIRS": pairs, "BUILD_WORKSPACE_DIRECTORY": ws}, "//a:t"},
		{"update", map[string]string{"GOLDEN_MODE": "update", "GOLDEN_PAIRS": pairs, "BUILD_WORKSPACE_DIRECTORY": ws, "GOLDEN_REGISTERED": "//a:t"}, ""},
	} {
		var out bytes.Buffer
		err := Main(func(k string) string { return c.env[k] }, under(rf), &out)
		switch {
		case c.err == "" && err != nil:
			t.Errorf("%s: %v", c.name, err)
		case c.err != "" && (err == nil || !strings.Contains(err.Error(), c.err)):
			t.Errorf("%s: err %v, want one with %q", c.name, err, c.err)
		}
	}
	if b, _ := os.ReadFile(filepath.Join(ws, "a/expected.csv")); string(b) != "new\n" {
		t.Errorf("after the update: %q", b)
	}
}

// A read-only golden file is updated and stays read-only; no temporary
// file is left beside it.
func TestUpdateReadOnly(t *testing.T) {
	rf := workspace(t, map[string]string{"_main/a/out.csv": "new\n"})
	ws := workspace(t, map[string]string{"a/expected.csv": "old\n"})
	dst := filepath.Join(ws, "a/expected.csv")
	if err := os.Chmod(dst, 0o444); err != nil {
		t.Fatal(err)
	}
	pairs := []Pair{{Test: "//a:t", Built: "_main/a/out.csv", Golden: "_main/a/expected.csv", Source: "a/expected.csv"}}
	if _, err := Update(pairs, ws, under(rf), &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(dst); fi.Mode().Perm() != 0o444 {
		t.Errorf("mode %v, want 0444", fi.Mode().Perm())
	}
	entries, _ := os.ReadDir(filepath.Dir(dst))
	if len(entries) != 1 {
		t.Errorf("files beside the golden file: %v", entries)
	}
}

// When only the last newline differs, the two lines printed differ too.
func TestCheckNewlineOnly(t *testing.T) {
	rf := workspace(t, map[string]string{"_main/out": "x\ny", "_main/want": "x\ny\n"})
	var out bytes.Buffer
	Check(Pair{Test: "//:t", Built: "_main/out", Golden: "_main/want", Source: "want"}, under(rf), &out)
	if !strings.Contains(out.String(), `built:  "y"`) || !strings.Contains(out.String(), `golden: "y\n"`) {
		t.Errorf("output:\n%s", out.String())
	}
}
