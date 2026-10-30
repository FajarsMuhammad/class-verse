package auth

import (
	"errors"
	"strings"
)

// Domain errors. The handler maps them to API error codes.
var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrEmailNotVerified   = errors.New("email not verified")
	ErrInvalidOTP         = errors.New("invalid otp")
	ErrOTPExpired         = errors.New("otp expired")
	ErrTooManyAttempts    = errors.New("too many otp attempts")
	ErrUnauthenticated    = errors.New("unauthenticated")
	ErrSessionRevoked     = errors.New("session revoked")
	ErrGoogleUnverified   = errors.New("google email not verified")
	ErrAccountConflict    = errors.New("email linked to another google account")
	ErrUserNotFound       = errors.New("user not found")
)

// ValidationError lists invalid input fields with user-facing messages.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string {
	keys := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		keys = append(keys, k)
	}
	return "invalid fields: " + strings.Join(keys, ", ")
}

func (e *ValidationError) add(field, msg string) {
	if e.Fields == nil {
		e.Fields = make(map[string]string)
	}
	if _, ok := e.Fields[field]; !ok {
		e.Fields[field] = msg
	}
}

func (e *ValidationError) orNil() error {
	if len(e.Fields) == 0 {
		return nil
	}
	return e
}
