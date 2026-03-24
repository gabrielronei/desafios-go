package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type CEPRequest struct {
	CEP string `json:"cep"`
}

func initTracer(ctx context.Context) (*sdktrace.TracerProvider, error) {
	otelEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if otelEndpoint == "" {
		otelEndpoint = "otel-collector:4317"
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otelEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName("service-a"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp, nil
}

func main() {
	ctx := context.Background()

	tp, err := initTracer(ctx)
	if err != nil {
		log.Fatalf("failed to initialize tracer: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("error shutting down tracer provider: %v", err)
		}
	}()

	serviceBURL := os.Getenv("SERVICE_B_URL")
	if serviceBURL == "" {
		serviceBURL = "http://service-b:8081"
	}

	cepRegex := regexp.MustCompile(`^\d{8}$`)
	httpClient := &http.Client{
		Timeout:   10 * time.Second,
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req CEPRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte("invalid zipcode"))
			return
		}

		if !cepRegex.MatchString(req.CEP) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte("invalid zipcode"))
			return
		}

		proxyReq, err := http.NewRequestWithContext(r.Context(), http.MethodGet,
			fmt.Sprintf("%s/weather/%s", serviceBURL, req.CEP), nil)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		otel.GetTextMapPropagator().Inject(r.Context(), propagation.HeaderCarrier(proxyReq.Header))

		resp, err := httpClient.Do(proxyReq)
		if err != nil {
			http.Error(w, "service b unavailable", http.StatusServiceUnavailable)
			return
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
		w.WriteHeader(resp.StatusCode)
		w.Write(body)
	})

	handler := otelhttp.NewHandler(mux, "service-a-request")

	log.Println("Service A running on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
