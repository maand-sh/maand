package deploy

import (
	"fmt"
	"maand/bucket"
	"maand/utils"
	"os"
	"os/exec"
	"path"
	"path/filepath"
)

func rsync(bucketID, workerIP string) error {
	conf := utils.GetMaandConf()
	user := conf.SSHUser
	keyFilePath, _ := filepath.Abs(path.Join(bucket.SecretLocation, conf.SSHKeyFile))
	useSUDO := conf.UseSUDO

	rs := "rsync"

	remoteRS := "/usr/bin/rsync"
	if useSUDO {
		remoteRS = "sudo /usr/bin/rsync"
	}

	ruleFilePath := path.Join(bucket.TempLocation, "workers", fmt.Sprintf("%s.rsync", workerIP))
	workerDir := path.Join(bucket.TempLocation, "workers", workerIP)

	rsOptions := []string{
		"--timeout=30",
		"--inplace",
		"--whole-file",
		"--checksum",
		"--recursive",
		"--force",
		"--delete-after",
		"--delete",
		"--group",
		"--owner",
		"--executability",
		"--compress",
		"--verbose",
		"--exclude=jobs/*/bin",
		"--exclude=jobs/*/data",
		"--exclude=jobs/*/logs",
		"--exclude=jobs/*/_modules",
		fmt.Sprintf("--rsync-path=%s", remoteRS),
		fmt.Sprintf("--filter=merge %s", ruleFilePath),
		fmt.Sprintf("--rsh=ssh -o BatchMode=true -o ConnectTimeout=10 -i '%s'", keyFilePath),
		fmt.Sprintf("%s/", workerDir),
		fmt.Sprintf("%s@%s:/opt/worker/%s", user, workerIP, bucketID),
	}

	cmd := exec.Command(rs, rsOptions...)
	cmd.Stdout = nil
	cmd.Stderr = os.Stderr

	return cmd.Run()
}
