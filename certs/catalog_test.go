package certs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCertExpiryStatus(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	assert.Equal(t, StatusExpired, CertExpiryStatus(now.Add(-24*time.Hour), 30, now))
	assert.Equal(t, StatusExpiring, CertExpiryStatus(now.Add(10*24*time.Hour), 30, now))
	assert.Equal(t, StatusOK, CertExpiryStatus(now.Add(90*24*time.Hour), 30, now))
}

func TestParseJobWorkerCertNamespace(t *testing.T) {
	job, worker, ok := ParseJobWorkerCertNamespace("maand/job/api/worker/10.0.0.1")
	assert.True(t, ok)
	assert.Equal(t, "api", job)
	assert.Equal(t, "10.0.0.1", worker)

	_, _, ok = ParseJobWorkerCertNamespace("vars/job/api")
	assert.False(t, ok)
}
