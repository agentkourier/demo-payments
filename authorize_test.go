package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
)

func TestAuthorize(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantCode int
		// wantBody is the decoded response body. authorization_id is
		// checked separately because it is random.
		wantBody map[string]any
	}{
		{
			name:     "approved",
			body:     `{"order_id":"ord_1001","amount_cents":4250,"currency":"USD"}`,
			wantCode: http.StatusOK,
			wantBody: map[string]any{
				"order_id": "ord_1001",
				"status":   "approved",
				"amount":   float64(4250),
				"currency": "USD",
			},
		},
		{
			name:     "missing order id",
			body:     `{"amount_cents":4250,"currency":"USD"}`,
			wantCode: http.StatusBadRequest,
			wantBody: map[string]any{"error": "order_id is required"},
		},
		{
			name:     "zero amount",
			body:     `{"order_id":"ord_1001","amount_cents":0,"currency":"USD"}`,
			wantCode: http.StatusBadRequest,
			wantBody: map[string]any{"error": "amount_cents must be positive"},
		},
		{
			name:     "bad currency",
			body:     `{"order_id":"ord_1001","amount_cents":4250,"currency":"usd"}`,
			wantCode: http.StatusBadRequest,
			wantBody: map[string]any{"error": "currency must be an ISO 4217 code"},
		},
		{
			name:     "unknown field",
			body:     `{"order_id":"ord_1001","amount_cents":4250,"currency":"USD","tip":1}`,
			wantCode: http.StatusBadRequest,
			wantBody: map[string]any{"error": "invalid request body"},
		},
		{
			name:     "malformed json",
			body:     `{"order_id":`,
			wantCode: http.StatusBadRequest,
			wantBody: map[string]any{"error": "invalid request body"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/authorize", strings.NewReader(tt.body))
			newTestRouter().ServeHTTP(rec, req)

			if rec.Code != tt.wantCode {
				t.Fatalf("status = %d, want %d", rec.Code, tt.wantCode)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got)
			}
			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if tt.wantCode == http.StatusOK {
				id, _ := got["authorization_id"].(string)
				if !strings.HasPrefix(id, "auth_") {
					t.Errorf("authorization_id = %q, want auth_ prefix", id)
				}
				delete(got, "authorization_id")
			}
			if !reflect.DeepEqual(got, tt.wantBody) {
				t.Errorf("body = %v, want %v", got, tt.wantBody)
			}
		})
	}
}

func TestAuthorizeMethodNotAllowed(t *testing.T) {
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/authorize", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestProbes(t *testing.T) {
	var ready atomic.Bool
	router := newRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), &ready)

	tests := []struct {
		path  string
		ready bool
		want  int
	}{
		{"/healthz", false, http.StatusOK},
		{"/readyz", false, http.StatusServiceUnavailable},
		{"/readyz", true, http.StatusOK},
	}
	for _, tt := range tests {
		ready.Store(tt.ready)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tt.path, nil))
		if rec.Code != tt.want {
			t.Errorf("GET %s (ready=%v) = %d, want %d", tt.path, tt.ready, rec.Code, tt.want)
		}
	}
}

func newTestRouter() http.Handler {
	var ready atomic.Bool
	ready.Store(true)
	return newRouter(slog.New(slog.NewTextHandler(io.Discard, nil)), &ready)
}
