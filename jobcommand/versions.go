// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"database/sql"
	"fmt"

	"maand/data"
)

func allocationVersionEnvForJobCommand(tx *sql.Tx, jobName, workerIP string) ([]string, error) {
	allocID, err := data.GetAllocationID(tx, workerIP, jobName)
	if err != nil {
		return nil, err
	}
	namespace := fmt.Sprintf("%s_allocation", jobName)
	versions, err := data.GetAllocationVersions(tx, namespace, allocID)
	if err != nil {
		return nil, err
	}
	return []string{
		fmt.Sprintf("CURRENT_VERSION=%s", versions.CurrentVersion),
		fmt.Sprintf("NEW_VERSION=%s", versions.NewVersion),
	}, nil
}
