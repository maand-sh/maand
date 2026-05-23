// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package deploy

import (
	"maand/bucket"
	"maand/prereq"
	"maand/worker"
)

func checkDeployPrerequisites(workers []string) error {
	if testHooks != nil {
		if testHooks.CheckWorkerPrerequisites != nil {
			return testHooks.CheckWorkerPrerequisites(nil, workers)
		}
		return nil
	}

	needsBun, err := prereq.WorkspaceUsesBun()
	if err != nil {
		return err
	}
	if err := prereq.CheckLocalDeploy(needsBun); err != nil {
		return err
	}

	conf, err := bucket.GetMaandConf()
	if err != nil {
		return err
	}
	return worker.CheckPrerequisites(workers, conf.UseSUDO)
}
