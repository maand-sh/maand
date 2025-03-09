// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

type DockerClient struct {
	cli         *client.Client
	ctx         context.Context
	baseImage   string
	containerID string
}

func (dc *DockerClient) start() (err error) {
	bucketAbsPath, err := filepath.Abs(path.Join(Location))
	if err != nil {
		return err
	}

	var binds = []string{fmt.Sprintf("%s:/bucket", bucketAbsPath)}

	resp, err := dc.cli.ContainerCreate(
		dc.ctx,
		&container.Config{
			Image: dc.baseImage,
			Cmd:   []string{"sh", "-c", "sleep infinity"},
			Tty:   false,
		},
		&container.HostConfig{
			Binds:       binds,
			NetworkMode: "host",
		},
		nil,
		nil,
		"",
	)
	if err != nil {
		return err
	}

	if err := dc.cli.ContainerStart(dc.ctx, resp.ID, container.StartOptions{}); err != nil {
		return err
	}

	dc.containerID = resp.ID

	return nil
}

func (dc *DockerClient) Stop() error {
	if dc.containerID != "" {
		if err := dc.cli.ContainerKill(dc.ctx, dc.containerID, "SIGKILL"); err != nil {
			return err
		}

		if err := dc.cli.ContainerRemove(dc.ctx, dc.containerID, container.RemoveOptions{}); err != nil {
			return err
		}
	}

	if err := dc.cli.Close(); err != nil {
		return err
	}
	return dc.ctx.Err()
}

func (dc *DockerClient) Exec(prefix string, command []string, envs []string, verbose bool) error {
	sessionFileName := fmt.Sprintf("%s.sh", uuid.NewString())
	sessionFilePath := path.Join("tmp", sessionFileName)
	sessionOutFilePath := path.Join("tmp", sessionFileName) + ".out"

	script := fmt.Sprintf(`#!/bin/bash
%s
echo $? > %s
sync`, strings.Join(command, "\n"), path.Join("/bucket/tmp", sessionFileName)+".out")

	err := os.WriteFile(path.Join(Location, sessionFilePath), []byte(script), os.ModePerm)
	if err != nil {
		return err
	}

	if dc.containerID == "" {
		return fmt.Errorf("container not started")
	}

	execConfig := container.ExecOptions{
		Cmd:          []string{"bash", path.Join("/bucket/tmp", sessionFileName)},
		AttachStdout: true,
		AttachStderr: true,
		Env:          envs,
	}
	execResp, err := dc.cli.ContainerExecCreate(dc.ctx, dc.containerID, execConfig)
	if err != nil {
		return err
	}

	attachResp, err := dc.cli.ContainerExecAttach(dc.ctx, execResp.ID, container.ExecAttachOptions{
		Detach: false,
		Tty:    false,
	})
	if err != nil {
		return err
	}

	defer attachResp.Close()

	reader := bufio.NewReader(attachResp.Reader)
	for {
		line, _, err := reader.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if len(line) >= 8 && (line[0] == byte(1) || line[0] == byte(2)) {
			line = line[8:]
		}
		log.Printf("[%-12s] %s", prefix, string(line))
	}

	errorCode, err := os.ReadFile(path.Join(Location, sessionOutFilePath))
	if err != nil {
		return err
	}

	if strings.TrimSpace(string(errorCode)) != "0" {
		return fmt.Errorf("exit status %s", string(errorCode))
	}

	return nil
}

func BuildBucketContainer(bucketID string) error {
	cmd := exec.Command("docker", "build", "-t", fmt.Sprintf("maand/%s", bucketID), path.Join(WorkspaceLocation, "docker"))

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

	handleOutput := func(pipe io.ReadCloser) {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			fmt.Printf("%s\n", scanner.Text())
		}
		if err := scanner.Err(); err != nil {
			fmt.Printf("%s\n", err.Error())
		}
	}

	go handleOutput(stdout)
	go handleOutput(stderr)

	err = cmd.Wait()
	if err != nil {
		return fmt.Errorf("failed 'docker build' : %w", err)
	}

	return nil
}

func IsBucketImageAvailable(bucketID string) (bool, error) {
	cmd := exec.Command("docker", "image", "ls")

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}

	err = cmd.Start()
	if err != nil {
		return false, err
	}

	found := false
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.Contains(line, fmt.Sprintf("maand/%s", bucketID)) {
			found = true
			break // Image found, no need to continue scanning
		}
	}

	if err := scanner.Err(); err != nil {
		return false, err
	}

	err = cmd.Wait()
	if err != nil {
		return false, fmt.Errorf("failed on 'docker image ls' : %w", err)
	}

	return found, nil
}

func newDockerContainer(baseImage string) (*DockerClient, error) {
	var ctx = context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	dc := &DockerClient{
		baseImage: baseImage,
		ctx:       ctx,
		cli:       cli,
	}

	return dc, nil
}

func SetupBucketContainer(bucketID string) (*DockerClient, error) {
	docker, err := newDockerContainer(fmt.Sprintf("maand/%s", bucketID))
	if err != nil {
		return nil, err
	}

	err = docker.start()
	if err != nil {
		return nil, err
	}

	// sigChan := make(chan os.Signal, 1)
	// signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	// done := make(chan bool)
	// go func() {
	// 	sig := <-sigChan
	// 	fmt.Println("\nReceived signal:", sig)
	// 	_ = docker.Stop()
	// 	done <- true
	// }()

	return docker, err
}
