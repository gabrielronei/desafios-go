package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gaboliveirap/rate-limiter/internal/middleware"
)

// mockLimiter is a simple Limiter stub for middleware tests.
type mockLimiter struct {
	allowed bool
	err     error
}

func (m *mockLimiter) Allow(_ context.Context, _, _ string) (bool, error) {
	return m.allowed, m.err
}

var okHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ok")) //nolint:errcheck
})

func TestMiddleware_AllowsRequest(t *testing.T) {
	handler := middleware.RateLimit(&mockLimiter{allowed: true})(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestMiddleware_BlocksRequest(t *testing.T) {
	handler := middleware.RateLimit(&mockLimiter{allowed: false})(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec.Code)
	}

	body := rec.Body.String()
	if body == "" {
		t.Fatal("expected non-empty body for 429 response")
	}
}

func TestMiddleware_BlocksRequest_CorrectMessage(t *testing.T) {
	handler := middleware.RateLimit(&mockLimiter{allowed: false})(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	want := middleware.RateLimitExceededMsg
	got := rec.Body.String()
	// http.Error appends a newline.
	if got != want+"\n" {
		t.Fatalf("body mismatch\nwant: %q\n got: %q", want, got)
	}
}

func TestMiddleware_TokenHeader(t *testing.T) {
	var capturedToken string
	captureLimiter := &capturingLimiter{captureToken: &capturedToken, allowed: true}

	handler := middleware.RateLimit(captureLimiter)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("API_KEY", "my-token-123")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedToken != "my-token-123" {
		t.Fatalf("expected token %q, got %q", "my-token-123", capturedToken)
	}
}

func TestMiddleware_IPExtraction(t *testing.T) {
	var capturedIP string
	captureLimiter := &capturingLimiter{captureIP: &capturedIP, allowed: true}

	handler := middleware.RateLimit(captureLimiter)(okHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "10.0.0.1, 10.0.0.2")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if capturedIP != "10.0.0.1" {
		t.Fatalf("expected IP %q, got %q", "10.0.0.1", capturedIP)
	}
}

// capturingLimiter records the IP and token passed to Allow.
type capturingLimiter struct {
	captureIP    *string
	captureToken *string
	allowed      bool
}

func (c *capturingLimiter) Allow(_ context.Context, ip, token string) (bool, error) {
	if c.captureIP != nil {
		*c.captureIP = ip
	}
	if c.captureToken != nil {
		*c.captureToken = token
	}
	return c.allowed, nil
}
