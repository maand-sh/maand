// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cmd

import (
	"log"

	"maand/cat"

	"github.com/spf13/cobra"
)

var catPrometheusCmd = &cobra.Command{
	Use:   "prometheus",
	Short: "Show _prometheus/ catalog (scrape, alerts, runbooks, dashboards)",
	Long: `List jobs that ship _prometheus/ content from the build catalog.

Use get to print one file under job/_prometheus/.
Use scrape to preview expanded scrape configs (same as deploy {{ scrapeConfigs }}).`,
	Run: runCatPrometheusList,
}

var catPrometheusGetCmd = &cobra.Command{
	Use:   "get <job> <path>",
	Short: "Print a file under job/_prometheus/",
	Long: `Path is relative to _prometheus/, for example:
  scrape.yaml  (or shorthand: scrape)
  alerts/slo.yaml
  runbooks/ApiDown.md
  dashboards/overview.html

Reads from the build catalog when available; otherwise from workspace/jobs/<job>/_prometheus/.`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		if err := cat.PrometheusGet(args[0], args[1]); err != nil {
			log.Fatalln(err)
		}
	},
}

var catPrometheusScrapeCmd = &cobra.Command{
	Use:   "scrape",
	Short: "Print expanded scrape configs (deploy preview)",
	Run: runCatPrometheusScrape,
}

func runCatPrometheusList(cmd *cobra.Command, _ []string) {
	jobsStr, _ := cmd.Flags().GetString("jobs")
	if err := cat.Prometheus(jobsStr); err != nil {
		log.Fatalln(err)
	}
}

func runCatPrometheusScrape(cmd *cobra.Command, _ []string) {
	jobsStr, _ := cmd.Flags().GetString("jobs")
	if err := cat.PrometheusScrape(jobsStr); err != nil {
		log.Fatalln(err)
	}
}

func init() {
	catCmd.AddCommand(catPrometheusCmd)
	catPrometheusCmd.AddCommand(catPrometheusGetCmd)
	catPrometheusCmd.AddCommand(catPrometheusScrapeCmd)
	catPrometheusCmd.Flags().String("jobs", "", "Comma-separated job names")
	catPrometheusScrapeCmd.Flags().String("jobs", "", "Comma-separated job names")
}
