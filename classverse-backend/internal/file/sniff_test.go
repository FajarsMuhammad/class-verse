package file

import (
	"errors"
	"testing"
)

func TestSniff(t *testing.T) {
	ftyp := append([]byte{0, 0, 0, 0x18}, []byte("ftypM4A \x00\x00\x00\x00")...)
	for _, tc := range []struct {
		name, kind string
		head, tail []byte
		want       string
		err        error
	}{
		{"pdf", "pdf", []byte("%PDF-1.7\n"), []byte("trailer\n%%EOF\n"), "application/pdf", nil},
		{"pdf with BOM junk", "pdf", []byte("\xef\xbb\xbf%PDF-1.4"), []byte("%%EOF"), "application/pdf", nil},
		{"encrypted pdf", "pdf", []byte("%PDF-1.7"), []byte("trailer << /Encrypt 5 0 R >>\n%%EOF"), "", ErrEncryptedPDF},
		{"truncated pdf", "pdf", []byte("%PDF-1.7"), []byte("endobj"), "", ErrInvalidUpload},
		{"html as pdf", "pdf", []byte("<html>"), []byte("%%EOF"), "", ErrInvalidUpload},
		{"mp3 id3", "audio", []byte("ID3\x04\x00"), nil, "audio/mpeg", nil},
		{"mp3 frame", "audio", []byte{0xff, 0xfb, 0x90, 0x64}, nil, "audio/mpeg", nil},
		{"m4a", "audio", ftyp, nil, "audio/mp4", nil},
		{"pdf as audio", "audio", []byte("%PDF-1.7"), nil, "", ErrInvalidUpload},
		{"exe as audio", "audio", []byte("MZ\x90\x00"), nil, "", ErrInvalidUpload},
		{"empty", "audio", nil, nil, "", ErrInvalidUpload},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sniff(tc.kind, tc.head, tc.tail)
			if !errors.Is(err, tc.err) || got != tc.want {
				t.Fatalf("got %q %v, want %q %v", got, err, tc.want, tc.err)
			}
		})
	}
}
