// Package moderation holds chat safety rules: text validation and masking of
// profanity and personal data (docs/spec.md, Keamanan 4).
package moderation

import (
	"errors"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Chat text limits.
const (
	MaxChatLen     = 500
	maxBlankLines  = 2
	maskedWord     = "****"
	maskedPersonal = "[disembunyikan]"
	minPhoneDigits = 9
	maxPhoneDigits = 13
)

// Validation errors for chat text.
var (
	ErrEmpty    = errors.New("chat text is empty")
	ErrTooLong  = errors.New("chat text is too long")
	ErrControls = errors.New("chat text has control characters")
)

// profanity is a small project-authored whole-word list (Indonesian and
// English). It is not a language classifier and intentionally avoids
// substring matching to limit false positives.
var profanity = map[string]bool{
	"anjing": true, "bangsat": true, "bajingan": true, "kontol": true, "memek": true, "ngentot": true,
	"fuck": true, "shit": true, "bitch": true, "asshole": true,
}

// IsProfane reports whether word (any case) is on the list.
func IsProfane(word string) bool { return profanity[strings.ToLower(word)] }

// ContainsProfanity reports whether any whole word of s is on the list.
func ContainsProfanity(s string) bool {
	for _, w := range strings.FieldsFunc(s, notWordRune) {
		if IsProfane(w) {
			return true
		}
	}
	return false
}

var (
	emailRe = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)
	// phoneRe finds Indonesian-looking numbers: 08…, +62…, 62…, digits possibly
	// separated by single spaces, dots, or dashes. Length is checked after.
	phoneRe = regexp.MustCompile(`(?:(?:\+62|62)[ .\-]?|0)8(?:[ .\-]?\d){6,12}`)
)

// FilterChat validates and normalizes chat text and masks profanity, phone
// numbers, and email addresses. Only the returned text may be stored or sent.
func FilterChat(text string) (string, error) {
	text = norm.NFC.String(text)
	for _, r := range text {
		if r != '\n' && (unicode.IsControl(r) || unicode.Is(unicode.Cf, r)) {
			return "", ErrControls
		}
	}
	text = collapseLines(text)
	switch n := utf8.RuneCountInString(text); {
	case n == 0:
		return "", ErrEmpty
	case n > MaxChatLen:
		return "", ErrTooLong
	}
	text = emailRe.ReplaceAllString(text, maskedPersonal)
	text = phoneRe.ReplaceAllStringFunc(text, func(m string) string {
		digits := 0
		for _, r := range m {
			if r >= '0' && r <= '9' {
				digits++
			}
		}
		if digits < minPhoneDigits || digits > maxPhoneDigits {
			return m
		}
		return maskedPersonal
	})
	return maskWords(text), nil
}

// collapseLines trims each line's outer spaces, the whole text, and keeps at
// most maxBlankLines consecutive line breaks.
func collapseLines(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, l := range lines {
		l = strings.Join(strings.Fields(l), " ")
		if l == "" {
			blank++
			if blank >= maxBlankLines {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, l)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// maskWords replaces whole profane words, keeping all other characters.
func maskWords(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	start := -1
	flush := func(end int) {
		if start < 0 {
			return
		}
		if w := s[start:end]; IsProfane(w) {
			b.WriteString(maskedWord)
		} else {
			b.WriteString(w)
		}
		start = -1
	}
	for i, r := range s {
		if notWordRune(r) {
			flush(i)
			b.WriteRune(r)
			continue
		}
		if start < 0 {
			start = i
		}
	}
	flush(len(s))
	return b.String()
}

func notWordRune(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }
