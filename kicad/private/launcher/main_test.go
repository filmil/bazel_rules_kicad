// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse(t *testing.T) {
	r, err := Parse([]string{"--home-env", "KICAD_CONFIG_HOME", "--env", "A=b=c", "--env-abs", "F=x/f.conf", "--output", "o/x", "--stdout", "o/out", "--", "tool", "-v", "--", "z"})
	if err != nil {
		t.Fatal(err)
	}
	if r.Tool != "tool" || strings.Join(r.Args, " ") != "-v -- z" || r.HomeEnv[0] != "KICAD_CONFIG_HOME" || r.Env[0] != "A=b=c" ||
		r.EnvAbs[0] != "F=x/f.conf" || r.Outputs[0] != "o/x" || r.Stdout != "o/out" {
		t.Errorf("got %+v", r)
	}
	for _, bad := range [][]string{{"tool"}, {"--"}, {"--env", "NOEQUALS", "--", "t"}, {"--env-abs", "F=", "--", "t"}, {"--env-abs", "=p", "--", "t"}, {"--stdout", "", "--", "t"}, {"--stdout", "a", "--stdout", "b", "--", "t"}, {"--bogus", "x", "--", "t"}, {"--output"}} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("%q: want an error", bad)
		}
	}
}

func TestEnvironPointsInsideHome(t *testing.T) {
	r := Request{HomeEnv: []string{"KICAD_CONFIG_HOME"}, Env: []string{"HOME=/never", "X=1", "KICAD9_TEMPLATE_DIR=/pinned"}, EnvAbs: []string{"REL=ext/f.conf", "ABS=/etc/f.conf"}, Outputs: []string{"a", "b"}, Stdout: "b"}
	env := map[string]string{}
	for _, kv := range Environ([]string{"HOME=/host/home", "PATH=/bin:/usr/bin", "XDG_CONFIG_HOME=/host/cfg", "DISPLAY=:0", "XDG_RUNTIME_DIR=/run/user/1", "KICAD9_SYMBOL_DIR=/host/sym", "FONTCONFIG_PATH=/etc/fonts", "FC_DEBUG=1"}, "/act/h", "/act", r) {
		k, v, _ := strings.Cut(kv, "=")
		env[k] = v
	}
	for k, want := range map[string]string{
		"XDG_CONFIG_HOME":   "/act/h/.config",
		"XDG_CACHE_HOME":    "/act/h/.cache",
		"TMPDIR":            "/act/h/tmp",
		"KICAD_CONFIG_HOME": "/act/h/KICAD_CONFIG_HOME",
		"PATH":              "/bin:/usr/bin",
		"X":                 "1",
		"RUN_TOOL_OUTPUTS":  "a\nb",
		// --env-abs: relative paths resolve against the working directory.
		"REL": "/act/ext/f.conf",
		// A fixed --env value passes even with a dropped prefix.
		"KICAD9_TEMPLATE_DIR": "/pinned",
		"RUN_TOOL_STDOUT":     "b",
		"ABS":                 "/etc/f.conf",
		// A fixed --env value wins, even over HOME.
		"HOME": "/never",
	} {
		if env[k] != want {
			t.Errorf("%s = %q, want %q", k, env[k], want)
		}
	}
	for _, k := range []string{"DISPLAY", "XDG_RUNTIME_DIR", "KICAD9_SYMBOL_DIR", "FONTCONFIG_PATH", "FC_DEBUG"} {
		if _, ok := env[k]; ok {
			t.Errorf("%s reached the tool", k)
		}
	}
}

