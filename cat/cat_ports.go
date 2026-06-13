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

func JobPorts(jobsCSV string) error {
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

	var jobsFilter []string
	if jobsCSV != "" {
		jobsFilter = strings.Split(jobsCSV, ",")
	}
	jobsFilter = utils.Unique(jobsFilter)

	if len(jobsFilter) > 0 {
		allJobs, err := data.GetJobs(tx)
		if err != nil {
			return err
		}
		if len(utils.Intersection(allJobs, jobsFilter)) == 0 {
			return fmt.Errorf("invalid input, jobs %v", jobsFilter)
		}
	}

	count := 0
	query := "SELECT count(*) FROM job_ports"
	row := tx.QueryRow(query)
	err = row.Scan(&count)
	if errors.Is(err, sql.ErrNoRows) || count == 0 {
		return bucket.NotFoundError("job ports")
	}

	sqlQuery := `SELECT j.name as job, jp.name, jp.port FROM job_ports jp JOIN job j ON jp.job_id = j.job_id`
	if len(jobsFilter) > 0 {
		sqlQuery = fmt.Sprintf("%s WHERE j.name IN ('%s')", sqlQuery, strings.Join(jobsFilter, "','"))
	}

	rows, err := tx.Query(sqlQuery)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	t := utils.GetTable(table.Row{"job", "name", "port"})

	for rows.Next() {
		var job string
		var name string
		var port int

		err = rows.Scan(&job, &name, &port)
		if err != nil {
			return bucket.DatabaseError(err)
		}

		t.AppendRows([]table.Row{{job, name, port}})
	}
	if err := data.RowsErr(rows); err != nil {
		return err
	}

	t.Render()

	if err := tx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}

	return nil
}
