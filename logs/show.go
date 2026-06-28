// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package logs

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"maand/bucket"
)

const (
	FormatRaw   = "raw"
	FormatHuman = "human"
)

// ShowOptions filters structured log lines.
type ShowOptions struct {
	Worker string
	RunID  string
	Job    string
	Phase  string
	Event  string
	Tail   int
	RunDir bool
	Format string
}

// Show prints matching log lines from bucket logs.
func Show(opts ShowOptions) error {
	paths, err := logPaths(opts)
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no log files matched")
	}

	switch strings.ToLower(strings.TrimSpace(opts.Format)) {
	case FormatHuman, "h":
		return showHuman(paths, opts)
	default:
		return showRaw(paths, opts)
	}
}

func showRaw(paths []string, opts ShowOptions) error {
	var lines []string
	for _, path := range paths {
		fileLines, err := readMatchingLines(path, opts)
		if err != nil {
			return err
		}
		lines = append(lines, fileLines...)
	}

	if opts.Tail > 0 && len(lines) > opts.Tail {
		lines = lines[len(lines)-opts.Tail:]
	}

	for _, line := range lines {
		fmt.Println(line)
	}
	return nil
}

func showHuman(paths []string, opts ShowOptions) error {
	var allBlocks []CommandBlock
	var allEvents []Entry
	for _, path := range paths {
		fileEntries, err := readAllEntries(path)
		if err != nil {
			return err
		}
		blocks, events := groupEntries(fileEntries)
		for _, block := range blocks {
			if blockMatchesFilters(block, opts) {
				allBlocks = append(allBlocks, block)
			}
		}
		for _, event := range events {
			if entryMatchesFilters(event, opts) {
				allEvents = append(allEvents, event)
			}
		}
	}

	sort.SliceStable(allBlocks, func(i, j int) bool {
		return field(allBlocks[i].Begin, "ts") < field(allBlocks[j].Begin, "ts")
	})
	sort.SliceStable(allEvents, func(i, j int) bool {
		return field(allEvents[i], "ts") < field(allEvents[j], "ts")
	})

	if opts.Tail > 0 {
		if len(allBlocks) > opts.Tail {
			allBlocks = allBlocks[len(allBlocks)-opts.Tail:]
		}
		if len(allEvents) > opts.Tail {
			allEvents = allEvents[len(allEvents)-opts.Tail:]
		}
	}

	out := RenderHuman(allBlocks, allEvents)
	if out != "" {
		fmt.Println(out)
	}
	return nil
}

func listLogFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".log") {
			continue
		}
		paths = append(paths, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(paths)
	return paths, nil
}

func logPaths(opts ShowOptions) ([]string, error) {
	if opts.RunDir && opts.RunID != "" {
		runDir := filepath.Join(bucket.RunLogLocation, opts.RunID)
		if opts.Worker != "" {
			return []string{filepath.Join(runDir, opts.Worker+".log")}, nil
		}
		return listLogFiles(runDir)
	}

	if opts.RunID != "" {
		runDir := filepath.Join(bucket.RunLogLocation, opts.RunID)
		if opts.Worker != "" {
			return []string{filepath.Join(runDir, opts.Worker+".log")}, nil
		}
		return listLogFiles(runDir)
	}

	if opts.Worker != "" {
		return []string{filepath.Join(bucket.LogLocation, opts.Worker+".log")}, nil
	}
	return listLogFiles(bucket.LogLocation)
}

func readAllEntries(path string) ([]Entry, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	entries := make([]Entry, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		entries = append(entries, ParseLine(line))
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func readMatchingLines(path string, opts ShowOptions) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer func() {
		_ = file.Close()
	}()

	lines := make([]string, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if matchesLine(line, opts) {
			lines = append(lines, line)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return lines, nil
}

func matchesLine(line string, opts ShowOptions) bool {
	entry := ParseLine(line)
	if field(entry, "stream") != "" && field(entry, "event") == "" && field(entry, "ts") == "" {
		return payloadLineMatchesOpenContext(line, opts)
	}
	return entryMatchesFilters(entry, opts)
}

func payloadLineMatchesOpenContext(line string, opts ShowOptions) bool {
	if opts.Event != "" {
		return false
	}
	if opts.RunID != "" || opts.Worker != "" || opts.Job != "" || opts.Phase != "" {
		return false
	}
	_ = line
	return true
}

func containsField(line, key, want string) bool {
	entry := ParseLine(line)
	return fieldMatches(field(entry, key), want)
}

// TailRunFile returns the last n lines from a per-run worker log file.
func TailRunFile(runID, workerIP string, n int, w io.Writer) error {
	path := filepath.Join(bucket.RunLogLocation, runID, workerIP+".log")
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	lines := make([]string, 0)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if n > 0 && len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for _, line := range lines {
		if _, err := io.WriteString(w, line+"\n"); err != nil {
			return err
		}
	}
	return nil
}
