// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"fmt"
	"os"
	"path"

	"maand/bucket"
)

func runCommandOnWorker(
	rt *bucket.Runtime,
	allocationID, jobName, workerIP, allocationIndex string,
	disabled int,
	commandName, event string,
	verbose bool,
	extraEnv []string,
) error {
	workerDir := bucket.GetTempWorkerPath(workerIP)
	moduleDir := path.Join(workerDir, "jobs", jobName, "_modules")

	runtime, scriptPath, err := ResolveCommandScript(moduleDir, commandName)
	if err != nil {
		return err
	}

	env := buildCommandEnv(allocationID, jobName, workerIP, allocationIndex, disabled, commandName, event, extraEnv)
	return rt.Exec(workerIP, CommandExecLines(moduleDir, scriptPath, runtime, jobName), env, verbose)
}

func buildCommandEnv(
	allocationID, jobName, workerIP, allocationIndex string,
	disabled int,
	commandName, event string,
	extraEnv []string,
) []string {
	env := append([]string{}, os.Environ()...)
	env = append(env, extraEnv...)
	env = append(env,
		fmt.Sprintf("ALLOCATION_ID=%s", allocationID),
		fmt.Sprintf("ALLOCATION_IP=%s", workerIP),
		fmt.Sprintf("ALLOCATION_INDEX=%s", allocationIndex),
		fmt.Sprintf("DISABLED=%d", disabled),
		fmt.Sprintf("JOB=%s", jobName),
		fmt.Sprintf("EVENT=%s", event),
		fmt.Sprintf("COMMAND=%s", commandName),
		fmt.Sprintf("%s=%s", EnvJobCommandAPIHost, runtimeAPIHost()),
	)
	return env
}

// runtimeAPIHost is the hostname job command scripts use to reach StartRuntimeAPI on the host.
func runtimeAPIHost() string {
	return "127.0.0.1"
}

func commandAllowed(allowed []string, commandName string) bool {
	for _, name := range allowed {
		if name == commandName {
			return true
		}
	}
	return false
}
