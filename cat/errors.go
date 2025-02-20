// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package cat

import "fmt"

var ErrNotFound *NotFoundError

type NotFoundError struct {
	Domain string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no %s found", e.Domain)
}
