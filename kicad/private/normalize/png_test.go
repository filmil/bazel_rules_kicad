// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
	"time"
)

// encodePNG returns a small PNG image, as Go's encoder writes it: IHDR,
// IDAT and IEND only.
func encodePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(1, 2, color.RGBA{R: 0x2c, G: 0x4a, B: 0x3a, A: 0xff})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// chunk returns a PNG chunk with a correct checksum.
func chunk(kind, data string) []byte {
	var b bytes.Buffer
	binary.Write(&b, binary.BigEndian, uint32(len(data)))
	b.WriteString(kind)
	b.WriteString(data)
	binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE([]byte(kind+data)))
	return b.Bytes()
}

// insertAfterIHDR puts chunks right after the IHDR chunk of p, where
// encoders put metadata.
func insertAfterIHDR(p []byte, chunks ...[]byte) []byte {
	at := len(pngSignature) + 12 + 13 // IHDR holds 13 bytes
	out := append([]byte(nil), p[:at]...)
	for _, c := range chunks {
		out = append(out, c...)
	}
	return append(out, p[at:]...)
}

func TestPNGMetadataIsDropped(t *testing.T) {
	plain := encodePNG(t)
	tagged := insertAfterIHDR(plain,
		chunk("tEXt", "Software\x00KiCad 9.0.9"),
		chunk("tIME", "\x07\xea\x0a\x1d\x07\x22\x00"),
		chunk("zTXt", "Comment\x00\x00x"),
		chunk("iTXt", "Title\x00\x00\x00\x00\x00board"),
		chunk("eXIf", "MM\x00*"),
	)
	got, f, err := Normalize(tagged, time.Unix(0, 0).UTC())
	if err != nil {
		t.Fatal(err)
	}
	if f != PNG {
		t.Errorf("format %s, want %s", f, PNG)
	}
	if !bytes.Equal(got, plain) {
		t.Errorf("the tagged image did not become the plain one")
	}
	if _, err := png.Decode(bytes.NewReader(got)); err != nil {
		t.Errorf("the output does not decode: %v", err)
	}
}

// An image without metadata, as KiCad 9.0.9 writes it, keeps every byte,
// and so do the chunks that describe the pixels.
func TestPNGWithoutMetadataUnchanged(t *testing.T) {
	p := insertAfterIHDR(encodePNG(t), chunk("sBIT", "\x08\x08\x08\x08"))
	got, err := StripPNG(p)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, p) {
		t.Errorf("an image without metadata changed")
	}
}

func TestPNGMalformedFails(t *testing.T) {
	plain := encodePNG(t)
	for name, b := range map[string][]byte{
		"cut short":       plain[:len(plain)-5],
		"no IEND":         plain[:len(plain)-12],
		"after IEND":      append(append([]byte(nil), plain...), 0),
		"IDAT first":      append(append([]byte(nil), pngSignature...), chunk("IDAT", "x")...),
		"length too long": append(append([]byte(nil), pngSignature...), 0xff, 0xff, 0xff, 0xff, 'I', 'H', 'D', 'R', 0, 0, 0, 0),
	} {
		if _, _, err := Normalize(b, time.Unix(0, 0).UTC()); err == nil || !strings.HasPrefix(err.Error(), "png:") {
			t.Errorf("%s: err %v, want a png: error", name, err)
		}
	}
}

func TestPNGDetectedBySignatureOnly(t *testing.T) {
	if f := Detect(encodePNG(t)); f != PNG {
		t.Errorf("Detect = %s, want %s", f, PNG)
	}
	// "PNG" as text is not a PNG image.
	if f := Detect([]byte("PNG\r\n")); f != Unknown {
		t.Errorf("Detect(text) = %s, want %s", f, Unknown)
	}
}
