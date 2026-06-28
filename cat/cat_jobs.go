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

func Jobs() error {
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

	var count int
	query := "SELECT count(*) FROM job"
	row := tx.QueryRow(query)
	err = row.Scan(&count)
	if errors.Is(err, sql.ErrNoRows) || count == 0 {
		return bucket.NotFoundError("jobs")
	}
	if err != nil {
		return bucket.DatabaseError(err)
	}

	rows, err := tx.Query(`SELECT job_id, name, version, disabled, deployment_seq, selectors, current_memory_mb, current_memory_source, current_cpu_mhz, current_cpu_source FROM cat_jobs`)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	t := utils.GetTable(table.Row{"job", "version", "disabled", "deployment_seq", "cpu", "memory", "selectors"})

	for rows.Next() {
		var jobID string
		var name string
		var version string
		var disabled int
		var deploymentSeq int
		var selectors string
		var memoryMB string
		var memorySource string
		var cpuMHz string
		var cpuSource string

		err = rows.Scan(
			&jobID, &name, &version, &disabled, &deploymentSeq, &selectors,
			&memoryMB, &memorySource, &cpuMHz, &cpuSource,
		)
		if err != nil {
			return bucket.DatabaseError(err)
		}

		t.AppendRows([]table.Row{
			{
				name,
				version,
				disabled,
				deploymentSeq,
				formatJobCPU(cpuMHz, cpuSource),
				formatJobMemory(memoryMB, memorySource),
				selectors,
			},
		})
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

func formatJobResource(value, unit, source string) string {
	source = strings.TrimSpace(source)
	if source == "" {
		source = "manifest"
	}
	value = strings.TrimSpace(value)
	if value == "" {
		value = "0"
	}
	return fmt.Sprintf("%s %s (%s)", value, unit, source)
}

func formatJobCPU(mhz, source string) string {
	return formatJobResource(mhz, "mhz", source)
}

func formatJobMemory(mb, source string) string {
	return formatJobResource(mb, "mb", source)
}
