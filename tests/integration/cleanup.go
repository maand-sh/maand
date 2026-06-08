// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

//go:build integration

package integration

import (
	"log"
	"os"

	"maand/bucket"
)

func cleanupIntegrationTestArtifacts() {
	removeLocalIntegrationTestBucket()
}

func removeLocalIntegrationTestBucket() {
	if err := os.RemoveAll(bucket.Location); err != nil && !os.IsNotExist(err) {
		log.Printf("integration cleanup: remove local test bucket: %v", err)
	}
}
