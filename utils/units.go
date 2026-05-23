// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package utils

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	memorySizePattern = regexp.MustCompile(`(?i)^([\d.]+)\s*([a-z]*)$`)
	cpuFreqPattern    = regexp.MustCompile(`(?i)^([\d.]+)\s*([a-z]+)$`)
)

var memoryUnitToMB = map[string]float64{
	"MB": 1,
	"GB": 1024,
	"TB": 1024 * 1024,
}

var cpuUnitToMHz = map[string]float64{
	"MHZ": 1,
	"GHZ": 1000,
	"THZ": 1000000,
}

// ParseMemoryMB parses strings like "512MB", "1 GB", or plain megabyte numbers.
func ParseMemoryMB(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}

	if value, err := strconv.ParseFloat(raw, 64); err == nil {
		return value, nil
	}

	matches := memorySizePattern.FindStringSubmatch(raw)
	if matches == nil {
		return 0, fmt.Errorf("invalid memory size %q", raw)
	}

	size, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, err
	}

	unit := strings.ToUpper(matches[2])
	if unit == "" {
		unit = "MB"
	}

	multiplier, ok := memoryUnitToMB[unit]
	if !ok {
		return 0, fmt.Errorf("unsupported memory unit %q", unit)
	}
	return size * multiplier, nil
}

// ParseCPUMHz parses strings like "2400MHz", "3.2 GHz", or plain MHz numbers.
func ParseCPUMHz(raw string) (float64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}

	if value, err := strconv.ParseFloat(raw, 64); err == nil {
		return value, nil
	}

	matches := cpuFreqPattern.FindStringSubmatch(raw)
	if matches == nil {
		return 0, fmt.Errorf("invalid cpu frequency %q", raw)
	}

	frequency, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, err
	}

	unit := strings.ToUpper(matches[2])
	multiplier, ok := cpuUnitToMHz[unit]
	if !ok {
		return 0, fmt.Errorf("unsupported cpu unit %q", unit)
	}
	return frequency * multiplier, nil
}

// ExtractSizeInMB is deprecated; use ParseMemoryMB.
func ExtractSizeInMB(sizeString string) (float64, error) {
	return ParseMemoryMB(sizeString)
}

// ExtractCPUFrequencyInMHz is deprecated; use ParseCPUMHz.
func ExtractCPUFrequencyInMHz(freqString string) (float64, error) {
	return ParseCPUMHz(freqString)
}

// ErrInvalidUnit is returned for unparseable resource strings.
var ErrInvalidUnit = errors.New("invalid unit")
