// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const logTimeFormat = "2006-01-02T15:04:05.000Z07:00"

// LogFormat selects structured log encoding.
type LogFormat string

const (
	LogFormatKV   LogFormat = "kv"
	LogFormatJSON LogFormat = "json"
)

func parseLogFormat(raw string) LogFormat {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(LogFormatJSON), "jsonl":
		return LogFormatJSON
	default:
		return LogFormatKV
	}
}

func currentLogFormat() LogFormat {
	conf, err := GetMaandConf()
	if err != nil {
		return LogFormatKV
	}
	if conf.LogFormat == "" {
		return LogFormatKV
	}
	return parseLogFormat(conf.LogFormat)
}

func formatLogLine(format LogFormat, fields map[string]string) string {
	switch format {
	case LogFormatJSON:
		return formatJSONLine(fields)
	default:
		return formatKVLine(fields)
	}
}

func formatKVLine(fields map[string]string) string {
	keys := []string{"ts", "level", "event", "run", "maand", "seq", "worker", "job", "phase", "action", "cmd", "stream", "exit", "duration_ms", "reason", "msg"}
	parts := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, key := range keys {
		value, ok := fields[key]
		if !ok || value == "" {
			continue
		}
		parts = append(parts, key+"="+quoteKVValue(value))
		seen[key] = struct{}{}
	}
	for key, value := range fields {
		if value == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		parts = append(parts, key+"="+quoteKVValue(value))
	}
	return strings.Join(parts, " ")
}

func quoteKVValue(value string) string {
	if value == "" {
		return `""`
	}
	needsQuotes := strings.ContainsAny(value, " \t=") || strings.Contains(value, `"`)
	if !needsQuotes {
		return value
	}
	return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
}

func formatJSONLine(fields map[string]string) string {
	payload := make(map[string]any, len(fields)+1)
	for key, value := range fields {
		if value == "" {
			continue
		}
		switch key {
		case "seq", "exit", "duration_ms":
			if n, err := strconv.Atoi(value); err == nil {
				payload[key] = n
				continue
			}
		}
		payload[key] = value
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return formatKVLine(fields)
	}
	return string(data)
}

func baseLogFields(run RunContext, workerIP string) map[string]string {
	fields := map[string]string{
		"ts":    time.Now().UTC().Format(logTimeFormat),
		"level": "INFO",
		"run":   run.RunID,
	}
	if run.MaandCmd != "" {
		fields["maand"] = run.MaandCmd
	}
	if run.UpdateSeq > 0 {
		fields["seq"] = strconv.Itoa(run.UpdateSeq)
	}
	if workerIP != "" {
		fields["worker"] = workerIP
	} else {
		fields["worker"] = "-"
	}
	return fields
}

func commandContextFields(cmdCtx CommandContext) map[string]string {
	fields := map[string]string{}
	if cmdCtx.Job != "" {
		fields["job"] = cmdCtx.Job
	}
	if cmdCtx.Phase != "" {
		fields["phase"] = cmdCtx.Phase
	}
	if cmdCtx.Action != "" {
		fields["action"] = cmdCtx.Action
	}
	if cmdCtx.Cmd != "" {
		fields["cmd"] = cmdCtx.Cmd
	}
	return fields
}

func formatCommandBegin(run RunContext, workerIP string, cmdCtx CommandContext) string {
	fields := baseLogFields(run, workerIP)
	fields["event"] = "command_begin"
	for key, value := range commandContextFields(cmdCtx) {
		fields[key] = value
	}
	return formatLogLine(currentLogFormat(), fields)
}

func formatCommandEnd(run RunContext, workerIP string, cmdCtx CommandContext, exitCode int, duration time.Duration) string {
	fields := baseLogFields(run, workerIP)
	fields["event"] = "command_end"
	fields["exit"] = strconv.Itoa(exitCode)
	fields["duration_ms"] = strconv.FormatInt(duration.Milliseconds(), 10)
	for key, value := range commandContextFields(cmdCtx) {
		fields[key] = value
	}
	return formatLogLine(currentLogFormat(), fields)
}

func formatStreamLine(run RunContext, workerIP string, cmdCtx CommandContext, stream, line string) string {
	fields := baseLogFields(run, workerIP)
	if stream != "" {
		fields["stream"] = stream
	}
	fields["msg"] = line
	for key, value := range commandContextFields(cmdCtx) {
		fields[key] = value
	}
	return formatLogLine(currentLogFormat(), fields)
}

// formatCLIStreamLine is the terminal view: job/worker prefix plus payload line.
func formatCLIStreamLine(workerIP string, cmdCtx CommandContext, stream, line string) string {
	delim := "| "
	if stream == "stderr" {
		delim = "! "
	}
	if prefix := cliStreamTarget(workerIP, cmdCtx.Job); prefix != "" {
		return prefix + " " + delim + line
	}
	return delim + line
}

func cliStreamTarget(workerIP, job string) string {
	if workerIP != "" && workerIP != "-" {
		if job != "" {
			return job + "@" + workerIP
		}
		return workerIP
	}
	return job
}

func formatEventLine(run RunContext, workerIP, event string, extra map[string]string) string {
	fields := baseLogFields(run, workerIP)
	fields["event"] = event
	for key, value := range extra {
		if value != "" {
			fields[key] = value
		}
	}
	return formatLogLine(currentLogFormat(), fields)
}

func workerLogFileName(workerIP string) string {
	if workerIP != "" {
		return workerIP + ".log"
	}
	return "maand.log"
}

func runLogDir(runID string) string {
	return fmt.Sprintf("%s/%s", RunLogLocation, runID)
}
