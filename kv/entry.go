// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

// Entry is one logical key in a namespace (latest version in memory).
type Entry struct {
	Value            string
	Version          int
	TTL              int
	Deleted          int
	Changed          bool
	LastModifiedTime int64
}

// KeyValueItem is deprecated; use Entry.
type KeyValueItem = Entry
