package job_command

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"maand/kv"
	"net/http"
)

func newMux(tx *sql.Tx, job, command, event string) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/kv", handleKV(tx, job, command, event))
	mux.HandleFunc("/demands", handleDemands(tx, job, command, event))
	return mux
}

func handleKV(tx *sql.Tx, job, command, event string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleKVGet(w, r, tx, job, command, event)
		case http.MethodPut:
			handleKVPut(w, r, tx, job, command, event)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func handleDemands(tx *sql.Tx, job, command, event string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		handleDemandsGet(w, r, tx, job, command, event)
	}
}

func validateRequest(w http.ResponseWriter, r *http.Request, tx *sql.Tx, job string) (workerIP string, body io.ReadCloser, err error) {
	allocID := r.Header.Get(headerAllocID)
	if allocID == "" {
		httpErrors.MissingAllocID.Write(w)
		err = fmt.Errorf("missing allocation id")
		return
	}

	err = tx.QueryRow("SELECT worker_ip FROM allocations WHERE alloc_id = ? AND job = ?", allocID, job).Scan(&workerIP)
	if errors.Is(err, sql.ErrNoRows) {
		httpErrors.InvalidAllocID.Write(w)
		err = fmt.Errorf("invalid allocation id")
		return
	} else if err != nil {
		return
	}

	body = r.Body
	return
}

func handleKVGet(w http.ResponseWriter, r *http.Request, tx *sql.Tx, job, command, event string) {
	workerIP, body, err := validateRequest(w, r, tx, job)
	if err != nil {
		return
	}
	defer func() {
		_ = body.Close()
	}()

	var req kvRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		handleJSONError(w, err)
		return
	}

	if err := validateKVRequest(req, job, workerIP, false); err != nil {
		err.Write(w)
		return
	}

	value, err := kv.GetKVStore().Get(tx, req.Namespace, req.Key)
	if err != nil {
		log.Printf("KV get error: %v", err)
		httpErrors.KVNotFound.Write(w)
		return
	}

	respondJSON(w, http.StatusOK, kvResponse{
		Namespace: req.Namespace,
		Key:       req.Key,
		Value:     value,
	})
}

func handleKVPut(w http.ResponseWriter, r *http.Request, tx *sql.Tx, job, command, event string) {
	workerIP, body, err := validateRequest(w, r, tx, job)
	if err != nil {
		return
	}
	defer func() {
		_ = body.Close()
	}()

	var req kvRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		handleJSONError(w, err)
		return
	}

	if err := validateKVRequest(req, job, workerIP, true); err != nil {
		err.Write(w)
		return
	}

	if event == "health_check" {
		log.Printf("KV put not allowed in health check")
		httpErrors.BadRequestBody.Write(w)
		return
	}

	if err := kv.GetKVStore().Put(tx, req.Namespace, req.Key, req.Value, 0); err != nil {
		log.Printf("KV put error: %v", err)
		httpErrors.KVStoreFailure.Write(w)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func handleDemandsGet(w http.ResponseWriter, r *http.Request, tx *sql.Tx, job, command, event string) {
	_, body, err := validateRequest(w, r, tx, job)
	if err != nil {
		return
	}
	defer func() {
		_ = body.Close()
	}()

	rows, err := tx.Query(`
		SELECT job AS requester_job, 
		       name AS requester_job_command, 
		       demand_config 
		FROM job_commands 
		WHERE demand_job = ? AND demand_command = ?`,
		job, command)
	if err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer func() {
		_ = rows.Close()
	}()

	demands := make([]demandResponse, 0)
	for rows.Next() {
		var resp demandResponse
		var configStr string
		if err := rows.Scan(&resp.Job, &resp.Command, &configStr); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		if err := json.Unmarshal([]byte(configStr), &resp.DemandConfig); err != nil {
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		demands = append(demands, resp)
	}

	if err := rows.Err(); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, demands)
}
