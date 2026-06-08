// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package bucket

import "os"

// TestMode reports whether MAAND_TEST=1 (set by the tests package TestMain).
func TestMode() bool {
	return os.Getenv("MAAND_TEST") == "1"
}

// QuietCLIOutput reports whether CLI table/init chatter should be suppressed.
func QuietCLIOutput() bool {
	return os.Getenv("MAAND_QUIET") == "1" || TestMode()
}
