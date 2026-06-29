// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workerfacts

import (
	"fmt"
	"strconv"
	"strings"

	"maand/workspace"
)

const probeScript = `set -eu
mem_mb=$(( ($(awk '/MemTotal/ {print $2; exit}' /proc/meminfo) + 1023) / 1024 ))
cpus=$(nproc 2>/dev/null || getconf _NPROCESSORS_ONLN)
mhz=$(awk -F: '/cpu MHz/{gsub(/^[ \t]+/,"",$2); print int($2); exit}' /proc/cpuinfo)
if [ -z "${mhz}" ] || [ "${mhz}" = 0 ]; then
  if command -v lscpu >/dev/null 2>&1; then
    mhz=$(lscpu 2>/dev/null | awk -F: '/CPU max MHz/{gsub(/^[ \t]+/,"",$2); print int($2); exit}')
  fi
fi
if [ -z "${mhz}" ] || [ "${mhz}" = 0 ]; then
  echo "CPU_MHZ=0" >&2
  exit 1
fi
cpu_mhz=$(( cpus * mhz ))
printf 'MEMORY_MB=%s\nCPU_MHZ=%s\n' "${mem_mb}" "${cpu_mhz}"
`

type probeOutput struct {
	MemoryMB float64
	CPUMHz   float64
}

func parseProbeOutput(output string) (probeOutput, error) {
	values := make(map[string]string)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.ToUpper(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}

	memoryRaw, ok := values["MEMORY_MB"]
	if !ok || memoryRaw == "" {
		return probeOutput{}, fmt.Errorf("probe output missing MEMORY_MB")
	}
	memoryMB, err := strconv.ParseFloat(memoryRaw, 64)
	if err != nil {
		return probeOutput{}, fmt.Errorf("parse MEMORY_MB %q: %w", memoryRaw, err)
	}
	if memoryMB <= 0 {
		return probeOutput{}, fmt.Errorf("probe reported invalid memory %s", memoryRaw)
	}

	cpuRaw, ok := values["CPU_MHZ"]
	if !ok || cpuRaw == "" {
		return probeOutput{}, fmt.Errorf("probe output missing CPU_MHZ")
	}
	cpuMHz, err := strconv.ParseFloat(cpuRaw, 64)
	if err != nil {
		return probeOutput{}, fmt.Errorf("parse CPU_MHZ %q: %w", cpuRaw, err)
	}
	if cpuMHz <= 0 {
		return probeOutput{}, fmt.Errorf("probe reported invalid cpu %s", cpuRaw)
	}

	return probeOutput{
		MemoryMB: memoryMB,
		CPUMHz:   cpuMHz,
	}, nil
}

func toWorkerFacts(probe probeOutput) workspace.WorkerFacts {
	return workspace.WorkerFacts{
		MemoryMB: probe.MemoryMB,
		CPUMHz:   probe.CPUMHz,
	}
}
