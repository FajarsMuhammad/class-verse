package db

import "testing"

func TestContainsIsLiteral(t *testing.T) {
	if got := Contains(`50%_a\b`); !got.Valid || got.String != `%50\%\_a\\b%` {
		t.Fatalf("pattern: %+v", got)
	}
	if got := Contains(""); got.Valid {
		t.Fatalf("empty must mean no filter: %+v", got)
	}
}
