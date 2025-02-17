package cat

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"maand/data"
	"maand/utils"
)

func Workers() {
	db, err := data.GetDatabase(true)
	utils.Check(err)

	tx, err := db.Begin()
	utils.Check(err)
	defer func() {
		_ = tx.Rollback()
	}()

	query := "SELECT count(*) FROM worker"
	row := tx.QueryRow(query)
	workerCount := 0
	if _ = row.Scan(&workerCount); workerCount == 0 {
		fmt.Println("No workers found")
		return
	}

	rows, err := tx.Query(`SELECT worker_id, worker_ip, available_memory_mb, available_cpu_mhz, position, labels FROM cat_workers`)
	utils.Check(err)

	t := utils.GetTable(table.Row{"Worker IP", "CPU (mhz)", "Memory (mb)", "Position", "labels"})

	for rows.Next() {
		var workerID string
		var workerIP string
		var position string
		var availableMemoryMB float64
		var availableCPUMHZ float64
		var labels string

		err = rows.Scan(&workerID, &workerIP, &availableCPUMHZ, &availableMemoryMB, &position, &labels)
		utils.Check(err)

		t.AppendRows([]table.Row{{workerIP, availableMemoryMB, availableCPUMHZ, position, labels}})
	}

	t.Render()
}
