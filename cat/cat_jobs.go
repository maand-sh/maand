package cat

import (
	"database/sql"
	"errors"
	"github.com/jedib0t/go-pretty/v6/table"
	"maand/data"
	"maand/utils"
)

func Jobs() error {
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

	var count int
	query := "SELECT count(*) FROM job"
	row := tx.QueryRow(query)
	err = row.Scan(&count)
	if errors.Is(err, sql.ErrNoRows) || count == 0 {
		return &NotFoundError{Domain: "jobs"}
	}
	if err != nil {
		return data.NewDatabaseError(err)
	}

	rows, err := tx.Query(`SELECT job_id, name, version, disabled, deployment_seq, selectors FROM cat_jobs`)
	if err != nil {
		return data.NewDatabaseError(err)
	}

	t := utils.GetTable(table.Row{"job", "version", "disabled", "deployment_seq", "selectors"})

	for rows.Next() {
		var jobID string
		var name string
		var version string
		var disabled int
		var deploymentSeq int
		var selectors string

		err = rows.Scan(&jobID, &name, &version, &disabled, &deploymentSeq, &selectors)
		if err != nil {
			return data.NewDatabaseError(err)
		}

		t.AppendRows([]table.Row{{name, version, disabled, deploymentSeq, selectors}})
	}

	t.Render()

	if err := tx.Commit(); err != nil {
		return data.NewDatabaseError(err)
	}

	if err = data.UpdateJournalModeDefault(db); err != nil {
		return err
	}

	return nil
}
