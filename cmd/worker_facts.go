// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"
	"os"

	"maand/workerfacts"

	"github.com/spf13/cobra"
)

var workerFactsCmd = &cobra.Command{
	Use:   "worker_facts",
	Short: "Probe workers and update CPU/memory in workers.json",
	Long: `SSH to workers listed in workspace/workers.json, read host memory and CPU
capacity, and write the values back to workers.json.

Memory comes from /proc/meminfo (MemTotal). CPU is computed as logical cores
times per-core MHz from /proc/cpuinfo or lscpu.

Examples:
  maand worker_facts
  maand worker_facts --dry-run
  maand worker_facts -w 10.0.0.1,10.0.0.2 -c 2
  maand worker_facts --build`,
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()

		workerCSV, _ := flags.GetString("workers")
		labelCSV, _ := flags.GetString("labels")
		concurrency, _ := flags.GetInt("concurrency")
		dryRun, _ := flags.GetBool("dry-run")
		runBuild, _ := flags.GetBool("build")

		err := workerfacts.Execute(workerfacts.Options{
			WorkerCSV:   workerCSV,
			LabelCSV:    labelCSV,
			Concurrency: concurrency,
			DryRun:      dryRun,
			RunBuild:    runBuild,
		})
		if err != nil {
			log.Println(err)
			os.Exit(1)
		}
	},
}

func init() {
	maandCmd.AddCommand(workerFactsCmd)
	workerFactsCmd.Flags().StringP("workers", "w", "", "Comma-separated worker IPs")
	workerFactsCmd.Flags().StringP("labels", "l", "", "Comma-separated worker labels")
	workerFactsCmd.Flags().IntP("concurrency", "c", 1, "Workers to probe in parallel")
	workerFactsCmd.Flags().Bool("dry-run", false, "Show changes without writing workers.json")
	workerFactsCmd.Flags().Bool("build", false, "Run maand build after updating workers.json")
}
