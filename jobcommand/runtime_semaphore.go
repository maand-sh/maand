// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"context"
	"fmt"
	"sync"
	"time"
)

const (
	defaultSemaphoreCapacity = 1
	maxSemaphoreCapacity     = 64
	maxAcquireTimeout        = 3600 * time.Second
	defaultAcquireTimeout    = 600 * time.Second
)

// semaphoreCoordinator tracks allocation semaphores for one command-runtime API session.
type semaphoreCoordinator struct {
	mu         sync.Mutex
	semaphores map[string]*jobSemaphore
}

func newSemaphoreCoordinator() *semaphoreCoordinator {
	return &semaphoreCoordinator{
		semaphores: make(map[string]*jobSemaphore),
	}
}

type jobSemaphore struct {
	capacity int
	holders  map[string]struct{}
	waiters  []*semaphoreWaiter
}

type semaphoreWaiter struct {
	allocationID string
	notify       chan struct{}
}

func (c *semaphoreCoordinator) acquire(
	ctx context.Context,
	scopeKey, allocationID string,
	capacity int,
) error {
	if capacity < 1 {
		capacity = defaultSemaphoreCapacity
	}
	if capacity > maxSemaphoreCapacity {
		return fmt.Errorf("capacity %d exceeds maximum %d", capacity, maxSemaphoreCapacity)
	}

	notifyCh, err := c.registerWait(scopeKey, allocationID, capacity)
	if err != nil {
		return err
	}
	if notifyCh == nil {
		return nil
	}

	select {
	case <-notifyCh:
		return nil
	case <-ctx.Done():
		c.cancelWait(scopeKey, notifyCh)
		return ctx.Err()
	}
}

func (c *semaphoreCoordinator) registerWait(scopeKey, allocationID string, capacity int) (chan struct{}, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sem := c.semaphores[scopeKey]
	if sem == nil {
		sem = &jobSemaphore{
			capacity: capacity,
			holders:  make(map[string]struct{}),
		}
		c.semaphores[scopeKey] = sem
	} else if sem.capacity != capacity {
		return nil, fmt.Errorf("semaphore capacity is %d, requested %d", sem.capacity, capacity)
	}

	if _, held := sem.holders[allocationID]; held {
		return nil, nil
	}
	if len(sem.holders) < sem.capacity {
		sem.holders[allocationID] = struct{}{}
		return nil, nil
	}

	waiter := &semaphoreWaiter{
		allocationID: allocationID,
		notify:       make(chan struct{}),
	}
	sem.waiters = append(sem.waiters, waiter)
	return waiter.notify, nil
}

func (c *semaphoreCoordinator) cancelWait(scopeKey string, notifyCh chan struct{}) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sem := c.semaphores[scopeKey]
	if sem == nil {
		return
	}

	remaining := sem.waiters[:0]
	for _, waiter := range sem.waiters {
		if waiter.notify == notifyCh {
			continue
		}
		remaining = append(remaining, waiter)
	}
	sem.waiters = remaining
}

func (c *semaphoreCoordinator) release(scopeKey, allocationID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	sem := c.semaphores[scopeKey]
	if sem == nil {
		return fmt.Errorf("semaphore %q not held", scopeKey)
	}

	if _, held := sem.holders[allocationID]; !held {
		return fmt.Errorf("allocation %s does not hold semaphore %q", allocationID, scopeKey)
	}

	delete(sem.holders, allocationID)
	c.grantNextWaiterLocked(sem)
	return nil
}

func (c *semaphoreCoordinator) grantNextWaiterLocked(sem *jobSemaphore) {
	for len(sem.holders) < sem.capacity && len(sem.waiters) > 0 {
		next := sem.waiters[0]
		sem.waiters = sem.waiters[1:]
		sem.holders[next.allocationID] = struct{}{}
		close(next.notify)
	}
}

func (c *semaphoreCoordinator) status(scopeKey string) (semaphoreStatusPayload, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sem := c.semaphores[scopeKey]
	if sem == nil {
		return semaphoreStatusPayload{Name: scopeKey}, false
	}

	holders := make([]string, 0, len(sem.holders))
	for allocationID := range sem.holders {
		holders = append(holders, allocationID)
	}

	return semaphoreStatusPayload{
		Name:        scopeKey,
		Capacity:    sem.capacity,
		Holders:     holders,
		Waiting:     len(sem.waiters),
		Available:   sem.capacity - len(sem.holders),
	}, true
}

func semaphoreScopeKey(jobName, event, semaphoreName string) string {
	return fmt.Sprintf("%s/%s/%s", jobName, event, semaphoreName)
}

func normalizeAcquireTimeout(seconds int) time.Duration {
	if seconds <= 0 {
		return defaultAcquireTimeout
	}
	timeout := time.Duration(seconds) * time.Second
	if timeout > maxAcquireTimeout {
		return maxAcquireTimeout
	}
	return timeout
}
