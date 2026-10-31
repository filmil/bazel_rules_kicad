// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const board = `(kicad_pcb
	(version 20241229)
	(footprint "LED_SMD:LED_0603_1608Metric"
		(property "Reference" "D1")
		(model "${KICAD9_3DMODEL_DIR}/LED_SMD.3dshapes/LED_0603_1608Metric.step"
			(offset (xyz 0 0 0))
		)
	)
	(footprint "Resistor_SMD:R_0603_1608Metric"
		(property "Reference" "R1")
		(model "${KICAD8_3DMODEL_DIR}/Resistor_SMD.3dshapes/R_0603_1608Metric.step")
	)
	(footprint "TestPoint:TestPoint_Pad_D1.5mm"
		(property "Reference" "TP1")
	)
	(footprint "Package_SO:SOIC-8_3.9x4.9mm_P1.27mm"
		(property "Reference" "U1")
		(model "${KICAD9_3DMODEL_DIR}/Package_SO.3dshapes/SOIC-8_3.9x4.9mm_P1.27mm.step"
			(hide yes)
		)
		(model "${KICAD9_3DMODEL_DIR}/Package_SO.3dshapes/other.step" hide)
	)
	(footprint "Old:Six"
		(fp_text reference "C1" (at 0 0))
		(model "${KISYS3DMOD}/Capacitor_SMD.3dshapes/C_0603_1608Metric.step")
	)
	(footprint "Mine:Local"
		(property "Reference" "J2")
		(model "${KIPRJMOD}/models/local.step")
	)
	(footprint "Mine:Relative"
		(property "Reference" "J3")
		(model "models/local.step" (hide no))
	)
)`

func stage(t *testing.T, files ...string) string {
	t.Helper()
	dir := t.TempDir()
	for _, f := range files {
		p := filepath.Join(dir, f)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func expect(t *testing.T, b, models, project string, want ...string) {
	t.Helper()
	got, err := Missing(b, models, project)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Missing:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestAllThere(t *testing.T) {
	models := stage(t,
		"Resistor_SMD.3dshapes/R_0603_1608Metric.step",
		"LED_SMD.3dshapes/LED_0603_1608Metric.step",
		"Capacitor_SMD.3dshapes/C_0603_1608Metric.step",
	)
	// U1's models are hidden and TP1 has none, so they need nothing.
	expect(t, board, models, stage(t, "models/local.step"))
}

func TestMissing(t *testing.T) {
	models := stage(t, "Resistor_SMD.3dshapes/R_0603_1608Metric.step")
	expect(t, board, models, t.TempDir(),
		// KiCad 6 and 7 boards name the part with fp_text.
		"C1: ${KISYS3DMOD}/Capacitor_SMD.3dshapes/C_0603_1608Metric.step not found; add @kicad_models//:Capacitor_SMD (or your own Capacitor_SMD.3dshapes) to models, or check that the library has this file",
		"D1: ${KICAD9_3DMODEL_DIR}/LED_SMD.3dshapes/LED_0603_1608Metric.step not found; add @kicad_models//:LED_SMD (or your own LED_SMD.3dshapes) to models, or check that the library has this file",
		"J2: ${KIPRJMOD}/models/local.step not found next to the board; models of the project are not staged, so put the model in a library under models",
		// (hide no) is shown, so it is checked.
		"J3: models/local.step not found next to the board; models of the project are not staged, so put the model in a library under models",
	)
}

func TestPaths(t *testing.T) {
	fp := func(ref, model string) string {
		return `(kicad_pcb (footprint "X:Y" (property "Reference" "` + ref + `") (model "` + model + `")))`
	}
	models := stage(t, "Lib.3dshapes/dir.step/x", "loose.step")
	for _, c := range []struct{ model, want string }{
		{"${KICAD10_3DMODEL_DIR}/Lib.3dshapes/a.step", `model "${KICAD10_3DMODEL_DIR}/Lib.3dshapes/a.step" is under neither ${KICAD9_3DMODEL_DIR} nor the project, so the build cannot give it`},
		{"/home/me/odd.step", `model "/home/me/odd.step" is under neither ${KICAD9_3DMODEL_DIR} nor the project, so the build cannot give it`},
		{"${KICAD9_3DMODEL_DIR}/../escape.step", `model "${KICAD9_3DMODEL_DIR}/../escape.step" leaves the model directory`},
		{"../escape.step", `model "../escape.step" leaves the project directory`},
		// A directory is not a model.
		{"${KICAD9_3DMODEL_DIR}/Lib.3dshapes/dir.step", "${KICAD9_3DMODEL_DIR}/Lib.3dshapes/dir.step not found; add @kicad_models//:Lib (or your own Lib.3dshapes) to models, or check that the library has this file"},
		{"${KICAD9_3DMODEL_DIR}/missing.step", "${KICAD9_3DMODEL_DIR}/missing.step not found; a model library is a <name>.3dshapes directory"},
	} {
		expect(t, fp("X1", c.model), models, t.TempDir(), "X1: "+c.want)
	}
	// A loose model file at the top of the directory is there.
	expect(t, fp("X1", "${KICAD9_3DMODEL_DIR}/loose.step"), models, t.TempDir())
	// A footprint with no reference is named by its footprint.
	expect(t, `(kicad_pcb (footprint "X:Y" (model "/abs.step")))`, models, t.TempDir(),
		`the footprint X:Y: model "/abs.step" is under neither ${KICAD9_3DMODEL_DIR} nor the project, so the build cannot give it`)
}

func TestBadBoard(t *testing.T) {
	for _, b := range []string{"(kicad_pcb", "(kicad_pcb))", `(kicad_pcb "x`, "(kicad_sch)", "", "(kicad_pcb) (kicad_pcb)"} {
		if _, err := Missing(b, t.TempDir(), t.TempDir()); err == nil {
			t.Errorf("Missing(%q): no error", b)
		}
	}
}
