// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

import (
	"crypto/md5"

	"github.com/google/uuid"
)

// StableJobID derives a deterministic UUID (v3-style) from arbitrary input.
func StableJobID(value string) string {
	hash := md5.Sum([]byte(value))
	hash[6] = (hash[6] & 0x0f) | 0x30 // Version 3
	hash[8] = (hash[8] & 0x3f) | 0x80 // Variant
	return uuid.UUID(hash).String()
}

// GetHashUUID is deprecated; use StableJobID.
func GetHashUUID(value string) string {
	return StableJobID(value)
}
