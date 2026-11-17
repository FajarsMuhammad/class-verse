package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"
	"golang.org/x/oauth2"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/api"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/httpx"
	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/platform/ratelimit"
)

const (
	refreshCookie  = "kv_refresh"
	refreshPath    = "/api/auth"
	oauthCookie    = "kv_oauth"
	oauthPath      = "/api/auth/google"
	oauthCookieTTL = 10 * time.Minute
)

// Google is the OAuth client used by the handler. Nil disables Google login.
type Google interface {
	AuthURL(state, verifier string) string
	Exchange(ctx context.Context, code, verifier string) (GoogleIdentity, error)
}

type limiters struct {
	loginIP, loginEmail, register, verify, refresh *ratelimit.Limiter
}

// newLimiters builds the per-endpoint rate limits from the slice 1 task.
func newLimiters(now func() time.Time) limiters {
	return limiters{
		loginIP:    ratelimit.New(5, time.Minute, now),
		loginEmail: ratelimit.New(5, time.Minute, now),
		register:   ratelimit.New(3, time.Minute, now),
		verify:     ratelimit.New(10, time.Minute, now),
		refresh:    ratelimit.New(30, time.Minute, now),
	}
}

// OrgLister lists the organizations of a user for GET /api/me
// (implemented by the org feature, wired in cmd/server).
type OrgLister interface {
	ListOrgSummaries(ctx context.Context, userID uuid.UUID) ([]api.OrgSummary, error)
}

// Handler serves /api/auth/* and /api/me.
type Handler struct {
	orgs        OrgLister
	svc         *Service
	tokens      *Tokens
	google      Google
	frontendURL string
	// DevOrigins are extra development-only origins (config.DevOrigins).
	DevOrigins []string
	log        *slog.Logger
	limit      limiters
}

// NewHandler wires the HTTP layer. google and orgs may be nil.
func NewHandler(svc *Service, tokens *Tokens, google Google, orgs OrgLister, frontendURL string, now func() time.Time, log *slog.Logger) *Handler {
	return &Handler{svc: svc, tokens: tokens, google: google, orgs: orgs, frontendURL: frontendURL, log: log, limit: newLimiters(now)}
}

// Routes mounts the auth endpoints under /api.
func (h *Handler) Routes(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.handle(h.register))
		r.Post("/verify-email", h.handle(h.verifyEmail))
		r.Post("/resend-otp", h.handle(h.resendOTP))
		r.Post("/login", h.handle(h.login))
		r.Post("/refresh", h.handle(h.refresh))
		r.Post("/logout", h.handle(h.logout))
		r.Get("/google/start", h.handle(h.googleStart))
		r.Get("/google/callback", h.googleCallback)
	})
	r.Group(func(r chi.Router) {
		r.Use(RequireUser(h.tokens, h.log))
		r.Get("/me", h.handle(h.me))
		r.Patch("/me", h.handle(h.updateMe))
	})
}

func (h *Handler) handle(fn func(w http.ResponseWriter, r *http.Request) error) http.HandlerFunc {
	return httpx.Handle(h.log, func(w http.ResponseWriter, r *http.Request) error {
		return toAPIError(fn(w, r))
	})
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
	BirthDate   string `json:"birth_date"`
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) error {
	if err := h.allow(w, h.limit.register, "ip:"+clientIP(r)); err != nil {
		return err
	}
	var req registerRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	err := h.svc.Register(r.Context(), RegisterInput(req))
	if err != nil {
		return err
	}
	w.WriteHeader(http.StatusAccepted)
	return nil
}

type emailRequest struct {
	Email string `json:"email"`
}

func (h *Handler) resendOTP(w http.ResponseWriter, r *http.Request) error {
	if err := h.allow(w, h.limit.register, "ip:"+clientIP(r)); err != nil {
		return err
	}
	var req emailRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if err := h.svc.ResendOTP(r.Context(), req.Email); err != nil {
		return err
	}
	w.WriteHeader(http.StatusAccepted)
	return nil
}

type verifyRequest struct {
	Email string `json:"email"`
	Code  string `json:"code"`
}

