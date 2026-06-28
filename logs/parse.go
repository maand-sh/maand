// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package logs

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Entry is one parsed structured log line.
type Entry struct {
	Raw    string
	Fields map[string]string
}

// ParseLine parses a bucket log line (kv, json, or payload-only stream).
func ParseLine(line string) Entry {
	line = strings.TrimSpace(line)
	if line == "" {
		return Entry{Raw: line, Fields: map[string]string{}}
	}
	if strings.HasPrefix(line, "{") {
		return parseJSONLine(line)
	}
	if strings.HasPrefix(line, "| ") || strings.HasPrefix(line, "! ") {
		stream := "stdout"
		msg := strings.TrimPrefix(line, "| ")
		if strings.HasPrefix(line, "! ") {
			stream = "stderr"
			msg = strings.TrimPrefix(line, "! ")
		}
		return Entry{
			Raw: line,
			Fields: map[string]string{
				"stream": stream,
				"msg":    msg,
			},
		}
	}
	return Entry{Raw: line, Fields: parseKVLine(line)}
}

func parseJSONLine(line string) Entry {
	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		return Entry{Raw: line, Fields: parseKVLine(line)}
	}
	fields := make(map[string]string, len(payload))
	for key, value := range payload {
		switch v := value.(type) {
		case string:
			fields[key] = v
		case float64:
			fields[key] = strconv.FormatInt(int64(v), 10)
		case bool:
			fields[key] = strconv.FormatBool(v)
		default:
			fields[key] = fmt.Sprint(v)
		}
	}
	return Entry{Raw: line, Fields: fields}
}

func parseKVLine(line string) map[string]string {
	fields := make(map[string]string)
	for len(line) > 0 {
		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			break
		}
		key := line[:eq]
		line = line[eq+1:]
		value, rest := readKVValue(line)
		fields[key] = value
		line = strings.TrimSpace(rest)
	}
	return fields
}

func readKVValue(line string) (value, rest string) {
	if line == "" {
		return "", ""
	}
	if line[0] == '"' {
		var b strings.Builder
		escaped := false
		for i := 1; i < len(line); i++ {
			ch := line[i]
			if escaped {
				b.WriteByte(ch)
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
				continue
			}
			if ch == '"' {
				return b.String(), strings.TrimSpace(line[i+1:])
			}
			b.WriteByte(ch)
		}
		return b.String(), ""
	}
	if space := strings.IndexByte(line, ' '); space >= 0 {
		return line[:space], line[space+1:]
	}
	return line, ""
}

func field(entry Entry, key string) string {
	if entry.Fields == nil {
		return ""
	}
	return entry.Fields[key]
}

func shortRunID(runID string) string {
	if len(runID) <= 8 {
		return runID
	}
	return runID[:8]
}
