package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-jose/go-jose/v4"
	"github.com/golang-jwt/jwt/v5"
)

const testFrontend = "http://localhost:5173"

func newTestServer(t *testing.T, e *env, google Google) http.Handler {
	t.Helper()
	h := NewHandler(e.svc, e.tokens, google, nil, testFrontend, e.clock.Now, slog.New(slog.NewTextHandler(io.Discard, nil)))
	r := chi.NewRouter()
	r.Route("/api", h.Routes)
	return r
}

func do(h http.Handler, method, path, body string, mods ...func(*http.Request)) *httptest.ResponseRecorder {
	var rd io.Reader = http.NoBody
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rd)
	req.Header.Set("Content-Type", "application/json")
	for _, m := range mods {
		m(req)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func withOrigin(o string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("Origin", o) }
}
func withCookie(c *http.Cookie) func(*http.Request) { return func(r *http.Request) { r.AddCookie(c) } }
func withBearer(tok string) func(*http.Request) {
	return func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+tok) }
}

func findCookie(t *testing.T, rec *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("cookie %q not set; headers %v", name, rec.Header())
	return nil
}

func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("not an error JSON: %q", rec.Body.String())
	}
	return body.Error.Code
}

func TestHandlerRegisterVerifyRefreshFlow(t *testing.T) {
	e := newEnv(t)
	h := newTestServer(t, e, nil)

	rec := do(h, http.MethodPost, "/api/auth/register", `{"email":"a@b.co","password":"rahasia-123","display_name":"Ani","birth_date":"2008-05-17"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("register = %d %s", rec.Code, rec.Body)
	}
	rec = do(h, http.MethodPost, "/api/auth/verify-email", `{"email":"a@b.co","code":"`+e.mailer.lastCode()+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("verify = %d %s", rec.Code, rec.Body)
	}
	c := findCookie(t, rec, refreshCookie)
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode || c.Path != "/api/auth" || c.MaxAge != 2592000 {
		t.Errorf("refresh cookie attributes = %+v", c)
	}
	var tok tokenResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &tok)

	rec = do(h, http.MethodGet, "/api/me", "", withBearer(tok.AccessToken))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"needs_profile":false`) || !strings.Contains(rec.Body.String(), `"orgs":[]`) {
		t.Errorf("me = %d %s", rec.Code, rec.Body)
	}
	if rec := do(h, http.MethodGet, "/api/me", ""); rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "unauthenticated" {
		t.Errorf("me without token = %d %s", rec.Code, rec.Body)
	}

	// Foreign Origin is rejected before the token is touched.
	rec = do(h, http.MethodPost, "/api/auth/refresh", "", withCookie(c), withOrigin("https://evil.example"))
	if rec.Code != http.StatusForbidden {
		t.Errorf("refresh foreign origin = %d", rec.Code)
	}
	rec = do(h, http.MethodPost, "/api/auth/refresh", "", withCookie(c), withOrigin(testFrontend))
	if rec.Code != http.StatusOK {
		t.Fatalf("refresh = %d %s", rec.Code, rec.Body)
	}
	next := findCookie(t, rec, refreshCookie)

	// Reuse of the old cookie revokes the session and clears the cookie.
	rec = do(h, http.MethodPost, "/api/auth/refresh", "", withCookie(c), withOrigin(testFrontend))
	if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "session_revoked" {
		t.Errorf("reuse = %d %s", rec.Code, rec.Body)
	}
	if cleared := findCookie(t, rec, refreshCookie); cleared.MaxAge >= 0 {
		t.Errorf("cookie not cleared: %+v", cleared)
	}
	rec = do(h, http.MethodPost, "/api/auth/logout", "", withCookie(next), withOrigin(testFrontend))
	if rec.Code != http.StatusNoContent {
		t.Errorf("logout = %d", rec.Code)
	}
}

func TestHandlerProfilePartial(t *testing.T) {
	e := newEnv(t)
	h := newTestServer(t, e, nil)
	session, err := e.svc.GoogleLogin(context.Background(), GoogleIdentity{Subject: "profile", Email: "profile@b.co", EmailVerified: true, Name: "Ani"})
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{"display_name":null}`, `{"avatar_key":null}`, `{"birth_date":null}`, `{"avatar_key":"../file"}`, `{"display_name":"bangsat"}`} {
		rec := do(h, http.MethodPatch, "/api/me", body, withBearer(session.AccessToken))
		if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "validation_failed" {
			t.Fatalf("invalid profile status %d", rec.Code)
		}
	}
	rec := do(h, http.MethodPatch, "/api/me", `{"avatar_key":"cap-mangga"}`, withBearer(session.AccessToken))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"display_name":"Ani"`) || !strings.Contains(rec.Body.String(), `"avatar_key":"cap-mangga"`) {
		t.Fatal("partial profile update failed")
	}
	rec = do(h, http.MethodPatch, "/api/me", `{"avatar_key":"sporty-headphones-zamrud"}`, withBearer(session.AccessToken))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"avatar_key":"sporty-headphones-zamrud"`) {
		t.Fatal("sporty profile update failed")
	}
	if rec := do(h, http.MethodPatch, "/api/me", `{}`); rec.Code != http.StatusUnauthorized {
		t.Fatal("unauthenticated patch accepted")
	}
}

