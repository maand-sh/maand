package utils

import "errors"

// GetSubList extracts a sub-slice of strings from an original slice.
// It takes the original slice, a start index (inclusive), and an end index (exclusive).
// It returns the sub-slice and an error if the indices are out of bounds or invalid.
func GetStringSubList(originalList []string, startIndex, endIndex int) ([]string, error) {
	// Get the length of the original list
	listLength := len(originalList)

	// Validate the start index
	if startIndex < 0 {
		return nil, errors.New("start index cannot be negative")
	}
	if startIndex > listLength {
		return nil, errors.New("start index cannot be greater than the list length")
	}

	// Validate the end index
	if endIndex < 0 {
		return nil, errors.New("end index cannot be negative")
	}
	if endIndex > listLength {
		return nil, errors.New("end index cannot be greater than the list length")
	}

	// Ensure start index is not greater than end index
	if startIndex > endIndex {
		return nil, errors.New("start index cannot be greater than end index")
	}

	// If start and end are the same, return an empty slice
	if startIndex == endIndex {
		return []string{}, nil
	}

	// Return the sub-slice using Go's slicing syntax
	return originalList[startIndex:endIndex], nil
}

// NewStringIterator creates and returns an iterator function for a given string slice.
// It allows specifying a batchSize to read multiple elements at a time.
// The returned function, when called, yields a batch of strings (a slice) and a boolean
// indicating if there are more elements (or batches) to read.
// If batchSize is 0 or negative, it defaults to 1.
func NewStringIterator(list []string, batchSize int) func() ([]string, bool) {
	// Ensure batchSize is at least 1
	if batchSize <= 0 {
		batchSize = 1
	}

	// Initialize an internal counter for the iterator's state
	index := 0
	listLength := len(list)

	// Return a closure that acts as the iterator
	return func() ([]string, bool) {
		// Check if there are more elements to iterate over
		if index < listLength {
			// Calculate the end of the current batch
			batchEnd := index + batchSize
			if batchEnd > listLength {
				batchEnd = listLength // Adjust batchEnd if it exceeds list length
			}

			// Extract the current batch
			batch := list[index:batchEnd]
			// Increment the index for the next call
			index = batchEnd
			// Return the batch and true (indicating more elements/batches)
			return batch, true
		}
		// If no more elements, return an empty slice and false
		return nil, false // Return nil slice for no more data
	}
}
