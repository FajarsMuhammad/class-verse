// Package seed embeds the CC0 sample files used by the explicit seed command.
package seed

import _ "embed"

// TataSurya is the sample e-book for the library catalog (CC0, see README.md).
//
//go:embed tata-surya.pdf
var TataSurya []byte
