// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"maand/bucket"
	"maand/kv"
)


func serveStoreKeys(tx *sql.Tx) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleStoreKeyGet(w, r, tx)
		case http.MethodPut:
			handleStoreKeyPut(w, r, tx)
		case http.MethodDelete:
			handleStoreKeyDelete(w, r, tx)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func serveStoreKeysList(tx *sql.Tx) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleStoreKeysList(w, r, tx)
	}
}

func serveStoreSecret(tx *sql.Tx) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			handleStoreSecretPut(w, r, tx)
		case http.MethodDelete:
			handleStoreSecretDelete(w, r, tx)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func serveCommandDemands(tx *sql.Tx) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleCommandDemandsGet(w, r, tx)
	}
}

// resolveAllocationFromRequest loads job and worker for the allocation named in X-ALLOCATION-ID.
func resolveAllocationFromRequest(w http.ResponseWriter, r *http.Request, tx *sql.Tx) (jobName, workerIP string, body io.ReadCloser, err error) {
	jobName, workerIP, _, body, err = resolveAllocationFromRequestWithID(w, r, tx)
	return jobName, workerIP, body, err
}

func resolveAllocationFromRequestWithID(
	w http.ResponseWriter,
	r *http.Request,
	tx *sql.Tx,
) (jobName, workerIP, allocationID string, body io.ReadCloser, err error) {
	allocationID = r.Header.Get(HeaderAllocationID)
	if allocationID == "" {
		runtimeAPIErrors.missingAllocation.write(w)
		return "", "", "", nil, fmt.Errorf("missing header %s", HeaderAllocationID)
	}

	err = tx.QueryRow(
		`SELECT job, worker_ip FROM allocations WHERE alloc_id = ?`,
		allocationID,
	).Scan(&jobName, &workerIP)
	if errors.Is(err, sql.ErrNoRows) {
		runtimeAPIErrors.unknownAllocation.write(w)
		return "", "", "", nil, fmt.Errorf("unknown allocation %s", allocationID)
	}
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return "", "", "", nil, bucket.DatabaseError(err)
	}

	return jobName, workerIP, allocationID, r.Body, nil
}

func handleStoreKeyGet(w http.ResponseWriter, r *http.Request, tx *sql.Tx) {
	jobName, workerIP, body, err := resolveAllocationFromRequest(w, r, tx)
	if err != nil {
		return
	}
	defer func() {
		_ = body.Close()
	}()

	var payload storeKeyPayload
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		writeJSONDecodeError(w, err)
		return
	}

	if apiErr := validateStoreKeyPayload(tx, payload, jobName, workerIP, false); apiErr != nil {
		apiErr.write(w)
		return
	}

	item, err := kv.GetKVStore().Get(payload.Namespace, payload.Key)
	if err != nil {
		log.Printf("runtime api store get: %v", err)
		runtimeAPIErrors.storeKeyNotFound.write(w)
		return
	}

	value := item.Value
	if kv.IsSecretNamespace(payload.Namespace) {
		plaintext, decryptErr := kv.DecryptStoredValue(item.Value)
		if decryptErr != nil {
			log.Printf("runtime api store get decrypt: %v", decryptErr)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		value = plaintext
	}

	writeJSONResponse(w, http.StatusOK, storeKeyPayload{
		Namespace: payload.Namespace,
		Key:       payload.Key,
		Value:     value,
	})
}

func handleStoreKeyPut(w http.ResponseWriter, r *http.Request, tx *sql.Tx) {
	jobName, workerIP, body, err := resolveAllocationFromRequest(w, r, tx)
	if err != nil {
		return
	}
	defer func() {
		_ = body.Close()
	}()

	var payload storeKeyPayload
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		writeJSONDecodeError(w, err)
		return
	}

	if apiErr := validateStoreKeyPayload(tx, payload, jobName, workerIP, true); apiErr != nil {
		apiErr.write(w)
		return
	}

	if kv.IsSecretNamespace(payload.Namespace) {
		runtimeAPIErrors.namespaceDenied.write(w)
		return
	}

	if r.Header.Get(HeaderCommandEvent) == "health_check" {
		log.Printf("runtime api: blocked store put during health_check for job %s", jobName)
		runtimeAPIErrors.writeDuringHealth.write(w)
		return
	}

	kv.GetKVStore().Put(payload.Namespace, payload.Key, payload.Value, 0)
	w.WriteHeader(http.StatusOK)
}

