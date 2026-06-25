// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cat

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
	"maand/build"
	"maand/data"
	"maand/utils"

	"github.com/jedib0t/go-pretty/v6/table"
)

const (
	certStatusOK       = "ok"
	certStatusExpiring = "expiring"
	certStatusExpired  = "expired"
	certStatusInvalid  = "invalid"
)

type certEntry struct {
	scope      string
	job        string
	workerIP   string
	certName   string
	commonName string
	notAfter   time.Time
	daysLeft   int
	status     string
}

func Certs(jobsCSV, workersCSV string) error {
	db, err := data.OpenDatabase(true)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	jobsFilter := parseCSVFilter(jobsCSV)
	workersFilter := parseCSVFilter(workersCSV)
	if err := validateDeploymentFilters(tx, jobsFilter, workersFilter); err != nil {
		return err
	}

	renewalBufferDays := 0
	if maandConf, confErr := bucket.GetMaandConf(); confErr == nil {
		renewalBufferDays = maandConf.CertsRenewalBuffer
	}

	entries, err := listCertificateEntries(tx, jobsFilter, workersFilter, renewalBufferDays)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return bucket.NotFoundError("certificates")
	}

	t := utils.GetTable(table.Row{
		"scope", "job", "worker", "cert", "common_name", "not_after", "days_left", "status",
	})
	for _, entry := range entries {
		t.AppendRows([]table.Row{{
			entry.scope,
			entry.job,
			entry.workerIP,
			entry.certName,
			entry.commonName,
			entry.notAfter.UTC().Format("2006-01-02 15:04:05 UTC"),
			entry.daysLeft,
			entry.status,
		}})
	}
	t.Render()

	if err := tx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

func listCertificateEntries(
	tx *sql.Tx,
	jobsFilter, workersFilter []string,
	renewalBufferDays int,
) ([]certEntry, error) {
	now := time.Now().UTC()
	entries := make([]certEntry, 0)

	if caEntry, ok := readCACertEntry(renewalBufferDays, now); ok {
		entries = append(entries, caEntry)
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
		if len(jobsFilter) > 0 && len(utils.Intersection(jobsFilter, []string{job})) == 0 {
			continue
		}
		if len(workersFilter) > 0 && len(utils.Intersection(workersFilter, []string{workerIP})) == 0 {
			continue
		}

		certName := strings.TrimSuffix(strings.TrimPrefix(key, "certs/"), ".crt")
		entry, err := certEntryFromPEM("job", job, workerIP, certName, []byte(value), renewalBufferDays, now)
		if err != nil {
			entry = certEntry{
				scope:    "job",
				job:      job,
				workerIP: workerIP,
				certName: certName,
				status:   certStatusInvalid,
				daysLeft: 0,
			}
		}
		entries = append(entries, entry)
	}
	if err := data.RowsErr(rows); err != nil {
		return nil, err
	}

	return entries, nil
}

func readCACertEntry(renewalBufferDays int, now time.Time) (certEntry, bool) {
	caPath := path.Join(bucket.SecretLocation, "ca.crt")
	pemBytes, err := os.ReadFile(caPath)
	if err != nil {
		return certEntry{}, false
	}
	entry, err := certEntryFromPEM("ca", "", "", "ca", pemBytes, renewalBufferDays, now)
	if err != nil {
		return certEntry{
			scope:    "ca",
			certName: "ca",
			status:   certStatusInvalid,
		}, true
	}
	return entry, true
}

func certEntryFromPEM(
	scope, job, workerIP, certName string,
	pemBytes []byte,
	renewalBufferDays int,
	now time.Time,
) (certEntry, error) {
	cert, err := parseX509CertPEM(pemBytes)
	if err != nil {
		return certEntry{}, err
	}
	daysLeft := certDaysLeft(cert.NotAfter, now)
	return certEntry{
		scope:      scope,
		job:        job,
		workerIP:   workerIP,
		certName:   certName,
		commonName: cert.Subject.CommonName,
		notAfter:   cert.NotAfter.UTC(),
		daysLeft:   daysLeft,
		status:     certExpiryStatus(cert.NotAfter, renewalBufferDays, now),
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
	notAfter = notAfter.UTC()
	if now.After(notAfter) {
		return certStatusExpired
	}
	if build.CertNeedsRenewal(notAfter, renewalBufferDays, now) {
		return certStatusExpiring
	}
	return certStatusOK
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
