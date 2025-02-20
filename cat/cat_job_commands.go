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

func JobCommands() error {
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

	var jobCommandsCount int
	query := "SELECT count(*) FROM job_commands"
	row := tx.QueryRow(query)
	err = row.Scan(&jobCommandsCount)
	if errors.Is(err, sql.ErrNoRows) || jobCommandsCount == 0 {
		return &NotFoundError{Domain: "job commands"}
	}
	if err != nil {
		return data.NewDatabaseError(err)
	}

	rows, err := tx.Query(`SELECT job, command_name, executed_on, demand_job, demand_command, demand_config FROM cat_job_commands`)
	if err != nil {
		return data.NewDatabaseError(err)
	}

	t := utils.GetTable(table.Row{"job", "command_name", "executed_on", "demand_job", "demand_command", "demand_config"})

	for rows.Next() {
		var jobName string
		var commandName string
		var executedOn string
		var demandJob string
		var demandCommand string
		var demandConfig string

		err = rows.Scan(&jobName, &commandName, &executedOn, &demandJob, &demandCommand, &demandConfig)
		if err != nil {
			return data.NewDatabaseError(err)
		}

		t.AppendRows([]table.Row{{jobName, commandName, executedOn, demandJob, demandCommand, demandConfig}})
	}

	t.Render()

	err = tx.Commit()
	if err != nil {
		return data.NewDatabaseError(err)
	}

	if err = data.UpdateJournalModeDefault(db); err != nil {
		return err
	}

	return nil
}
