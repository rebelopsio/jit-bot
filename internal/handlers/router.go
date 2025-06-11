package handlers

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/rebelopsio/jit-bot/internal/config"
	"github.com/rebelopsio/jit-bot/pkg/auth"
	"github.com/rebelopsio/jit-bot/pkg/slack"
	"github.com/rebelopsio/jit-bot/pkg/store"
)

func NewRouter(cfg *config.Config) (http.Handler, error) {
	mux := http.NewServeMux()

	rbac := auth.NewRBAC(cfg.Auth.AdminUsers)
	for _, approver := range cfg.Auth.Approvers {
		rbac.SetUserRole(approver, auth.RoleApprover)
	}

	memStore := store.NewMemoryStore()
	slackMiddleware := slack.NewSlackMiddleware(cfg.Slack.SigningSecret)
	commandHandler := slack.NewCommandHandler(rbac, memStore)

	h := &Handler{
		config: cfg,
		rbac:   rbac,
		store:  memStore,
	}

	adminHandler := NewAdminHandler(rbac, memStore)

	accessHandler, err := NewAccessHandler(rbac, memStore, cfg.AWS.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to create access handler: %w", err)
	}

	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/ready", h.Ready)

	slackMux := http.NewServeMux()
	slackMux.HandleFunc("/slack/events", h.SlackEvents)
	slackMux.HandleFunc("/slack/commands", commandHandler.HandleJITCommand)

	mux.Handle("/slack/", slackMiddleware.VerifyRequest(slackMux))

	mux.HandleFunc("/api/v1/clusters", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			adminHandler.ListClusters(w, r)
		case http.MethodPost:
			adminHandler.CreateCluster(w, r)
		case http.MethodPut:
			adminHandler.UpdateCluster(w, r)
		case http.MethodDelete:
			adminHandler.DeleteCluster(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/v1/users/role", adminHandler.ManageUser)

	// Access management endpoints
	mux.HandleFunc("/api/v1/access/grant", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		accessHandler.GrantAccess(w, r)
	})

	mux.HandleFunc("/api/v1/access/revoke", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		accessHandler.RevokeAccess(w, r)
	})

	mux.HandleFunc("/api/v1/access", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		accessHandler.ListAccess(w, r)
	})

	mux.HandleFunc("/api/v1/access/status", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		accessHandler.GetAccessStatus(w, r)
	})

	mux.HandleFunc("/api/v1/access/cleanup", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		accessHandler.CleanupExpiredAccess(w, r)
	})

	return mux, nil
}

type Handler struct {
	config *config.Config
	rbac   *auth.RBAC
	store  *store.MemoryStore
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		// Log error but don't fail the health check
		slog.Error("Failed to write health check response", "error", err)
	}
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("Ready")); err != nil {
		// Log error but don't fail the readiness check
		slog.Error("Failed to write readiness check response", "error", err)
	}
}

func (h *Handler) SlackEvents(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) SlackCommands(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
