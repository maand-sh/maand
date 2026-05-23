// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import (
	"fmt"
	"os"
	"path"
	"strings"
)

// BuildCommandScript assembles a bash script body for remote execution.
// Environment variables must be included in the script; they are not applied separately over SSH.
func BuildCommandScript(commands []string, envVars []string) string {
	lines := []string{"#!/bin/bash", "set -e", "set -u"}
	for _, envVar := range envVars {
		envVar = strings.TrimSpace(envVar)
		envVar = strings.TrimPrefix(envVar, "export ")
		if envVar != "" {
			lines = append(lines, fmt.Sprintf("export %s", envVar))
		}
	}
	lines = append(lines, commands...)
	return strings.Join(lines, "\n") + "\n"
}

// GenerateCommandScript writes a bash script under bucket/tmp and returns its filename (e.g. "<uuid>.sh").
func GenerateCommandScript(commands []string, envVars []string) (scriptFileName string, err error) {
	scriptFileName = newCommandScriptName()
	scriptPath := path.Join(TempLocation, scriptFileName)

	if err := os.MkdirAll(TempLocation, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(scriptPath, []byte(BuildCommandScript(commands, envVars)), 0o700); err != nil {
		return "", err
	}

	return scriptFileName, nil
}

func newCommandScriptName() string {
	return newUniqueName() + ".sh"
}
