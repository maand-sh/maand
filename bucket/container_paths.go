// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import (
	"path/filepath"

	"github.com/google/uuid"
)

// HostTmpDir is the host path to bucket/tmp.
func HostTmpDir() string {
	root, err := filepath.Abs(Location)
	if err != nil {
		root = Location
	}
	return filepath.Join(root, "tmp")
}

func newUniqueName() string {
	return uuid.NewString()
}
