package module

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strings"
)

const maxRichBytes = 128 * 1024

// richChildren is the editor schema allowlist: parent node type -> permitted child types.
var richChildren = map[string][]string{
	"": {"doc"}, "doc": {"paragraph", "heading", "bulletList", "orderedList", "codeBlock", "callout", "taskList"},
	"paragraph": {"text", "hardBreak"}, "heading": {"text", "hardBreak"}, "codeBlock": {"text"},
	"bulletList": {"listItem"}, "orderedList": {"listItem"}, "listItem": {"paragraph", "bulletList", "orderedList"},
	"callout": {"paragraph"}, "taskList": {"taskItem"}, "taskItem": {"paragraph"},
}

type richNode struct {
	Type    string                     `json:"type"`
	Attrs   map[string]json.RawMessage `json:"attrs,omitempty"`
	Content []richNode                 `json:"content,omitempty"`
	Text    string                     `json:"text,omitempty"`
	Marks   []richMark                 `json:"marks,omitempty"`
}
type richMark struct {
	Type  string                     `json:"type"`
	Attrs map[string]json.RawMessage `json:"attrs,omitempty"`
}

func decodeRich(raw []byte, target any) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	return d.Decode(target)
}

// validateRichContent accepts only the shared editor schema, never arbitrary HTML or executable nodes.
func validateRichContent(raw json.RawMessage) (json.RawMessage, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	if len(raw) > maxRichBytes {
		return nil, fmt.Errorf("rich content exceeds size limit")
	}
	var root richNode
	if err := decodeRich(raw, &root); err != nil {
		return nil, fmt.Errorf("decode rich content: %w", err)
	}
	count := 0
	ids := map[string]bool{}
	var walk func(richNode, string, int) error
	walk = func(n richNode, parent string, depth int) error {
		count++
		if depth > 12 || count > 1000 {
			return fmt.Errorf("rich content exceeds structural limit")
		}
		if !slices.Contains(richChildren[parent], n.Type) {
			return fmt.Errorf("node %s not allowed inside %s", n.Type, parent)
		}
		attrs := []string{}
		if n.Type != "doc" && n.Type != "text" && n.Type != "hardBreak" {
			attrs = append(attrs, "id")
			var id string
			if json.Unmarshal(n.Attrs["id"], &id) != nil || len(id) < 1 || len(id) > 80 || ids[id] {
				return fmt.Errorf("block id missing or duplicated")
			}
			ids[id] = true
		}
		switch n.Type {
		case "doc":
			attrs = append(attrs, "schema_version")
			if raw, ok := n.Attrs["schema_version"]; ok {
				var version int
				if json.Unmarshal(raw, &version) != nil || version != 1 {
					return fmt.Errorf("unsupported rich content schema")
				}
			}
		case "heading":
			attrs = append(attrs, "level")
			var level int
			if json.Unmarshal(n.Attrs["level"], &level) != nil || level < 2 || level > 4 {
				return fmt.Errorf("invalid heading level")
			}
		case "orderedList":
			attrs = append(attrs, "start")
			if v, ok := n.Attrs["start"]; ok {
				var start int
				if json.Unmarshal(v, &start) != nil || start < 1 || start > 1000 {
					return fmt.Errorf("invalid list start")
				}
			}
		case "codeBlock":
			attrs = append(attrs, "language")
			if v, ok := n.Attrs["language"]; ok && string(v) != "null" {
				var lang string
				if json.Unmarshal(v, &lang) != nil || len(lang) > 30 || strings.ContainsAny(lang, "<>\"'") {
					return fmt.Errorf("invalid code language")
				}
			}
		case "taskItem":
			attrs = append(attrs, "checked")
			var checked bool
			if json.Unmarshal(n.Attrs["checked"], &checked) != nil {
				return fmt.Errorf("invalid checklist state")
			}
		}
		for a := range n.Attrs {
			if !slices.Contains(attrs, a) {
				return fmt.Errorf("attribute %s not allowed", a)
			}
		}
		if n.Type != "text" && (n.Text != "" || len(n.Marks) > 0) {
			return fmt.Errorf("only text has marks and text")
		}
		if n.Type == "text" && (len(n.Content) > 0 || n.Text == "") {
			return fmt.Errorf("invalid text node")
		}
		for _, m := range n.Marks {
			if !slices.Contains([]string{"bold", "italic", "code", "link"}, m.Type) || parent == "codeBlock" {
				return fmt.Errorf("invalid mark")
			}
			if m.Type != "link" {
				if len(m.Attrs) > 0 {
					return fmt.Errorf("invalid mark attributes")
				}
				continue
			}
			for a := range m.Attrs {
				if a != "href" {
					return fmt.Errorf("invalid link attribute")
				}
			}
			var href string
			if json.Unmarshal(m.Attrs["href"], &href) != nil || len(href) > 2048 {
				return fmt.Errorf("invalid link")
			}
			u, err := url.Parse(href)
			if err != nil || !slices.Contains([]string{"https", "http", "mailto"}, u.Scheme) || strings.ContainsAny(href, "\r\n\t") || (u.Scheme != "mailto" && (u.Host == "" || u.User != nil)) {
				return fmt.Errorf("unsafe link")
			}
		}
		for _, child := range n.Content {
			if err := walk(child, n.Type, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(root, "", 0); err != nil {
		return nil, err
	}
	root.Attrs = map[string]json.RawMessage{"schema_version": json.RawMessage("1")}
	out, err := json.Marshal(root)
	if err != nil {
		return nil, fmt.Errorf("encode rich content: %w", err)
	}
	return out, nil
}
