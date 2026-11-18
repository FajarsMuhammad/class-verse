package module

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestValidateRichContent(t *testing.T) {
	valid := `{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"block-1"},"content":[{"type":"text","text":"Hello <script>","marks":[{"type":"bold"},{"type":"link","attrs":{"href":"https://example.org"}}]}]}]}`
	for _, tt := range []struct {
		name, raw string
		valid     bool
	}{
		{"document", valid, true},
		{"javascript", strings.Replace(valid, "https://example.org", "javascript:alert(1)", 1), false},
		{"relative link", strings.Replace(valid, "https://example.org", "//evil.test", 1), false},
		{"event handler", strings.Replace(valid, `"href":`, `"onclick":"alert(1)","href":`, 1), false},
		{"html node", strings.Replace(valid, `"paragraph"`, `"iframe"`, 1), false},
		{"heading depth", `{"type":"doc","content":[{"type":"heading","attrs":{"id":"h","level":1}}]}`, false},
		{"unknown field", strings.Replace(valid, `"type":"doc"`, `"type":"doc","html":"x"`, 1), false},
		{"huge payload", strings.Repeat("x", 131073), false},
		{"missing block id", `{"type":"doc","content":[{"type":"paragraph"}]}`, false},
		{"empty paragraph", `{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"p"}}]}`, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateRichContent(json.RawMessage(tt.raw))
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v error=%v", tt.valid, err)
			}
		})
	}
}

func TestRichContentStructuralLimits(t *testing.T) {
	for _, raw := range []string{
		`{"type":"doc","content":[{"type":"orderedList","attrs":{"id":"o","start":0}}]}`,
		`{"type":"doc","content":[{"type":"codeBlock","attrs":{"id":"c","language":"<bad>"}}]}`,
		`{"type":"doc","content":[{"type":"taskList","attrs":{"id":"t"},"content":[{"type":"taskItem","attrs":{"id":"i","checked":"yes"}}]}]}`,
		`{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"p","evil":1}}]}`,
		`{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"p"},"text":"wrong"}]}`,
		`{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"p"},"content":[{"type":"text","text":"","content":[{"type":"text","text":"x"}]}]}]}`,
		`{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"p"},"content":[{"type":"text","text":"x","marks":[{"type":"underline"}]}]}]}`,
		`{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"p"},"content":[{"type":"text","text":"x","marks":[{"type":"bold","attrs":{"evil":1}}]}]}]}`,
		`{"type":"doc","content":[{"type":"paragraph","attrs":{"id":"p"},"content":[{"type":"text","text":"x","marks":[{"type":"link","attrs":{"href":42}}]}]}]}`,
	} {
		if _, err := validateRichContent(json.RawMessage(raw)); err == nil {
			t.Fatalf("accepted invalid structure: %s", raw)
		}
	}
	if _, err := validateRichContent(nil); err != nil {
		t.Fatal(err)
	}
	nodes := make([]string, 1001)
	for i := range nodes {
		nodes[i] = fmt.Sprintf(`{"type":"paragraph","attrs":{"id":"p%d"}}`, i)
	}
	if _, err := validateRichContent(json.RawMessage(`{"type":"doc","content":[` + strings.Join(nodes, ",") + `]}`)); err == nil {
		t.Fatal("node limit absent")
	}
	valid := `{"type":"doc","content":[{"type":"heading","attrs":{"id":"h","level":3},"content":[{"type":"text","text":"Heading"}]},{"type":"orderedList","attrs":{"id":"o","start":2},"content":[{"type":"listItem","attrs":{"id":"i"},"content":[{"type":"paragraph","attrs":{"id":"p"}}]}]},{"type":"codeBlock","attrs":{"id":"c","language":"go"},"content":[{"type":"text","text":"fmt.Println()"}]},{"type":"taskList","attrs":{"id":"t"},"content":[{"type":"taskItem","attrs":{"id":"ti","checked":false},"content":[{"type":"paragraph","attrs":{"id":"tp"}}]}]}]}`
	if _, err := validateRichContent(json.RawMessage(valid)); err != nil {
		t.Fatal(err)
	}
}
