// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import "fmt"

func runnerCommand(bucketID, action, job string) string {
	return fmt.Sprintf(
		"python3 /opt/worker/%s/bin/runner.py %s %s --jobs %s",
		bucketID, bucketID, action, job,
	)
}
