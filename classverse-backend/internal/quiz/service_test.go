package quiz

import (
	"errors"
	"strings"
	"testing"
)

func TestValidateQuestion(t *testing.T) {
	ok := QuestionInput{Prompt: "Planet terbesar?", Options: []string{"Bumi", "Jupiter", "Mars", "Venus"}, Answer: 1}
	tests := []struct {
		name   string
		modify func(*QuestionInput)
		field  string
	}{
		{"valid", func(*QuestionInput) {}, ""},
		{"empty prompt", func(q *QuestionInput) { q.Prompt = "  " }, "prompt"},
		{"long prompt", func(q *QuestionInput) { q.Prompt = strings.Repeat("a", 501) }, "prompt"},
		{"three options", func(q *QuestionInput) { q.Options = q.Options[:3] }, "options"},
		{"five options", func(q *QuestionInput) { q.Options = append(q.Options, "Saturnus") }, "options"},
		{"blank option", func(q *QuestionInput) { q.Options = []string{"Bumi", " ", "Mars", "Venus"} }, "options"},
		{"duplicate option", func(q *QuestionInput) { q.Options = []string{"Bumi", "bumi ", "Mars", "Venus"} }, "options"},
		{"long option", func(q *QuestionInput) { q.Options = []string{strings.Repeat("x", 201), "b", "c", "d"} }, "options"},
		{"answer below range", func(q *QuestionInput) { q.Answer = -1 }, "answer"},
		{"answer above range", func(q *QuestionInput) { q.Answer = 4 }, "answer"},
		{"long explanation", func(q *QuestionInput) { q.Explanation = strings.Repeat("e", 501) }, "explanation"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := ok
			in.Options = append([]string(nil), ok.Options...)
			tt.modify(&in)
			_, err := validateQuestion(in)
			if tt.field == "" {
				if err != nil {
					t.Fatalf("unexpected %v", err)
				}
				return
			}
			var v *ValidationError
			if !errors.As(err, &v) || v.Fields[tt.field] == "" {
				t.Fatalf("want field %s, got %v", tt.field, err)
			}
		})
	}
}

func TestValidateQuizPassScore(t *testing.T) {
	s := &Service{buildings: fakeBuildings{"sains": false, "ruang-guru": true}}
	base := Input{BuildingKey: "sains", Level: "SD", Title: "Kuis Planet", PassScore: 60}
	for _, tc := range []struct {
		pass  int
		staff bool
		field string
	}{{60, false, ""}, {1, false, ""}, {100, false, ""}, {0, false, "pass_score"}, {101, false, "pass_score"}, {60, true, "building_key"}} {
		in := base
		in.PassScore = tc.pass
		if tc.staff {
			in.BuildingKey = "ruang-guru"
		}
		_, err := s.validate(in)
		if tc.field == "" && err != nil {
			t.Fatalf("pass=%d: %v", tc.pass, err)
		}
		if tc.field != "" {
			var v *ValidationError
			if !errors.As(err, &v) || v.Fields[tc.field] == "" {
				t.Fatalf("pass=%d staff=%v: want %s, got %v", tc.pass, tc.staff, tc.field, err)
			}
		}
	}
}

func TestGrade(t *testing.T) {
	qs := []Snapshot{{Answer: 0}, {Answer: 1}, {Answer: 2}}
	for _, tc := range []struct {
		answers        []int16
		correct, score int
	}{
		{[]int16{0, 1, 2}, 3, 100},
		{[]int16{0, 1, 3}, 2, 66}, // floor, never rounds up into a pass
		{[]int16{0, -1, -1}, 1, 33},
		{[]int16{-1, -1, -1}, 0, 0},
	} {
		c, s := grade(qs, tc.answers)
		if c != tc.correct || s != tc.score {
			t.Fatalf("%v: got %d/%d want %d/%d", tc.answers, c, s, tc.correct, tc.score)
		}
	}
}

type fakeBuildings map[string]bool

func (b fakeBuildings) Building(key string) (bool, bool) {
	staff, ok := b[key]
	return staff, ok
}
