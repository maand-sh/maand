// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cat

import (
	"maand/data"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
)

func Info() error {
	db, err := data.GetDatabase(true)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()

	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}

	updateSeq, err := data.GetUpdateSeq(tx)
	if err != nil {
		return err
	}

	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return err
	}

	jobs, err := data.GetJobs(tx)
	if err != nil {
		return err
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Description", "Value"})
	t.SetStyle(table.StyleRounded)
	t.AppendRow(table.Row{"Bucket ID", bucketID})
	t.AppendRow(table.Row{"Update Sequence", updateSeq})
	t.AppendRow(table.Row{"Number of Allocations", len(workers)})
	t.AppendRow(table.Row{"Number of Jobs", len(jobs)})
	t.Render()

	return nil
}
