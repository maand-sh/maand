// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package logs

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const humanTimeLayout = "2006-01-02 15:04:05"

// CommandBlock groups one command invocation for human display.
type CommandBlock struct {
	Begin   Entry
	Streams []Entry
	End     *Entry
}

// RenderHuman formats parsed log entries for terminal reading.
func RenderHuman(blocks []CommandBlock, events []Entry) string {
	var b strings.Builder
	for _, event := range events {
		writeHumanEvent(&b, event)
	}
	for _, block := range blocks {
		writeHumanBlock(&b, block)
	}
	return strings.TrimRight(b.String(), "\n")
}

func writeHumanEvent(b *strings.Builder, entry Entry) {
	header := humanHeader(entry)
	detail := humanEventDetail(entry)
	if header == "" && detail == "" {
		return
	}
	if header != "" {
		b.WriteString(header)
		b.WriteByte('\n')
	}
	if detail != "" {
		b.WriteString("  ")
		b.WriteString(detail)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
}

func writeHumanBlock(b *strings.Builder, block CommandBlock) {
	header := humanHeader(block.Begin)
	if header != "" {
		b.WriteString(header)
		b.WriteByte('\n')
	}
	if sub := humanCommandSubheader(block.Begin); sub != "" {
		b.WriteString("  ")
		b.WriteString(sub)
		b.WriteByte('\n')
	}
	if cmd := field(block.Begin, "cmd"); cmd != "" {
		b.WriteString("  $ ")
		b.WriteString(cmd)
		b.WriteByte('\n')
	}
	for _, stream := range block.Streams {
		msg := field(stream, "msg")
		if msg == "" {
			continue
		}
		delim := "| "
		if field(stream, "stream") == "stderr" {
			delim = "! "
		}
		b.WriteString("  ")
		if target := humanStreamTarget(stream, block.Begin); target != "" {
			b.WriteString(target)
			b.WriteByte(' ')
		}
		b.WriteString(delim)
		b.WriteString(msg)
		b.WriteByte('\n')
	}
	if block.End != nil {
		b.WriteString("  ")
		b.WriteString(humanCommandFooter(*block.End))
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
}

func humanHeader(entry Entry) string {
	ts := formatHumanTime(field(entry, "ts"))
	parts := make([]string, 0, 4)
	if ts != "" {
		parts = append(parts, ts)
	}
	if maand := field(entry, "maand"); maand != "" {
		parts = append(parts, maand)
	}
	if seq := field(entry, "seq"); seq != "" {
		parts = append(parts, "seq="+seq)
	}
	if run := field(entry, "run"); run != "" {
		parts = append(parts, "run="+shortRunID(run))
	}
	return strings.Join(parts, "  ")
}

func humanCommandSubheader(entry Entry) string {
	phase := field(entry, "phase")
	action := field(entry, "action")
	target := humanTarget(entry)
	parts := make([]string, 0, 3)
	if phase != "" {
		parts = append(parts, phase)
	}
	if action != "" {
		parts = append(parts, action)
	}
	if target != "" {
		parts = append(parts, target)
	}
	return strings.Join(parts, "  ")
}

func humanTarget(entry Entry) string {
	return cliStreamTargetFromFields(field(entry, "worker"), field(entry, "job"))
}

func humanStreamTarget(stream, begin Entry) string {
	target := cliStreamTargetFromFields(field(stream, "worker"), field(stream, "job"))
	if target != "" {
		return target
	}
	return humanTarget(begin)
}

func cliStreamTargetFromFields(workerIP, job string) string {
	if workerIP != "" && workerIP != "-" {
		if job != "" {
			return job + "@" + workerIP
		}
		return workerIP
	}
	return job
}

func humanEventDetail(entry Entry) string {
	event := field(entry, "event")
	switch event {
	case "deploy_skip", "reconcile_skip_stop":
		parts := []string{event}
		if job := field(entry, "job"); job != "" {
			parts = append(parts, "job="+job)
		}
		if reason := field(entry, "reason"); reason != "" {
			parts = append(parts, "reason="+reason)
		}
		if worker := field(entry, "worker"); worker != "" && worker != "-" {
			parts = append(parts, "worker="+worker)
		}
		return strings.Join(parts, "  ")
	default:
		if msg := field(entry, "msg"); msg != "" {
			return msg
		}
		if event != "" {
			return event
		}
	}
	return ""
}

func humanCommandFooter(entry Entry) string {
	exitCode := field(entry, "exit")
	duration := field(entry, "duration_ms")
	status := "ok"
	if exitCode != "" && exitCode != "0" {
		status = "FAIL"
	}
	parts := []string{status}
	if exitCode != "" {
		parts = append(parts, "exit="+exitCode)
	}
	if duration != "" {
		parts = append(parts, formatHumanDuration(duration))
	}
	return strings.Join(parts, "  ")
}

func formatHumanTime(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		parsed, err = time.Parse("2006-01-02T15:04:05.000Z07:00", raw)
		if err != nil {
			return raw
		}
	}
	return parsed.UTC().Format(humanTimeLayout)
}

func formatHumanDuration(durationMS string) string {
	ms, err := strconv.ParseInt(durationMS, 10, 64)
	if err != nil {
		return durationMS + "ms"
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	seconds := float64(ms) / 1000
	return fmt.Sprintf("%.1fs", seconds)
}

func groupEntries(entries []Entry) (blocks []CommandBlock, events []Entry) {
	var current *CommandBlock
	flush := func() {
		if current == nil {
			return
		}
		blocks = append(blocks, *current)
		current = nil
	}
	for _, entry := range entries {
		event := field(entry, "event")
		switch event {
		case "command_begin":
			flush()
			current = &CommandBlock{Begin: entry}
		case "command_end":
			if current != nil {
				end := entry
				current.End = &end
				flush()
				continue
			}
			events = append(events, entry)
		case "":
			if field(entry, "msg") != "" || field(entry, "stream") != "" {
				if current != nil {
					current.Streams = append(current.Streams, entry)
					continue
				}
			}
			if entry.Raw != "" {
				events = append(events, entry)
			}
		default:
			flush()
			events = append(events, entry)
		}
	}
	flush()
	return blocks, events
}

func blockMatchesFilters(block CommandBlock, opts ShowOptions) bool {
	return entryMatchesFilters(block.Begin, opts)
}

func entryMatchesFilters(entry Entry, opts ShowOptions) bool {
	if opts.RunID != "" && !fieldMatches(field(entry, "run"), opts.RunID) {
		return false
	}
	if opts.Worker != "" && !fieldMatches(field(entry, "worker"), opts.Worker) {
		return false
	}
	if opts.Job != "" && !fieldMatches(field(entry, "job"), opts.Job) {
		return false
	}
	if opts.Phase != "" && field(entry, "phase") != opts.Phase {
		return false
	}
	if opts.Event != "" && field(entry, "event") != opts.Event {
		return false
	}
	return true
}

func fieldMatches(value, want string) bool {
	if value == "" {
		return false
	}
	if value == want || strings.HasPrefix(value, want) {
		return true
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == want || strings.HasPrefix(part, want) {
			return true
		}
	}
	return false
}
