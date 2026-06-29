// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

// HealthFailedPreviousHash is stored in previous_hash when an allocation was marked
// for redeploy after a health-check failure (legacy; no longer set by maand health_check).
const HealthFailedPreviousHash = "__maand_health_failed__"
