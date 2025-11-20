// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package data

import (
	"errors"
	"fmt"
)

var (
	ErrNotInitialized = errors.New("maand is not initialized")
	ErrDatabase       = errors.New("database error")
)

func NewDatabaseError(err error) error {
	return fmt.Errorf("%w: %w", ErrDatabase, err)
}
