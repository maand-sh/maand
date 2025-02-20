// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package worker

import (
	"fmt"
	"maand/utils"
	"os"
)

type KeyScanError struct {
	WorkerIP string
	Err      error
}

func (e KeyScanError) Error() string {
	return e.Err.Error()
}

func KeyScan(workerIP string) error {
	if os.Getenv("CONTAINER") == "1" {
		cmd := fmt.Sprintf(`ssh-keyscan -H %s >> ~/.ssh/known_hosts`, workerIP)
		err := utils.ExecuteCommand([]string{"mkdir -p ~/.ssh", cmd})
		if err != nil {
			return &KeyScanError{WorkerIP: workerIP, Err: err}
		}
	}
	return nil
}