func TestHandlerErrors(t *testing.T) {
	e := newEnv(t)
	h := newTestServer(t, e, nil)

	rec := do(h, http.MethodPost, "/api/auth/register", `{"email":"bad","password":"x","display_name":"A","birth_date":""}`)
	if rec.Code != http.StatusBadRequest || errorCode(t, rec) != "validation_failed" || !strings.Contains(rec.Body.String(), `"fields"`) {
		t.Errorf("validation = %d %s", rec.Code, rec.Body)
	}
	rec = do(h, http.MethodPost, "/api/auth/login", `{"email":"x@b.co","password":"whatever-123"}`)
	if rec.Code != http.StatusUnauthorized || errorCode(t, rec) != "invalid_credentials" {
		t.Errorf("login unknown = %d %s", rec.Code, rec.Body)
	}
	rec = do(h, http.MethodPost, "/api/auth/login", `{"email":`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("bad json = %d", rec.Code)
	}
}

func TestHandlerLoginRateLimit(t *testing.T) {
	e := newEnv(t)
	h := newTestServer(t, e, nil)
	body := `{"email":"x@b.co","password":"whatever-123"}`
	for i := range 5 {
		if rec := do(h, http.MethodPost, "/api/auth/login", body); rec.Code != http.StatusUnauthorized {
			t.Fatalf("attempt %d = %d", i+1, rec.Code)
		}
	}
	rec := do(h, http.MethodPost, "/api/auth/login", body)
	if rec.Code != http.StatusTooManyRequests || errorCode(t, rec) != "rate_limited" || rec.Header().Get("Retry-After") == "" {
		t.Errorf("6th attempt = %d %s %v", rec.Code, rec.Body, rec.Header())
	}
	e.clock.Advance(time.Minute)
	if rec := do(h, http.MethodPost, "/api/auth/login", body); rec.Code != http.StatusUnauthorized {
		t.Errorf("after window = %d", rec.Code)
	}
}

// fakeOIDC serves discovery, JWKS, and a token endpoint like Google does.
type fakeOIDC struct {
	srv      *httptest.Server
	key      *rsa.PrivateKey
	clientID string
	claims   jwt.MapClaims
	gotPKCE  string
}

func newFakeOIDC(t *testing.T, clientID string) *fakeOIDC {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeOIDC{key: key, clientID: clientID}
	mux := http.NewServeMux()
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)

	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                f.srv.URL,
			"authorization_endpoint":                f.srv.URL + "/auth",
			"token_endpoint":                        f.srv.URL + "/token",
			"jwks_uri":                              f.srv.URL + "/jwks",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("/jwks", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		f.gotPKCE = r.PostForm.Get("code_verifier")
		claims := jwt.MapClaims{
			"iss": f.srv.URL, "aud": f.clientID,
			"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		}
		for k, v := range f.claims {
			claims[k] = v
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = "k1"
		raw, err := tok.SignedString(key)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": raw})
	})
	return f
}

func TestHandlerGoogleFlow(t *testing.T) {
	tests := []struct {
		name       string
		claims     jwt.MapClaims
		tamper     func(q url.Values)
		wantError  string
		wantCookie bool
	}{
		{
			name:       "new user",
			claims:     jwt.MapClaims{"sub": "g-1", "email": "ani@b.co", "email_verified": true, "name": "Ani"},
			wantCookie: true,
		},
		{
			name:      "email not verified",
			claims:    jwt.MapClaims{"sub": "g-1", "email": "ani@b.co", "email_verified": false},
			wantError: "google_email_unverified",
		},
		{
			name:      "state mismatch",
			claims:    jwt.MapClaims{"sub": "g-1", "email": "ani@b.co", "email_verified": true},
			tamper:    func(q url.Values) { q.Set("state", "other") },
			wantError: "invalid_state",
		},
		{
			name:      "wrong audience",
			claims:    jwt.MapClaims{"sub": "g-1", "email": "ani@b.co", "email_verified": true, "aud": "someone-else"},
			wantError: "google_failed",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t)
			f := newFakeOIDC(t, "client-1")
			f.claims = tt.claims
			gc, err := NewGoogleClient(context.Background(), f.srv.URL, "client-1", "secret", testFrontend+"/api/auth/google/callback")
			if err != nil {
				t.Fatal(err)
			}
			h := newTestServer(t, e, gc)

			rec := do(h, http.MethodGet, "/api/auth/google/start", "")
			if rec.Code != http.StatusFound {
				t.Fatalf("start = %d", rec.Code)
			}
			oc := findCookie(t, rec, oauthCookie)
			if !oc.HttpOnly || !oc.Secure || oc.SameSite != http.SameSiteLaxMode || oc.Path != "/api/auth/google" || oc.MaxAge != 600 {
				t.Errorf("oauth cookie = %+v", oc)
			}
			loc, _ := url.Parse(rec.Header().Get("Location"))
			if loc.Query().Get("code_challenge_method") != "S256" || loc.Query().Get("code_challenge") == "" {
				t.Errorf("auth URL lacks PKCE: %s", loc)
			}

			cb := url.Values{"state": {loc.Query().Get("state")}, "code": {"c1"}}
			if tt.tamper != nil {
				tt.tamper(cb)
			}
			rec = do(h, http.MethodGet, "/api/auth/google/callback?"+cb.Encode(), "", withCookie(oc))
			if rec.Code != http.StatusFound {
				t.Fatalf("callback = %d", rec.Code)
			}
			if cleared := findCookie(t, rec, oauthCookie); cleared.MaxAge >= 0 {
				t.Error("oauth cookie not cleared")
			}
			target := rec.Header().Get("Location")
			if tt.wantError != "" {
				if target != testFrontend+"/auth/callback?error="+tt.wantError {
					t.Errorf("redirect = %q", target)
				}
				return
			}
			if target != testFrontend+"/auth/callback" {
				t.Errorf("redirect = %q", target)
			}
			if f.gotPKCE == "" || !strings.HasSuffix(oc.Value, "."+f.gotPKCE) {
				t.Error("token exchange did not send the stored PKCE verifier")
			}
			if tt.wantCookie {
				findCookie(t, rec, refreshCookie)
			}
		})
	}
}
