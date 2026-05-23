// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package utils

import "errors"

// SliceRange returns original[start:end] with bounds validation.
func SliceRange(items []string, start, end int) ([]string, error) {
	length := len(items)
	if start < 0 || end < 0 {
		return nil, errors.New("indices cannot be negative")
	}
	if start > length || end > length {
		return nil, errors.New("index out of range")
	}
	if start > end {
		return nil, errors.New("start index cannot be greater than end index")
	}
	return items[start:end], nil
}

// NewBatchIterator returns a function that yields successive batches from items.
// batchSize defaults to 1 when zero or negative.
func NewBatchIterator(items []string, batchSize int) func() ([]string, bool) {
	if batchSize <= 0 {
		batchSize = 1
	}
	index := 0
	return func() ([]string, bool) {
		if index >= len(items) {
			return nil, false
		}
		end := index + batchSize
		if end > len(items) {
			end = len(items)
		}
		batch := items[index:end]
		index = end
		return batch, true
	}
}

// GetStringSubList is deprecated; use SliceRange.
func GetStringSubList(originalList []string, startIndex, endIndex int) ([]string, error) {
	return SliceRange(originalList, startIndex, endIndex)
}

// NewStringIterator is deprecated; use NewBatchIterator.
func NewStringIterator(list []string, batchSize int) func() ([]string, bool) {
	return NewBatchIterator(list, batchSize)
}
