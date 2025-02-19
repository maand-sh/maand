package cat

import (
	"database/sql"
	"errors"
	"github.com/jedib0t/go-pretty/v6/table"
	"maand/data"
	"maand/utils"
)

// TODO: job's filter

func Allocations() error {
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

	var workerCount int

	query := "SELECT count(*) FROM allocations"
	row := tx.QueryRow(query)

	err = row.Scan(&workerCount)
	if workerCount == 0 || errors.Is(err, sql.ErrNoRows) {
		return &NotFoundError{Domain: "allocations"}
	}
	if err != nil {
		return data.NewDatabaseError(err)
	}

	rows, err := tx.Query("SELECT alloc_id, worker_ip, job, disabled, removed FROM cat_allocations")
	if err != nil {
		return data.NewDatabaseError(err)
	}

	t := utils.GetTable(table.Row{"allocation_id", "worker_ip", "job", "disabled", "removed"})

	for rows.Next() {
		var allocID string
		var workerIP string
		var job string
		var disabled int
		var removed int

		err = rows.Scan(&allocID, &workerIP, &job, &disabled, &removed)
		if err != nil {
			return data.NewDatabaseError(err)
		}

		t.AppendRows([]table.Row{{allocID, workerIP, job, disabled, removed}})
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
