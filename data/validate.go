package data

import (
	"database/sql"
	"fmt"
	"maand/worker"
	"sync"
)

func ValidateBucketUpdateSeq(tx *sql.Tx, workers []string) {
	bucketID := GetBucketID(tx)
	updateSeq := GetUpdateSeq(tx)

	var errs = make(map[string]error)

	var mu sync.Mutex
	var wait sync.WaitGroup

	for _, workerIP := range workers {
		wait.Add(1)
		workerID := GetWorkerID(tx, workerIP)
		go func(tWorkerID, tWorkerIP string) {
			defer wait.Done()
			err := worker.ExecuteCommand(tWorkerIP, []string{fmt.Sprintf("python3 /opt/worker/%s/bin/worker.py %s %s %d", bucketID, bucketID, tWorkerID, updateSeq)}, nil)
			if err != nil {
				mu.Lock()
				errs[tWorkerIP] = err
				mu.Unlock()
			}
		}(workerID, workerIP)
	}
	wait.Wait()

	for workerIP, err := range errs {
		if err.Error() == "exit status 1" {
			panic(fmt.Sprintf("bucket id mismatch, worker %s", workerIP))
		}
		if err.Error() == "exit status 2" {
			panic(fmt.Sprintf("worker id mismatch, worker %s", workerIP))
		}
		if err.Error() == "exit status 3" {
			panic(fmt.Sprintf("update seq mismatch, worker %s", workerIP))
		}
	}
}
