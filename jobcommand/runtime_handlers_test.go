// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"maand/bucket"
	"maand/kv"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mattn/go-sqlite3"
)

func setupRuntimeHandlerTest(t *testing.T) (*sql.Tx, string) {
	t.Helper()

	root := t.TempDir()
	bucket.Location = root
	bucket.SecretLocation = path.Join(root, "secrets")
	require.NoError(t, os.MkdirAll(bucket.SecretLocation, 0o755))
	require.NoError(t, kv.EnsureEncryptionKey())
	kv.ResetEncryptionKeyCacheForTest()

	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		CREATE TABLE allocations (
			alloc_id TEXT PRIMARY KEY, worker_ip TEXT, job TEXT,
			disabled INT, removed INT, deployment_seq INT
		);
		CREATE TABLE job_commands (
			job TEXT, name TEXT, executed_on TEXT,
			demand_job TEXT, demand_command TEXT, demand_config TEXT
		);
		CREATE TABLE key_value (
			key TEXT, value TEXT, namespace TEXT, version INT,
			ttl INT, created_date INT, deleted INT
		);
	`)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO allocations (alloc_id, worker_ip, job, disabled, removed, deployment_seq)
		 VALUES ('alloc-1', '10.0.0.1', 'api', 0, 0, 0)`,
	)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO job_commands (job, name, executed_on, demand_job, demand_command, demand_config)
		 VALUES ('other', 'migrate', 'pre_deploy', 'api', 'seed', '{}')`,
	)
	require.NoError(t, err)

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, kv.Initialize(tx))

	return tx, "alloc-1"
}

func runtimeRequest(t *testing.T, tx *sql.Tx, method, route string, body any, event string) *httptest.ResponseRecorder {
	t.Helper()
	mux := newRuntimeAPIMux(&runtimeAPIContext{tx: tx, semaphores: newSemaphoreCoordinator()})

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		require.NoError(t, err)
	}

	req := httptest.NewRequest(method, route, bytes.NewReader(payload))
	req.Header.Set(HeaderAllocationID, "alloc-1")
	req.Header.Set(HeaderCommandName, "seed")
	req.Header.Set(HeaderCommandEvent, event)
	req.Header.Set("Content-Type", "application/json")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestRuntimeAPI_kvPutGetDelete(t *testing.T) {
	tx, _ := setupRuntimeHandlerTest(t)

	put := runtimeRequest(t, tx, http.MethodPut, RouteStoreKeys, storeKeyPayload{
		Namespace: "vars/job/api",
		Key:       "url",
		Value:     "postgres://db",
	}, "pre_deploy")
	require.Equal(t, http.StatusOK, put.Code)

	get := runtimeRequest(t, tx, http.MethodGet, RouteStoreKeys, storeKeyPayload{
		Namespace: "vars/job/api",
		Key:       "url",
	}, "pre_deploy")
	require.Equal(t, http.StatusOK, get.Code)

	var got storeKeyPayload
	require.NoError(t, json.Unmarshal(get.Body.Bytes(), &got))
	assert.Equal(t, "postgres://db", got.Value)

	list := runtimeRequest(t, tx, http.MethodGet, RouteStoreKeysList, storeKeyPayload{}, "pre_deploy")
	require.Equal(t, http.StatusOK, list.Code)

	var listed listKeysResponse
	require.NoError(t, json.Unmarshal(list.Body.Bytes(), &listed))
	assert.Contains(t, listed.Namespaces["vars/job/api"], "url")

	del := runtimeRequest(t, tx, http.MethodDelete, RouteStoreKeys, storeKeyPayload{
		Namespace: "vars/job/api",
		Key:       "url",
	}, "pre_deploy")
	require.Equal(t, http.StatusOK, del.Code)

	missing := runtimeRequest(t, tx, http.MethodGet, RouteStoreKeys, storeKeyPayload{
		Namespace: "vars/job/api",
		Key:       "url",
	}, "pre_deploy")
	assert.Equal(t, http.StatusNotFound, missing.Code)
}

func TestRuntimeAPI_kvSecretPutGetDelete(t *testing.T) {
	tx, _ := setupRuntimeHandlerTest(t)

	put := runtimeRequest(t, tx, http.MethodPut, RouteStoreSecret, storeKeyPayload{
		Namespace: "secrets/job/api",
		Key:       "token",
		Value:     "plain-secret",
	}, "post_deploy")
	require.Equal(t, http.StatusOK, put.Code)

	get := runtimeRequest(t, tx, http.MethodGet, RouteStoreKeys, storeKeyPayload{
		Namespace: "secrets/job/api",
		Key:       "token",
	}, "post_deploy")
	require.Equal(t, http.StatusOK, get.Code)

	var got storeKeyPayload
	require.NoError(t, json.Unmarshal(get.Body.Bytes(), &got))
	assert.Equal(t, "plain-secret", got.Value)

	del := runtimeRequest(t, tx, http.MethodDelete, RouteStoreSecret, storeKeyPayload{
		Namespace: "secrets/job/api",
		Key:       "token",
	}, "post_deploy")
	require.Equal(t, http.StatusOK, del.Code)
}

func TestRuntimeAPI_deployOrderPutGet(t *testing.T) {
	tx, _ := setupRuntimeHandlerTest(t)

	put := runtimeRequest(t, tx, http.MethodPut, RouteStoreKeys, storeKeyPayload{
		Namespace: "maand/job/api",
		Key:       "deploy_order",
		Value:     "10.0.0.2,10.0.0.1",
	}, "pre_deploy")
	require.Equal(t, http.StatusOK, put.Code)

	get := runtimeRequest(t, tx, http.MethodGet, RouteStoreKeys, storeKeyPayload{
		Namespace: "maand/job/api",
		Key:       "deploy_order",
	}, "pre_deploy")
	require.Equal(t, http.StatusOK, get.Code)

	var got storeKeyPayload
	require.NoError(t, json.Unmarshal(get.Body.Bytes(), &got))
	assert.Equal(t, "10.0.0.2,10.0.0.1", got.Value)
}

func TestRuntimeAPI_deployOrderWriteDeniedWrongEvent(t *testing.T) {
	tx, _ := setupRuntimeHandlerTest(t)

	put := runtimeRequest(t, tx, http.MethodPut, RouteStoreKeys, storeKeyPayload{
		Namespace: "maand/job/api",
		Key:       "deploy_order",
		Value:     "10.0.0.1",
	}, "post_deploy")
	assert.Equal(t, http.StatusBadRequest, put.Code)
}

func TestRuntimeAPI_deployOrderWriteDeniedWrongKey(t *testing.T) {
	tx, _ := setupRuntimeHandlerTest(t)

	put := runtimeRequest(t, tx, http.MethodPut, RouteStoreKeys, storeKeyPayload{
		Namespace: "maand/job/api",
		Key:       "workers",
		Value:     "10.0.0.1",
	}, "pre_deploy")
	assert.Equal(t, http.StatusBadRequest, put.Code)
}

func TestRuntimeAPI_kvWriteBlockedDuringHealthCheck(t *testing.T) {
	tx, _ := setupRuntimeHandlerTest(t)

	put := runtimeRequest(t, tx, http.MethodPut, RouteStoreKeys, storeKeyPayload{
		Namespace: "vars/job/api",
		Key:       "x",
		Value:     "y",
	}, "health_check")
	assert.Equal(t, http.StatusBadRequest, put.Code)
	assert.Contains(t, put.Body.String(), "health_check")

	secret := runtimeRequest(t, tx, http.MethodPut, RouteStoreSecret, storeKeyPayload{
		Namespace: "secrets/job/api",
		Key:       "x",
		Value:     "y",
	}, "health_check")
	assert.Equal(t, http.StatusBadRequest, secret.Code)

	del := runtimeRequest(t, tx, http.MethodDelete, RouteStoreKeys, storeKeyPayload{
		Namespace: "vars/job/api",
		Key:       "x",
	}, "health_check")
	assert.Equal(t, http.StatusBadRequest, del.Code)

	deployOrder := runtimeRequest(t, tx, http.MethodPut, RouteStoreKeys, storeKeyPayload{
		Namespace: "maand/job/api",
		Key:       "deploy_order",
		Value:     "10.0.0.1",
	}, "health_check")
	assert.Equal(t, http.StatusBadRequest, deployOrder.Code)
}

func TestRuntimeAPI_demandsGet(t *testing.T) {
	tx, _ := setupRuntimeHandlerTest(t)

	rec := runtimeRequest(t, tx, http.MethodGet, RouteDemands, storeKeyPayload{}, "pre_deploy")
	require.Equal(t, http.StatusOK, rec.Code)

	var demands []commandDemandPayload
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &demands))
	require.Len(t, demands, 1)
	assert.Equal(t, "other", demands[0].Job)
	assert.Equal(t, "migrate", demands[0].Command)
}
