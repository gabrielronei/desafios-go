# Rate Limiter — Go Challenge

HTTP rate limiter implemented as a middleware for a Go web server.
Supports per-IP and per-token limiting with a Redis backend and a swappable storage strategy.

---

## Architecture

```
cmd/server/main.go          → entry point, wires everything together
internal/
  config/config.go          → loads configuration from app.env / env vars
  storage/
    storage.go              → Storage interface  (Strategy pattern)
    redis.go                → Redis implementation
    memory.go               → in-memory implementation (used in tests)
  limiter/
    limiter.go              → core rate-limiting logic (no HTTP dependency)
    limiter_test.go         → unit tests with injectable clock
  middleware/
    middleware.go           → net/http middleware that wraps the limiter
    middleware_test.go      → middleware unit tests with mock limiter
```

### How It Works

1. Every request passes through the `RateLimit` middleware.
2. The middleware extracts the client IP (honouring `X-Forwarded-For`) and the `API_KEY` header.
3. When `API_KEY` is present its limit is used; otherwise the IP limit applies.
   → **token config always overrides IP config.**
4. The limiter checks a **fixed-window counter** stored in Redis (key bucketed to the current second).
5. If the counter exceeds `MaxRequests` the key is blocked for `BlockDuration` seconds.
6. While blocked every request immediately returns **HTTP 429** without touching the counter.

### Strategy Pattern

`storage.Storage` is an interface. Swap the Redis backend for any other store (Postgres, Memcached, …) by implementing the four methods:

```go
type Storage interface {
    Increment(ctx context.Context, key string, window time.Duration) (int64, error)
    IsBlocked(ctx context.Context, key string) (bool, error)
    Block(ctx context.Context, key string, duration time.Duration) error
    Reset(ctx context.Context, key string) error
}
```

---

## Configuration

All settings live in `app.env` (or can be set as environment variables directly).

| Variable | Default | Description |
|---|---|---|
| `REDIS_ADDR` | `localhost:6379` | Redis address |
| `REDIS_PASSWORD` | *(empty)* | Redis password |
| `REDIS_DB` | `0` | Redis database index |
| `RATE_LIMIT_IP_MAX_REQUESTS` | `10` | Max requests/second per IP |
| `RATE_LIMIT_IP_BLOCK_DURATION_SECONDS` | `300` | Block duration (seconds) after IP exceeds limit |
| `RATE_LIMIT_TOKEN_MAX_REQUESTS` | `100` | Default max requests/second for any token |
| `RATE_LIMIT_TOKEN_BLOCK_DURATION_SECONDS` | `300` | Default block duration (seconds) for any token |
| `TOKEN_RATE_LIMITS` | *(empty)* | Per-token overrides (see below) |

### Per-Token Configuration

```
TOKEN_RATE_LIMITS=abc123:200:60,xyz789:50:600
```

Format: `<token>:<maxRequests>:<blockSeconds>` — comma-separated list.
Tokens not listed here fall back to the default token limit.

---

## Running

### With Docker Compose (recommended)

```bash
docker compose up --build
```

The server starts on **port 8080**, Redis on **6379**.

### Locally (requires a running Redis)

```bash
# adjust REDIS_ADDR in app.env if needed
go run ./cmd/server
```

---

## API

### Health check (not rate-limited)

```
GET /health
→ 200 {"status":"ok"}
```

### Any other endpoint

```
GET /
# With a token
GET / -H "API_KEY: abc123"
```

**Success (200)**
```json
{"message":"Hello, World!"}
```

**Rate limit exceeded (429)**
```
you have reached the maximum number of requests or actions allowed within a certain time frame
```

---

## Running Tests

```bash
go test ./internal/... -v
```

All tests use the `MemoryStorage` backend with an injectable clock, so they run instantly without Redis.

---

## Quick Demo

```bash
# Start the stack
docker compose up -d --build

# Send 12 requests (IP limit = 10 → 11th should get 429)
for i in $(seq 1 12); do
  code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/)
  echo "Request $i: $code"
done

# Test token (limit = 100)
curl -s http://localhost:8080/ -H "API_KEY: mytoken"
```
