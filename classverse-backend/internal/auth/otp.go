package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"math/big"

	"github.com/google/uuid"
)

// generateOTP returns a uniformly random 6-digit code.
func generateOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", fmt.Errorf("generate otp: %w", err)
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// hashOTP binds the code to the user, so a leaked hash table cannot be
// brute-forced offline without the server key.
func hashOTP(key []byte, userID uuid.UUID, code string) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(userID[:])
	m.Write([]byte(code))
	return m.Sum(nil)
}
