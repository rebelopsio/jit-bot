package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rebelopsio/jit-bot/internal/config"
	"github.com/rebelopsio/jit-bot/internal/handlers"
)

type Server struct {
	config  *config.Config
	handler http.Handler
}

func New(cfg *config.Config) (*Server, error) {
	handler, err := handlers.NewRouter(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create router: %w", err)
	}

	return &Server{
		config:  cfg,
		handler: handler,
	}, nil
}

func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{
		Addr:         ":" + s.config.Server.Port,
		Handler:      s.handler,
		ReadTimeout:  s.config.Server.ReadTimeout,
		WriteTimeout: s.config.Server.WriteTimeout,
		IdleTimeout:  s.config.Server.IdleTimeout,
	}

	errChan := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errChan:
		return err
	}
}
