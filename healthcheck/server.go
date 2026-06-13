// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package healthcheck

import (
	"log"
	"net/http"
	"time"

	"maand/bucket"
	"maand/data"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	jobHealthStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "maand_health_check_status",
			Help: "Health check status of a job (1 = healthy, 0 = unhealthy)",
		},
		[]string{"job"},
	)
	lastCheckTime = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "maand_health_check_last_run_timestamp_seconds",
			Help: "Timestamp of the last health check run",
		},
	)
	checkDuration = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "maand_health_check_duration_seconds",
			Help: "Duration of the last health check run in seconds",
		},
	)
)

func init() {
	prometheus.MustRegister(jobHealthStatus)
	prometheus.MustRegister(lastCheckTime)
	prometheus.MustRegister(checkDuration)
}

// Serve runs a metrics server that periodically executes health checks.
func Serve(addr string, intervalSeconds int, jobsComma string) error {
	log.Printf("Starting health check server on %s", addr)
	log.Printf("Health check interval: %d seconds", intervalSeconds)

	interval := time.Duration(intervalSeconds) * time.Second

	http.Handle("/metrics", promhttp.Handler())

	go func() {
		for {
			start := time.Now()
			log.Printf("Running periodic health checks...")
			
			err := runServerHealthChecks(jobsComma)
			if err != nil {
				log.Printf("Health check run failed: %v", err)
			} else {
				log.Printf("Health check run completed successfully")
			}

			duration := time.Since(start)
			lastCheckTime.Set(float64(time.Now().Unix()))
			checkDuration.Set(duration.Seconds())

			time.Sleep(interval)
		}
	}()

	return http.ListenAndServe(addr, nil)
}

func runServerHealthChecks(jobsComma string) error {
	db, err := data.OpenDatabase(true)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	cancel, err := PrepareRuntime(tx)
	if err != nil {
		return err
	}
	defer cancel()

	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}

	rt, err := bucket.SetupRuntime(bucketID)
	if err != nil {
		return err
	}
	defer rt.Stop()

	jobNames, err := data.GetJobs(tx)
	if err != nil {
		return err
	}

	jobFilter := parseJobFilter(jobsComma)
	if len(jobFilter) > 0 {
		jobNames = jobFilter
	}

	for _, jobName := range jobNames {
		// HealthCheck returns (hashMarked, error). Success is error == nil.
		_, err := HealthCheck(tx, rt, false, false, jobName, false)
		
		status := 1.0
		if err != nil {
			status = 0.0
			log.Printf("Health check failed for job %s: %v", jobName, err)
		}
		jobHealthStatus.WithLabelValues(jobName).Set(status)
	}

	return nil
}
