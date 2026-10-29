// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

func TestAppend(t *testing.T) {
	for _, in := range []string{
		"(version 1)\n(rule \"a\"\n  (constraint clearance (min 0.2mm)))\n",
		"(version 1)\n(rule \"a\"\n  (constraint clearance (min 0.2mm)))", // no final newline
	} {
		out := string(Append([]byte(in)))
		if !strings.HasPrefix(out, in) {
			t.Errorf("the user's rules changed:\n%s", out)
		}
		if !strings.HasSuffix(out, Rule) {
			t.Errorf("the canary is not last:\n%s", out)
		}
		if strings.Contains(out, ")))(rule") {
			t.Errorf("the canary runs into the last rule:\n%s", out)
		}
	}
	if !strings.Contains(Rule, "47.11mm") || Width != "47.1100 mm" {
		t.Error("Rule and Width disagree")
	}
}

// An empty rule file gets the canary alone (KiCad then drops it for lack of
// a (version 1) header, which the check reports, as for any broken file).
func TestAppendEmpty(t *testing.T) {
	out := string(Append(nil))
	if !strings.HasSuffix(out, Rule) || strings.HasPrefix(out, "\n\n") {
		t.Errorf("got %q", out)
	}
}
