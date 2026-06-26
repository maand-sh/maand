// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package certs

import (
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"os"
	"path"
	"strings"
	"time"

	"maand/bucket"
	"maand/data"
)

const (
	StatusOK       = "ok"
	StatusExpiring = "expiring"
	StatusExpired  = "expired"
	StatusInvalid  = "invalid"
)

// Metric describes one TLS certificate for inspection or Prometheus push.
type Metric struct {
	Scope      string
	Job        string
	WorkerIP   string
	CertName   string
	CommonName string
	NotAfter   time.Time
	DaysLeft   int
	Status     string
}

// ListCertMetrics returns CA and job leaf certificates from KV (same source as maand cat certs).
func ListCertMetrics(tx *sql.Tx, jobsFilter, workersFilter []string) ([]Metric, error) {
	renewalBufferDays := 0
	if maandConf, err := bucket.GetMaandConf(); err == nil {
		renewalBufferDays = maandConf.CertsRenewalBuffer
	}
	return listCertMetrics(tx, jobsFilter, workersFilter, renewalBufferDays)
}

func listCertMetrics(
	tx *sql.Tx,
	jobsFilter, workersFilter []string,
	renewalBufferDays int,
) ([]Metric, error) {
	now := time.Now().UTC()
	metrics := make([]Metric, 0)

	if caMetric, ok := readCAMetric(renewalBufferDays, now); ok {
		metrics = append(metrics, caMetric)
	}

	rows, err := tx.Query(`
		SELECT namespace, key, value
		FROM key_value kv
		WHERE deleted = 0
		  AND key LIKE 'certs/%.crt'
		  AND namespace LIKE 'maand/job/%/worker/%'
		  AND version = (
		    SELECT MAX(v2.version)
		    FROM key_value v2
		    WHERE v2.namespace = kv.namespace AND v2.key = kv.key
		  )
		ORDER BY namespace, key`)
	if err != nil {
		return nil, bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var namespace, key, value string
		if err := rows.Scan(&namespace, &key, &value); err != nil {
			return nil, bucket.DatabaseError(err)
		}

		job, workerIP, ok := parseJobWorkerCertNamespace(namespace)
		if !ok {
			continue
		}
		if len(jobsFilter) > 0 && !contains(jobsFilter, job) {
			continue
		}
		if len(workersFilter) > 0 && !contains(workersFilter, workerIP) {
			continue
		}

		certName := strings.TrimSuffix(strings.TrimPrefix(key, "certs/"), ".crt")
		metric, err := metricFromPEM("job", job, workerIP, certName, []byte(value), renewalBufferDays, now)
		if err != nil {
			metrics = append(metrics, Metric{
				Scope:    "job",
				Job:      job,
				WorkerIP: workerIP,
				CertName: certName,
				Status:   StatusInvalid,
			})
			continue
		}
		metrics = append(metrics, metric)
	}
	if err := data.RowsErr(rows); err != nil {
		return nil, err
	}

	return metrics, nil
}

func readCAMetric(renewalBufferDays int, now time.Time) (Metric, bool) {
	caPath := path.Join(bucket.SecretLocation, "ca.crt")
	pemBytes, err := os.ReadFile(caPath)
	if err != nil {
		return Metric{}, false
	}
	metric, err := metricFromPEM("ca", "", "", "ca", pemBytes, renewalBufferDays, now)
	if err != nil {
		return Metric{
			Scope:    "ca",
			CertName: "ca",
			Status:   StatusInvalid,
		}, true
	}
	return metric, true
}

func metricFromPEM(
	scope, job, workerIP, certName string,
	pemBytes []byte,
	renewalBufferDays int,
	now time.Time,
) (Metric, error) {
	cert, err := parseX509CertPEM(pemBytes)
	if err != nil {
		return Metric{}, err
	}
	notAfter := cert.NotAfter.UTC()
	return Metric{
		Scope:      scope,
		Job:        job,
		WorkerIP:   workerIP,
		CertName:   certName,
		CommonName: cert.Subject.CommonName,
		NotAfter:   notAfter,
		DaysLeft:   certDaysLeft(notAfter, now),
		Status:     certExpiryStatus(notAfter, renewalBufferDays, now),
	}, nil
}

func parseX509CertPEM(pemBytes []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("invalid PEM")
	}
	return x509.ParseCertificate(block.Bytes)
}

func certDaysLeft(notAfter, now time.Time) int {
	return int(notAfter.Sub(now).Hours() / 24)
}

func certExpiryStatus(notAfter time.Time, renewalBufferDays int, now time.Time) string {
	if now.After(notAfter.UTC()) {
		return StatusExpired
	}
	if CertNeedsRenewal(notAfter, renewalBufferDays, now) {
		return StatusExpiring
	}
	return StatusOK
}

func parseJobWorkerCertNamespace(namespace string) (job, workerIP string, ok bool) {
	const prefix = "maand/job/"
	const workerPart = "/worker/"
	if !strings.HasPrefix(namespace, prefix) {
		return "", "", false
	}
	rest := strings.TrimPrefix(namespace, prefix)
	idx := strings.Index(rest, workerPart)
	if idx < 0 {
		return "", "", false
	}
	job = rest[:idx]
	workerIP = rest[idx+len(workerPart):]
	if job == "" || workerIP == "" {
		return "", "", false
	}
	return job, workerIP, true
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

// ParseJobWorkerCertNamespace parses maand/job/<job>/worker/<ip> KV namespaces.
func ParseJobWorkerCertNamespace(namespace string) (job, workerIP string, ok bool) {
	return parseJobWorkerCertNamespace(namespace)
}

// CertExpiryStatus returns ok, expiring, or expired using maand renewal buffer semantics.
func CertExpiryStatus(notAfter time.Time, renewalBufferDays int, now time.Time) string {
	return certExpiryStatus(notAfter, renewalBufferDays, now)
}

// MetricFromPEM builds a Metric from one PEM-encoded certificate.
func MetricFromPEM(
	scope, job, workerIP, certName string,
	pemBytes []byte,
	renewalBufferDays int,
	now time.Time,
) (Metric, error) {
	return metricFromPEM(scope, job, workerIP, certName, pemBytes, renewalBufferDays, now)
}
