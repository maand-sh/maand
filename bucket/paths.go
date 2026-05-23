// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import (
	"fmt"
	"path/filepath"
	"strings"
)

// BucketPath returns the absolute host path for a file under the bucket root.
func BucketPath(hostPath string) (string, error) {
	absHost, err := filepath.Abs(hostPath)
	if err != nil {
		return "", UnexpectedError(err)
	}

	absBucket, err := filepath.Abs(Location)
	if err != nil {
		return "", UnexpectedError(err)
	}

	underBucket, err := isPathUnderBucketRoot(absHost, absBucket)
	if err != nil {
		return "", UnexpectedError(err)
	}
	if !underBucket {
		return "", UnexpectedError(fmt.Errorf("path %s is outside bucket root", hostPath))
	}

	return absHost, nil
}

func isPathUnderBucketRoot(absHostPath, absBucketPath string) (bool, error) {
	rel, err := filepath.Rel(absBucketPath, absHostPath)
	if err != nil {
		return false, err
	}
	return !strings.HasPrefix(rel, ".."), nil
}

// ContainerPath is an alias for BucketPath (legacy name).
func ContainerPath(hostPath string) (string, error) {
	return BucketPath(hostPath)
}
