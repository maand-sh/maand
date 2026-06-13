// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"

	"maand/healthcheck"

	"github.com/spf13/cobra"
)

var healthCheckCmd = &cobra.Command{
	Use:   "health_check",
	Short: "Run health_check job commands",
	Long: `Run health_check commands defined in each job manifest.

Use --jobs to limit which jobs are checked. With --wait, each job is retried until
its health_check commands pass or the retry limit is reached.

In --server mode, health checks are executed periodically and results are exported
as Prometheus metrics.`,
	Run: func(cmd *cobra.Command, args []string) {
		flags := cmd.Flags()
		wait, _ := flags.GetBool("wait")
		jobsComma, _ := flags.GetString("jobs")
		verbose, _ := flags.GetBool("verbose")
		updateHash, _ := flags.GetBool("update-hash")
		server, _ := flags.GetBool("server")
		addr, _ := flags.GetString("addr")
		interval, _ := flags.GetInt("interval")

		if server {
			if err := healthcheck.Serve(addr, interval, jobsComma); err != nil {
				log.Fatalln(err)
			}
			return
		}

		if err := healthcheck.Execute(wait, verbose, jobsComma, updateHash); err != nil {
			log.Fatalln(err)
		}
	},
}

func init() {
	maandCmd.AddCommand(healthCheckCmd)
	healthCheckCmd.Flags().Bool("verbose", false, "Stream command output from workers")
	healthCheckCmd.Flags().Bool("wait", false, "Retry until health checks pass (up to 30 attempts per job)")
	healthCheckCmd.Flags().String("jobs", "", "Comma-separated job names (default: all jobs)")
	healthCheckCmd.Flags().Bool(
		"update-hash",
		false,
		"Mark failed allocations for redeploy when health_check commands fail",
	)
	healthCheckCmd.Flags().Bool("server", false, "Run as a metrics server")
	healthCheckCmd.Flags().String("addr", ":9101", "Address to listen on for metrics")
	healthCheckCmd.Flags().Int("interval", 30, "Interval in seconds between health checks in server mode")
}
