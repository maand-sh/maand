// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package utils

import (
	"fmt"
	"github.com/google/uuid"
	"maand/bucket"
	"os"
	"path"
	"strings"
)

func GenerateCommandScript(commands []string, env []string) (string, error) {
	newUUID := uuid.New()

	commandScriptPath := fmt.Sprintf(path.Join(bucket.TempLocation, "%s.sh"), newUUID.String())

	scriptLines := []string{"#!/bin/bash", "set -e", "set -u"}
	for _, envVar := range env {
		scriptLines = append(scriptLines, fmt.Sprintf("export %s", envVar))
	}
	scriptLines = append(scriptLines, commands...)

	script := strings.Join(scriptLines, "\n")

	err := os.MkdirAll(bucket.TempLocation, os.ModePerm)
	if err != nil {
		return "", err
	}

	err = os.WriteFile(commandScriptPath, []byte(script), 0700)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.sh", newUUID), nil
}
