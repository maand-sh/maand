// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package workspace

// MinMemory returns the manifest minimum memory requirement with a default.
func (m Manifest) MinMemory() string {
	if m.Resources.Memory.Min == "" {
		return "0 mb"
	}
	return m.Resources.Memory.Min
}

// MaxMemory returns the manifest maximum memory requirement with a default.
func (m Manifest) MaxMemory() string {
	if m.Resources.Memory.Max == "" {
		return "0 mb"
	}
	return m.Resources.Memory.Max
}

// MinCPU returns the manifest minimum CPU requirement with a default.
func (m Manifest) MinCPU() string {
	if m.Resources.CPU.Min == "" {
		return "0 mhz"
	}
	return m.Resources.CPU.Min
}

// MaxCPU returns the manifest maximum CPU requirement with a default.
func (m Manifest) MaxCPU() string {
	if m.Resources.CPU.Max == "" {
		return "0 mhz"
	}
	return m.Resources.CPU.Max
}

// JobVersion returns the manifest version with a default.
func (m Manifest) JobVersion() string {
	if m.Version == "" {
		return "unknown"
	}
	return m.Version
}

// ParallelDeployCount returns first-deploy batch size; 0 means all allocations in one batch.
func (m Manifest) ParallelDeployCount() int {
	if m.DeployParallelCount < 0 {
		return 0
	}
	return m.DeployParallelCount
}

// ParallelUpdateCount returns a positive rollout parallelism (minimum 1).
func (m Manifest) ParallelUpdateCount() int {
	if m.UpdateParallelCount <= 0 {
		return 1
	}
	return m.UpdateParallelCount
}

// ListedCommands returns manifest commands with names filled from map keys.
func (m Manifest) ListedCommands() []JobCommand {
	commands := make([]JobCommand, 0, len(m.Commands))
	for name, command := range m.Commands {
		command.Name = name
		if command.Demands.Config == nil {
			command.Demands.Config = make(map[string]interface{})
		}
		commands = append(commands, command)
	}
	return commands
}

// GetMinMemory is deprecated; use Manifest.MinMemory.
func GetMinMemory(manifest Manifest) string {
	return manifest.MinMemory()
}

// GetMaxMemory is deprecated; use Manifest.MaxMemory.
func GetMaxMemory(manifest Manifest) string {
	return manifest.MaxMemory()
}

// GetMinCPU is deprecated; use Manifest.MinCPU.
func GetMinCPU(manifest Manifest) string {
	return manifest.MinCPU()
}

// GetMaxCPU is deprecated; use Manifest.MaxCPU.
func GetMaxCPU(manifest Manifest) string {
	return manifest.MaxCPU()
}

// GetVersion is deprecated; use Manifest.JobVersion.
func GetVersion(manifest Manifest) string {
	return manifest.JobVersion()
}

// GetCommands is deprecated; use Manifest.ListedCommands.
func GetCommands(manifest Manifest) []JobCommand {
	return manifest.ListedCommands()
}

// GetDeployParallelCount returns manifest deploy_parallel_count (0 = all-at-once on first deploy).
func GetDeployParallelCount(manifest Manifest) int {
	return manifest.ParallelDeployCount()
}

// GetUpdateParallelCount is deprecated; use Manifest.ParallelUpdateCount.
func GetUpdateParallelCount(manifest Manifest) int {
	return manifest.ParallelUpdateCount()
}
