package cat

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"maand/data"
	"maand/utils"
)

// TODO: job's filter

func Allocations() {
	db, err := data.GetDatabase(true)
	utils.Check(err)

	tx, err := db.Begin()
	utils.Check(err)
	defer func() {
		_ = tx.Rollback()
	}()

	query := "SELECT count(*) FROM allocations"
	row := tx.QueryRow(query)
	workerCount := 0
	if _ = row.Scan(&workerCount); workerCount == 0 {
		fmt.Println("No allocations found")
		return
	}

	rows, err := tx.Query("SELECT alloc_id, worker_ip, job, disabled, removed FROM cat_allocations")
	utils.Check(err)

	t := utils.GetTable(table.Row{"allocation_id", "worker_ip", "job", "disabled", "removed"})

	for rows.Next() {
		var allocID string
		var workerIP string
		var job string
		var disabled int
		var removed int

		err = rows.Scan(&allocID, &workerIP, &job, &disabled, &removed)
		utils.Check(err)

		t.AppendRows([]table.Row{{allocID, workerIP, job, disabled, removed}})
	}

	t.Render()
}
