// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"log"

	"maand/bucket"
)

func runWorkerCommandOrAssumeDead(
	rt *bucket.Runtime,
	workerIP string,
	commands []string,
	env []string,
) {
	if err := runWorkerCommand(rt, workerIP, commands, env); err != nil {
		log.Printf("deploy: removed worker %s unreachable, assuming dead: %v", workerIP, err)
	}
}

func finishRemovedWorkerCommand(workerIP string, err error, assumeDead bool) error {
	if err == nil {
		return nil
	}
	if assumeDead {
		log.Printf("deploy: removed worker %s unreachable, assuming dead: %v", workerIP, err)
		return nil
	}
	return err
}
