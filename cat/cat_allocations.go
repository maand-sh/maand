// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cat

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/utils"

	"github.com/jedib0t/go-pretty/v6/table"
)

func Allocations(jobsCSV, workersCSV string) error {
	db, err := data.GetDatabase(true)
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

	var jobsFilter []string
	if jobsCSV != "" {
		jobsFilter = strings.Split(jobsCSV, ",")
	}

	var workersFilter []string
	if workersCSV != "" {
		workersFilter = strings.Split(workersCSV, ",")
	}

	jobsFilter = utils.Unique(jobsFilter)
	workersFilter = utils.Unique(workersFilter)

	allJobs, err := data.GetAllAllocatedJobs(tx)
	if err != nil {
		return err
	}

	if len(jobsFilter) > 0 && len(utils.Intersection(allJobs, jobsFilter)) == 0 {
		return fmt.Errorf("invalid input, jobs %v", jobsFilter)
	}

	allWorkers, err := data.GetAllWorkers(tx)
	if err != nil {
		return err
	}

	if len(workersFilter) > 0 && len(utils.Intersection(allWorkers, workersFilter)) == 0 {
		return fmt.Errorf("invalid input, workers %v", workersFilter)
	}

	var workerCount int

	query := "SELECT count(*) FROM allocations"
	row := tx.QueryRow(query)

	err = row.Scan(&workerCount)
	if workerCount == 0 || errors.Is(err, sql.ErrNoRows) {
		return bucket.NotFoundError("allocations")
	}
	if err != nil {
		return bucket.DatabaseError(err)
	}

	t := utils.GetTable(table.Row{"allocation_id", "worker_ip", "job", "disabled", "removed"})

	query = "SELECT alloc_id, worker_ip, job, disabled, removed FROM cat_allocations"
	if len(jobsFilter) > 0 || len(workersFilter) > 0 {
		query = fmt.Sprintf("%s WHERE", query)
	}

	if len(jobsFilter) > 0 {
		query = fmt.Sprintf("%s job IN ('%s') ", query, strings.Join(jobsFilter, "','"))
		if len(workersFilter) > 0 {
			query = fmt.Sprintf("%s AND", query)
		}
	}
	if len(workersFilter) > 0 {
		query = fmt.Sprintf("%s worker_ip IN ('%s') ", query, strings.Join(workersFilter, "','"))
	}

	rows, err := tx.Query(query)
	if err != nil {
		return bucket.DatabaseError(err)
	}

	for rows.Next() {
		var allocID string
		var workerIP string
		var job string
		var disabled int
		var removed int

		err = rows.Scan(&allocID, &workerIP, &job, &disabled, &removed)
		if err != nil {
			return bucket.DatabaseError(err)
		}

		t.AppendRows([]table.Row{{allocID, workerIP, job, disabled, removed}})
	}

	t.Render()

	err = tx.Commit()
	if err != nil {
		return bucket.DatabaseError(err)
	}

	return nil
}
