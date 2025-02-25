package job_command

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// handleJSONError writes an error response based on JSON decoding issues.
func handleJSONError(w http.ResponseWriter, err error) {
	if err == io.EOF {
		httpErrors.BadRequestBody.Write(w)
	} else {
		httpErrors.InvalidFormat.Write(w)
	}
}

// respondJSON encodes the payload into JSON and writes it to the response.
func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("JSON encoding error: %v", err)
	}
}

// validateKVRequest checks if the key-value request is valid.
// For GET requests, it validates allowed namespaces using data.GetAllowedNamespaces.
// For PUT requests, it ensures the namespace is exactly "vars/job/<job>".
func validateKVRequest(req kvRequest, job, workerIP string, isPut bool) *httpError {
	if req.Namespace == "" || req.Key == "" || (isPut && req.Value == "") {
		return httpErrors.MissingFields
	}
	if isPut {
		expected := fmt.Sprintf("vars/job/%s", job)
		if req.Namespace != expected {
			return httpErrors.InvalidNamespace
		}
	} else {
		// TODO: limit scope for kv access.
		//allowed := data.GetAllowedNamespaces(job, workerIP)
		//if len(utils.Intersection(allowed, []string{req.Namespace})) == 0 {
		//	return httpErrors.InvalidNamespace
		//}
	}
	return nil
}
