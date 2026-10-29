// SPDX-License-Identifier: Apache-2.0

// Package files_test reads back the real outputs of kicad_drill.
package files_test

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	drill    = flag.String("drill", "", "the kicad_drill output directory, as a runfiles path")
	drillMap = flag.String("drill_map", "", "the output with separate_th and a gerberx2 map, as a runfiles path")
)

func dir(t *testing.T, path string) string {
	d, err := runfiles.Rlocation(path)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func names(t *testing.T, d string) string {
	entries, err := os.ReadDir(d)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return fmt.Sprint(out)
}

func read(t *testing.T, path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// One Excellon file for every hole, named after the board.
func TestOneFile(t *testing.T) {
	if got, want := names(t, dir(t, *drill)), "[main.drl]"; got != want {
		t.Errorf("outputs %s, want %s", got, want)
	}
}

// The header says Excellon in millimetres with decimal coordinates, and
// both its dates are the epoch (normalize, at the default
// source_date_epoch of 0).
func TestHeaderAtTheEpoch(t *testing.T) {
	drl := read(t, filepath.Join(dir(t, *drill), "main.drl"))
	for _, want := range []string{
		"M48\n",
		" date 1970-01-01T00:00:00+0000\n",
		"; FORMAT={-:-/ absolute / metric / decimal}\n",
		"; #@! TF.CreationDate,1970-01-01T00:00:00+00:00\n",
		"METRIC\n",
	} {
		if !strings.Contains(drl, want) {
			t.Errorf("main.drl lacks %q", want)
		}
	}
}

// The board's two hole sizes: its via, 0.3 mm, and its two through-hole
// pads, 1.0 mm.
func TestTheBoardsHoles(t *testing.T) {
	drl := read(t, filepath.Join(dir(t, *drill), "main.drl"))
	for _, want := range []string{"T1C0.300\n", "T2C1.000\n"} {
		if !strings.Contains(drl, want) {
			t.Errorf("main.drl lacks the tool %q", strings.TrimSpace(want))
		}
	}
}

// Plated and unplated holes apart, each with its Gerber map. kicad-cli
// writes the NPTH file though the board has no unplated hole.
func TestSeparateWithMaps(t *testing.T) {
	d := dir(t, *drillMap)
	want := "[main-NPTH-drl_map.gbr main-NPTH.drl main-PTH-drl_map.gbr main-PTH.drl]"
	if got := names(t, d); got != want {
		t.Errorf("outputs %s, want %s", got, want)
	}
	if pth := read(t, filepath.Join(d, "main-PTH.drl")); !strings.Contains(pth, "TF.FileFunction,Plated,") {
		t.Errorf("main-PTH.drl is not the plated file")
	}
	m := read(t, filepath.Join(d, "main-PTH-drl_map.gbr"))
	if !strings.Contains(m, "TF.CreationDate,1970-01-01T00:00:00+00:00") {
		t.Errorf("the map's date is not the epoch")
	}
}
