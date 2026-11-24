package httpx

import (
	"net/http/httptest"
	"testing"
)

func TestPagination(t *testing.T) {
	for _, raw := range []string{"?limit=0", "?limit=101", "?offset=-1", "?offset=bad"} {
		if _, _, err := Pagination(httptest.NewRequest("GET", "/"+raw, nil)); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	limit, offset, err := Pagination(httptest.NewRequest("GET", "/?limit=20&offset=40", nil))
	if err != nil || limit != 20 || offset != 40 {
		t.Fatalf("page: %d %d %v", limit, offset, err)
	}
}
