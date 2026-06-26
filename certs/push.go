// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package certs

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"maand/bucket"
	"maand/data"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/prometheus/prompb"
)

const (
	prometheusJobName      = "prometheus"
	prometheusHTTPPortName = "prometheus_port_http"
	metricCertNotAfter     = "maand_cert_not_after_seconds"
	metricCertExpiring     = "maand_cert_expiring"
	metricCertExpired      = "maand_cert_expired"
	certMetricsPushTimeout = 15 * time.Second
)

// PushMetrics sends current certificate expiry gauges to Prometheus remote write.
// Called after deploy; failures are logged and do not fail deploy.
func PushMetrics(db *sql.DB) {
	if err := pushMetrics(db); err != nil {
		log.Printf("cert metrics: %v", err)
	}
}

func pushMetrics(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	writeURL, err := discoverPrometheusRemoteWriteURL(tx)
	if err != nil {
		return err
	}
	if writeURL == "" {
		return nil
	}

	metrics, err := ListCertMetrics(tx, nil, nil)
	if err != nil {
		return err
	}
	if len(metrics) == 0 {
		return nil
	}

	return remoteWriteMetrics(writeURL, metrics)
}

func discoverPrometheusRemoteWriteURL(tx *sql.Tx) (string, error) {
	hasConfig, err := data.JobHasPrometheusServerConfig(tx, prometheusJobName)
	if err != nil {
		return "", err
	}
	if !hasConfig {
		return "", nil
	}

	workers, err := data.GetNonRemovedAllocations(tx, prometheusJobName)
	if err != nil {
		return "", err
	}
	if len(workers) == 0 {
		return "", nil
	}

	port, err := data.GetJobPortNumber(tx, prometheusJobName, prometheusHTTPPortName)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("http://%s:%d/api/v1/write", workers[0], port), nil
}

func remoteWriteMetrics(writeURL string, metrics []Metric) error {
	now := time.Now()
	series := make([]prompb.TimeSeries, 0, len(metrics)*3)
	for _, metric := range metrics {
		labels := metricLabels(metric)
		if metric.Status != StatusInvalid && !metric.NotAfter.IsZero() {
			series = append(series, newGaugeSeries(labels, metricCertNotAfter, float64(metric.NotAfter.Unix()), now))
		}
		expiring := float64(0)
		if metric.Status == StatusExpiring {
			expiring = 1
		}
		series = append(series, newGaugeSeries(labels, metricCertExpiring, expiring, now))

		expired := float64(0)
		if metric.Status == StatusExpired {
			expired = 1
		}
		series = append(series, newGaugeSeries(labels, metricCertExpired, expired, now))
	}

	req := &prompb.WriteRequest{Timeseries: series}
	payload, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal remote write: %w", err)
	}
	encoded := snappy.Encode(nil, payload)

	ctx, cancel := context.WithTimeout(context.Background(), certMetricsPushTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, writeURL, bytes.NewReader(encoded))
	if err != nil {
		return err
	}
	httpReq.Header.Set("Content-Type", "application/x-protobuf")
	httpReq.Header.Set("Content-Encoding", "snappy")
	httpReq.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("remote write request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("remote write status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func metricLabels(metric Metric) []prompb.Label {
	labels := []prompb.Label{
		{Name: "__name__", Value: metricCertNotAfter},
		{Name: "scope", Value: metric.Scope},
		{Name: "job", Value: metric.Job},
		{Name: "worker", Value: metric.WorkerIP},
		{Name: "cert", Value: metric.CertName},
		{Name: "common_name", Value: metric.CommonName},
		{Name: "status", Value: metric.Status},
	}
	sort.Slice(labels, func(i, j int) bool {
		return labels[i].Name < labels[j].Name
	})
	return labels
}

func newGaugeSeries(baseLabels []prompb.Label, metricName string, value float64, ts time.Time) prompb.TimeSeries {
	labels := make([]prompb.Label, len(baseLabels))
	copy(labels, baseLabels)
	for i := range labels {
		if labels[i].Name == "__name__" {
			labels[i].Value = metricName
			break
		}
	}
	sort.Slice(labels, func(i, j int) bool {
		return labels[i].Name < labels[j].Name
	})
	return prompb.TimeSeries{
		Labels: labels,
		Samples: []prompb.Sample{{
			Value:     value,
			Timestamp: ts.UnixMilli(),
		}},
	}
}
