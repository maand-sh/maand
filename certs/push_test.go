package certs

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRemoteWriteMetrics(t *testing.T) {
	var received prompb.WriteRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "application/x-protobuf", r.Header.Get("Content-Type"))
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		decoded, err := snappy.Decode(nil, body)
		require.NoError(t, err)
		require.NoError(t, proto.Unmarshal(decoded, &received))
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	notAfter := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	err := remoteWriteMetrics(srv.URL, []Metric{{
		Scope:      "job",
		Job:        "api",
		WorkerIP:   "10.0.0.1",
		CertName:   "tls",
		CommonName: "api.internal",
		NotAfter:   notAfter,
		Status:     StatusOK,
	}}, "", "")
	require.NoError(t, err)
	require.Len(t, received.Timeseries, 3)

	names := make(map[string]float64)
	for _, ts := range received.Timeseries {
		var metricName string
		for _, label := range ts.Labels {
			if label.Name == "__name__" {
				metricName = label.Value
			}
		}
		require.Len(t, ts.Samples, 1)
		names[metricName] = ts.Samples[0].Value
	}
	assert.Equal(t, float64(notAfter.Unix()), names[metricCertNotAfter])
	assert.Equal(t, float64(0), names[metricCertExpiring])
	assert.Equal(t, float64(0), names[metricCertExpired])
}

func TestRemoteWriteMetricsBasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		require.True(t, ok)
		assert.Equal(t, "promadmin", user)
		assert.Equal(t, "s3cret", pass)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	err := remoteWriteMetrics(srv.URL, []Metric{{
		Scope:    "job",
		Job:      "api",
		Status:   StatusOK,
		NotAfter: time.Now().Add(24 * time.Hour),
	}}, "promadmin", "s3cret")
	require.NoError(t, err)
}

func TestRemoteWriteMetricsRetries503(t *testing.T) {
	oldAttempts := certMetricsPushAttempts
	oldBackoff := certMetricsRetryBackoff
	certMetricsPushAttempts = 4
	certMetricsRetryBackoff = 5 * time.Millisecond
	t.Cleanup(func() {
		certMetricsPushAttempts = oldAttempts
		certMetricsRetryBackoff = oldBackoff
	})

	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		if attempts < 3 {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	notAfter := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
	err := remoteWriteMetricsWithRetry(srv.URL, []Metric{{
		Scope:      "job",
		Job:        "api",
		WorkerIP:   "10.0.0.1",
		CertName:   "tls",
		CommonName: "api.internal",
		NotAfter:   notAfter,
		Status:     StatusOK,
	}}, "", "")
	require.NoError(t, err)
	assert.Equal(t, 3, attempts)
}

func TestRemoteWriteMetricsDoesNotRetry400(t *testing.T) {
	oldAttempts := certMetricsPushAttempts
	oldBackoff := certMetricsRetryBackoff
	certMetricsPushAttempts = 5
	certMetricsRetryBackoff = 5 * time.Millisecond
	t.Cleanup(func() {
		certMetricsPushAttempts = oldAttempts
		certMetricsRetryBackoff = oldBackoff
	})

	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		attempts++
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	t.Cleanup(srv.Close)

	err := remoteWriteMetricsWithRetry(srv.URL, []Metric{{
		Scope:    "job",
		Job:      "api",
		Status:   StatusOK,
		NotAfter: time.Now().Add(24 * time.Hour),
	}}, "", "")
	require.Error(t, err)
	assert.Equal(t, 1, attempts)
	var rw *remoteWriteError
	require.ErrorAs(t, err, &rw)
	assert.Equal(t, http.StatusBadRequest, rw.statusCode)
}
