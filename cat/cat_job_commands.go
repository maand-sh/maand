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

	rows, err := tx.Query(`SELECT job, command_name, executed_on, depend_on_job, depend_on_command, depend_on_config FROM cat_job_commands`)
	if err != nil {
		return data.NewDatabaseError(err)
	}

	t := utils.GetTable(table.Row{"job", "command_name", "executed_on", "depend_on_job", "depend_on_command", "depend_on_config"})

	for rows.Next() {
		var jobName string
		var commandName string
		var executedOn string
		var dependOnJob string
		var dependOnCommand string
		var dependOnConfig string

		err = rows.Scan(&jobName, &commandName, &executedOn, &dependOnJob, &dependOnCommand, &dependOnConfig)
		if err != nil {
			return data.NewDatabaseError(err)
		}

		t.AppendRows([]table.Row{{jobName, commandName, executedOn, dependOnJob, dependOnCommand, dependOnConfig}})
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
