// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package worker

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"maand/bucket"
)

const (
	defaultSSHUser        = "agent"
	defaultSSHKeyFile     = "worker.key"
	defaultSSHTimeoutSecs = 300
	defaultSSHConnectSecs = 15
)

// sshSettings applies defaults and validates SSH configuration for remote execution.
func sshSettings(conf bucket.MaandConf) (user string, keyHostPath string, useSudo bool, err error) {
	user = strings.TrimSpace(conf.SSHUser)
	if user == "" {
		user = defaultSSHUser
	}

	keyName := strings.TrimSpace(conf.SSHKeyFile)
	if keyName == "" {
		keyName = defaultSSHKeyFile
	}

	keyHostPath = filepath.Join(bucket.SecretLocation, keyName)
	info, statErr := os.Stat(keyHostPath)
	if statErr != nil {
		return "", "", false, fmt.Errorf("ssh key %s: %w", keyHostPath, statErr)
	}
	if info.IsDir() {
		return "", "", false, fmt.Errorf("ssh key %s is a directory", keyHostPath)
	}

	return user, keyHostPath, conf.UseSUDO, nil
}

// EnsureSSHStateDir creates bucket/tmp/ssh for per-worker control sockets.
func EnsureSSHStateDir() error {
	return os.MkdirAll(filepath.Join(bucket.HostTmpDir(), "ssh"), 0o755)
}

// SSHClientArgs returns ssh(1) arguments shared by worker commands and rsync --rsh.
// workerIP selects a dedicated control socket path so parallel rsync does not race on ControlMaster.
func SSHClientArgs(keyPath, workerIP string) []string {
	controlPath := filepath.Join(bucket.HostTmpDir(), "ssh", controlSocketName(workerIP))
	return []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "BatchMode=yes",
		"-o", fmt.Sprintf("ConnectTimeout=%d", defaultSSHConnectSecs),
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=3",
		"-o", "IdentitiesOnly=yes",
		"-o", "ControlMaster=auto",
		"-o", "ControlPath=" + controlPath,
		"-o", "ControlPersist=60",
		"-i", keyPath,
	}
}

func controlSocketName(workerIP string) string {
	replacer := strings.NewReplacer(".", "_", ":", "_", "/", "_")
	return replacer.Replace(workerIP)
}

// shellQuote wraps a value for a single-quoted POSIX shell argument.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func sshTarget(user, workerIP string) string {
	host := workerIP
	if parsed := net.ParseIP(workerIP); parsed != nil && parsed.To4() == nil {
		host = "[" + workerIP + "]"
	}
	return user + "@" + host
}

// remoteShellCommand is the remote command that reads a bash script from stdin.
func remoteShellCommand(useSudo bool) string {
	parts := []string{fmt.Sprintf("timeout %d", defaultSSHTimeoutSecs)}
	if useSudo {
		parts = append(parts, "sudo", "-E", "bash", "-s")
	} else {
		parts = append(parts, "bash", "-s")
	}
	return strings.Join(parts, " ")
}

func ensureCommandScript(hostScriptPath string) error {
	info, err := os.Stat(hostScriptPath)
	if err != nil {
		return fmt.Errorf("command script: %w", err)
	}
	if info.IsDir() {
		return fmt.Errorf("command script %s is a directory", hostScriptPath)
	}
	if info.Size() == 0 {
		return fmt.Errorf("command script %s is empty", hostScriptPath)
	}
	return nil
}

func remoteExecTimeout() time.Duration {
	return defaultSSHTimeoutSecs * time.Second
}
