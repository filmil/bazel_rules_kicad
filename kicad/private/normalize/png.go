// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

// PNG is a PNG image, such as kicad_render's.
const PNG Format = "png"

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// pngMetadata lists the PNG chunks that hold text, a time or Exif data
// rather than pixels. KiCad 9.0.9 writes none of them; StripPNG drops them
// in case a later version does.
var pngMetadata = map[string]bool{
	"tEXt": true,
	"zTXt": true,
	"iTXt": true,
	"tIME": true,
	"eXIf": true,
}

// StripPNG returns the PNG image b without its metadata chunks. The other
// chunks are copied byte for byte, with their checksums. An image that is
// cut short, or does not start with IHDR and end with IEND, is an error.
func StripPNG(b []byte) ([]byte, error) {
	if !bytes.HasPrefix(b, pngSignature) {
		return nil, fmt.Errorf("png: no PNG signature")
	}
	out := append([]byte(nil), pngSignature...)
	rest := b[len(pngSignature):]
	first := true
	for {
		// A chunk is a 4-byte length, a 4-byte type, the data and a
		// 4-byte checksum.
		if len(rest) < 12 {
			return nil, fmt.Errorf("png: cut short, or no IEND chunk")
		}
		n := binary.BigEndian.Uint32(rest[:4])
		if uint64(n) > uint64(len(rest)-12) {
			return nil, fmt.Errorf("png: a chunk of %d bytes runs past the end", n)
		}
		kind := string(rest[4:8])
		chunk := rest[:12+n]
		rest = rest[12+n:]
		if first && kind != "IHDR" {
			return nil, fmt.Errorf("png: the first chunk is %q, not IHDR", kind)
		}
		first = false
		if !pngMetadata[kind] {
			out = append(out, chunk...)
		}
		if kind == "IEND" {
			if len(rest) != 0 {
				return nil, fmt.Errorf("png: %d bytes after IEND", len(rest))
			}
			return out, nil
		}
	}
}
