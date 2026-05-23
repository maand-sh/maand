// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Runtime runs bucket-local and worker-prep commands on the maand CLI host.
type Runtime struct {
	logMu sync.Mutex
}

// SetupRuntime prepares host execution for a bucket session.
func SetupRuntime(_ string) (*Runtime, error) {
	return &Runtime{}, nil
}

// Stop releases runtime resources (no-op on host).
func (r *Runtime) Stop() error {
	return nil
}

// Exec runs bash commands locally on the CLI host and logs output per workerIP.
// Pass an empty workerIP for bucket-local commands (logged to maand.log).
func (r *Runtime) Exec(workerIP string, commandLines []string, env []string, _ bool) error {
	script := strings.Join([]string{
		"#!/bin/bash",
		"set -e",
		"set -u",
		strings.Join(commandLines, "\n"),
	}, "\n") + "\n"

	cmd := exec.Command("bash", "-s")
	cmd.Stdin = strings.NewReader(script)
	if len(env) > 0 {
		cmd.Env = env
	} else {
		cmd.Env = os.Environ()
	}

	return r.RunCommand(workerIP, cmd)
}

// RunCommand starts cmd, streams stdout/stderr to logs, and returns on completion.
func (r *Runtime) RunCommand(workerIP string, cmd *exec.Cmd) error {
	if cmd.Dir == "" {
		bucketRoot, err := filepath.Abs(Location)
		if err != nil {
			return UnexpectedError(err)
		}
		cmd.Dir = bucketRoot
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return UnexpectedError(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return UnexpectedError(err)
	}

	if err := cmd.Start(); err != nil {
		return UnexpectedError(err)
	}

	var (
		wg        sync.WaitGroup
		streamErr error
		streamMu  sync.Mutex
	)

	stream := func(reader io.Reader) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			log.Printf("[%-12s] %s", workerIP, line)
			if err := r.appendWorkerLog(workerIP, line); err != nil {
				streamMu.Lock()
				if streamErr == nil {
					streamErr = err
				}
				streamMu.Unlock()
			}
		}
		if err := scanner.Err(); err != nil {
			streamMu.Lock()
			if streamErr == nil {
				streamErr = UnexpectedError(err)
			}
			streamMu.Unlock()
		}
	}

	wg.Add(2)
	go stream(stdout)
	go stream(stderr)
	wg.Wait()

	if streamErr != nil {
		_ = cmd.Process.Kill()
		return streamErr
	}

	if err := cmd.Wait(); err != nil {
		return commandFailedError(err)
	}
	return nil
}

func commandFailedError(err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("command execution failed (exit %d)", exitErr.ExitCode())
	}
	return UnexpectedError(err)
}

func (r *Runtime) appendWorkerLog(workerIP, line string) error {
	logFileName := "maand.log"
	if workerIP != "" {
		logFileName = workerIP + ".log"
	}

	r.logMu.Lock()
	defer r.logMu.Unlock()

	if err := os.MkdirAll(LogLocation, 0o755); err != nil {
		return UnexpectedError(err)
	}

	logFile, err := os.OpenFile(filepath.Join(LogLocation, logFileName), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return UnexpectedError(err)
	}
	defer func() {
		_ = logFile.Close()
	}()

	if _, err := logFile.WriteString(line + "\n"); err != nil {
		return UnexpectedError(err)
	}
	return nil
}
