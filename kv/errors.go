// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package kv

import (
	"errors"
	"fmt"
)

var ErrNotFound = errors.New("key not found")

func NewNotFoundError(namespace, key string) error {
	return fmt.Errorf("%w: namespace %s, key %s", ErrNotFound, namespace, key)
}
