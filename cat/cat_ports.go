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

func JobPorts() error {
	// TODO: jobs filter

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
	query := "SELECT count(*) FROM job_ports"
	row := tx.QueryRow(query)
	err = row.Scan(&count)
	if errors.Is(err, sql.ErrNoRows) || count == 0 {
		return &NotFoundError{Domain: "job ports"}
	}

	rows, err := tx.Query(`SELECT (SELECT name FROM job WHERE job_id = jp.job_id) as job, name, port FROM job_ports jp`)
	if err != nil {
		return data.NewDatabaseError(err)
	}

	t := utils.GetTable(table.Row{"job", "name", "port"})

	for rows.Next() {
		var job string
		var name string
		var port int

		err = rows.Scan(&job, &name, &port)
		if err != nil {
			return data.NewDatabaseError(err)
		}

		t.AppendRows([]table.Row{{job, name, port}})
	}

	t.Render()

	if err := tx.Commit(); err != nil {
		return data.NewDatabaseError(err)
	}

	return nil
}
