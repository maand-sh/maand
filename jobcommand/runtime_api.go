// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

// HTTP routes and headers for the command runtime API.
//
// Python and Bun.js job commands run inside the maand container and call this API on the
// host process to read/write the in-memory KV store and query command demands.
const (
	RuntimeAPIListenAddr = "localhost:8080"
	RuntimeAPIPort         = 8080

	RouteStoreKeys         = "/kv"
	RouteStoreKeysList     = "/kv/keys"
	RouteStoreSecret       = "/kv/secret"
	RouteDemands           = "/demands"
	RouteSemaphoreAcquire  = "/semaphore/acquire"
	RouteSemaphoreRelease  = "/semaphore/release"
	RouteSemaphoreStatus   = "/semaphore/status"

	// EnvJobCommandAPIHost is set on the container exec env so maand.py / maand.ts can reach the API.
	EnvJobCommandAPIHost = "JOB_COMMAND_API_HOST"

	HeaderAllocationID = "X-ALLOCATION-ID"
	HeaderCommandEvent = "EVENT"
	HeaderCommandName  = "COMMAND"
)

// listKeysResponse is returned by GET /kv/keys.
type listKeysResponse struct {
	Namespaces map[string][]string `json:"namespaces"`
}

// storeKeyPayload is the JSON body for GET/PUT/DELETE /kv.
type storeKeyPayload struct {
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
}

// commandDemandPayload is one dependent job command returned by GET /demands.
type commandDemandPayload struct {
	Job          string         `json:"job"`
	Command      string         `json:"command"`
	DemandConfig map[string]any `json:"demand_config"`
}

// semaphoreAcquirePayload is the JSON body for POST /semaphore/acquire.
type semaphoreAcquirePayload struct {
	Name           string `json:"name"`
	Capacity       int    `json:"capacity,omitempty"`
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"`
}

// semaphoreReleasePayload is the JSON body for POST /semaphore/release.
type semaphoreReleasePayload struct {
	Name string `json:"name"`
}

// semaphoreStatusPayload describes current holders for a job/event semaphore.
type semaphoreStatusPayload struct {
	Name      string   `json:"name"`
	Capacity  int      `json:"capacity"`
	Holders   []string `json:"holders"`
	Waiting   int      `json:"waiting"`
	Available int      `json:"available"`
}

// semaphoreAcquireResponse is returned when an allocation acquires a slot.
type semaphoreAcquireResponse struct {
	Name         string `json:"name"`
	AllocationID string `json:"allocation_id"`
	Capacity     int    `json:"capacity"`
	Acquired     bool   `json:"acquired"`
}
