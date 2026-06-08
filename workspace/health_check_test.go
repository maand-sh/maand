package workspace

import (
	"testing"

	"maand/bucket"

	"github.com/stretchr/testify/assert"
)

func TestValidateHealthCheck(t *testing.T) {
	manifest := Manifest{
		Resources: struct {
			Memory struct {
				Min string `json:"min"`
				Max string `json:"max"`
			} `json:"memory"`
			CPU struct {
				Min string `json:"min"`
				Max string `json:"max"`
			} `json:"cpu"`
			Ports ManifestPorts `json:"ports"`
		}{
			Ports: ManifestPorts{"api_port": ManifestPortBinding{}},
		},
		HealthCheck: &ManifestHealthCheck{
			Checks: []HealthCheckProbe{
				{Type: "tcp", Port: "api_port"},
				{Type: "http", Port: "api_port", Path: "/health"},
			},
		},
	}
	assert.NoError(t, ValidateHealthCheck("api", manifest))

	manifest.HealthCheck.Checks[0].Port = "missing"
	assert.ErrorIs(t, ValidateHealthCheck("api", manifest), bucket.ErrInvalidManifest)

	sshManifest := Manifest{
		HealthCheck: &ManifestHealthCheck{
			Checks: []HealthCheckProbe{
				{Type: "ssh", Command: "systemctl is-active cassandra"},
			},
		},
	}
	assert.NoError(t, ValidateHealthCheck("cassandra", sshManifest))

	sshManifest.HealthCheck.Checks[0].Command = ""
	assert.ErrorIs(t, ValidateHealthCheck("cassandra", sshManifest), bucket.ErrInvalidManifest)
}

func TestValidateHealthCheck_rejectsManifestAndCommand(t *testing.T) {
	manifest := Manifest{
		Resources: struct {
			Memory struct {
				Min string `json:"min"`
				Max string `json:"max"`
			} `json:"memory"`
			CPU struct {
				Min string `json:"min"`
				Max string `json:"max"`
			} `json:"cpu"`
			Ports ManifestPorts `json:"ports"`
		}{
			Ports: ManifestPorts{"api_port": ManifestPortBinding{}},
		},
		HealthCheck: &ManifestHealthCheck{
			Checks: []HealthCheckProbe{{Type: "tcp", Port: "api_port"}},
		},
		Commands: map[string]JobCommand{
			"command_health": {
				Name:       "command_health",
				ExecutedOn: []string{"health_check"},
			},
		},
	}
	assert.ErrorIs(t, ValidateHealthCheck("api", manifest), bucket.ErrInvalidManifest)
}
