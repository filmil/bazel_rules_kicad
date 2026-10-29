// SPDX-License-Identifier: Apache-2.0

// Command models3d splits KiCad's 3D models into one archive per library,
// with a web-seeded .torrent each, and prints the pins (issue #43).
//
// Check (what agents run; it writes files, uploads nothing):
//
//	bazel run //tools:models3d -- --archive=/path/kicad-packages3D-9.0.9.tar.gz --out=/path/out
//
// Upload (the operator only). The login is "publisher"; its password is in
// the Kubernetes Secret bt/bt-upload, which serving's bt.k8s/make-secrets.sh
// made:
//
//	BT_UPLOAD_PASSWORD="$(kubectl -n bt get secret bt-upload -o jsonpath='{.data.password}' | base64 -d)" \
//	  bazel run //tools:models3d -- --archive=... --out=... --upload --user=publisher
//
// --archive must hash to the pinned sha256, so every run publishes the
// same bytes. --pins writes the Starlark file of pins.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/filmil/bazel_rules_kicad/tools/models3d"
)

// The upstream release: kicad-packages3D at tag 9.0.9, as GitLab serves it.
const (
	version        = "9.0.9"
	upstreamURL    = "https://gitlab.com/kicad/libraries/kicad-packages3D/-/archive/9.0.9/kicad-packages3D-9.0.9.tar.gz"
	upstreamSHA256 = "047fb04452806869e5ff06ce1380652401e596b11221d0b366ba48b6adb552ac"
	defaultBase    = "https://bt.hdlfactory.com/kicad/packages3D/9.0.9/"
)

func main() {
	archive := flag.String("archive", "", "the upstream archive, downloaded from "+upstreamURL)
	out := flag.String("out", "", "the directory to write the archives, torrents and license into")
	base := flag.String("base_url", defaultBase, "where the files are served; ends in /")
	pins := flag.String("pins", "", "write the Starlark pins to this file")
	upload := flag.Bool("upload", false, "upload the files to --base_url (needs --user and $BT_UPLOAD_PASSWORD)")
	user := flag.String("user", "", "the upload user")
	flag.Parse()
	if err := run(*archive, upstreamSHA256, *out, *base, *pins, *upload, *user, os.Getenv("BT_UPLOAD_PASSWORD")); err != nil {
		fmt.Fprintln(os.Stderr, "models3d:", err)
		os.Exit(1)
	}
}

// run checks archive against wantSHA256, splits it into out, writes the
// pins, and uploads when asked.
func run(archive, wantSHA256, out, base, pins string, upload bool, user, password string) error {
	if archive == "" || out == "" {
		return fmt.Errorf("--archive and --out are required")
	}
	if upload && (user == "" || password == "") {
		return fmt.Errorf("--upload needs --user and $BT_UPLOAD_PASSWORD")
	}
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != wantSHA256 {
		return fmt.Errorf("%s: sha256 %s, want %s (%s)", archive, got, wantSHA256, upstreamURL)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return err
	}
	// The extracted upstream tree, beside --out, removed at the end.
	out = filepath.Clean(out)
	work := out + ".work"
	if err := os.RemoveAll(work); err != nil {
		return err
	}
	if err := os.MkdirAll(work, 0o755); err != nil {
		return err
	}
	defer os.RemoveAll(work)
	libs, err := models3d.Split(f, out, work, base)
	if err != nil {
		return err
	}
	var total int64
	files := []string{models3d.LicenseFile}
	for _, l := range libs {
		total += l.Size
		files = append(files, l.Archive, l.Torrent)
	}
	fmt.Printf("%d libraries, %d bytes of archives, in %s\n", len(libs), total, out)
	if pins != "" {
		p, err := models3d.Pins(version, base, libs)
		if err != nil {
			return err
		}
		if err := os.WriteFile(pins, []byte(p), 0o644); err != nil {
			return err
		}
		fmt.Println("pins:", pins)
	}
	if !upload {
		fmt.Println("not uploaded; the operator adds --upload --user=publisher, with $BT_UPLOAD_PASSWORD from the Secret bt/bt-upload")
		return nil
	}
	client := &http.Client{Timeout: 30 * time.Minute}
	return models3d.Upload(client, out, base, user, password, files, os.Stdout)
}
