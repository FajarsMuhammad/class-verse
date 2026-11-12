package file

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestStorageConfig(t *testing.T) {
	for _, tc := range []struct{ name, endpoint, bucket string }{
		{"URL instead of host", "https://localhost:9000", "materials"},
		{"missing bucket", "localhost:9000", ""},
		{"missing endpoint", "", "materials"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewStorage(tc.endpoint, tc.bucket, "access", "secret", false); err == nil {
				t.Fatal("expected invalid config")
			}
		})
	}
}

func TestStorageReadRange(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/materials/org/file" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Range") != "bytes=2-4" {
			t.Errorf("range: %s", r.Header.Get("Range"))
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "AWS4-HMAC-SHA256 ") {
			t.Error("missing signed authentication")
		}
		w.Header().Set("Last-Modified", time.Now().UTC().Format(http.TimeFormat))
		w.Header().Set("Content-Range", "bytes 2-4/6")
		w.Header().Set("Content-Length", "3")
		w.Header().Set("Content-Type", "application/pdf")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, "cde")
	}))
	defer server.Close()
	storage, err := NewStorage(strings.TrimPrefix(server.URL, "http://"), "materials", "access", "secret", false)
	if err != nil {
		t.Fatal(err)
	}
	body, info, headers, err := storage.Read(context.Background(), "org/file", 2, 4)
	if err != nil {
		t.Fatal(err)
	}
	defer body.Close()
	got, err := io.ReadAll(body)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "cde" || info.Size != 3 || headers.Get("Content-Range") != "bytes 2-4/6" {
		t.Fatalf("unexpected range response: %q, %d", got, info.Size)
	}
}

func TestStorageErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `<Error><Code>AccessDenied</Code><Message>Denied</Message></Error>`)
	}))
	defer server.Close()
	s, err := NewStorage(strings.TrimPrefix(server.URL, "http://"), "materials", "access", "secret", false)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := s.Put(ctx, "file", strings.NewReader("test"), -1, "application/pdf"); err == nil {
		t.Fatal("unknown upload size accepted")
	}
	if err := s.Put(ctx, "file", strings.NewReader("test"), 4, "application/pdf"); err == nil {
		t.Fatal("upload denial ignored")
	}
	if err := s.Delete(ctx, "file"); err == nil {
		t.Fatal("delete denial ignored")
	}
	if _, _, _, err := s.Read(ctx, "file", 0, -1); err == nil {
		t.Fatal("read denial ignored")
	}
	for _, bounds := range [][2]int64{{-1, 4}, {4, 2}} {
		if _, _, _, err := s.Read(ctx, "file", bounds[0], bounds[1]); err == nil {
			t.Fatal("invalid range accepted")
		}
	}
}

func TestStorageIntegration(t *testing.T) {
	endpoint := os.Getenv("TEST_STORAGE_ENDPOINT")
	if endpoint == "" {
		t.Skip("set TEST_STORAGE_* to run against private AIStor")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	s, err := NewStorage(endpoint, os.Getenv("TEST_STORAGE_BUCKET"), os.Getenv("TEST_STORAGE_ACCESS_KEY"), os.Getenv("TEST_STORAGE_SECRET_KEY"), false)
	if err != nil {
		t.Fatal(err)
	}
	key := "smoke/" + time.Now().UTC().Format("20060102T150405.000000000")
	data := "%PDF-1.7\nstorage range smoke\n%%EOF\n"
	if err = s.Put(ctx, key, strings.NewReader(data), int64(len(data)), "application/pdf"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := s.Delete(context.Background(), key); err != nil {
			t.Error(err)
		}
	}()
	for _, tc := range []struct {
		name       string
		start, end int64
		want       string
	}{
		{"full", 0, -1, data}, {"partial", 0, 4, "%PDF-"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, _, _, err := s.Read(ctx, key, tc.start, tc.end)
			if err != nil {
				t.Fatal(err)
			}
			defer body.Close()
			got, err := io.ReadAll(body)
			if err != nil || string(got) != tc.want {
				t.Fatalf("content mismatch, err=%v", err)
			}
		})
	}
	if _, _, _, err := s.Read(ctx, key, int64(len(data)+1), int64(len(data)+2)); err == nil {
		t.Fatal("out-of-bounds range accepted")
	}
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get("http://" + endpoint + "/" + os.Getenv("TEST_STORAGE_BUCKET") + "/" + key)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("anonymous read status %d", res.StatusCode)
	}
}
