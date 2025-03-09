// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package worker

import (
	"fmt"
	"maand/bucket"
)

func KeyScan(dockerClient *bucket.DockerClient, workerIP string) error {
	return dockerClient.Exec("", []string{"mkdir -p ~/.ssh", fmt.Sprintf("ssh-keyscan -H %s >> ~/.ssh/known_hosts", workerIP)}, nil, false)
}
