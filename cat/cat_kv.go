package cat

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"maand/data"
	"maand/utils"
	"strings"
)

func KV() {

	db, err := data.GetDatabase(true)
	utils.Check(err)
	defer func() {
		_ = db.Close()
	}()

	tx, err := db.Begin()
	utils.Check(err)
	defer func() {
		_ = tx.Rollback()
	}()

	query := "SELECT count(*) FROM key_value"
	row := tx.QueryRow(query)
	workerCount := 0
	if _ = row.Scan(&workerCount); workerCount == 0 {
		fmt.Println("No key values found")
		return
	}

	rows, err := tx.Query(`SELECT namespace, key, value, version, ttl, created_date, deleted FROM cat_kv`)
	utils.Check(err)

	t := utils.GetTable(table.Row{"Namespace", "Key", "Value", "Version", "ttl", "createdDate", "deleted"})

	for rows.Next() {
		var namespace string
		var key string
		var value string
		var version string
		var ttl int
		var createdDate string
		var deleted int

		err = rows.Scan(&namespace, &key, &value, &version, &ttl, &createdDate, &deleted)
		utils.Check(err)

		if strings.HasPrefix(key, "certs/") {
			value = strings.Split(value, "\n")[0]
		}

		t.AppendRows([]table.Row{{namespace, key, value, version, ttl, createdDate, deleted}})
	}

	t.Render()
}
