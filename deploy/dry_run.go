// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"maand/bucket"
	"maand/data"
	"maand/kv"
)

const (
	rolloutActionStart   = "start"
	rolloutActionRestart = "restart"
	rolloutActionReload  = "reload"
	rolloutActionSync    = "sync"
	rolloutActionSkip    = "skip"
	rolloutActionStop    = "stop"
	rolloutActionPromote = "promote"
	rolloutActionStopPromote = "stop+promote"
)

// AllocationPlan describes one worker allocation after plan hashes are refreshed.
type AllocationPlan struct {
	WorkerIP     string
	Action       string
	PreviousHash string
	CurrentHash  string
	MatchedPaths []string
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
func DryRun(jobsFilter []string, opts Options) (DryRunResult, error) {
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
			plan, err := planJobRollout(tx, job, deploymentSeq, opts)
			if err != nil {
				return result, err
			}
			result.Jobs = append(result.Jobs, plan)
			if plan.NeedsRollout {
				result.Required = true
				if opts.SyncOnly {
					if err := validateSyncOnlyRollout(tx, job); err != nil {
						return result, err
					}
				}
			}
		}
	}

	return result, nil
}

func planJobRollout(tx *sql.Tx, job string, deploymentSeq int, opts Options) (JobPlan, error) {
	plan := JobPlan{
		Job:           job,
		DeploymentSeq: deploymentSeq,
	}

	workers, err := data.GetNonRemovedAllocationsOrdered(tx, job)
	if err != nil {
		return plan, err
	}
	if len(workers) == 0 {
		plan.SkipReason = "no allocations"
		return plan, nil
	}

	policy, err := data.GetRestartPolicy(tx, job)
	if err != nil {
		return plan, err
	}
	globs, err := data.GetRestartGlobs(tx, job)
	if err != nil {
		return plan, err
	}

	namespace := fmt.Sprintf("%s_allocation", job)
	for _, workerIP := range workers {
		disabled, err := data.IsAllocationDisabled(tx, workerIP, job)
		if err != nil {
			return plan, err
		}
		if disabled == 1 {
			ap, needs, err := planDisabledAllocation(tx, job, workerIP, namespace, opts)
			if err != nil {
				return plan, err
			}
			if needs {
				plan.NeedsRollout = true
			}
			plan.Allocations = append(plan.Allocations, ap)
			continue
		}

		ap, needs, err := planActiveAllocation(tx, job, workerIP, namespace, opts, policy, globs)
		if err != nil {
			return plan, err
		}
		if needs {
			plan.NeedsRollout = true
		}
		plan.Allocations = append(plan.Allocations, ap)
	}

	if !plan.NeedsRollout {
		plan.SkipReason = "already promoted on all allocations"
	}
	return plan, nil
}

func planActiveAllocation(
	tx *sql.Tx,
	job, workerIP, namespace string,
	opts Options,
	policy string,
	globs []string,
) (AllocationPlan, bool, error) {
	allocID, err := data.GetAllocationID(tx, workerIP, job)
	if err != nil {
		return AllocationPlan{}, false, err
	}

	current, previous, ok, err := data.GetAllocationHash(tx, namespace, allocID)
	if err != nil {
		return AllocationPlan{}, false, err
	}

	ap := AllocationPlan{WorkerIP: workerIP}
	switch {
	case !ok:
		ap.Action = rolloutActionStart
		return ap, true, nil
	case previous == "":
		ap.Action = rolloutActionStart
		ap.CurrentHash = current
		return ap, true, nil
	case previous != current:
		action, matched, err := resolveAllocationLifecycle(tx, job, workerIP, opts, policy, globs, true, false)
		if err != nil {
			return AllocationPlan{}, false, err
		}
		ap.Action = action
		ap.MatchedPaths = matched
		ap.PreviousHash = previous
		ap.CurrentHash = current
		return ap, true, nil
	case opts.Force:
		action, matched, err := resolveAllocationLifecycle(tx, job, workerIP, opts, policy, globs, false, false)
		if err != nil {
			return AllocationPlan{}, false, err
		}
		ap.Action = action
		ap.MatchedPaths = matched
		ap.PreviousHash = previous
		ap.CurrentHash = current
		return ap, true, nil
	default:
		needsVersion, err := data.AllocationNeedsVersionRollout(tx, job, workerIP)
		if err != nil {
			return AllocationPlan{}, false, err
		}
		if needsVersion {
			action, matched, err := resolveAllocationLifecycle(tx, job, workerIP, opts, policy, globs, false, true)
			if err != nil {
				return AllocationPlan{}, false, err
			}
			ap.Action = action
			ap.MatchedPaths = matched
			ap.PreviousHash = previous
			ap.CurrentHash = current
			return ap, true, nil
		}
		ap.Action = rolloutActionSkip
		ap.PreviousHash = previous
		ap.CurrentHash = current
		return ap, false, nil
	}
}

