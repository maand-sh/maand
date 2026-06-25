package build

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestCertNeedsRenewal(t *testing.T) {
	now := time.Date(2026, 6, 25, 12, 0, 0, 0, time.UTC)

	// Expired.
	assert.True(t, CertNeedsRenewal(now.Add(-24*time.Hour), 30, now))

	// Within 30-day early renewal window (10 days left).
	assert.True(t, CertNeedsRenewal(now.Add(10*24*time.Hour), 30, now))

	// Outside renewal window (90 days left).
	assert.False(t, CertNeedsRenewal(now.Add(90*24*time.Hour), 30, now))

	// buffer=0: not expired yet.
	assert.False(t, CertNeedsRenewal(now.Add(10*24*time.Hour), 0, now))

	// buffer=0: expired.
	assert.True(t, CertNeedsRenewal(now.Add(-1*time.Hour), 0, now))

	// buffer > ttl: 60-day cert with 59 days left and buffer 61 always renews.
	assert.True(t, CertNeedsRenewal(now.Add(59*24*time.Hour), 61, now))
}
