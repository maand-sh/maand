// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package utils

import (
	"bufio"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"os/exec"
	"strings"
)

func GenerateScript(commands []string, env []string) (string, error) {
	newUUID := uuid.New()
	scriptPath := fmt.Sprintf("/tmp/%s.sh", newUUID.String())

	scriptLines := []string{"#!/bin/bash", "set -e", "set -u"}
	for _, envVar := range env {
		scriptLines = append(scriptLines, fmt.Sprintf("export %s", envVar))
	}
	scriptLines = append(scriptLines, commands...)

	script := strings.Join(scriptLines, "\n")

	err := os.WriteFile(scriptPath, []byte(script), 0700)
	if err != nil {
		return "", err
	}

	return scriptPath, nil
}

func ExecuteShellCommand(cmdString string, prefix string) error {
	cmd := exec.Command("bash", "-c", cmdString)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	handleOutput := func(pipe io.ReadCloser, label string) {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			fmt.Printf("[%s] %s\n", label, scanner.Text())
		}
		if scanner.Err() != nil && scanner.Err().Error() == "read |0: file already closed" {
			return
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("[%s] %s\n", label, err.Error())
		}
	}

	go handleOutput(stdout, prefix)
	go handleOutput(stderr, prefix)

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	return nil
}

func ExecuteCommand(commands []string) error {
	scriptPath, err := GenerateScript(commands, nil)
	if err != nil {
		return err
	}

	defer func() {
		_ = os.Remove(scriptPath)
	}()

	localCmd := fmt.Sprintf("bash < %s", scriptPath)
	return ExecuteShellCommand(localCmd, "local")
}
