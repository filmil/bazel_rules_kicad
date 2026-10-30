// SPDX-License-Identifier: Apache-2.0

// Command stepcheck fails when kicad-cli left a part out of a STEP model
// for want of its 3D model (issue #43).
//
// Usage:
//
//	stepcheck --stamp STAMP LOG
//
// LOG is kicad-cli's standard output of `pcb export step`. For each part
// whose model it could not load, kicad-cli prints "Could not add 3D model
// for <ref>." and then "File not found: <path>", and still exits 0. The
// check names each such part and the @kicad_models target that holds its
// library. With none, it writes STAMP.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var (
	couldNot = regexp.MustCompile(`^Could not add 3D model for (.+?)\.?$`)
	notFound = regexp.MustCompile(`^File not found: (.*)$`)
	library  = regexp.MustCompile(`([^/]+)\.3dshapes/`)
)

// Missing returns one line per part whose model kicad-cli did not load.
func Missing(log io.Reader) ([]string, error) {
	var out []string
	s := bufio.NewScanner(log)
	ref := ""
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if m := couldNot.FindStringSubmatch(line); m != nil {
			if ref != "" {
				out = append(out, fmt.Sprintf("%s: no 3D model", ref))
			}
			ref = m[1]
			continue
		}
		if ref == "" {
			continue
		}
		// kicad-cli prints the file right after the part. Any other line
		// means it named none: the model was there but did not load.
		if m := notFound.FindStringSubmatch(line); m != nil {
			msg := fmt.Sprintf("%s: %s not found", ref, m[1])
			if l := library.FindStringSubmatch(m[1]); l != nil {
				msg += fmt.Sprintf("; add @kicad_models//:%s (or your own %s.3dshapes) to models, or check that the library has this file", l[1], l[1])
			}
			out = append(out, msg)
		} else {
			out = append(out, fmt.Sprintf("%s: no 3D model", ref))
		}
		ref = ""
	}
	if ref != "" {
		out = append(out, fmt.Sprintf("%s: no 3D model", ref))
	}
	return out, s.Err()
}

func main() {
	stamp := flag.String("stamp", "", "the file to write when no model is missing")
	flag.Parse()
	if flag.NArg() != 1 || *stamp == "" {
		fmt.Fprintln(os.Stderr, "usage: stepcheck --stamp STAMP LOG")
		os.Exit(2)
	}
	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, "stepcheck:", err)
		os.Exit(1)
	}
	missing, err := Missing(f)
	f.Close()
	if err != nil {
		fmt.Fprintln(os.Stderr, "stepcheck:", err)
		os.Exit(1)
	}
	if len(missing) > 0 {
		fmt.Fprintf(os.Stderr, "the STEP model lacks %d part(s):\n", len(missing))
		for _, m := range missing {
			fmt.Fprintln(os.Stderr, "  "+m)
		}
		os.Exit(1)
	}
	if err := os.WriteFile(*stamp, nil, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "stepcheck:", err)
		os.Exit(1)
	}
}
