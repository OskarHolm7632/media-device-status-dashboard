package dashboard

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestQueryMetricsIncludesRequiredParameters(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/v1/metrics/query" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.URL.Query().Get("name") != "media_device_status" || r.URL.Query().Get("agg") != "avg" {
			t.Fatalf("missing required metrics parameters: %s", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":[]}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	var metrics json.RawMessage
	metrics, err = client.QueryMetrics(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if string(metrics) != "[]" {
		t.Fatalf("got %s, want []", metrics)
	}
}

func TestPublishRetriesRateLimitWithSameIdempotencyKey(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost || r.URL.Path != "/v1/realtime/publish" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Idempotency-Key") != "operation-8" {
			t.Fatal("missing auth or stable idempotency header")
		}
		w.Header().Set("Content-Type", "application/json")
		if requests == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"error":{"message":"retry later"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{}}`))
	}))
	defer server.Close()

	client, err := NewClient(server.URL, "test-key", server.Client())
	if err != nil {
		t.Fatal(err)
	}
	client.sleep = func(time.Duration) {}
	if err := client.Publish(context.Background(), "media-device-status", "device.status", map[string]string{"health": "healthy"}, "operation-8"); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("got %d requests, want 2", requests)
	}
}

func TestEnvelopeErrorPrecedesHTTPStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"message":"channel is required"}}`))
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, "test-key", server.Client())

	err := client.CreateChannel(context.Background(), "media-device-status")
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusBadRequest {
		t.Fatalf("got %#v, want decoded API error", err)
	}
}