func handleStoreKeyDelete(w http.ResponseWriter, r *http.Request, tx *sql.Tx) {
	jobName, workerIP, body, err := resolveAllocationFromRequest(w, r, tx)
	if err != nil {
		return
	}
	defer func() {
		_ = body.Close()
	}()

	var payload storeKeyPayload
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		writeJSONDecodeError(w, err)
		return
	}

	if payload.Namespace == "" || payload.Key == "" {
		runtimeAPIErrors.missingKeyFields.write(w)
		return
	}

	if kv.IsSecretNamespace(payload.Namespace) {
		runtimeAPIErrors.namespaceDenied.write(w)
		return
	}

	expectedNamespace := fmt.Sprintf("vars/job/%s", jobName)
	if payload.Namespace != expectedNamespace {
		runtimeAPIErrors.namespaceDenied.write(w)
		return
	}

	if apiErr := validateReadNamespace(tx, payload.Namespace, jobName, workerIP); apiErr != nil {
		apiErr.write(w)
		return
	}

	if blockedWriteDuringHealthCheck(w, r, jobName) {
		return
	}

	if err := kv.GetKVStore().Delete(payload.Namespace, payload.Key); err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			runtimeAPIErrors.storeKeyNotFound.write(w)
			return
		}
		log.Printf("runtime api store delete: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func handleStoreKeysList(w http.ResponseWriter, r *http.Request, tx *sql.Tx) {
	jobName, workerIP, body, err := resolveAllocationFromRequest(w, r, tx)
	if err != nil {
		return
	}
	defer func() {
		_ = body.Close()
	}()

	var payload storeKeyPayload
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		writeJSONDecodeError(w, err)
		return
	}

	namespaces := jobLevelKVNamespaces(jobName)
	if payload.Namespace != "" {
		if !isJobLevelListNamespace(payload.Namespace, jobName) {
			runtimeAPIErrors.namespaceDenied.write(w)
			return
		}
		if apiErr := validateReadNamespace(tx, payload.Namespace, jobName, workerIP); apiErr != nil {
			apiErr.write(w)
			return
		}
		namespaces = []string{payload.Namespace}
	}

	store := kv.GetKVStore()
	result := make(map[string][]string, len(namespaces))
	for _, namespace := range namespaces {
		keys, err := store.GetKeys(namespace)
		if err != nil {
			log.Printf("runtime api store list keys: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		result[namespace] = keys
	}

	writeJSONResponse(w, http.StatusOK, listKeysResponse{Namespaces: result})
}

func handleStoreSecretPut(w http.ResponseWriter, r *http.Request, tx *sql.Tx) {
	jobName, workerIP, body, err := resolveAllocationFromRequest(w, r, tx)
	if err != nil {
		return
	}
	defer func() {
		_ = body.Close()
	}()

	var payload storeKeyPayload
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		writeJSONDecodeError(w, err)
		return
	}

	if apiErr := validateStoreSecretPayload(payload, jobName, workerIP); apiErr != nil {
		apiErr.write(w)
		return
	}

	if r.Header.Get(HeaderCommandEvent) == "health_check" {
		log.Printf("runtime api: blocked secret put during health_check for job %s", jobName)
		runtimeAPIErrors.writeDuringHealth.write(w)
		return
	}

	if err := kv.GetKVStore().PutSecret(payload.Namespace, payload.Key, payload.Value, 0); err != nil {
		log.Printf("runtime api store secret put: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func handleStoreSecretDelete(w http.ResponseWriter, r *http.Request, tx *sql.Tx) {
	jobName, workerIP, body, err := resolveAllocationFromRequest(w, r, tx)
	if err != nil {
		return
	}
	defer func() {
		_ = body.Close()
	}()

	var payload storeKeyPayload
	if err := json.NewDecoder(body).Decode(&payload); err != nil {
		writeJSONDecodeError(w, err)
		return
	}

	if apiErr := validateStoreSecretKeyPayload(payload, jobName, workerIP); apiErr != nil {
		apiErr.write(w)
		return
	}

	if blockedWriteDuringHealthCheck(w, r, jobName) {
		return
	}

	if err := kv.GetKVStore().Delete(payload.Namespace, payload.Key); err != nil {
		if errors.Is(err, kv.ErrNotFound) {
			runtimeAPIErrors.storeKeyNotFound.write(w)
			return
		}
		log.Printf("runtime api store secret delete: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func blockedWriteDuringHealthCheck(w http.ResponseWriter, r *http.Request, jobName string) bool {
	if r.Header.Get(HeaderCommandEvent) != "health_check" {
		return false
	}
	log.Printf("runtime api: blocked kv write during health_check for job %s", jobName)
	runtimeAPIErrors.writeDuringHealth.write(w)
	return true
}

func jobLevelKVNamespaces(jobName string) []string {
	return []string{
		fmt.Sprintf("vars/job/%s", jobName),
		kv.SecretJobNamespace(jobName),
	}
}

func handleCommandDemandsGet(w http.ResponseWriter, r *http.Request, tx *sql.Tx) {
	jobName, _, body, err := resolveAllocationFromRequest(w, r, tx)
	if err != nil {
		return
	}
	defer func() {
		_ = body.Close()
	}()

	commandName := r.Header.Get(HeaderCommandName)
	rows, err := tx.Query(`
		SELECT job AS requester_job,
		       name AS requester_job_command,
		       demand_config
		FROM job_commands
		WHERE demand_job = ? AND demand_command = ?`,
		jobName, commandName,
	)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer func() {
		_ = rows.Close()
	}()

	demands := make([]commandDemandPayload, 0)
	for rows.Next() {
		var entry commandDemandPayload
		var configJSON string
		if err := rows.Scan(&entry.Job, &entry.Command, &configJSON); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if err := json.Unmarshal([]byte(configJSON), &entry.DemandConfig); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		demands = append(demands, entry)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	writeJSONResponse(w, http.StatusOK, demands)
}
