// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/jobcommand"
)

const (
	deployPhaseNew    = "new"
	deployPhaseUpdate = "update"
	deployPhaseStop   = "stop"
)

// BatchContext describes one start/restart/stop batch for allocation hooks.
type BatchContext struct {
	Job             string
	Phase           string
	BatchIndex      int
	BatchCount      int
	BatchAllocation []string
	DeployOrder     string
	OrderSource     string
}

func batchCount(total, batchSize int) int {
	if total == 0 {
		return 0
	}
	if batchSize < 1 {
		batchSize = total
	}
	return (total + batchSize - 1) / batchSize
}

func batchEnv(ctx BatchContext) []string {
	return []string{
		fmt.Sprintf("BATCH_ALLOCATIONS=%s", strings.Join(ctx.BatchAllocation, ",")),
		fmt.Sprintf("BATCH_INDEX=%d", ctx.BatchIndex),
		fmt.Sprintf("BATCH_COUNT=%d", ctx.BatchCount),
		fmt.Sprintf("DEPLOY_PHASE=%s", ctx.Phase),
		fmt.Sprintf("DEPLOY_ORDER=%s", ctx.DeployOrder),
		fmt.Sprintf("DEPLOY_ORDER_SOURCE=%s", ctx.OrderSource),
		fmt.Sprintf("JOB=%s", ctx.Job),
	}
}

func executeAllocationEventHooks(
	tx *sql.Tx,
	rt *bucket.Runtime,
	job, event string,
	workerIPs []string,
	ctx BatchContext,
) error {
	if len(workerIPs) == 0 {
		return nil
	}
	commands, err := data.GetJobCommands(tx, job, event)
	if err != nil {
		return err
	}
	if len(commands) == 0 {
		return nil
	}

	ctx.Job = job
	extraEnv := batchEnv(ctx)
	concurrency := len(workerIPs)
	if concurrency < 1 {
		concurrency = 1
	}

	for _, command := range commands {
		if err := jobcommand.JobCommandOnWorkers(
			tx, rt, job, command, event, workerIPs, concurrency, true, extraEnv,
		); err != nil {
			return err
		}
	}
	return nil
}

func executeAfterAllocationStarted(
	tx *sql.Tx,
	rt *bucket.Runtime,
	job string,
	workerIPs []string,
	ctx BatchContext,
) error {
	return executeAllocationEventHooks(tx, rt, job, "after_allocation_started", workerIPs, ctx)
}

func executeAfterAllocationStopped(
	tx *sql.Tx,
	rt *bucket.Runtime,
	job, workerIP string,
) error {
	ctx := BatchContext{
		Job:             job,
		Phase:           deployPhaseStop,
		BatchIndex:      0,
		BatchCount:      1,
		BatchAllocation: []string{workerIP},
	}
	return executeAllocationEventHooks(tx, rt, job, "after_allocation_stopped", []string{workerIP}, ctx)
}
