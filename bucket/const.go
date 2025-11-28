// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import (
	"path"
)

var (
	Location          = "."
	WorkspaceLocation = path.Join(Location, "workspace")
	SecretLocation    = path.Join(Location, "secrets")
	TempLocation      = path.Join(Location, "tmp")
	LogLocation       = path.Join(Location, "logs")
)

func UpdatePath() {
	WorkspaceLocation = path.Join(Location, "workspace")
	SecretLocation = path.Join(Location, "secrets")
	TempLocation = path.Join(Location, "tmp")
	LogLocation = path.Join(Location, "logs")
}

func GetTempWorkerPath(workerIP string) string {
	return path.Join(TempLocation, "workers", workerIP)
}
