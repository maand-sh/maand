package data

import (
	"fmt"
	"runtime"
)

type DatabaseError struct {
	Err   error
	Stack string
}

func (e *DatabaseError) Error() string {
	return fmt.Sprintf("database error: %v\nStack trace:\n%s", e.Err, e.Stack)
}

func (e *DatabaseError) Unwrap() error {
	return e.Err
}

func NewDatabaseError(err error) error {
	stackBuf := make([]byte, 4096) // Allocate buffer for stack trace
	n := runtime.Stack(stackBuf, false)
	return &DatabaseError{
		Err:   err,
		Stack: string(stackBuf[:n]),
	}
}
