package cmd

import (
	"github.com/spf13/cobra"
	"maand/data"
	"maand/gc"
	"maand/utils"
)

var gcCmd = &cobra.Command{
	Use:   "gc",
	Short: "Cleanup unused objects in the bucket",
	Run: func(cmd *cobra.Command, args []string) {
		db, err := data.GetDatabase(true)
		utils.Check(err)

		tx, err := db.Begin()
		utils.Check(err)
		defer func() {
			_ = tx.Rollback()
		}()

		gc.Collect(tx)

		err = tx.Commit()
		utils.Check(err)
	},
}

func init() {
	maandCmd.AddCommand(gcCmd)
}
