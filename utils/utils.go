package utils

import (
	"errors"
	"github.com/jedib0t/go-pretty/v6/table"
	"os"
	"regexp"
	"strconv"
	"strings"
)

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func ExtractSizeInMB(sizeString string) (float64, error) {
	unitToMB := map[string]float64{
		"MB": 1,
		"GB": 1024,
		"TB": 1024 * 1024,
	}

	trimmed := strings.TrimSpace(sizeString)
	if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return strconv.ParseFloat(trimmed, 64)
	}

	re := regexp.MustCompile(`([\d.]+)\s*([a-zA-Z]*)`)
	matches := re.FindStringSubmatch(sizeString)
	if matches == nil {
		return 0, errors.New("invalid size input: " + sizeString)
	}

	size, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, err
	}

	unit := strings.ToUpper(matches[2])
	if unit == "" {
		unit = "MB"
	}

	if multiplier, found := unitToMB[unit]; found {
		return size * multiplier, nil
	}
	return 0, errors.New("unit smaller than MB or invalid: " + unit)
}

func ExtractCPUFrequencyInMHz(freqString string) (float64, error) {
	unitToMHz := map[string]float64{
		"MHZ": 1,
		"GHZ": 1000,
		"THZ": 1000000,
	}

	// Trim spaces and check if it's a valid number (without unit)
	trimmed := strings.TrimSpace(freqString)
	if _, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return strconv.ParseFloat(trimmed, 64)
	}

	// Match the frequency string like '3.2 GHz'
	re := regexp.MustCompile(`([\d.]+)\s*([a-zA-Z]+)`)
	matches := re.FindStringSubmatch(freqString)
	if matches == nil {
		return 0, errors.New("invalid frequency string format: " + freqString)
	}

	frequency, err := strconv.ParseFloat(matches[1], 64)
	if err != nil {
		return 0, err
	}

	unit := strings.ToUpper(matches[2])
	if multiplier, found := unitToMHz[unit]; found {
		return frequency * multiplier, nil
	}
	return 0, errors.New("unsupported or invalid unit: " + unit + " (unit must be MHz or larger)")
}

func GetTable(header table.Row) table.Writer {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(header)
	t.SetStyle(table.StyleRounded)
	return t
}
