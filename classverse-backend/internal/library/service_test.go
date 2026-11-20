package library

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestValidate(t *testing.T) {
	file := uuid.New()
	ok := Input{Title: " Atlas ", Author: "Tim", Shelf: "sains", Level: "SD", Format: "buku", Copies: 3, Cover: "ipa"}
	for _, tc := range []struct {
		name  string
		edit  func(*Input)
		field string
	}{
		{"empty title", func(in *Input) { in.Title = "  " }, "title"},
		{"long author", func(in *Input) { in.Author = string(make([]rune, 121)) }, "author"},
		{"control char", func(in *Input) { in.Title = "a\x00b" }, "title"},
		{"unknown shelf", func(in *Input) { in.Shelf = "musik" }, "shelf"},
		{"unknown level", func(in *Input) { in.Level = "TK" }, "level"},
		{"unknown cover", func(in *Input) { in.Cover = "#fff" }, "cover"},
		{"unknown format", func(in *Input) { in.Format = "video" }, "format"},
		{"physical with file", func(in *Input) { in.FileID = &file }, "file_id"},
		{"negative copies", func(in *Input) { in.Copies = -1 }, "copies"},
		{"too many copies", func(in *Input) { in.Copies = 1000 }, "copies"},
		{"ebook without file", func(in *Input) { in.Format = "ebook" }, "file_id"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := ok
			tc.edit(&in)
			_, err := validate(in)
			var v *ValidationError
			if !errors.As(err, &v) || v.Fields[tc.field] == "" {
				t.Fatalf("got %v, want field %s", err, tc.field)
			}
		})
	}
	got, err := validate(ok)
	if err != nil || got.Title != "Atlas" {
		t.Fatalf("valid input: %+v %v", got, err)
	}
	anonymous := ok
	anonymous.Author = "  "
	if got, err := validate(anonymous); err != nil || got.Author != "" {
		t.Fatalf("author is optional: %+v %v", got, err)
	}
	digital := ok
	digital.Format, digital.FileID, digital.Copies = "audio", &file, 7
	if got, err := validate(digital); err != nil || got.Copies != 0 {
		t.Fatalf("digital copies must reset to 0: %+v %v", got, err)
	}
}

func TestCursorAndEscape(t *testing.T) {
	id := uuid.New()
	title, got, err := decodeCursor(encodeCursor("A|B c", id))
	if err != nil || title != "A|B c" || got != id {
		t.Fatalf("round trip: %q %v %v", title, got, err)
	}
	for _, bad := range []string{"!!", "bm8tc2VwYXJhdG9y", encodeCursor("x", uuid.Nil)[:4]} {
		if _, _, err := decodeCursor(bad); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
