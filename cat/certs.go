// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cat

import (
	"maand/bucket"
	"maand/certs"
	"maand/data"
	"maand/utils"

	"github.com/jedib0t/go-pretty/v6/table"
)

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

	entries, err := certs.ListCertMetrics(tx, jobsFilter, workersFilter)
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
			entry.Scope,
			entry.Job,
			entry.WorkerIP,
			entry.CertName,
			entry.CommonName,
			entry.NotAfter.UTC().Format("2006-01-02 15:04:05 UTC"),
			entry.DaysLeft,
			entry.Status,
		}})
	}
	t.Render()

	if err := tx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}
