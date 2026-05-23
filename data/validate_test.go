// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkerSyncError(t *testing.T) {
	err := &WorkerSyncError{WorkerIP: "10.0.0.1", Reason: "update_seq mismatch"}
	assert.Contains(t, err.Error(), "10.0.0.1")
	assert.Contains(t, err.Error(), "maand deploy")
}
