package cat

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"maand/data"
	"maand/utils"
)

func JobPorts() {
	db, err := data.GetDatabase(true)
	utils.Check(err)

	tx, err := db.Begin()
	utils.Check(err)
	defer func() {
		_ = tx.Rollback()
	}()

	query := "SELECT count(*) FROM job_ports"
	row := tx.QueryRow(query)
	workerCount := 0
	if _ = row.Scan(&workerCount); workerCount == 0 {
		fmt.Println("No job ports found")
		return
	}

	rows, err := tx.Query(`SELECT (SELECT name FROM job WHERE job_id = jp.job_id) as job, name, port FROM job_ports jp`)
	utils.Check(err)

	t := utils.GetTable(table.Row{"job", "name", "port"})

	for rows.Next() {
		var job string
		var name string
		var port int

		err = rows.Scan(&job, &name, &port)
		utils.Check(err)

		t.AppendRows([]table.Row{{job, name, port}})
	}

	t.Render()
}
