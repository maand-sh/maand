// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"database/sql"
	"fmt"
	"strings"

	"maand/data"
	"maand/utils/pathmatch"
	"maand/workspace"
)

func effectiveUpdateAction(syncOnly bool, policy string) string {
	action, _ := resolveUpdateAction(syncOnly, policy, nil, nil, nil, true, false, false)
	return action
}

func resolveUpdateAction(
	syncOnly bool,
	policy string,
	globs []string,
	previousFiles, currentFiles data.FileManifest,
	hashChanged, versionOnly, legacyNoManifest bool,
) (string, []string) {
	if syncOnly {
		return rolloutActionSync, nil
	}
	switch policy {
	case workspace.RestartPolicyNever:
		return rolloutActionSync, nil
	case workspace.RestartPolicyAlways:
		return rolloutActionRestart, nil
	case workspace.RestartPolicyReload:
		if versionOnly && !hashChanged {
			return rolloutActionReload, nil
		}
		if legacyNoManifest && hashChanged {
			return rolloutActionReload, nil
		}
		if len(globs) == 0 {
			return rolloutActionReload, nil
		}
		changed := data.DiffFileManifests(previousFiles, currentFiles)
		matched := pathmatch.MatchAny(globs, changed)
		if len(matched) > 0 {
			return rolloutActionRestart, matched
		}
		return rolloutActionReload, nil
	default:
		return rolloutActionRestart, nil
	}
}

func validateSyncOnlyRollout(tx *sql.Tx, job string) error {
	newAllocations, err := data.GetNewAllocations(tx, job)
	if err != nil {
		return err
	}
	if len(newAllocations) == 0 {
		return nil
	}
	return fmt.Errorf(
		"sync-only deploy cannot start new allocations on job %q: %s",
		job, strings.Join(newAllocations, ","),
	)
}

func resolveAllocationLifecycle(
	tx *sql.Tx,
	job, workerIP string,
	opts Options,
	policy string,
	globs []string,
	hashChanged, versionOnly bool,
) (string, []string, error) {
	allocID, err := data.GetAllocationID(tx, workerIP, job)
	if err != nil {
		return "", nil, err
	}
	namespace := fmt.Sprintf("%s_allocation", job)
	manifests, err := data.GetAllocationFileManifests(tx, namespace, allocID)
	if err != nil {
		return "", nil, err
	}
	action, matched := resolveUpdateAction(
		opts.SyncOnly, policy, globs,
		manifests.Previous, manifests.Current,
		hashChanged, versionOnly, !manifests.HasPreviousFiles,
	)
	return action, matched, nil
}
