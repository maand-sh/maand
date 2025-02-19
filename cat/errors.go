package cat

import "fmt"

var ErrNotFound *NotFoundError

type NotFoundError struct {
	Domain string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("no %s found", e.Domain)
}
