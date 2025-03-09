// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import (
	"database/sql"
	"maand/bucket"
	"maand/data"
	"maand/utils"
	"path"
)

func Container(tx *sql.Tx) error {
	bucketID, err := data.GetBucketID(tx)
	if err != nil {
		return err
	}

	hash, err := utils.CalculateDirMD5(path.Join(bucket.WorkspaceLocation, "docker"))
	if err != nil {
		return err
	}

	err = data.UpdateHash(tx, "build", "container", hash)
	if err != nil {
		return err
	}

	prevHash, err := data.GetPreviousHash(tx, "build", "container")
	if err != nil {
		return err
	}

	found, err := bucket.IsBucketImageAvailable(bucketID)
	if err != nil {
		return err
	}

	if !found || hash != prevHash {
		err = bucket.BuildBucketContainer(bucketID)
		if err != nil {
			return err
		}
	}

	err = data.PromoteHash(tx, "build", "container")
	if err != nil {
		return err
	}

	return nil
}
