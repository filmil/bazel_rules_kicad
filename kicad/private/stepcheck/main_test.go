// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

// As kicad-cli 9.0.9 prints it, two parts without their libraries.
const missingLog = `Build STEP data
Could not add 3D model for D1.
File not found: ${KICAD9_3DMODEL_DIR}/LED_SMD.3dshapes/LED_0603_1608Metric.step
Could not add 3D model for J1.
File not found: ${KICAD9_3DMODEL_DIR}/Connector_PinHeader_2.54mm.3dshapes/PinHeader_1x02_P2.54mm_Vertical.step
Create PCB solid model.
`

func TestMissing(t *testing.T) {
	got, err := Missing(strings.NewReader(missingLog))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"D1: ${KICAD9_3DMODEL_DIR}/LED_SMD.3dshapes/LED_0603_1608Metric.step not found; add @kicad_models//:LED_SMD (or your own LED_SMD.3dshapes) to models, or check that the library has this file",
		"J1: ${KICAD9_3DMODEL_DIR}/Connector_PinHeader_2.54mm.3dshapes/PinHeader_1x02_P2.54mm_Vertical.step not found; add @kicad_models//:Connector_PinHeader_2.54mm (or your own Connector_PinHeader_2.54mm.3dshapes) to models, or check that the library has this file",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestComplete(t *testing.T) {
	got, err := Missing(strings.NewReader("Build STEP data\nCreate PCB solid model.\nExport time 0.123 s\n"))
	if err != nil || len(got) != 0 {
		t.Errorf("got %v, %v; want nothing", got, err)
	}
}

// A part reported without the line that names its file still counts.
func TestNoPath(t *testing.T) {
	got, _ := Missing(strings.NewReader("Could not add 3D model for U7.\nCreate PCB solid model.\n"))
	if len(got) != 1 || got[0] != "U7: no 3D model" {
		t.Errorf("got %v", got)
	}
}

// A part whose model did not load, with no file named, is reported at
// once; a later "File not found" line is not paired with it.
func TestNoPathThenOther(t *testing.T) {
	log := "Could not add 3D model for U1.\nReading the model failed\n" +
		"File not found: ${KICAD9_3DMODEL_DIR}/X.3dshapes/x.step\n" +
		"Could not add 3D model for U2.\nCould not add 3D model for U3.\n" +
		"File not found: /elsewhere/u3.step\n"
	got, _ := Missing(strings.NewReader(log))
	want := []string{"U1: no 3D model", "U2: no 3D model", "U3: /elsewhere/u3.step not found"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
