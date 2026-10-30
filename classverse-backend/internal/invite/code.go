// Package invite manages invite codes and joining an organization.
package invite

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
)

// alphabet leaves out look-alike characters (0/O, 1/I/L).
const (
	alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789"
	codeLen  = 8
)

// newCode returns a random invite code in stored form (no dash).
func newCode() (string, error) {
	var b strings.Builder
	size := big.NewInt(int64(len(alphabet)))
	for range codeLen {
		n, err := rand.Int(rand.Reader, size)
		if err != nil {
			return "", fmt.Errorf("generate invite code: %w", err)
		}
		b.WriteByte(alphabet[n.Int64()])
	}
	return b.String(), nil
}

// normalizeCode uppercases input and drops dashes and spaces. It reports
// false when the result cannot be a valid code.
func normalizeCode(in string) (string, bool) {
	var b strings.Builder
	for _, r := range strings.ToUpper(in) {
		if r == '-' || r == ' ' {
			continue
		}
		if !strings.ContainsRune(alphabet, r) {
			return "", false
		}
		b.WriteRune(r)
	}
	s := b.String()
	return s, len(s) == codeLen
}

// displayCode formats a stored code as ABCD-EFGH.
func displayCode(code string) string {
	if len(code) != codeLen {
		return code
	}
	return code[:4] + "-" + code[4:]
}
