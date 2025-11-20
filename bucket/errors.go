// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

// Package bucket provides interfaces to work with bucket
package bucket

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidMaandConf  = errors.New("invalid maand.conf")
	ErrInvalidBucketConf = errors.New("invalid bucket.conf")
	ErrUnexpectedError   = errors.New("unexpected error")
)

func NewUnexpectedError(err error) error {
	return fmt.Errorf("%w: %w", ErrUnexpectedError, err)
}
