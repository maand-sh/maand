// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package health_check

import "fmt"

type HealthCheckError struct {
	Job string
	Err error
}

func (e *HealthCheckError) Error() string {
	return fmt.Sprintf("health check failed: %v", e.Err)
}

func (e *HealthCheckError) Unwrap() error {
	return e.Err
}
