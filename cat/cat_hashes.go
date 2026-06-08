// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cat

import (
	"database/sql"
	"fmt"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/utils"

	"github.com/jedib0t/go-pretty/v6/table"
)

func Hashes(jobsCSV, workersCSV string, activeOnly bool) error {
	db, err := data.OpenDatabase(true)
	if err != nil {
		return bucket.DatabaseError(err)
	}

	tx, err := db.Begin()
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	jobsFilter := parseCSVFilter(jobsCSV)
	workersFilter := parseCSVFilter(workersCSV)

	if err := validateHashFilters(tx, jobsFilter, workersFilter); err != nil {
		return err
	}

	query := `
		SELECT a.alloc_id, a.worker_ip, a.job, a.disabled, a.removed,
		       ifnull(h.current_hash, ''), ifnull(h.previous_hash, ''),
		       ifnull(h.current_version, ''), ifnull(a.new_version, '')
		FROM allocations a
		LEFT JOIN hash h ON h.namespace = (a.job || '_allocation') AND h.key = a.alloc_id`

	var conditions []string
	if len(jobsFilter) > 0 {
		conditions = append(conditions, fmt.Sprintf("a.job IN ('%s')", strings.Join(jobsFilter, "','")))
	}
	if len(workersFilter) > 0 {
		conditions = append(conditions, fmt.Sprintf("a.worker_ip IN ('%s')", strings.Join(workersFilter, "','")))
	}
	if activeOnly {
		conditions = append(conditions, "a.removed = 0", "a.disabled = 0")
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY a.job, a.worker_ip"

	rows, err := tx.Query(query)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	t := utils.GetTable(table.Row{
		"job", "worker_ip", "alloc_id", "rollout",
		"current_hash", "previous_hash",
		"current_version", "new_version",
		"disabled", "removed",
	})

	rowCount := 0
	for rows.Next() {
		var (
			allocID, workerIP, job       string
			disabled, removed            int
			currentHash, previousHash    string
			currentVersion, newVersion   string
		)
		if err := rows.Scan(
			&allocID, &workerIP, &job, &disabled, &removed,
			&currentHash, &previousHash, &currentVersion, &newVersion,
		); err != nil {
			return bucket.DatabaseError(err)
		}

		t.AppendRows([]table.Row{{
			job, workerIP, allocID,
			rolloutStatus(disabled, removed, currentHash, previousHash, currentVersion, newVersion),
			currentHash, previousHash,
			displayVersion(currentVersion), displayVersion(newVersion),
			disabled, removed,
		}})
		rowCount++
	}
	if err := data.RowsErr(rows); err != nil {
		return err
	}
	if rowCount == 0 {
		return bucket.NotFoundError("hashes")
	}

	t.Render()

	if err := tx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}
	return nil
}

func parseCSVFilter(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return utils.Unique(out)
}

func validateHashFilters(tx *sql.Tx, jobsFilter, workersFilter []string) error {
	if len(jobsFilter) > 0 {
		allJobs, err := data.GetAllAllocatedJobs(tx)
		if err != nil {
			return err
		}
		if len(utils.Intersection(allJobs, jobsFilter)) == 0 {
			return fmt.Errorf("invalid input, jobs %v", jobsFilter)
		}
	}
	if len(workersFilter) > 0 {
		allocatedWorkers, err := data.GetAllocatedWorkerIPs(tx)
		if err != nil {
			return err
		}
		if len(utils.Intersection(allocatedWorkers, workersFilter)) == 0 {
			return fmt.Errorf("invalid input, workers %v", workersFilter)
		}
	}
	return nil
}

func rolloutStatus(disabled, removed int, currentHash, previousHash, currentVersion, newVersion string) string {
	if removed == 1 {
		return "removed"
	}
	if disabled == 1 {
		if previousHash != "" && previousHash != currentHash {
			return "disabled_restart"
		}
		if displayVersion(currentVersion) != displayVersion(newVersion) {
			return "disabled_restart"
		}
		return "disabled"
	}
	if currentHash == "" && previousHash == "" {
		return "new"
	}
	if previousHash == "" {
		return "new"
	}
	if previousHash == data.HealthFailedPreviousHash {
		return "health_failed"
	}
	if previousHash != currentHash {
		return "restart"
	}
	if displayVersion(currentVersion) != displayVersion(newVersion) {
		return "restart"
	}
	return "promoted"
}

func displayVersion(version string) string {
	if version == "" {
		return data.DefaultAllocationVersion
	}
	return version
}
