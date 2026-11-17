package class

import (
	"strings"
	"testing"
)

func TestValidation(t *testing.T) {
	for _, tc := range []struct {
		input string
		valid bool
	}{
		{"2026/2027", true}, {" 2026/2027 ", true}, {"2026/2028", false}, {"2026", false}, {"9999/0000", false}, {"2026/2027\nother", false},
	} {
		if _, ok := validLabel(tc.input); ok != tc.valid {
			t.Errorf("validLabel(%q)=%v", tc.input, ok)
		}
	}
	for _, tc := range []struct {
		input string
		valid bool
	}{
		{" XI  IPA 1 ", true}, {"", false}, {strings.Repeat("a", 41), false}, {"Kelas\x00A", false}, {strings.Repeat("字", 40), true},
	} {
		if _, ok := validName(tc.input); ok != tc.valid {
			t.Errorf("validName(%q)=%v", tc.input, ok)
		}
	}
}
