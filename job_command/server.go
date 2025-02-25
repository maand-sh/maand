package job_command

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

func NewServer(tx *sql.Tx, job, command, event string) *Server {
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      newMux(tx, job, command, event),
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
