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
	"time"
)

// Runtime runs bucket-local and worker-prep commands on the maand CLI host.
type Runtime struct {
	logMu sync.Mutex
	run   RunContext
}

// SetupRuntime prepares host execution for a bucket session.
func SetupRuntime(_ string, run RunContext) (*Runtime, error) {
	return &Runtime{run: run}, nil
}

// Run returns the run context attached to this runtime.
func (r *Runtime) Run() RunContext {
	return r.run
}

// Stop releases runtime resources (no-op on host).
func (r *Runtime) Stop() error {
	return nil
}

// LogEvent writes a structured event line to bucket logs.
func (r *Runtime) LogEvent(workerIP, event string, extra map[string]string) error {
	line := formatEventLine(r.run, workerIP, event, extra)
	log.Printf("%s", line)
	return r.appendLog(workerIP, line)
}

// Exec runs bash commands locally on the CLI host and logs output per workerIP.
// Pass an empty workerIP for bucket-local commands (logged to maand.log).
func (r *Runtime) Exec(workerIP string, cmdCtx CommandContext, commandLines []string, env []string) error {
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

	return r.RunCommand(workerIP, cmdCtx, cmd)
}

// RunCommand starts cmd, streams stdout/stderr to logs, and returns on completion.
func (r *Runtime) RunCommand(workerIP string, cmdCtx CommandContext, cmd *exec.Cmd) error {
	cmdCtx = cmdCtx.withDefaults(workerIP, cmd, nil)

	if cmd.Dir == "" {
		bucketRoot, err := filepath.Abs(Location)
		if err != nil {
			return UnexpectedError(err)
		}
		cmd.Dir = bucketRoot
	}

	if err := r.appendLog(workerIP, formatCommandBegin(r.run, workerIP, cmdCtx)); err != nil {
		return err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return UnexpectedError(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return UnexpectedError(err)
	}

	started := time.Now()
	if err := cmd.Start(); err != nil {
		return UnexpectedError(err)
	}

	var (
		wg        sync.WaitGroup
		streamErr error
		streamMu  sync.Mutex
	)

	stream := func(reader io.Reader, streamName string) {
		defer wg.Done()
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			line := scanner.Text()
			formatted := formatStreamLine(r.run, workerIP, cmdCtx, streamName, line)
			log.Printf("%s", formatCLIStreamLine(workerIP, cmdCtx, streamName, line))
			if err := r.appendLog(workerIP, formatted); err != nil {
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
	go stream(stdout, "stdout")
	go stream(stderr, "stderr")
	wg.Wait()

	exitCode := 0
	waitErr := cmd.Wait()
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}

	if err := r.appendLog(workerIP, formatCommandEnd(r.run, workerIP, cmdCtx, exitCode, time.Since(started))); err != nil {
		return err
	}

	if streamErr != nil {
		_ = cmd.Process.Kill()
		return streamErr
	}

	if waitErr != nil {
		return commandFailedError(waitErr)
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

func (r *Runtime) appendLog(workerIP, line string) error {
	r.logMu.Lock()
	defer r.logMu.Unlock()

	if err := os.MkdirAll(LogLocation, 0o755); err != nil {
		return UnexpectedError(err)
	}

	if err := appendLine(filepath.Join(LogLocation, workerLogFileName(workerIP)), line); err != nil {
		return err
	}

	if r.run.RunID == "" {
		return nil
	}

	runDir := runLogDir(r.run.RunID)
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return UnexpectedError(err)
	}
	return appendLine(filepath.Join(runDir, workerLogFileName(workerIP)), line)
}

func appendLine(path, line string) error {
	logFile, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
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
