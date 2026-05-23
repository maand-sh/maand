// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

import "errors"

var (
	// ErrNotFound is returned when a key is missing or marked deleted.
	ErrNotFound = errors.New("key not found or deleted")
	// ErrNamespaceNotFound is returned when Delete targets an unknown namespace.
	ErrNamespaceNotFound = errors.New("namespace not found")
	// ErrStoreNotInitialized is returned when the session store is nil.
	ErrStoreNotInitialized = errors.New("kv store not initialized")
)

// ErrValueNotFound is deprecated; use ErrNotFound.
var ErrValueNotFound = ErrNotFound
