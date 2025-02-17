package gc

import (
	"database/sql"
	"maand/utils"
)

func Collect(tx *sql.Tx) {
	// TODO : clean allocation hash first
	deletes := []string{
		"DELETE FROM allocations WHERE removed = 1",
	}

	for _, query := range deletes {
		_, err := tx.Exec(query)
		utils.Check(err)
	}

	err := utils.GetKVStore().GC(tx, -1)
	utils.Check(err)
}
