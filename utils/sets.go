// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package utils

// Unique returns deduplicated strings preserving arbitrary order.
func Unique(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

// Union returns elements present in either slice.
func Union(a, b []string) []string {
	return Unique(append(append([]string{}, a...), b...))
}

// Intersection returns elements present in both slices.
func Intersection(a, b []string) []string {
	if len(a) == 0 || len(b) == 0 {
		return nil
	}
	index := make(map[string]struct{}, len(a))
	for _, value := range a {
		index[value] = struct{}{}
	}
	matches := make([]string, 0)
	for _, value := range b {
		if _, ok := index[value]; ok {
			matches = append(matches, value)
		}
	}
	return matches
}

// Difference returns elements in a that are not in b.
func Difference(a, b []string) []string {
	if len(a) == 0 {
		return nil
	}
	exclude := make(map[string]struct{}, len(b))
	for _, value := range b {
		exclude[value] = struct{}{}
	}
	diff := make([]string, 0)
	for _, value := range a {
		if _, ok := exclude[value]; !ok {
			diff = append(diff, value)
		}
	}
	return diff
}
