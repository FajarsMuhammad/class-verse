package moderation

import (
	"errors"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestFilterChat(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
		err  error
	}{
		{"plain", "Halo semua", "Halo semua", nil},
		{"trims and collapses spaces", "  halo   kelas  ", "halo kelas", nil},
		{"nfc", "Cafe\u0301", "Café", nil},
		{"keeps newline", "baris 1\nbaris 2", "baris 1\nbaris 2", nil},
		{"limits blank lines", "a\n\n\n\n\nb", "a\n\nb", nil},
		{"profanity whole word", "dasar Bangsat!", "dasar ****!", nil},
		{"profanity not substring", "anjingan kampung", "anjingan kampung", nil},
		{"phone 08", "wa aku 081234567890 ya", "wa aku [disembunyikan] ya", nil},
		{"phone +62 separated", "+62 812-3456-7890", "[disembunyikan]", nil},
		{"phone 62 dots", "62.812.3456.789", "[disembunyikan]", nil},
		{"short number kept", "halaman 0812", "halaman 0812", nil},
		{"year kept", "tahun 2026 dan 2027", "tahun 2026 dan 2027", nil},
		{"email", "kirim ke siswa.satu@sekolah.sch.id ok", "kirim ke [disembunyikan] ok", nil},
		{"empty", "   \n  ", "", ErrEmpty},
		{"too long", strings.Repeat("a", MaxChatLen+1), "", ErrTooLong},
		{"max length ok", strings.Repeat("界", MaxChatLen), strings.Repeat("界", MaxChatLen), nil},
		{"control", "halo\x07", "", ErrControls},
		{"bidi override", "halo\u202e", "", ErrControls},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterChat(tt.in)
			if !errors.Is(err, tt.err) {
				t.Fatalf("err = %v, want %v", err, tt.err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestContainsProfanity(t *testing.T) {
	if !ContainsProfanity("Budi FUCK") || ContainsProfanity("Budi Santoso") {
		t.Fatal("ContainsProfanity mismatch")
	}
}

var leakedPhone = regexp.MustCompile(`(?:\+?62|0)8\d{8,11}`)

func FuzzFilterChat(f *testing.F) {
	for _, s := range []string{"halo", "081234567890", "a@b.co", "bangsat", "\n\n\n", "+62 812 3456 7890"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, in string) {
		out, err := FilterChat(in)
		if err != nil {
			return
		}
		if !utf8.ValidString(out) || utf8.RuneCountInString(out) == 0 {
			t.Fatalf("bad output %q", out)
		}
		if leakedPhone.MatchString(strings.NewReplacer(" ", "", ".", "", "-", "").Replace(out)) &&
			!leakedPhone.MatchString(strings.NewReplacer(" ", "", ".", "", "-", "").Replace(in)) {
			t.Fatalf("filter created a phone number: %q", out)
		}
		again, err := FilterChat(out)
		if err != nil || again != out {
			t.Fatalf("not idempotent: %q -> %q (%v)", out, again, err)
		}
	})
}
