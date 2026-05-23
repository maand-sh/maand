// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"fmt"
	"sort"

	"maand/bucket"
	"maand/data"
	"maand/workspace"
)

type portAllocator struct {
	range_      bucket.PortRange
	existing    data.JobPortAssignments
	usedNumbers map[int]struct{}
}

func newPortAllocator(existing data.JobPortAssignments, portRange bucket.PortRange) *portAllocator {
	used := make(map[int]struct{})
	for _, ports := range existing {
		for _, port := range ports {
			used[port] = struct{}{}
		}
	}
	return &portAllocator{
		range_:      portRange,
		existing:    existing,
		usedNumbers: used,
	}
}

func (a *portAllocator) assign(jobName string, portName string) (int, error) {
	if err := workspace.ValidatePortKey(portName); err != nil {
		return 0, err
	}

	if prev, ok := a.existing[jobName][portName]; ok {
		if prev < a.range_.Min || prev > a.range_.Max {
			return 0, fmt.Errorf("%w: job %s port %q assigned %d outside range %d-%d",
				bucket.ErrInvalidPortRange, jobName, portName, prev, a.range_.Min, a.range_.Max)
		}
		return prev, nil
	}

	for port := a.range_.Min; port <= a.range_.Max; port++ {
		if _, taken := a.usedNumbers[port]; taken {
			continue
		}
		a.usedNumbers[port] = struct{}{}
		if a.existing[jobName] == nil {
			a.existing[jobName] = make(map[string]int)
		}
		a.existing[jobName][portName] = port
		return port, nil
	}

	return 0, fmt.Errorf("%w: no free ports in range %d-%d", bucket.ErrPortRangeExhausted, a.range_.Min, a.range_.Max)
}

func (a *portAllocator) releaseRemoved(jobName string, keepNames []string) {
	prev := a.existing[jobName]
	if len(prev) == 0 {
		return
	}
	keep := make(map[string]struct{}, len(keepNames))
	for _, name := range keepNames {
		keep[name] = struct{}{}
	}
	for name, port := range prev {
		if _, still := keep[name]; still {
			continue
		}
		delete(a.usedNumbers, port)
		delete(prev, name)
	}
}

func syncJobPorts(
	tx *sql.Tx,
	jobID, jobName string,
	portNames []string,
	allocator *portAllocator,
) error {
	allocator.releaseRemoved(jobName, portNames)

	_, err := tx.Exec("DELETE FROM job_ports WHERE job_id = ?", jobID)
	if err != nil {
		return bucket.DatabaseError(err)
	}

	for _, portName := range portNames {
		port, err := allocator.assign(jobName, portName)
		if err != nil {
			return err
		}

		_, err = tx.Exec(
			"INSERT INTO job_ports (job_id, name, port) VALUES (?, ?, ?)",
			jobID, portName, port,
		)
		if err != nil {
			return bucket.DatabaseError(err)
		}
	}

	return nil
}

func buildPortAllocator(tx *sql.Tx, workspaceJobNames []string, jobWorkspace *workspace.DefaultWorkspace) (*portAllocator, error) {
	needsPorts := false
	for _, jobName := range workspaceJobNames {
		manifest, err := jobWorkspace.GetJobManifest(jobName)
		if err != nil {
			return nil, err
		}
		if len(manifest.Resources.Ports) > 0 {
			needsPorts = true
			break
		}
	}

	portRange, err := bucket.LoadPortRange()
	if err != nil {
		return nil, err
	}
	if needsPorts {
		if err := portRange.Validate(); err != nil {
			return nil, err
		}
	}

	existing, err := data.GetAllJobPortAssignments(tx)
	if err != nil {
		return nil, err
	}

	return newPortAllocator(existing, portRange), nil
}

func sortedJobNames(names []string) []string {
	out := append([]string(nil), names...)
	sort.Strings(out)
	return out
}
