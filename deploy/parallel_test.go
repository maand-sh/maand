package deploy

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func TestRunWorkerBatchesIndexWorkers(t *testing.T) {
	workers := []string{"w1", "w2", "w3"}
	var seen []string

	err := runWorkerBatches(workers, 2, func(ip string) error {
		seen = append(seen, ip)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 3 {
		t.Fatalf("seen %#v", seen)
	}
}

func TestRunParallelWorkersCollectsErrors(t *testing.T) {
	var calls atomic.Int32
	err := runParallelWorkers([]string{"a", "b"}, 2, func(workerIP string) error {
		calls.Add(1)
		if workerIP == "a" {
			return fmt.Errorf("fail a")
		}
		return nil
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if calls.Load() != 2 {
		t.Fatalf("calls %d", calls.Load())
	}
}
