package cat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatJobCPU(t *testing.T) {
	assert.Equal(t, "2000 mhz (manifest)", formatJobCPU("2000", "manifest"))
	assert.Equal(t, "512 mhz (bucket.jobs.prod.conf)", formatJobCPU("512", "bucket.jobs.prod.conf"))
	assert.Equal(t, "0 mhz (manifest)", formatJobCPU("", ""))
}

func TestFormatJobMemory(t *testing.T) {
	assert.Equal(t, "192 mb (bucket.jobs.conf)", formatJobMemory("192", "bucket.jobs.conf"))
	assert.Equal(t, "256 mb (manifest)", formatJobMemory("256", "manifest"))
	assert.Equal(t, "0 mb (manifest)", formatJobMemory("0", ""))
}
