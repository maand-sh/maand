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
	Check(err)

	return scriptPath, nil
}

func ExecuteShellCommand(cmdString string, prefix string) error {
	cmd := exec.Command("bash", "-c", cmdString)

	stdout, err := cmd.StdoutPipe()
	Check(err)

	stderr, err := cmd.StderrPipe()
	Check(err)

	err = cmd.Start()
	Check(err)

	handleOutput := func(pipe io.ReadCloser, label string) {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			fmt.Printf("[%s] %s\n", label, scanner.Text())
		}
		if scanner.Err() != nil && scanner.Err().Error() == "read |0: file already closed" {
			return
		}
		Check(scanner.Err())
	}

	go handleOutput(stdout, prefix)
	go handleOutput(stderr, prefix)

	return cmd.Wait()
}

func ExecuteCommand(commands []string) error {
	scriptPath, err := GenerateScript(commands, nil)
	Check(err)
	defer func() {
		_ = os.Remove(scriptPath)
	}()

	localCmd := fmt.Sprintf("bash < %s", scriptPath)
	return ExecuteShellCommand(localCmd, "local")
}
