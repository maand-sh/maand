// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package worker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"maand/bucket"
)

// RunRemoteScript runs script on workerIP over SSH, piping script to remote bash -s.
// Environment variables must already be present in the script body.
func RunRemoteScript(rt *bucket.Runtime, workerIP string, cmdCtx bucket.CommandContext, script io.Reader, _ bool) error {
	user, keyPath, useSudo, err := sshSettingsFromConf()
	if err != nil {
		return err
	}

	if err := EnsureSSHStateDir(); err != nil {
		return err
	}

	args := SSHClientArgs(keyPath, workerIP)
	args = append(args, sshTarget(user, workerIP), remoteShellCommand(useSudo))

	ctx, cancel := contextWithRemoteTimeout()
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = script
	if cmdCtx.Cmd == "" {
		cmdCtx.Action = "ssh"
	}
	return rt.RunCommand(workerIP, cmdCtx, cmd)
}

// RunRemoteScriptFile runs a local script file on workerIP over SSH.
func RunRemoteScriptFile(rt *bucket.Runtime, workerIP string, cmdCtx bucket.CommandContext, scriptPath string, _ bool) error {
	if err := ensureCommandScript(scriptPath); err != nil {
		return err
	}

	file, err := os.Open(scriptPath)
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	if cmdCtx.Cmd == "" {
		cmdCtx.Cmd = scriptPath
	}
	return RunRemoteScript(rt, workerIP, cmdCtx, file, false)
}

// RemoteShellCommand runs a single remote shell command and returns an error on non-zero exit.
func RemoteShellCommand(workerIP, command string, timeout time.Duration) error {
	_, err := remoteShellOutput(workerIP, command, timeout)
	return err
}

// RemoteShellOutput runs a single remote shell command and returns combined stdout/stderr.
func RemoteShellOutput(workerIP, command string) (string, error) {
	return remoteShellOutput(workerIP, command, remoteExecTimeout())
}

func remoteShellOutput(workerIP, command string, timeout time.Duration) (string, error) {
	user, keyPath, useSudo, err := sshSettingsFromConf()
	if err != nil {
		return "", err
	}

	if err := EnsureSSHStateDir(); err != nil {
		return "", err
	}

	remoteCommand := command
	if useSudo {
		remoteCommand = fmt.Sprintf("sudo -E bash -lc %s", shellQuote(command))
	}

	args := SSHClientArgs(keyPath, workerIP)
	args = append(args, sshTarget(user, workerIP), remoteCommand)

	if timeout <= 0 {
		timeout = remoteExecTimeout()
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	out, err := exec.CommandContext(ctx, "ssh", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// RunRemoteScriptCombined runs script on workerIP and returns combined stdout/stderr.
func RunRemoteScriptCombined(workerIP string, script io.Reader) (string, error) {
	user, keyPath, useSudo, err := sshSettingsFromConf()
	if err != nil {
		return "", err
	}

	if err := EnsureSSHStateDir(); err != nil {
		return "", err
	}

	args := SSHClientArgs(keyPath, workerIP)
	args = append(args, sshTarget(user, workerIP), remoteShellCommand(useSudo))

	ctx, cancel := contextWithRemoteTimeout()
	defer cancel()

	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = script
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// RSHShell returns the ssh command rsync should use via --rsh.
func RSHShell(keyPath, workerIP string) string {
	absKey, err := filepath.Abs(keyPath)
	if err != nil {
		absKey = keyPath
	}
	return "ssh " + strings.Join(SSHClientArgs(absKey, workerIP), " ")
}

func sshSettingsFromConf() (user, keyPath string, useSudo bool, err error) {
	conf, err := bucket.GetMaandConf()
	if err != nil {
		return "", "", false, err
	}
	return sshSettings(conf)
}

func contextWithRemoteTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), remoteExecTimeout())
}
