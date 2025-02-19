package bucket

import (
	"path"
	"path/filepath"
)

var Location = "."
var WorkspaceLocation = path.Join(Location, "workspace")
var SecretLocation = path.Join(Location, "secrets")
var TempLocation = path.Join(Location, "tmp")

func UpdatePath() {
	WorkspaceLocation = path.Join(Location, "workspace")
	SecretLocation = path.Join(Location, "secrets")
	TempLocation = path.Join(Location, "tmp")
}

func GetTempWorkerPath(workerIP string) string {
	return path.Join(TempLocation, "workers", workerIP)
}

func GetDatabaseAbsPath() string {
	p, _ := filepath.Abs(path.Join(Location, "data", "maand.db"))
	return p
}
