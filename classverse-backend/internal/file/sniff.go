package file

import (
	"bytes"
	"errors"
)

// sniffBytes covers the PDF header window and the trailer where /Encrypt lives.
const sniffBytes = 64 << 10

// ErrEncryptedPDF lets the UI tell teachers to remove the password first.
var ErrEncryptedPDF = errors.New("encrypted pdf")

// sniff checks content, not the client's MIME type or extension.
// shortcut: no full PDF parse; PDF.js rejects deeper corruption in the reader, add a parser if that becomes common.
func sniff(kind string, head, tail []byte) (string, error) {
	switch kind {
	case "pdf":
		// PDF spec allows the header within the first 1024 bytes.
		if !bytes.Contains(head[:min(len(head), 1024)], []byte("%PDF-")) || !bytes.Contains(tail, []byte("%%EOF")) {
			return "", ErrInvalidUpload
		}
		if bytes.Contains(tail, []byte("/Encrypt")) || bytes.Contains(head, []byte("/Encrypt")) {
			return "", ErrEncryptedPDF
		}
		return "application/pdf", nil
	case "audio":
		switch {
		case bytes.HasPrefix(head, []byte("ID3")), len(head) > 1 && head[0] == 0xff && head[1]&0xe0 == 0xe0:
			return "audio/mpeg", nil
		case len(head) >= 12 && bytes.Equal(head[4:8], []byte("ftyp")):
			return "audio/mp4", nil
		}
	}
	return "", ErrInvalidUpload
}
