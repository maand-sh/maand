// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package tests

import (
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
)

func TestBuildRejectsJobMemoryAboveWorkerCapacity(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1","memory":"128mb"}]`)

	writeMinimalJob(t, "heavy", `{
		"selectors": ["worker"],
		"resources": {
			"memory": {"min": "256mb", "max": "512mb"}
		}
	}`)

	err := executeBuildErr(t)
	assert.ErrorIs(t, err, bucket.ErrInsufficientResource)
}

func TestBuildRejectsDuplicateWorkerHost(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"},{"host":"10.0.0.1"}]`)

	err := executeBuildErr(t)
	assert.ErrorIs(t, err, bucket.ErrInvalidWorkerJSON)
}

func TestBuildRejectsJobMemoryWhenWorkerUnconfigured(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)

	writeMinimalJob(t, "heavy", `{
		"selectors": ["worker"],
		"resources": {
			"memory": {"min": "256mb", "max": "512mb"}
		}
	}`)

	err := executeBuildErr(t)
	assert.ErrorIs(t, err, bucket.ErrInsufficientResource)
	assert.Contains(t, err.Error(), "must specify memory")
}

func TestBuildRejectsJobBelowMinAllocationsCount(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"}]`)

	writeMinimalJob(t, "api", `{
		"selectors": ["worker"],
		"min_allocations_count": 2
	}`)

	err := executeBuildErr(t)
	assert.ErrorIs(t, err, bucket.ErrInsufficientAllocations)
	assert.Contains(t, err.Error(), "job api has 1 allocation(s)")
}

func TestBuildAcceptsJobAtMinAllocationsCount(t *testing.T) {
	initFreshBucket(t)
	writeWorkersJSON(t, `[{"host":"10.0.0.1"},{"host":"10.0.0.2"}]`)

	writeMinimalJob(t, "api", `{
		"selectors": ["worker"],
		"min_allocations_count": 2
	}`)

	err := executeBuildErr(t)
	assert.NoError(t, err)
}
