package handlers

import (
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

	return mux, nil
}

type Handler struct {
	config *config.Config
	rbac   *auth.RBAC
	store  *store.MemoryStore
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Ready"))
}

func (h *Handler) SlackEvents(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (h *Handler) SlackCommands(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
