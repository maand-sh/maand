package worker

import (
	"fmt"
	"maand/utils"
	"os"
)

func KeyScan(workerIP string) {
	if os.Getenv("CONTAINER") == "1" {
		cmd := fmt.Sprintf(`ssh-keyscan -H %s >> ~/.ssh/known_hosts`, workerIP)
		err := utils.ExecuteCommand([]string{"mkdir -p ~/.ssh", cmd})
		utils.Check(err)
	}
}
