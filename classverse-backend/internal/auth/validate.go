package auth

import (
	"net/mail"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/moderation"
)

const (
	minPasswordLen    = 10
	maxPasswordLen    = 128
	minDisplayNameLen = 3
	maxDisplayNameLen = 24
	maxEmailLen       = 254
	dateLayout        = time.DateOnly
)

// normalizeEmail trims and lowercases email and reports whether it is a bare
// valid address (no display name).
func normalizeEmail(email string) (string, bool) {
	e := strings.ToLower(strings.TrimSpace(email))
	if e == "" || len(e) > maxEmailLen {
		return e, false
	}
	addr, err := mail.ParseAddress(e)
	if err != nil || addr.Address != e || !strings.Contains(e[strings.LastIndex(e, "@"):], ".") {
		return e, false
	}
	return e, true
}

func validatePassword(v *ValidationError, password string) {
	n := utf8.RuneCountInString(password)
	switch {
	case n < minPasswordLen:
		v.add("password", "Password minimal 10 karakter")
	case n > maxPasswordLen:
		v.add("password", "Password maksimal 128 karakter")
	}
}

func validateDisplayName(v *ValidationError, name string) string {
	for _, r := range name {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			v.add("display_name", "Nama tidak boleh mengandung karakter kontrol")
			break
		}
	}
	name = norm.NFC.String(strings.Join(strings.Fields(name), " "))
	n := utf8.RuneCountInString(name)
	switch {
	case n < minDisplayNameLen:
		v.add("display_name", "Nama tampilan minimal 3 karakter")
	case n > maxDisplayNameLen:
		v.add("display_name", "Nama tampilan maksimal 24 karakter")
	}
	if prohibitedName(name) {
		v.add("display_name", "Nama mengandung kata yang tidak diperbolehkan")
	}
	return name
}

// prohibitedName uses the project's whole-word list shared with chat.
func prohibitedName(name string) bool { return moderation.ContainsProfanity(name) }

func validAvatar(key string) bool {
	if key == "default" {
		return true
	}
	accessory, color, ok := strings.Cut(key, "-")
	if !ok || (color != "nila" && color != "mangga") {
		return false
	}
	switch accessory {
	case "round", "cap", "glasses", "headset", "bun", "beret":
		return true
	default:
		return false
	}
}

// validateBirthDate parses a YYYY-MM-DD date that is not in the future
// relative to today's UTC date.
func validateBirthDate(v *ValidationError, s string, now time.Time) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		v.add("birth_date", "Tanggal lahir wajib diisi")
		return time.Time{}
	}
	d, err := time.Parse(dateLayout, s)
	if err != nil {
		v.add("birth_date", "Format tanggal lahir tidak valid")
		return time.Time{}
	}
	today := now.UTC().Truncate(24 * time.Hour)
	switch {
	case d.After(today):
		v.add("birth_date", "Tanggal lahir tidak boleh di masa depan")
	case d.Year() < 1900:
		v.add("birth_date", "Tanggal lahir tidak valid")
	}
	return d
}

// truncateRunes cuts s to at most n runes.
func truncateRunes(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
