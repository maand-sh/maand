// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"maand/data"
	"maand/kv"
	"maand/utils"
)

func writeJSONDecodeError(w http.ResponseWriter, err error) {
	if err == io.EOF {
		runtimeAPIErrors.emptyBody.write(w)
		return
	}
	runtimeAPIErrors.invalidJSON.write(w)
}

func writeJSONResponse(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("runtime api json encode: %v", err)
	}
}

func validateStoreKeyPayload(payload storeKeyPayload, jobName, workerIP string, isWrite bool) *apiResponseError {
	if payload.Namespace == "" || payload.Key == "" || (isWrite && payload.Value == "") {
		return runtimeAPIErrors.missingKeyFields
	}

	if isWrite {
		expectedNamespace := fmt.Sprintf("vars/job/%s", jobName)
		if payload.Namespace != expectedNamespace {
			return runtimeAPIErrors.namespaceDenied
		}
		return nil
	}

	allowedNamespaces := data.AllowedKVNamespaces(jobName, workerIP)
	if len(utils.Intersection(allowedNamespaces, []string{payload.Namespace})) == 0 {
		return runtimeAPIErrors.namespaceDenied
	}
	return nil
}

func validateStoreSecretPayload(payload storeKeyPayload, jobName, workerIP string) *apiResponseError {
	if payload.Namespace == "" || payload.Key == "" || payload.Value == "" {
		return runtimeAPIErrors.missingKeyFields
	}
	return validateSecretNamespace(payload.Namespace, jobName, workerIP)
}

func validateStoreSecretKeyPayload(payload storeKeyPayload, jobName, workerIP string) *apiResponseError {
	if payload.Namespace == "" || payload.Key == "" {
		return runtimeAPIErrors.missingKeyFields
	}
	return validateSecretNamespace(payload.Namespace, jobName, workerIP)
}

func validateSecretNamespace(namespace, jobName, workerIP string) *apiResponseError {
	expectedNamespace := kv.SecretJobNamespace(jobName)
	if namespace != expectedNamespace {
		return runtimeAPIErrors.namespaceDenied
	}

	allowedNamespaces := data.AllowedKVNamespaces(jobName, workerIP)
	if len(utils.Intersection(allowedNamespaces, []string{namespace})) == 0 {
		return runtimeAPIErrors.namespaceDenied
	}
	return nil
}

func validateReadNamespace(namespace, jobName, workerIP string) *apiResponseError {
	allowedNamespaces := data.AllowedKVNamespaces(jobName, workerIP)
	if len(utils.Intersection(allowedNamespaces, []string{namespace})) == 0 {
		return runtimeAPIErrors.namespaceDenied
	}
	return nil
}

func isJobLevelListNamespace(namespace, jobName string) bool {
	return namespace == fmt.Sprintf("vars/job/%s", jobName) || namespace == kv.SecretJobNamespace(jobName)
}
