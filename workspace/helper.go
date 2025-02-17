package workspace

import (
	"crypto/md5"
	"github.com/google/uuid"
)

func GetMinMemory(manifest Manifest) string {
	if manifest.Resources.Memory.Min == "" {
		manifest.Resources.Memory.Min = "0 mb"
	}
	return manifest.Resources.Memory.Min
}

func GetMaxMemory(manifest Manifest) string {
	if manifest.Resources.Memory.Max == "" {
		manifest.Resources.Memory.Max = "0 mb"
	}
	return manifest.Resources.Memory.Max
}

func GetMinCPU(manifest Manifest) string {
	if manifest.Resources.CPU.Min == "" {
		manifest.Resources.CPU.Min = "0 mhz"
	}
	return manifest.Resources.CPU.Min
}

func GetMaxCPU(manifest Manifest) string {
	if manifest.Resources.CPU.Max == "" {
		manifest.Resources.CPU.Max = "0 mhz"
	}
	return manifest.Resources.CPU.Max
}

func GetVersion(manifest Manifest) string {
	if manifest.Version == "" {
		manifest.Version = "unknown"
	}
	return manifest.Version
}

func GetCommands(manifest Manifest) []AllocationCommand {
	var commands []AllocationCommand
	for name, command := range manifest.Commands {
		command.Name = name
		if command.DependsOn.Config == nil {
			command.DependsOn.Config = make(map[string]interface{})
		}
		commands = append(commands, command)
	}
	return commands
}

func GetHashUUID(value string) string {
	hash := md5.Sum([]byte(value))
	hash[6] = (hash[6] & 0x0f) | 0x30 // Version 3
	hash[8] = (hash[8] & 0x3f) | 0x80 // Variant

	return uuid.UUID(hash).String()
}

func GetUpdateParallelCount(manifest Manifest) int {
	if manifest.UpdateParallelCount <= 0 {
		manifest.UpdateParallelCount = 1
	}
	return manifest.UpdateParallelCount
}
