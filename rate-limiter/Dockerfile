# ── Build stage ──────────────────────────────────────────────────────────────
FROM golang:1.21-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /rate-limiter ./cmd/server

# ── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /rate-limiter .
COPY app.env .

EXPOSE 8080

ENTRYPOINT ["/app/rate-limiter"]
