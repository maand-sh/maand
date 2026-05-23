// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSemaphoreCapacityOneSerializesAllocations(t *testing.T) {
	coord := newSemaphoreCoordinator()
	scope := semaphoreScopeKey("app", "pre_deploy", "deploy_leader")

	require.NoError(t, coord.acquire(context.Background(), scope, "alloc-a", 1))

	done := make(chan struct{})
	go func() {
		_ = coord.acquire(context.Background(), scope, "alloc-b", 1)
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("second acquire should block")
	case <-time.After(100 * time.Millisecond):
	}

	require.NoError(t, coord.release(scope, "alloc-a"))

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("second acquire should proceed after release")
	}

	require.NoError(t, coord.release(scope, "alloc-b"))
}

func TestSemaphoreCapacityAllowsParallelHolders(t *testing.T) {
	coord := newSemaphoreCoordinator()
	scope := semaphoreScopeKey("app", "pre_deploy", "workers")

	require.NoError(t, coord.acquire(context.Background(), scope, "alloc-a", 2))
	require.NoError(t, coord.acquire(context.Background(), scope, "alloc-b", 2))

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	err := coord.acquire(ctx, scope, "alloc-c", 2)
	require.Error(t, err)

	require.NoError(t, coord.release(scope, "alloc-a"))
	require.NoError(t, coord.acquire(context.Background(), scope, "alloc-c", 2))
}

func TestSemaphoreAcquireTimeout(t *testing.T) {
	coord := newSemaphoreCoordinator()
	scope := semaphoreScopeKey("app", "pre_deploy", "gate")

	require.NoError(t, coord.acquire(context.Background(), scope, "alloc-a", 1))

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err := coord.acquire(ctx, scope, "alloc-b", 1)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestSemaphoreLeaderThenFollowersPattern(t *testing.T) {
	coord := newSemaphoreCoordinator()
	var wg sync.WaitGroup
	order := make([]string, 0)
	var orderMu sync.Mutex

	runAlloc := func(allocID, semName string, capacity int) {
		defer wg.Done()
		scope := semaphoreScopeKey("app", "pre_deploy", semName)
		require.NoError(t, coord.acquire(context.Background(), scope, allocID, capacity))
		orderMu.Lock()
		order = append(order, allocID)
		orderMu.Unlock()
		time.Sleep(20 * time.Millisecond)
		require.NoError(t, coord.release(scope, allocID))
	}

	wg.Add(3)
	go runAlloc("leader", "deploy_leader", 1)
	time.Sleep(30 * time.Millisecond)
	go runAlloc("follower-1", "deploy_followers", 2)
	go runAlloc("follower-2", "deploy_followers", 2)
	wg.Wait()

	require.Len(t, order, 3)
	assert.Equal(t, "leader", order[0], "leader allocation should acquire deploy_leader first")
}