// A host RUN_TOOL_ variable does not reach the tool: RUN_TOOL_STDOUT only
// with --stdout, and RUN_TOOL_OUTPUTS is the flags' value (issue #82).
func TestEnvironDropsHostRunToolVariables(t *testing.T) {
	host := []string{"RUN_TOOL_STDOUT=/host/out", "RUN_TOOL_OUTPUTS=/host/x", "RUN_TOOL_OTHER=1", "PATH=/bin"}
	env := Environ(host, "/act/h", "/act", Request{Outputs: []string{"a"}})
	got := map[string]string{}
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		got[k] = v
	}
	if v, ok := got["RUN_TOOL_STDOUT"]; ok {
		t.Errorf("host RUN_TOOL_STDOUT=%s reached the tool", v)
	}
	if v, ok := got["RUN_TOOL_OTHER"]; ok {
		t.Errorf("host RUN_TOOL_OTHER=%s reached the tool", v)
	}
	if got["RUN_TOOL_OUTPUTS"] != "a" {
		t.Errorf("RUN_TOOL_OUTPUTS is %q, want the flag's \"a\"", got["RUN_TOOL_OUTPUTS"])
	}
	if got["PATH"] != "/bin" {
		t.Errorf("PATH is %q: other variables must pass", got["PATH"])
	}
	// With --stdout, the flag's value, once.
	var stdout []string
	for _, kv := range Environ(host, "/act/h", "/act", Request{Stdout: "o.txt"}) {
		if strings.HasPrefix(kv, "RUN_TOOL_STDOUT=") {
			stdout = append(stdout, kv)
		}
	}
	if len(stdout) != 1 || stdout[0] != "RUN_TOOL_STDOUT=o.txt" {
		t.Errorf("with --stdout, RUN_TOOL_STDOUT entries are %q, want just RUN_TOOL_STDOUT=o.txt", stdout)
	}
}

// TestMain lets the test binary stand in for a tool: with
// LAUNCHER_TEST_TOOL set, it records what a tool sees and exits 3. No host
// shell or other host tool is run.
func TestMain(m *testing.M) {
	if os.Getenv("LAUNCHER_TEST_TOOL") == "stdout" {
		os.Stdout.WriteString("to stdout\n")
		os.Exit(0)
	}
	if os.Getenv("LAUNCHER_TEST_TOOL") == "1" {
		home, cfg, own := os.Getenv("HOME"), os.Getenv("XDG_CONFIG_HOME"), os.Getenv("TOOL_CONFIG")
		for _, p := range []string{filepath.Join(home, "written"), filepath.Join(cfg, "x"), filepath.Join(own, "y")} {
			if err := os.WriteFile(p, nil, 0o644); err != nil {
				os.Exit(10)
			}
		}
		// A read-only directory must not stop the cleanup.
		ro := filepath.Join(home, "ro")
		if os.Mkdir(ro, 0o755) != nil || os.WriteFile(filepath.Join(ro, "f"), nil, 0o444) != nil || os.Chmod(ro, 0o500) != nil {
			os.Exit(12)
		}
		if err := os.WriteFile(os.Args[1], []byte(home+"|"+cfg+"|"+own), 0o644); err != nil {
			os.Exit(11)
		}
		os.Exit(3)
	}
	os.Exit(m.Run())
}

// The tool sees a home inside the action directory, can write there, and
// the directory is gone afterwards. The tool's exit status comes back.
func TestRunUsesPrivateHome(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "seen")
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	r := Request{
		HomeEnv: []string{"TOOL_CONFIG"},
		Env:     []string{"LAUNCHER_TEST_TOOL=1"},
		Tool:    self,
		Args:    []string{out},
	}
	code, err := Run(r, dir)
	if err != nil {
		t.Fatal(err)
	}
	if code != 3 {
		t.Errorf("exit code %d, want the tool's 3", code)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(string(b), "|")
	if len(parts) != 3 || !strings.HasPrefix(parts[0], dir+string(filepath.Separator)+"_run_tool_home_") ||
		parts[1] != filepath.Join(parts[0], ".config") || parts[2] != filepath.Join(parts[0], "TOOL_CONFIG") {
		t.Errorf("tool saw %q", parts)
	}
	if _, err := os.Stat(parts[0]); !os.IsNotExist(err) {
		t.Errorf("home %s still exists after the run", parts[0])
	}
}

// --stdout sends the tool's standard output to a file.
func TestRunStdoutToFile(t *testing.T) {
	dir := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.txt")
	// A relative --stdout is relative to the action directory.
	code, err := Run(Request{Env: []string{"LAUNCHER_TEST_TOOL=stdout"}, Stdout: "out.txt", Tool: self}, dir)
	if err != nil || code != 0 {
		t.Fatalf("code %d, err %v", code, err)
	}
	if b, err := os.ReadFile(out); err != nil || string(b) != "to stdout\n" {
		t.Errorf("stdout file has %q (%v)", b, err)
	}
}
