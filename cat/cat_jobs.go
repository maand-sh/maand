package cat

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"maand/data"
	"maand/utils"
)

func Jobs() {
	db, err := data.GetDatabase(true)
	utils.Check(err)

	tx, err := db.Begin()
	utils.Check(err)
	defer func() {
		_ = tx.Rollback()
	}()

	query := "SELECT count(*) FROM job"
	row := tx.QueryRow(query)
	workerCount := 0
	if _ = row.Scan(&workerCount); workerCount == 0 {
		fmt.Println("No jobs found")
		return
	}

	rows, err := tx.Query(`SELECT job_id, name, version, disabled, deployment_seq, selectors FROM cat_jobs`)
	utils.Check(err)

	t := utils.GetTable(table.Row{"job", "version", "disabled", "deployment_seq", "selectors"})

	for rows.Next() {
		var jobID string
		var name string
		var version string
		var disabled int
		var deploymentSeq int
		var selectors string

		err = rows.Scan(&jobID, &name, &version, &disabled, &deploymentSeq, &selectors)
		utils.Check(err)

		t.AppendRows([]table.Row{{name, version, disabled, deploymentSeq, selectors}})
	}

	t.Render()
}
