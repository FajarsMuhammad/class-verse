package file

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/auth"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/org"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/dbtest"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/db/query"
)

func TestUploadHTTP(t *testing.T) {
	if os.Getenv("TEST_STORAGE_ENDPOINT") == "" {
		t.Skip("TEST_STORAGE_* not configured")
	}
	pool := dbtest.New(t)
	ctx := t.Context()
	clock := func() time.Time { return time.Now().UTC() }
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	q := query.New(pool)
	user, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: "http-chunks@example.test", DisplayName: "Teacher", BirthDate: pgtype.Date{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	school, err := org.NewService(org.NewStore(pool), clock, log).Create(ctx, user.ID, org.CreateInput{Name: "HTTP School", Slug: "http-school"})
	if err != nil {
		t.Fatal(err)
	}
	storage, err := NewStorage(os.Getenv("TEST_STORAGE_ENDPOINT"), os.Getenv("TEST_STORAGE_BUCKET"), os.Getenv("TEST_STORAGE_ACCESS_KEY"), os.Getenv("TEST_STORAGE_SECRET_KEY"), false)
	if err != nil {
		t.Fatal(err)
	}
	tokens := auth.NewTokens(ed25519.NewKeyFromSeed(make([]byte, 32)), clock)
	token, err := tokens.Issue(user.ID)
	if err != nil {
		t.Fatal(err)
	}
	r := chi.NewRouter()
	r.Use(auth.RequireUser(tokens, log))
	NewHandler(NewUploader(NewService(NewStore(pool), clock), storage), log, clock).Routes(r)
	request := func(method, path string, body io.Reader, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, body)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w
	}
	base := "/orgs/" + school.ID.String() + "/uploads"
	request("POST", base, strings.NewReader(`{"size":0,"kind":"pdf","sha256":"bad"}`), 400)
	pdf := []byte("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF\n")
	digest := sha256.Sum256(pdf)
	startBody := fmt.Sprintf(`{"id":"%s","name":"Modul.pdf","size":%d,"kind":"pdf","sha256":"%s"}`, uuid.New(), len(pdf), hex.EncodeToString(digest[:]))
	request("POST", base, strings.NewReader(strings.Replace(startBody, "Modul.pdf", "", 1)), 400)
	w := request("POST", base, strings.NewReader(startBody), 201)
	request("POST", base, strings.NewReader(startBody), 201)
	var s UploadProgress
	if err := json.Unmarshal(w.Body.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	path := base + "/" + s.ID.String()
	request("GET", path, nil, 200)
	request("PUT", path+"/parts/0", bytes.NewReader(pdf), 400)
	request("PUT", path+"/parts/1", strings.NewReader("a"), 400)
	request("POST", path+"/complete", nil, 409)
	request("PUT", path+"/parts/1", bytes.NewReader(pdf), 204)
	request("GET", path, nil, 200)
	request("POST", path+"/complete", nil, 200)
	request("POST", path+"/complete", nil, 200)

	files := "/orgs/" + school.ID.String() + "/files"
	var list struct {
		Items     []Item `json:"items"`
		UsedBytes int64  `json:"used_bytes"`
	}
	if err := json.Unmarshal(request("GET", files, nil, 200).Body.Bytes(), &list); err != nil || len(list.Items) != 1 || list.UsedBytes != int64(len(pdf)) {
		t.Fatalf("list %+v %v", list, err)
	}
	content := files + "/" + s.ID.String() + "/content"
	full := request("GET", content, nil, 200)
	if !bytes.Equal(full.Body.Bytes(), pdf) || full.Header().Get("Content-Type") != "application/pdf" || full.Header().Get("Accept-Ranges") != "bytes" || full.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("content %q %v", full.Body.String(), full.Header())
	}
	ranged := func(header string, want int) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest("GET", content, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Range", header)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != want {
			t.Fatalf("range %s: %d %s", header, rec.Code, rec.Body.String())
		}
		return rec
	}
	if rec := ranged("bytes=0-3", 206); rec.Body.String() != "%PDF" || rec.Header().Get("Content-Range") != fmt.Sprintf("bytes 0-3/%d", len(pdf)) {
		t.Fatalf("206 %q %v", rec.Body.String(), rec.Header())
	}
	if rec := ranged("bytes=-6", 206); rec.Body.String() != "%%EOF\n" {
		t.Fatalf("suffix %q", rec.Body.String())
	}
	if rec := ranged(fmt.Sprintf("bytes=%d-", len(pdf)), 416); rec.Header().Get("Content-Range") != fmt.Sprintf("bytes */%d", len(pdf)) {
		t.Fatalf("416 %v", rec.Header())
	}
	ranged("bytes=0-1,4-5", 416)
	request("PUT", files+"/"+s.ID.String()+"/position", strings.NewReader(`{"position":2}`), 204)
	request("PUT", files+"/"+s.ID.String()+"/position", strings.NewReader(`{"position":-1}`), 400)

	outsider, err := q.CreateTestUser(ctx, query.CreateTestUserParams{Email: "outsider@example.test", DisplayName: "Outsider", BirthDate: pgtype.Date{Time: time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC), Valid: true}})
	if err != nil {
		t.Fatal(err)
	}
	outsiderToken, err := tokens.Issue(outsider.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{content, files} {
		req := httptest.NewRequest("GET", p, nil)
		req.Header.Set("Authorization", "Bearer "+outsiderToken)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		if rec.Code != 403 || rec.Body.Len() > 200 {
			t.Fatalf("outsider %s: %d", p, rec.Code)
		}
	}

	request("DELETE", path, nil, 409)
	request("DELETE", files+"/"+s.ID.String(), nil, 204)
	request("DELETE", files+"/"+s.ID.String(), nil, 204)
	request("GET", content, nil, 404)
	request("GET", base+"/bad", nil, 404)
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Fatal(rec.Code)
	}
}
