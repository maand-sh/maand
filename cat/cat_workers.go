// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cat

import (
	"database/sql"
	"errors"
	"github.com/jedib0t/go-pretty/v6/table"
	"maand/data"
	"maand/utils"
)

func Workers() error {

	// TODO: labels filter

	db, err := data.GetDatabase(true)
	if err != nil {
		return data.NewDatabaseError(err)
	}

	tx, err := db.Begin()
	if err != nil {
		return data.NewDatabaseError(err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	count := 0
	query := "SELECT count(*) FROM worker"
	row := tx.QueryRow(query)
	err = row.Scan(&count)
	if errors.Is(err, sql.ErrNoRows) || count == 0 {
		return &NotFoundError{Domain: "workers"}
	}

	rows, err := tx.Query(`SELECT worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position, labels FROM cat_workers`)
	if err != nil {
		return data.NewDatabaseError(err)
	}

	t := utils.GetTable(table.Row{"Worker IP", "CPU (mhz)", "Memory (mb)", "Position", "labels"})

	for rows.Next() {
		var workerID string
		var workerIP string
		var position string
		var availableMemoryMB float64
		var availableCPUMHZ float64
		var labels string

		err = rows.Scan(&workerID, &workerIP, &availableCPUMHZ, &availableMemoryMB, &position, &labels)
		if err != nil {
			return data.NewDatabaseError(err)
		}

		t.AppendRows([]table.Row{{workerIP, availableMemoryMB, availableCPUMHZ, position, labels}})
	}

	t.Render()

	if err = tx.Commit(); err != nil {
		return data.NewDatabaseError(err)
	}

	return nil
}
