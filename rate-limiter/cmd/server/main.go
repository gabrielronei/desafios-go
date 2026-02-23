package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gaboliveirap/rate-limiter/internal/config"
	"github.com/gaboliveirap/rate-limiter/internal/limiter"
	"github.com/gaboliveirap/rate-limiter/internal/middleware"
	"github.com/gaboliveirap/rate-limiter/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	store, err := storage.NewRedisStorage(cfg.RedisAddr, cfg.RedisPassword, cfg.RedisDB)
	if err != nil {
		log.Fatalf("connecting to Redis: %v", err)
	}
	log.Printf("connected to Redis at %s", cfg.RedisAddr)

	// Convert per-token config map.
	tokenCfgs := make(map[string]limiter.LimitConfig, len(cfg.TokenConfigs))
	for token, tc := range cfg.TokenConfigs {
		tokenCfgs[token] = limiter.LimitConfig{
			MaxRequests:   tc.MaxRequests,
			BlockDuration: tc.BlockDuration,
		}
	}

	rl := limiter.New(
		store,
		limiter.LimitConfig{
			MaxRequests:   cfg.IPMaxRequests,
			BlockDuration: cfg.IPBlockDuration,
		},
		limiter.LimitConfig{
			MaxRequests:   cfg.TokenMaxRequests,
			BlockDuration: cfg.TokenBlockDuration,
		},
		tokenCfgs,
	)

	log.Printf("rate limiter configured — IP: %d req/s (block %s), Token: %d req/s (block %s)",
		cfg.IPMaxRequests, cfg.IPBlockDuration.Round(time.Second),
		cfg.TokenMaxRequests, cfg.TokenBlockDuration.Round(time.Second),
	)

	mux := http.NewServeMux()

	// Health-check endpoint (not rate-limited).
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	// Root endpoint — rate-limited.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, `{"message":"Hello, World!"}`)
	})

	// Apply rate-limit middleware to the whole mux.
	handler := middleware.RateLimit(rl)(mux)

	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	log.Println("server listening on :8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
