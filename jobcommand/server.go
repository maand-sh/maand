// Copyright 2025 Kiruba Sankar Swaminathan. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package jobcommand

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"time"
)

type kvRequest struct {
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
}

type kvResponse struct {
	Namespace string `json:"namespace"`
	Key       string `json:"key"`
	Value     string `json:"value"`
}

type demandResponse struct {
	Job          string                 `json:"job"`
	Command      string                 `json:"command"`
	DemandConfig map[string]interface{} `json:"demand_config"`
}

const (
	headerAllocID = "X-ALLOCATION-ID"
	headerEvent   = "EVENT"
	headerCommand = "COMMAND"
	serverAddr    = "localhost:8080"
)

var serverConfig = struct {
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	IdleTimeout     time.Duration
	ShutdownTimeout time.Duration
}{
	ReadTimeout:     10 * time.Second,
	WriteTimeout:    10 * time.Second,
	IdleTimeout:     30 * time.Second,
	ShutdownTimeout: 5 * time.Second,
}

type Server struct {
	*http.Server
	tx *sql.Tx
}

func newServer(tx *sql.Tx) *Server {
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      newMux(tx),
		ReadTimeout:  serverConfig.ReadTimeout,
		WriteTimeout: serverConfig.WriteTimeout,
		IdleTimeout:  serverConfig.IdleTimeout,
	}
	return &Server{Server: srv, tx: tx}
}

func (s *Server) Start(ctx context.Context) error {
	errChan := make(chan error, 1)
	go func() {
		if err := s.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("server start error: %w", err)
		}
		close(errChan)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), serverConfig.ShutdownTimeout)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}

func SetupServer(tx *sql.Tx) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())
	// Start the server in a goroutine
	errChan := make(chan error, 1)
	go func() {
		server := newServer(tx)
		errChan <- server.Start(ctx)
	}()
	return cancel
}
