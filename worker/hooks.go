// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package worker

import "maand/bucket"

// TestHooks overrides worker side effects during tests. Clear with ClearTestHooks when done.
type TestHooks struct {
	ExecuteCommand func(rt *bucket.Runtime, workerIP string, commands []string, env []string) error
}

var testHooks *TestHooks

// SetTestHooks installs worker test doubles. Not for production use.
func SetTestHooks(h *TestHooks) {
	testHooks = h
}

// ClearTestHooks removes worker test doubles.
func ClearTestHooks() {
	testHooks = nil
}
