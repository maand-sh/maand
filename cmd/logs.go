// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"

	"maand/logs"

	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Read structured bucket logs",
}

var logsShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show filtered structured log lines",
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		worker, _ := flags.GetString("worker")
		runID, _ := flags.GetString("run")
		job, _ := flags.GetString("job")
		phase, _ := flags.GetString("phase")
		event, _ := flags.GetString("event")
		tail, _ := flags.GetInt("tail")
		format, _ := flags.GetString("format")
		runDir, _ := flags.GetBool("run-dir")

		err := logs.Show(logs.ShowOptions{
			Worker: worker,
			RunID:  runID,
			Job:    job,
			Phase:  phase,
			Event:  event,
			Tail:   tail,
			RunDir: runDir,
			Format: format,
		})
		if err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(logsCmd)
	logsCmd.AddCommand(logsShowCmd)

	logsShowCmd.Flags().String("worker", "", "filter by worker IP")
	logsShowCmd.Flags().String("run", "", "filter by run id (reads logs/runs/<run>/ when set)")
	logsShowCmd.Flags().String("job", "", "filter by job name")
	logsShowCmd.Flags().String("phase", "", "filter by phase (reconcile, rsync, rollout, ...)")
	logsShowCmd.Flags().String("event", "", "filter by event (command_begin, deploy_skip, ...)")
	logsShowCmd.Flags().Int("tail", 0, "print only the last N matching lines (human: last N command blocks)")
	logsShowCmd.Flags().Bool("run-dir", false, "read from logs/runs/<run>/ instead of worker aggregate log")
	logsShowCmd.Flags().String("format", logs.FormatRaw, "output format: raw (structured lines) or human (grouped blocks)")
}
