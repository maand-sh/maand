// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

import "fmt"

// DeployOrderKey is the KV key for rollout worker order under JobCatalogNamespace.
const DeployOrderKey = "deploy_order"

// JobCatalogNamespace returns maand/job/<job> (build-synced catalog metadata).
func JobCatalogNamespace(job string) string {
	return fmt.Sprintf("maand/job/%s", job)
}
