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
	)
	(footprint "Mine:Odd"
		(property "Reference" "J1")
		(model "/home/me/odd.step")
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

func TestMissing(t *testing.T) {
	dir := stage(t, "Resistor_SMD.3dshapes/R_0603_1608Metric.step")
	got, err := Missing(board, dir)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"D1: ${KICAD9_3DMODEL_DIR}/LED_SMD.3dshapes/LED_0603_1608Metric.step not found; add @kicad_models//:LED_SMD (or your own LED_SMD.3dshapes) to models, or check that the library has this file",
		`J1: model "/home/me/odd.step" is not under ${KICAD9_3DMODEL_DIR}, so no models library can give it`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Missing:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestNoneMissing(t *testing.T) {
	b := strings.Replace(board, `(model "/home/me/odd.step")`, "", 1)
	dir := stage(t,
		"Resistor_SMD.3dshapes/R_0603_1608Metric.step",
		"LED_SMD.3dshapes/LED_0603_1608Metric.step",
	)
	got, err := Missing(b, dir)
	if err != nil || len(got) != 0 {
		t.Errorf("Missing: %q, %v; want none (U1's model is hidden, TP1 has none)", got, err)
	}
}

func TestBadBoard(t *testing.T) {
	for _, b := range []string{"(kicad_pcb", "(kicad_pcb))", `(kicad_pcb "x`, "(kicad_sch)", ""} {
		if _, err := Missing(b, t.TempDir()); err == nil {
			t.Errorf("Missing(%q): no error", b)
		}
	}
}
