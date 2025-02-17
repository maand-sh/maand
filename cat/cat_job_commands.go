package cat

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"maand/data"
	"maand/utils"
)

func JobCommands() {
	db, err := data.GetDatabase(true)
	utils.Check(err)

	tx, err := db.Begin()
	utils.Check(err)
	defer func() {
		_ = tx.Rollback()
	}()

	query := "SELECT count(*) FROM job_commands"
	row := tx.QueryRow(query)
	jobCommandsCount := 0
	if _ = row.Scan(&jobCommandsCount); jobCommandsCount == 0 {
		fmt.Println("No job commands found")
		return
	}

	rows, err := tx.Query(`SELECT job, command_name, executed_on, depend_on_job, depend_on_command, depend_on_config FROM cat_job_commands`)
	utils.Check(err)

	t := utils.GetTable(table.Row{"job", "command_name", "executed_on", "depend_on_job", "depend_on_command", "depend_on_config"})

	for rows.Next() {
		var jobName string
		var commandName string
		var executedOn string
		var dependOnJob string
		var dependOnCommand string
		var dependOnConfig string

		err = rows.Scan(&jobName, &commandName, &executedOn, &dependOnJob, &dependOnCommand, &dependOnConfig)
		utils.Check(err)

		t.AppendRows([]table.Row{{jobName, commandName, executedOn, dependOnJob, dependOnCommand, dependOnConfig}})
	}

	t.Render()
}
