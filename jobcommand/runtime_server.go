// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
)

type runtimeAPIServer struct {
	*http.Server
}

type runtimeServerTimeouts struct {
	read     time.Duration
	write    time.Duration
	idle     time.Duration
	shutdown time.Duration
}

var defaultRuntimeServerTimeouts = runtimeServerTimeouts{
	read:     10 * time.Second,
	write:    10 * time.Second,
	idle:     30 * time.Second,
	shutdown: 5 * time.Second,
}

func newRuntimeAPIServer(tx *sql.Tx) *runtimeAPIServer {
	apiCtx := &runtimeAPIContext{
		tx:         tx,
		semaphores: newSemaphoreCoordinator(),
	}
	return &runtimeAPIServer{
		Server: &http.Server{
			Addr:         RuntimeAPIListenAddr,
			Handler:      newRuntimeAPIMux(apiCtx),
			ReadTimeout:  defaultRuntimeServerTimeouts.read,
			WriteTimeout: 0, // per-handler deadlines (semaphore acquire may block)
			IdleTimeout:  defaultRuntimeServerTimeouts.idle,
		},
	}
}

func (s *runtimeAPIServer) runUntilCancelled(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("runtime api listen: %w", err)
			return
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), defaultRuntimeServerTimeouts.shutdown)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

// StartRuntimeAPI serves /kv, /demands, and /semaphore/* for in-container job commands.
// Call the returned stop function when the surrounding command finishes.
func StartRuntimeAPI(tx *sql.Tx) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		server := newRuntimeAPIServer(tx)
		if err := server.runUntilCancelled(ctx); err != nil && !errors.Is(err, context.Canceled) {
			log.Printf("command runtime api: %v", err)
		}
	}()

	return cancel
}

// SetupServer is deprecated; use StartRuntimeAPI.
func SetupServer(tx *sql.Tx) context.CancelFunc {
	return StartRuntimeAPI(tx)
}
