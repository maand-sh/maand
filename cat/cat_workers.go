// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cat

import (
	"database/sql"
	"errors"

	"maand/bucket"
	"maand/data"
	"maand/utils"

	"github.com/jedib0t/go-pretty/v6/table"
)

func Workers() error {
	// TODO: labels filter
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

	workers, err := data.GetAllWorkers(tx)
	if errors.Is(err, bucket.ErrDatabase) {
		return err
	}
	if errors.Is(err, sql.ErrNoRows) || len(workers) == 0 {
		return bucket.NotFoundError("workers")
	}

	rows, err := tx.Query(`SELECT worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position, labels, zone FROM cat_workers`)
	if err != nil {
		return bucket.DatabaseError(err)
	}
	defer func() {
		_ = rows.Close()
	}()

	t := utils.GetTable(table.Row{"Worker IP", "zone", "CPU (mhz)", "Memory (mb)", "Position", "labels"})

	for rows.Next() {
		var workerID string
		var workerIP string
		var position string
		var availableMemoryMB float64
		var availableCPUMHZ float64
		var labels string
		var zone sql.NullString

		err = rows.Scan(&workerID, &workerIP, &availableCPUMHZ, &availableMemoryMB, &position, &labels, &zone)
		if err != nil {
			return bucket.DatabaseError(err)
		}

		t.AppendRows([]table.Row{{workerIP, zone.String, availableMemoryMB, availableCPUMHZ, position, labels}})
	}
	if err := data.RowsErr(rows); err != nil {
		return err
	}

	t.Render()

	if err = tx.Commit(); err != nil {
		return bucket.DatabaseError(err)
	}

	return nil
}
