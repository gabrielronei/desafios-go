// Package middleware provides the HTTP middleware that integrates the rate limiter
// into any net/http-compatible server.
package middleware

import (
	"context"
	"log"
	"net"
	"net/http"
	"strings"
)

const (
	// RateLimitExceededMsg is the message returned with HTTP 429.
	RateLimitExceededMsg = "you have reached the maximum number of requests or actions allowed within a certain time frame"

	// APIKeyHeader is the header used to pass access tokens.
	APIKeyHeader = "API_KEY"
)

// Limiter is the interface the middleware depends on.
// Using an interface keeps the middleware decoupled from the concrete limiter.
type Limiter interface {
	Allow(ctx context.Context, ip, token string) (bool, error)
}

// RateLimit returns an HTTP middleware that enforces the provided Limiter.
func RateLimit(l Limiter) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := extractIP(r)
			token := r.Header.Get(APIKeyHeader)

			allowed, err := l.Allow(r.Context(), ip, token)
			if err != nil {
				log.Printf("rate limiter error: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}

			if !allowed {
				http.Error(w, RateLimitExceededMsg, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// extractIP resolves the real client IP, honoring common proxy headers.
func extractIP(r *http.Request) string {
	// Prefer the leftmost address in X-Forwarded-For (set by reverse proxies).
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if ip, _, err := net.SplitHostPort(strings.TrimSpace(strings.Split(xff, ",")[0])); err == nil {
			return ip
		}
		return strings.TrimSpace(strings.Split(xff, ",")[0])
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
