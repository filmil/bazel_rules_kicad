// SPDX-License-Identifier: Apache-2.0

package prebuilt

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/runfiles"
)

func readRunfile(t *testing.T, path string) []byte {
	t.Helper()
	p, err := runfiles.Rlocation("_main/" + path)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

var sha = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Every input is pinned: the image by digest, the packages by snapshot, the
// two downloads by sha256, which the script checks before it unpacks them.
func TestInputsArePinned(t *testing.T) {
	if !regexp.MustCompile(`@sha256:[0-9a-f]{64}$`).MatchString(Image) {
		t.Errorf("image %q is not pinned by digest", Image)
	}
	if !regexp.MustCompile(`^\d{8}T\d{6}Z$`).MatchString(AptSnapshot) {
		t.Errorf("apt snapshot %q", AptSnapshot)
	}
	for _, in := range []Input{DwarFS, AppImage} {
		if !strings.HasPrefix(in.URL, "https://") || !sha.MatchString(in.SHA256) {
			t.Errorf("input %+v", in)
		}
	}
	for _, want := range []string{
		`fetch "$DWARFS_URL" "$DWARFS_SHA256"`,
		`fetch "$APPIMAGE_URL" "$APPIMAGE_SHA256"`,
		`sha256sum -c -`,
		`snapshot.ubuntu.com/ubuntu/$APT_SNAPSHOT/`,
		`-DFETCHCONTENT_FULLY_DISCONNECTED=ON`,
	} {
		if !strings.Contains(BuildScript, want) {
			t.Errorf("build.sh lacks %q", want)
		}
	}
}

// The command passes every input to the container, and nothing of the
// host's but the output directory.
func TestCommand(t *testing.T) {
	cmd := Command("/o", 3)
	line := strings.Join(cmd, " ")
	for _, want := range []string{
		"-v /o:/out:Z", "JOBS=3", "--cpus=3", "APT_SNAPSHOT=" + AptSnapshot,
		"DWARFS_SHA256=" + DwarFS.SHA256, "APPIMAGE_SHA256=" + AppImage.SHA256,
		"APPIMAGE_NAME=" + AppImageName, "PREBUILT=" + Prebuilt, Image + " bash -s",
	} {
		if !strings.Contains(line, want) {
			t.Errorf("command lacks %q:\n%s", want, line)
		}
	}
	if !strings.HasPrefix(line, "podman run --rm -i ") {
		t.Errorf("command: %s", line)
	}
	if n := strings.Count(line, " -v "); n != 1 {
		t.Errorf("%d volumes, want only /out:\n%s", n, line)
	}
}

// The tool makes what the toolchain downloads: the archive's sha256 is the
// one extensions.bzl pins, and the manifest's built asset names the same
// AppImage and this tool.
func TestMatchesToolchainAndManifest(t *testing.T) {
	ext := string(readRunfile(t, "kicad/private/extensions.bzl"))
	if !strings.Contains(ext, `"`+PrebuiltSHA256+`"`) {
		t.Errorf("kicad/private/extensions.bzl does not pin %s", PrebuiltSHA256)
	}
	var m struct {
		Releases []struct {
			Tag    string `json:"tag"`
			Notes  string `json:"notes"`
			Assets []struct {
				Name   string `json:"name"`
				SHA256 string `json:"sha256"`
				Built  *struct {
					Source       string `json:"source"`
					SourceSHA256 string `json:"source_sha256"`
					Script       string `json:"script"`
				} `json:"built"`
			} `json:"assets"`
		} `json:"releases"`
	}
	if err := json.Unmarshal(readRunfile(t, "tools/mirror/manifest.json"), &m); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, r := range m.Releases {
		for _, a := range r.Assets {
			if a.Name != Prebuilt+".tar.xz" {
				continue
			}
			found = true
			if a.SHA256 != PrebuiltSHA256 {
				t.Errorf("manifest: %s has sha256 %s, the tool %s", a.Name, a.SHA256, PrebuiltSHA256)
			}
			if a.Built == nil || a.Built.Source != AppImage.URL || a.Built.SourceSHA256 != AppImage.SHA256 {
				t.Errorf("manifest: %s is not built from %+v", a.Name, AppImage)
			}
			if a.Built != nil && !strings.Contains(a.Built.Script, "//tools:kicad_prebuilt") {
				t.Errorf("manifest: %s names script %q, not //tools:kicad_prebuilt", a.Name, a.Built.Script)
			}
		}
	}
	if !found {
		t.Errorf("manifest has no asset %s.tar.xz", Prebuilt)
	}
}
