// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"
	"os"

	"maand/bucket"
	"maand/data"
	"maand/kv"
)

const (
	rolloutActionStart   = "start"
	rolloutActionRestart = "restart"
	rolloutActionSkip    = "skip"
)

// AllocationPlan describes one worker allocation after plan hashes are refreshed.
type AllocationPlan struct {
	WorkerIP     string
	Action       string
	PreviousHash string
	CurrentHash  string
}

// JobPlan summarizes whether a job would be deployed in the current wave.
type JobPlan struct {
	Job           string
	DeploymentSeq int
	NeedsRollout  bool
	SkipReason    string
	Allocations   []AllocationPlan
}

// DryRunResult is the outcome of a deploy dry-run.
type DryRunResult struct {
	Jobs     []JobPlan
	Required bool
}

// DryRun stages job files locally, refreshes allocation content hashes in a rolled-back
// transaction, and reports which jobs and allocations would be deployed. No workers are
// contacted and no hash promotions are persisted.
func DryRun(jobsFilter []string, force bool) (DryRunResult, error) {
	var result DryRunResult

	db, err := data.OpenDatabase(true)
	if err != nil {
		return result, err
	}
	defer func() {
		_ = db.Close()
	}()

	if err := os.RemoveAll(bucket.TempLocation); err != nil {
		return result, bucket.UnexpectedError(err)
	}
	defer func() {
		_ = os.RemoveAll(bucket.TempLocation)
	}()

	tx, err := db.Begin()
	if err != nil {
		return result, bucket.DatabaseError(err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	if err := kv.Initialize(tx); err != nil {
		return result, err
	}

	workers, err := data.GetWorkers(tx, nil)
	if err != nil {
		return result, err
	}

	maxDeploymentSequence, err := data.GetMaxDeploymentSeq(tx)
	if err != nil {
		return result, err
	}

	jobsFilter = normalizeJobFilter(jobsFilter)

	for deploymentSeq := 0; deploymentSeq <= maxDeploymentSequence; deploymentSeq++ {
		availableJobs, err := data.GetJobsByDeploymentSeq(tx, deploymentSeq)
		if err != nil {
			return result, err
		}
		jobs := selectJobsForDeploy(availableJobs, jobsFilter)
		if len(jobs) == 0 {
			continue
		}

		if err := prepareWorkersFiles(tx, workers); err != nil {
			return result, err
		}
		if err := refreshPlanHashesForJobs(tx, jobs); err != nil {
			return result, err
		}

		for _, job := range jobs {
			plan, err := planJobRollout(tx, job, deploymentSeq, force)
			if err != nil {
				return result, err
			}
			result.Jobs = append(result.Jobs, plan)
			if plan.NeedsRollout {
				result.Required = true
			}
		}
	}

	return result, nil
}

func planJobRollout(tx *sql.Tx, job string, deploymentSeq int, force bool) (JobPlan, error) {
	plan := JobPlan{
		Job:           job,
		DeploymentSeq: deploymentSeq,
	}

	activeWorkers, err := data.GetActiveAllocations(tx, job)
	if err != nil {
		return plan, err
	}
	if len(activeWorkers) == 0 {
		plan.SkipReason = "no active allocations"
		return plan, nil
	}

	namespace := fmt.Sprintf("%s_allocation", job)
	for _, workerIP := range activeWorkers {
		allocID, err := data.GetAllocationID(tx, workerIP, job)
		if err != nil {
			return plan, err
		}

		current, previous, ok, err := data.GetAllocationHash(tx, namespace, allocID)
		if err != nil {
			return plan, err
		}

		ap := AllocationPlan{WorkerIP: workerIP}
		switch {
		case !ok:
			ap.Action = rolloutActionStart
			plan.NeedsRollout = true
		case previous == "":
			ap.Action = rolloutActionStart
			ap.CurrentHash = current
			plan.NeedsRollout = true
		case previous != current:
			ap.Action = rolloutActionRestart
			ap.PreviousHash = previous
			ap.CurrentHash = current
			plan.NeedsRollout = true
		case force:
			ap.Action = rolloutActionRestart
			ap.PreviousHash = previous
			ap.CurrentHash = current
			plan.NeedsRollout = true
		default:
			needsVersion, err := data.AllocationNeedsVersionRollout(tx, job, workerIP)
			if err != nil {
				return plan, err
			}
			if needsVersion {
				ap.Action = rolloutActionRestart
				ap.PreviousHash = previous
				ap.CurrentHash = current
				plan.NeedsRollout = true
			} else {
				ap.Action = rolloutActionSkip
				ap.PreviousHash = previous
				ap.CurrentHash = current
			}
		}
		plan.Allocations = append(plan.Allocations, ap)
	}

	if !plan.NeedsRollout {
		plan.SkipReason = "already promoted on all allocations"
	}
	return plan, nil
}

// PrintDryRun writes a human-readable dry-run report to stdout.
func PrintDryRun(result DryRunResult) {
	if len(result.Jobs) == 0 {
		fmt.Println("deploy dry-run: no jobs matched")
		return
	}

	if result.Required {
		fmt.Println("deploy dry-run: deployment required")
	} else {
		fmt.Println("deploy dry-run: no deployment required")
	}
	fmt.Println()

	currentSeq := -1
	for _, job := range result.Jobs {
		if job.DeploymentSeq != currentSeq {
			currentSeq = job.DeploymentSeq
			fmt.Printf("deployment sequence %d:\n", currentSeq)
		}

		if job.NeedsRollout {
			fmt.Printf("  job %q: deploy required\n", job.Job)
			for _, alloc := range job.Allocations {
				printAllocationPlan(alloc)
			}
			continue
		}

		reason := job.SkipReason
		if reason == "" {
			reason = "no rollout needed"
		}
		fmt.Printf("  job %q: skip (%s)\n", job.Job, reason)
	}
}

func printAllocationPlan(alloc AllocationPlan) {
	switch alloc.Action {
	case rolloutActionStart:
		if alloc.CurrentHash != "" {
			fmt.Printf("    %s  start   current_hash=%s\n", alloc.WorkerIP, alloc.CurrentHash)
		} else {
			fmt.Printf("    %s  start   (no hash row yet)\n", alloc.WorkerIP)
		}
	case rolloutActionRestart:
		fmt.Printf(
			"    %s  restart previous_hash=%s current_hash=%s\n",
			alloc.WorkerIP, alloc.PreviousHash, alloc.CurrentHash,
		)
	case rolloutActionSkip:
		fmt.Printf("    %s  skip    hash=%s\n", alloc.WorkerIP, alloc.CurrentHash)
	default:
		fmt.Printf("    %s  %s\n", alloc.WorkerIP, alloc.Action)
	}
}