func (h *Handler) verifyEmail(w http.ResponseWriter, r *http.Request) error {
	if err := h.allow(w, h.limit.verify, "ip:"+clientIP(r)); err != nil {
		return err
	}
	var req verifyRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	sess, err := h.svc.VerifyEmail(r.Context(), req.Email, req.Code)
	if err != nil {
		return err
	}
	h.writeSession(w, r, sess)
	return nil
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) error {
	var req loginRequest
	if err := decodeJSON(r, &req); err != nil {
		return err
	}
	if err := h.allow(w, h.limit.loginIP, "ip:"+clientIP(r)); err != nil {
		return err
	}
	email, _ := normalizeEmail(req.Email)
	if err := h.allow(w, h.limit.loginEmail, "email:"+email); err != nil {
		return err
	}
	sess, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		return err
	}
	h.writeSession(w, r, sess)
	return nil
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) error {
	if err := h.checkOrigin(r); err != nil {
		return err
	}
	if err := h.allow(w, h.limit.refresh, "ip:"+clientIP(r)); err != nil {
		return err
	}
	sess, err := h.svc.Refresh(r.Context(), readCookie(r, refreshCookie))
	if err != nil {
		if errors.Is(err, ErrUnauthenticated) || errors.Is(err, ErrSessionRevoked) {
			clearCookie(w, refreshCookie, refreshPath, http.SameSiteStrictMode, h.secure(r))
		}
		return err
	}
	h.writeSession(w, r, sess)
	return nil
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) error {
	if err := h.checkOrigin(r); err != nil {
		return err
	}
	if err := h.svc.Logout(r.Context(), readCookie(r, refreshCookie)); err != nil {
		return err
	}
	clearCookie(w, refreshCookie, refreshPath, http.SameSiteStrictMode, h.secure(r))
	w.WriteHeader(http.StatusNoContent)
	return nil
}

func (h *Handler) googleStart(w http.ResponseWriter, r *http.Request) error {
	if h.google == nil {
		return &httpx.Error{Status: http.StatusNotFound, Code: "not_found", Message: "Login Google belum diaktifkan"}
	}
	// Both values are 32 random bytes in base64url, which never contains '.',
	// so they can share one cookie separated by a dot.
	state := oauth2.GenerateVerifier()
	verifier := oauth2.GenerateVerifier()
	http.SetCookie(w, &http.Cookie{
		Name:     oauthCookie,
		Value:    state + "." + verifier,
		Path:     oauthPath,
		MaxAge:   int(oauthCookieTTL.Seconds()),
		HttpOnly: true,
		Secure:   true,
		// Lax, not Strict: the cookie must be sent on the cross-site redirect back from Google.
		SameSite: http.SameSiteLaxMode,
	})
	http.Redirect(w, r, h.google.AuthURL(state, verifier), http.StatusFound)
	return nil
}

// googleCallback always redirects to the frontend; errors go in ?error=<code>.
func (h *Handler) googleCallback(w http.ResponseWriter, r *http.Request) {
	stored := readCookie(r, oauthCookie)
	clearCookie(w, oauthCookie, oauthPath, http.SameSiteLaxMode, true)

	fail := func(code string) {
		http.Redirect(w, r, h.frontendURL+"/auth/callback?error="+url.QueryEscape(code), http.StatusFound)
	}
	if h.google == nil {
		fail("google_disabled")
		return
	}
	q := r.URL.Query()
	if q.Get("error") != "" {
		fail("google_cancelled")
		return
	}
	state, verifier, ok := strings.Cut(stored, ".")
	if !ok || state == "" || q.Get("state") != state || q.Get("code") == "" {
		fail("invalid_state")
		return
	}
	id, err := h.google.Exchange(r.Context(), q.Get("code"), verifier)
	if err != nil {
		h.log.WarnContext(r.Context(), "google exchange failed", slog.Any("err", err))
		fail("google_failed")
		return
	}
	sess, err := h.svc.GoogleLogin(r.Context(), id)
	switch {
	case errors.Is(err, ErrGoogleUnverified):
		fail("google_email_unverified")
		return
	case errors.Is(err, ErrAccountConflict):
		fail("account_conflict")
		return
	case err != nil:
		h.log.ErrorContext(r.Context(), "google login failed", slog.Any("err", err))
		fail("internal_error")
		return
	}
	// Top-level redirect without Origin: stays Secure (Google login needs HTTPS or localhost).
	setRefreshCookie(w, sess.RefreshToken, true)
	http.Redirect(w, r, h.frontendURL+"/auth/callback", http.StatusFound)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) error {
	id, _ := UserID(r.Context())
	p, err := h.svc.Me(r.Context(), id)
	if err != nil {
		return err
	}
	return h.writeMe(w, r, p)
}

func (h *Handler) writeMe(w http.ResponseWriter, r *http.Request, p Profile) error {
	resp := toMeResponse(p)
	if h.orgs != nil {
		orgs, err := h.orgs.ListOrgSummaries(r.Context(), p.ID)
		if err != nil {
			return fmt.Errorf("list orgs: %w", err)
		}
		resp.Orgs = orgs
	}
	httpx.JSON(w, http.StatusOK, resp)
	return nil
}

// SetProfilePublisher wires realtime notifications before requests are served.
func (h *Handler) SetProfilePublisher(p ProfilePublisher) { h.svc.SetProfilePublisher(p) }

type updateMeRequest struct {
	DisplayName *string `json:"display_name"`
	BirthDate   *string `json:"birth_date"`
	AvatarKey   *string `json:"avatar_key"`
}

