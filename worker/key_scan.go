// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package worker

import (
	"fmt"
	"path/filepath"

	"maand/bucket"
)

func KeyScan1(rt *bucket.Runtime, workerIP string) error {
	sshDir := filepath.Join(bucket.Location, ".ssh")
	return rt.Exec("", []string{
		fmt.Sprintf("mkdir -p %s", sshDir),
		fmt.Sprintf("ssh-keyscan -H %s >> %s/known_hosts", workerIP, sshDir),
	}, nil, false)
}
