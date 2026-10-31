// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	dir := t.TempDir()
	put := func(name, text string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	base := "(kicad_pcb\n\t(version 1)\n\t(net 0 \"\")\n)\n"
	o := put("o", base)
	a := put("a", strings.Replace(base, ")\n)", ")\n\t(net 1 \"A\")\n)", 1))
	b := put("b", strings.Replace(base, ")\n)", ")\n\t(net 2 \"B\")\n)", 1))
	if code, err := run([]string{o, a, b, "board.kicad_pcb"}); code != 0 || err != nil {
		t.Fatalf("clean merge: %d %v", code, err)
	}
	got, _ := os.ReadFile(a)
	if !strings.Contains(string(got), `"A"`) || !strings.Contains(string(got), `"B"`) {
		t.Errorf("ours after the merge:\n%s", got)
	}

	// A conflict: exit 1, ours left as it was.
	a2 := put("a2", strings.Replace(base, `""`, `"X"`, 1))
	b2 := put("b2", strings.Replace(base, `""`, `"Y"`, 1))
	if code, _ := run([]string{o, a2, b2}); code != 1 {
		t.Errorf("conflict: exit %d, want 1", code)
	}
	if got, _ := os.ReadFile(a2); !strings.Contains(string(got), `"X"`) {
		t.Errorf("a conflict changed ours")
	}
	// --out: the result goes there, ours stays.
	o3, a3, b3 := put("o3", base), put("a3", strings.Replace(base, ")\n)", ")\n\t(net 1 \"A\")\n)", 1)), put("b3", base)
	dest := filepath.Join(dir, "dest")
	if code, err := run([]string{"--out", dest, o3, a3, b3}); code != 0 || err != nil {
		t.Fatalf("--out: %d %v", code, err)
	}
	if got, _ := os.ReadFile(dest); !strings.Contains(string(got), `"A"`) {
		t.Errorf("--out wrote %q", got)
	}
	if code, _ := run([]string{o}); code != 2 {
		t.Errorf("one argument: exit %d, want 2", code)
	}
}
