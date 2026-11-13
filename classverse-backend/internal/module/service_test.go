package module

import (
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

type fakeBuildings map[string]bool

func (f fakeBuildings) Building(key string) (bool, bool) {
	staff, ok := f[key]
	return staff, ok
}

func TestValidate(t *testing.T) {
	s := &Service{buildings: fakeBuildings{"sains": false, "ruang-guru": true}}
	ok := Input{BuildingKey: "sains", Level: "SD", Title: " Tata Surya ", Description: "Baris satu\nbaris dua"}
	got, err := s.validate(ok)
	if err != nil || got.Title != "Tata Surya" {
		t.Fatalf("valid: %+v %v", got, err)
	}
	for _, tc := range []struct {
		name  string
		edit  func(*Input)
		field string
	}{
		{"empty title", func(in *Input) { in.Title = " " }, "title"},
		{"control char", func(in *Input) { in.Title = "a\x00b" }, "title"},
		{"long description", func(in *Input) { in.Description = strings.Repeat("a", 501) }, "description"},
		{"unknown level", func(in *Input) { in.Level = "TK" }, "level"},
		{"unknown building", func(in *Input) { in.BuildingKey = "kantin" }, "building_key"},
		{"staff-only building", func(in *Input) { in.BuildingKey = "ruang-guru" }, "building_key"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			in := ok
			tc.edit(&in)
			_, err := s.validate(in)
			var v *ValidationError
			if !errors.As(err, &v) || v.Fields[tc.field] == "" {
				t.Fatalf("want %s error, got %v", tc.field, err)
			}
		})
	}
}

func TestValidateStep(t *testing.T) {
	file := uuid.New()
	for _, tc := range []struct {
		name  string
		in    StepInput
		field string
	}{
		{"materi needs body", StepInput{Kind: "materi", Title: "A", Minutes: 5}, "body"},
		{"berkas needs file", StepInput{Kind: "berkas", Title: "A", Minutes: 5}, "file_id"},
		{"video needs link", StepInput{Kind: "video", Title: "A", Minutes: 5, VideoID: "https://vimeo.com/1"}, "video_id"},
		{"bad kind", StepInput{Kind: "kuis", Title: "A", Minutes: 5}, "kind"},
		{"zero minutes", StepInput{Kind: "materi", Title: "A", Body: "x", Minutes: 0}, "minutes"},
		{"long body", StepInput{Kind: "materi", Title: "A", Body: strings.Repeat("a", 5001), Minutes: 1}, "body"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := validateStep(tc.in)
			var v *ValidationError
			if !errors.As(err, &v) || v.Fields[tc.field] == "" {
				t.Fatalf("want %s error, got %v", tc.field, err)
			}
		})
	}
	// Fields that do not belong to the kind are dropped, so the DB CHECKs always hold.
	got, err := validateStep(StepInput{Kind: "materi", Title: "A", Body: "isi", Minutes: 3, FileID: &file, VideoID: "dQw4w9WgXcQ"})
	if err != nil || got.FileID != nil || got.VideoID != "" {
		t.Fatalf("materi cleanup: %+v %v", got, err)
	}
	got, err = validateStep(StepInput{Kind: "berkas", Title: "A", Minutes: 3, FileID: &file, VideoID: "dQw4w9WgXcQ"})
	if err != nil || got.VideoID != "" || got.FileID == nil {
		t.Fatalf("berkas cleanup: %+v %v", got, err)
	}
}

func TestYouTubeID(t *testing.T) {
	const id = "dQw4w9WgXcQ"
	for _, in := range []string{
		id,
		" https://www.youtube.com/watch?v=" + id + "&t=42 ",
		"https://youtube.com/watch?v=" + id,
		"https://m.youtube.com/watch?v=" + id,
		"https://youtu.be/" + id + "?si=abc",
		"https://www.youtube.com/embed/" + id,
		"https://www.youtube-nocookie.com/embed/" + id,
		"https://www.youtube.com/shorts/" + id,
	} {
		if got := youTubeID(in); got != id {
			t.Errorf("youTubeID(%q) = %q", in, got)
		}
	}
	for _, in := range []string{
		"", "dQw4w9WgXc", "https://evil.example/watch?v=" + id, "http://youtu.be/" + id,
		"https://youtu.be/" + id + "x", "javascript:alert(1)", "https://www.youtube.com/watch?list=x&v=" + id,
	} {
		if got := youTubeID(in); got != "" {
			t.Errorf("youTubeID(%q) = %q, want rejected", in, got)
		}
	}
}