func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) error {
	id, _ := UserID(r.Context())
	var fields map[string]json.RawMessage
	if err := decodeJSON(r, &fields); err != nil {
		return err
	}
	if fields == nil {
		return httpx.ValidationFailed(map[string]string{"profile": "Profil harus berupa objek"})
	}
	for key, value := range fields {
		if key != "display_name" && key != "birth_date" && key != "avatar_key" {
			return httpx.ValidationFailed(map[string]string{key: "Field tidak dikenal"})
		}
		if string(value) == "null" {
			return httpx.ValidationFailed(map[string]string{key: "Nilai tidak boleh null"})
		}
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	var req updateMeRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return httpx.ValidationFailed(map[string]string{"profile": "Format profil tidak valid"})
	}
	p, err := h.svc.UpdateProfile(r.Context(), id, ProfileInput(req))
	if err != nil {
		return err
	}
	return h.writeMe(w, r, p)
}

func toMeResponse(p Profile) api.Me {
	resp := api.Me{
		Id:           p.ID,
		Email:        p.Email,
		DisplayName:  p.DisplayName,
		AvatarKey:    p.AvatarKey,
		NeedsProfile: p.NeedsProfile,
		Orgs:         []api.OrgSummary{},
	}
	if p.BirthDate != nil {
		resp.BirthDate = &openapi_types.Date{Time: *p.BirthDate}
	}
	return resp
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

func (h *Handler) writeSession(w http.ResponseWriter, r *http.Request, sess Session) {
	setRefreshCookie(w, sess.RefreshToken, h.secure(r))
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, tokenResponse{AccessToken: sess.AccessToken})
}

func setRefreshCookie(w http.ResponseWriter, token string, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookie,
		Value:    token,
		Path:     refreshPath,
		MaxAge:   int(refreshTokenTTL.Seconds()),
		HttpOnly: true,
		// Secure except for development LAN origins (see Handler.secure).
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
	})
}

func clearCookie(w http.ResponseWriter, name, path string, sameSite http.SameSite, secure bool) {
	http.SetCookie(w, &http.Cookie{
		Name: name, Value: "", Path: path, MaxAge: -1,
		HttpOnly: true, Secure: secure, SameSite: sameSite,
	})
}

// secure is false only for a configured plain-HTTP development origin, where
// browsers would drop a Secure cookie (localhost is exempt, LAN IPs are not).
// DevOrigins is empty in production (config refuses it).
func (h *Handler) secure(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	return !strings.HasPrefix(origin, "http://") || !slices.Contains(h.DevOrigins, origin)
}

func readCookie(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}

// checkOrigin guards cookie-authenticated endpoints against cross-site requests.
func (h *Handler) checkOrigin(r *http.Request) error {
	if o := r.Header.Get("Origin"); o != h.frontendURL && !slices.Contains(h.DevOrigins, o) {
		return &httpx.Error{Status: http.StatusForbidden, Code: "forbidden", Message: "Asal permintaan tidak diizinkan"}
	}
	return nil
}

func (h *Handler) allow(w http.ResponseWriter, l *ratelimit.Limiter, key string) error {
	return httpx.RateLimit(w, l, key)
}

func clientIP(r *http.Request) string {
	if ip := middleware.GetClientIP(r.Context()); ip != "" {
		return ip
	}
	return r.RemoteAddr
}

func decodeJSON(r *http.Request, v any) error {
	return httpx.DecodeJSON(r, v)
}

// toAPIError maps domain errors to the API error shape.
func toAPIError(err error) error {
	if err == nil {
		return nil
	}
	var verr *ValidationError
	if errors.As(err, &verr) {
		return httpx.ValidationFailed(verr.Fields)
	}
	type mapping struct {
		target  error
		status  int
		code    string
		message string
	}
	for _, m := range []mapping{
		{ErrInvalidCredentials, http.StatusUnauthorized, "invalid_credentials", "Email atau password salah"},
		{ErrEmailNotVerified, http.StatusForbidden, "email_not_verified", "Email belum diverifikasi"},
		{ErrInvalidOTP, http.StatusBadRequest, "invalid_otp", "Kode verifikasi salah"},
		{ErrOTPExpired, http.StatusBadRequest, "otp_expired", "Kode verifikasi sudah kedaluwarsa"},
		{ErrTooManyAttempts, http.StatusBadRequest, "otp_attempts_exceeded", "Terlalu banyak percobaan. Minta kode baru."},
		{ErrUnauthenticated, http.StatusUnauthorized, "unauthenticated", "Sesi berakhir. Silakan masuk lagi."},
		{ErrSessionRevoked, http.StatusUnauthorized, "session_revoked", "Sesi dicabut. Silakan masuk lagi."},
		{ErrUserNotFound, http.StatusUnauthorized, "unauthenticated", "Sesi berakhir. Silakan masuk lagi."},
	} {
		if errors.Is(err, m.target) {
			return &httpx.Error{Status: m.status, Code: m.code, Message: m.message}
		}
	}
	return fmt.Errorf("auth: %w", err)
}
