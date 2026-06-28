// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"maand/bucket"
	"maand/worker"
)

func rsync(rt *bucket.Runtime, bucketID, workerIP string, jobs []string) error {
	conf, err := bucket.GetMaandConf()
	if err != nil {
		return err
	}

	user := strings.TrimSpace(conf.SSHUser)
	if user == "" {
		user = "agent"
	}
	keyName := strings.TrimSpace(conf.SSHKeyFile)
	if keyName == "" {
		keyName = "worker.key"
	}
	keyFilePath, err := filepath.Abs(path.Join(bucket.SecretLocation, keyName))
	if err != nil {
		return err
	}

	if err := worker.EnsureSSHStateDir(); err != nil {
		return err
	}

	remoteRS := "rsync"
	if conf.UseSUDO {
		remoteRS = "sudo rsync"
	}

	ruleFilePath, err := filepath.Abs(path.Join(bucket.TempLocation, "workers", fmt.Sprintf("%s.rsync", workerIP)))
	if err != nil {
		return err
	}
	workerDir, err := filepath.Abs(bucket.GetTempWorkerPath(workerIP))
	if err != nil {
		return err
	}

	args := []string{
		"--timeout=30",
		"--inplace",
		"--whole-file",
		"--checksum",
		"--recursive",
		"--force",
		"--delete-after",
		"--delete",
		"--group",
		"--owner",
		"--executability",
		"--compress",
		"--verbose",
		"--exclude=jobs/*/bin",
		"--exclude=jobs/*/data",
		"--exclude=jobs/*/logs",
		"--exclude=jobs/*/_modules",
		"--exclude=jobs/*/_prometheus",
		"--rsync-path=" + remoteRS,
		"--filter=merge " + ruleFilePath,
		"--rsh=" + worker.RSHShell(keyFilePath, workerIP),
		workerDir + string(filepath.Separator),
		fmt.Sprintf("%s@%s:/opt/worker/%s", user, workerIP, bucketID),
	}

	cmd := exec.Command("rsync", args...)
	cmdCtx := bucket.CommandContext{
		Job:    strings.Join(jobs, ","),
		Phase:  "rsync",
		Action: "rsync",
		Cmd:    bucket.SummarizeExecCmd(cmd),
	}
	if err := rt.RunCommand(workerIP, cmdCtx, cmd); err != nil {
		return fmt.Errorf("rsync failed: worker %s: %w", workerIP, err)
	}
	return nil
}
