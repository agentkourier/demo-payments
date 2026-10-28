package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricsCountRequestsByHandlerAndCode(t *testing.T) {
	var ready atomic.Bool
	ready.Store(true)
	router := newRouter(discardLogger(), prometheus.NewRegistry(), &ready)

	for _, body := range []string{
		`{"order_id":"ord_1","amount_cents":100,"currency":"USD"}`,
		`{"order_id":"ord_2","amount_cents":200,"currency":"USD"}`,
		`{"order_id":"","amount_cents":200,"currency":"USD"}`,
	} {
		router.ServeHTTP(httptest.NewRecorder(),
			httptest.NewRequest(http.MethodPost, "/v1/authorize", strings.NewReader(body)))
	}

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	out, _ := io.ReadAll(rec.Body)

	for _, want := range []string{
		`http_requests_total{code="200",handler="/v1/authorize"} 2`,
		`http_requests_total{code="400",handler="/v1/authorize"} 1`,
		`http_request_duration_seconds_count{handler="/v1/authorize"} 3`,
	} {
		if !strings.Contains(string(out), want) {
			t.Errorf("metrics output missing %q", want)
		}
	}
}