func planDisabledAllocation(
	tx *sql.Tx,
	job, workerIP, namespace string,
	opts Options,
) (AllocationPlan, bool, error) {
	allocID, err := data.GetAllocationID(tx, workerIP, job)
	if err != nil {
		return AllocationPlan{}, false, err
	}

	current, previous, ok, err := data.GetAllocationHash(tx, namespace, allocID)
	if err != nil {
		return AllocationPlan{}, false, err
	}

	ap := AllocationPlan{
		WorkerIP:     workerIP,
		PreviousHash: previous,
		CurrentHash:  current,
	}

	wasDeployed := ok && previous != ""
	needsVersion, err := data.AllocationNeedsVersionRollout(tx, job, workerIP)
	if err != nil {
		return AllocationPlan{}, false, err
	}
	hashMismatch := wasDeployed && previous != current
	needsPromote := hashMismatch || needsVersion || (opts.Force && wasDeployed)

	needs := false
	if wasDeployed {
		ap.Action = rolloutActionStop
		needs = true
	}
	if needsPromote {
		if ap.Action == rolloutActionStop {
			ap.Action = rolloutActionStopPromote
		} else {
			ap.Action = rolloutActionPromote
		}
		needs = true
	}
	if ap.Action == "" {
		ap.Action = rolloutActionSkip
	}
	return ap, needs, nil
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
		if len(alloc.MatchedPaths) > 0 {
			fmt.Printf(
				"    %s  restart previous_hash=%s current_hash=%s matched=%s\n",
				alloc.WorkerIP, alloc.PreviousHash, alloc.CurrentHash, strings.Join(alloc.MatchedPaths, ","),
			)
			break
		}
		fmt.Printf(
			"    %s  restart previous_hash=%s current_hash=%s\n",
			alloc.WorkerIP, alloc.PreviousHash, alloc.CurrentHash,
		)
	case rolloutActionReload:
		fmt.Printf(
			"    %s  reload  previous_hash=%s current_hash=%s\n",
			alloc.WorkerIP, alloc.PreviousHash, alloc.CurrentHash,
		)
	case rolloutActionSync:
		fmt.Printf(
			"    %s  sync    previous_hash=%s current_hash=%s\n",
			alloc.WorkerIP, alloc.PreviousHash, alloc.CurrentHash,
		)
	case rolloutActionSkip:
		fmt.Printf("    %s  skip    hash=%s\n", alloc.WorkerIP, alloc.CurrentHash)
	case rolloutActionStop:
		fmt.Printf("    %s  stop    previous_hash=%s\n", alloc.WorkerIP, alloc.PreviousHash)
	case rolloutActionPromote:
		fmt.Printf(
			"    %s  promote previous_hash=%s current_hash=%s\n",
			alloc.WorkerIP, alloc.PreviousHash, alloc.CurrentHash,
		)
	case rolloutActionStopPromote:
		fmt.Printf(
			"    %s  stop+promote previous_hash=%s current_hash=%s\n",
			alloc.WorkerIP, alloc.PreviousHash, alloc.CurrentHash,
		)
	default:
		fmt.Printf("    %s  %s\n", alloc.WorkerIP, alloc.Action)
	}
}
