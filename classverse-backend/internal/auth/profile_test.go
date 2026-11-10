package auth

import (
	"strings"
	"testing"
)

func TestProfileNameRules(t *testing.T) {
	tests := []struct {
		name, input, want string
		valid             bool
	}{
		{"minimum", "Ani", "Ani", true}, {"short", "Al", "Al", false},
		{"maximum", strings.Repeat("界", 24), strings.Repeat("界", 24), true},
		{"long", strings.Repeat("界", 25), strings.Repeat("界", 25), false},
		{"normalize", "  Jose\u0301   Putra  ", "José Putra", true},
		{"control", "Ani\nPutra", "Ani Putra", false},
		{"format", "Ani\u200b", "Ani\u200b", false},
		{"blocked", "Bu BANGSAT", "Bu BANGSAT", false},
		{"punctuation", "fuck!", "fuck!", false},
		{"substring", "Scunthorpe", "Scunthorpe", true},
		{"ordinary", "Anjingan", "Anjingan", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var v ValidationError
			got := validateDisplayName(&v, tt.input)
			if got != tt.want || (v.orNil() == nil) != tt.valid {
				t.Fatalf("got %q valid=%v", got, v.orNil() == nil)
			}
		})
	}
}

func TestAvatarAllowlist(t *testing.T) {
	for _, a := range []string{"round", "cap", "glasses", "headset", "bun", "beret"} {
		for _, c := range []string{"nila", "mangga"} {
			if !validAvatar(a + "-" + c) {
				t.Fatal("missing preset")
			}
		}
	}
	for _, style := range []string{"runner", "cap", "headphones", "varsity", "ponytail", "hoodie", "glasses"} {
		for _, color := range []string{"nila", "mangga", "zamrud", "laut", "koral", "grafit"} {
			if !validAvatar("sporty-" + style + "-" + color) {
				t.Fatalf("missing sporty preset %s-%s", style, color)
			}
		}
	}
	for _, key := range []string{"https://example.com/a.svg", "../cap-nila", "cap", "cap-red", "cap-nila-extra", "", "sporty-cap-red", "sporty-round-nila", "sporty-cap-nila-extra", "sporty-../cap-nila", "cap-zamrud"} {
		if validAvatar(key) {
			t.Errorf("accepted %q", key)
		}
	}
	if !validAvatar("default") {
		t.Fatal("legacy key rejected")
	}
}
