// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package job_command

import (
	"fmt"
	"net/http"
)

type JobCommandNotFoundError struct {
	Job     string
	Command string
	Event   string
}

func (e *JobCommandNotFoundError) Error() string {
	return fmt.Sprintf("Job '%s' command '%s' event '%s' not found", e.Job, e.Command, e.Event)
}

type JobCommandError struct {
	Job     string
	Command string
	Err     map[string]error
}

func (e *JobCommandError) Error() string {
	return fmt.Sprintf("Job '%s' command '%s' failed", e.Job, e.Command)
}

type httpError struct {
	Message string
	Code    int
}

func (e *httpError) Write(w http.ResponseWriter) {
	http.Error(w, e.Message, e.Code)
}

var httpErrors = struct {
	InvalidContentType *httpError
	MissingAllocID     *httpError
	InvalidAllocID     *httpError
	BadRequestBody     *httpError
	InvalidFormat      *httpError
	MissingFields      *httpError
	InvalidNamespace   *httpError
	KVStoreFailure     *httpError
	KVNotFound         *httpError
}{
	InvalidContentType: &httpError{Message: "Content-Type must be application/json", Code: http.StatusUnsupportedMediaType},
	MissingAllocID:     &httpError{Message: "X-ALLOCATION-ID header is missing", Code: http.StatusUnsupportedMediaType},
	InvalidAllocID:     &httpError{Message: "Invalid allocation ID", Code: http.StatusNotFound},
	BadRequestBody:     &httpError{Message: "Failed to read request body", Code: http.StatusBadRequest},
	InvalidFormat:      &httpError{Message: "Invalid JSON format", Code: http.StatusBadRequest},
	MissingFields:      &httpError{Message: "Both namespace and key are required", Code: http.StatusBadRequest},
	InvalidNamespace:   &httpError{Message: "Invalid or unauthorized namespace", Code: http.StatusBadRequest},
	KVStoreFailure:     &httpError{Message: "KV store operation failed", Code: http.StatusInternalServerError},
	KVNotFound:         &httpError{Message: "KV get operation failed", Code: http.StatusNotFound},
}
