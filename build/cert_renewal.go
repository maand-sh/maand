// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package build

import "time"

// CertNeedsRenewal reports whether a leaf certificate should be regenerated.
// When renewalBufferDays > 0, renewal starts when now reaches NotAfter minus that
// many days (early renewal window). When renewalBufferDays is 0, renewal starts
// only after NotAfter has passed.
func CertNeedsRenewal(notAfter time.Time, renewalBufferDays int, now time.Time) bool {
	notAfter = notAfter.UTC()
	now = now.UTC()
	if !now.Before(notAfter) {
		return true
	}
	if renewalBufferDays <= 0 {
		return false
	}
	renewalStart := notAfter.Add(-time.Duration(renewalBufferDays) * 24 * time.Hour)
	return !now.Before(renewalStart)
}
