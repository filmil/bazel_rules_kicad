// SPDX-License-Identifier: Apache-2.0

// Package files_test checks the Gerber files kicad_gerbers writes for the
// two-layer board of //tests/design.
package files_test

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

var (
	all  = flag.String("all", "", "the output of :gerbers (default layers), as a runfiles path")
	some = flag.String("some", "", "the output of :gerbers_some (F.Cu, Edge.Cuts), as a runfiles path")
)

func files(t *testing.T, rpath string) (string, []string) {
	t.Helper()
	dir, err := runfiles.Rlocation(rpath)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return dir, names
}

// The board has two copper layers: kicad-cli skips In1.Cu to In30.Cu. The
// extensions are Protel's.
func TestDefaultLayers(t *testing.T) {
	_, got := files(t, *all)
	want := []string{
		"main-B_Cu.gbl", "main-B_Mask.gbs", "main-B_Paste.gbp", "main-B_Silkscreen.gbo",
		"main-Edge_Cuts.gm1", "main-F_Cu.gtl", "main-F_Mask.gts", "main-F_Paste.gtp",
		"main-F_Silkscreen.gto", "main-job.gbrjob",
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("files\n got %v\nwant %v", got, want)
	}
}

func TestGivenLayers(t *testing.T) {
	_, got := files(t, *some)
	if want := "main-Edge_Cuts.gm1 main-F_Cu.gtl main-job.gbrjob"; strings.Join(got, " ") != want {
		t.Errorf("files %v, want %s", got, want)
	}
}

// Gerber X2 with netlist attributes, 6 decimals in mm, every date at the
// epoch (--@hdlf_rules_kicad//kicad:source_date_epoch defaults to 0).
func TestCopperFile(t *testing.T) {
	dir, _ := files(t, *all)
	b, err := os.ReadFile(filepath.Join(dir, "main-F_Cu.gtl"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{
		"%TF.GenerationSoftware,KiCad,Pcbnew,",          // X2 file attributes
		"%TF.FileFunction,Copper,L1,Top*%",              // X2 layer function
		"%TO.N,",                                         // netlist attributes
		"%FSLAX46Y46*%",                                  // 6 decimals
		"%MOMM*%",                                        // millimeters
		"%TF.CreationDate,1970-01-01T00:00:00+00:00*%",  // normalized
		"date 1970-01-01 00:00:00*",                      // normalized comment
	} {
		if !strings.Contains(s, want) {
			t.Errorf("main-F_Cu.gtl lacks %q", want)
		}
	}
}

func TestJobFile(t *testing.T) {
	dir, _ := files(t, *all)
	b, err := os.ReadFile(filepath.Join(dir, "main-job.gbrjob"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"CreationDate": "1970-01-01T00:00:00+00:00"`) {
		t.Error("main-job.gbrjob: the creation date is not at the epoch")
	}
}
