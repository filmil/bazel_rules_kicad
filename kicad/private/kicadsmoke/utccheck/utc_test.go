// SPDX-License-Identifier: Apache-2.0

// Package utccheck checks that kicad-cli, run through run_tool, writes its
// time stamps in UTC whatever the host's time zone (TZ=UTC0 in the KiCad
// toolchain).
package utccheck

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReportDateIsUTC(t *testing.T) {
	// ERC_REPORT is relative to the main repository's runfiles directory;
	// a go_test runs in its own package's directory.
	p := filepath.Join(os.Getenv("TEST_SRCDIR"), os.Getenv("TEST_WORKSPACE"), os.Getenv("ERC_REPORT"))
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var r struct {
		Date string `json:"date"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		t.Fatal(err)
	}
	if r.Date == "" || !strings.HasSuffix(r.Date, "+0000") {
		t.Errorf("the ERC report's date is %q; want one in UTC, ending in +0000", r.Date)
	}
}
