package certs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCertNeedsRenewal(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)

	assert.True(t, CertNeedsRenewal(now.Add(-24*time.Hour), 30, now))
	assert.True(t, CertNeedsRenewal(now.Add(10*24*time.Hour), 30, now))
	assert.False(t, CertNeedsRenewal(now.Add(90*24*time.Hour), 30, now))

	assert.False(t, CertNeedsRenewal(now.Add(10*24*time.Hour), 0, now))
	assert.True(t, CertNeedsRenewal(now.Add(-1*time.Hour), 0, now))

	assert.True(t, CertNeedsRenewal(now.Add(59*24*time.Hour), 61, now))
}
