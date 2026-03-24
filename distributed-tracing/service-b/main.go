package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

type ViaCEPResponse struct {
	Localidade string `json:"localidade"`
	Erro       string `json:"erro,omitempty"`
}

type WeatherAPIResponse struct {
	Current struct {
		TempC float64 `json:"temp_c"`
	} `json:"current"`
}

type WeatherOutput struct {
	City  string  `json:"city"`
	TempC float64 `json:"temp_C"`
	TempF float64 `json:"temp_F"`
	TempK float64 `json:"temp_K"`
}

var httpClient = &http.Client{Timeout: 10 * time.Second}

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
			semconv.ServiceName("service-b"),
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

func getCityByCEP(ctx context.Context, cep string) (string, error) {
	tracer := otel.Tracer("service-b")
	ctx, span := tracer.Start(ctx, "fetch-cep-viacep")
	defer span.End()

	span.SetAttributes(attribute.String("cep", cep))

	resp, err := httpClient.Get(fmt.Sprintf("https://viacep.com.br/ws/%s/json/", cep))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "viacep request failed")
		return "", fmt.Errorf("viacep request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		span.SetStatus(codes.Error, "cep not found")
		return "", fmt.Errorf("cep not found")
	}

	var viacep ViaCEPResponse
	if err := json.NewDecoder(resp.Body).Decode(&viacep); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode response")
		return "", fmt.Errorf("failed to decode viacep response: %w", err)
	}

	if viacep.Erro == "true" || viacep.Localidade == "" {
		span.SetStatus(codes.Error, "cep not found")
		return "", fmt.Errorf("cep not found")
	}

	span.SetAttributes(attribute.String("city", viacep.Localidade))
	return viacep.Localidade, nil
}

func getTemperature(ctx context.Context, city string) (float64, error) {
	tracer := otel.Tracer("service-b")
	ctx, span := tracer.Start(ctx, "fetch-temperature-weatherapi")
	defer span.End()

	span.SetAttributes(attribute.String("city", city))

	apiKey := os.Getenv("WEATHER_API_KEY")
	if apiKey == "" {
		span.SetStatus(codes.Error, "WEATHER_API_KEY not configured")
		return 0, fmt.Errorf("WEATHER_API_KEY not set")
	}

	weatherURL := fmt.Sprintf("https://api.weatherapi.com/v1/current.json?key=%s&q=%s",
		apiKey, url.QueryEscape(city))

	resp, err := httpClient.Get(weatherURL)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "weatherapi request failed")
		return 0, fmt.Errorf("weatherapi request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		span.SetStatus(codes.Error, fmt.Sprintf("weatherapi returned status %d", resp.StatusCode))
		return 0, fmt.Errorf("weatherapi returned status %d", resp.StatusCode)
	}

	var weather WeatherAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&weather); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "failed to decode response")
		return 0, fmt.Errorf("failed to decode weather response: %w", err)
	}

	span.SetAttributes(attribute.Float64("temp_c", weather.Current.TempC))
	return weather.Current.TempC, nil
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

	cepRegex := regexp.MustCompile(`^\d{8}$`)

	mux := http.NewServeMux()
	mux.HandleFunc("/weather/", func(w http.ResponseWriter, r *http.Request) {
		cep := r.URL.Path[len("/weather/"):]

		if !cepRegex.MatchString(cep) {
			w.WriteHeader(http.StatusUnprocessableEntity)
			w.Write([]byte("invalid zipcode"))
			return
		}

		city, err := getCityByCEP(r.Context(), cep)
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("can not find zipcode"))
			return
		}

		tempC, err := getTemperature(r.Context(), city)
		if err != nil {
			http.Error(w, "failed to get temperature", http.StatusInternalServerError)
			return
		}

		output := WeatherOutput{
			City:  city,
			TempC: tempC,
			TempF: tempC*1.8 + 32,
			TempK: tempC + 273,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(output)
	})

	handler := otelhttp.NewHandler(mux, "service-b-request")

	log.Println("Service B running on :8081")
	if err := http.ListenAndServe(":8081", handler); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}
