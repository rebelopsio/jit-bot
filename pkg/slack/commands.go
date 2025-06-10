package slack

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rebelopsio/jit-bot/pkg/auth"
	"github.com/rebelopsio/jit-bot/pkg/models"
	"github.com/rebelopsio/jit-bot/pkg/store"
)

const (
	cmdAdmin = "admin"
)

type CommandHandler struct {
	rbac  *auth.RBAC
	store *store.MemoryStore
}

func NewCommandHandler(rbac *auth.RBAC, store *store.MemoryStore) *CommandHandler {
	return &CommandHandler{
		rbac:  rbac,
		store: store,
	}
}

type SlackCommand struct {
	Token       string `form:"token"`
	TeamID      string `form:"team_id"`
	TeamDomain  string `form:"team_domain"`
	ChannelID   string `form:"channel_id"`
	ChannelName string `form:"channel_name"`
	UserID      string `form:"user_id"`
	UserName    string `form:"user_name"`
	Command     string `form:"command"`
	Text        string `form:"text"`
	ResponseURL string `form:"response_url"`
}

func (h *CommandHandler) HandleJITCommand(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	cmd := SlackCommand{
		UserID:   r.FormValue("user_id"),
		UserName: r.FormValue("user_name"),
		Command:  r.FormValue("command"),
		Text:     r.FormValue("text"),
	}

	parts := strings.Fields(cmd.Text)
	if len(parts) == 0 {
		h.sendHelp(w)
		return
	}

	subcommand := parts[0]
	args := parts[1:]

	switch subcommand {
	case "request":
		h.handleRequestAccess(w, cmd, args)
	case "list":
		h.handleListClusters(w, cmd)
	case "status":
		h.handleStatus(w, cmd)
	case cmdAdmin:
		h.handleAdmin(w, cmd, args)
	case "help":
		h.sendHelp(w)
	default:
		h.sendError(w, fmt.Sprintf("Unknown command: %s. Use `/jit help` for available commands.", subcommand))
	}
}

func (h *CommandHandler) handleRequestAccess(w http.ResponseWriter, cmd SlackCommand, args []string) {
	if len(args) < 2 {
		h.sendError(w, "Usage: `/jit request <cluster> <reason>` - Example: `/jit request prod-cluster debugging issue #1234`")
		return
	}

	clusterID := args[0]
	reason := strings.Join(args[1:], " ")

	cluster, err := h.store.GetCluster(clusterID)
	if err != nil {
		h.sendError(w, fmt.Sprintf("Cluster '%s' not found. Use `/jit list` to see available clusters.", clusterID))
		return
	}

	if !cluster.Enabled {
		h.sendError(w, fmt.Sprintf("Cluster '%s' is currently disabled.", clusterID))
		return
	}

	access := &models.ClusterAccess{
		ID:          uuid.New().String(),
		ClusterID:   clusterID,
		UserID:      cmd.UserID,
		UserEmail:   fmt.Sprintf("%s@company.com", cmd.UserName),
		Reason:      reason,
		Duration:    cluster.MaxDuration,
		Status:      models.AccessStatusPending,
		RequestedAt: time.Now(),
	}

	if err := h.store.CreateAccess(access); err != nil {
		h.sendError(w, "Failed to create access request")
		return
	}

	response := map[string]interface{}{
		"response_type": "in_channel",
		"text":          fmt.Sprintf("Access request submitted for cluster `%s`", cluster.DisplayName),
		"attachments": []map[string]interface{}{
			{
				"color": "good",
				"fields": []map[string]interface{}{
					{"title": "Request ID", "value": access.ID, "short": true},
					{"title": "Cluster", "value": cluster.DisplayName, "short": true},
					{"title": "Duration", "value": cluster.MaxDuration.String(), "short": true},
					{"title": "Status", "value": "Pending Approval", "short": true},
					{"title": "Reason", "value": reason, "short": false},
				},
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *CommandHandler) handleListClusters(w http.ResponseWriter, cmd SlackCommand) {
	clusters, err := h.store.ListClusters()
	if err != nil {
		h.sendError(w, "Failed to retrieve clusters")
		return
	}

	if len(clusters) == 0 {
		h.sendMessage(w, "No clusters available. Contact an admin to add clusters.")
		return
	}

	fields := make([]map[string]interface{}, 0)
	for _, cluster := range clusters {
		if cluster.Enabled {
			status := "✅ Available"
			if !cluster.Enabled {
				status = "❌ Disabled"
			}
			fields = append(fields, map[string]interface{}{
				"title": cluster.DisplayName,
				"value": fmt.Sprintf("ID: `%s`\nEnvironment: %s\nMax Duration: %s\nStatus: %s",
					cluster.ID, cluster.Environment, cluster.MaxDuration.String(), status),
				"short": true,
			})
		}
	}

	response := map[string]interface{}{
		"response_type": "ephemeral",
		"text":          "Available clusters:",
		"attachments": []map[string]interface{}{
			{
				"color":  "good",
				"fields": fields,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *CommandHandler) handleStatus(w http.ResponseWriter, cmd SlackCommand) {
	accesses, err := h.store.ListUserAccesses(cmd.UserID)
	if err != nil {
		h.sendError(w, "Failed to retrieve access status")
		return
	}

	if len(accesses) == 0 {
		h.sendMessage(w, "You have no active or pending access requests.")
		return
	}

	fields := make([]map[string]interface{}, 0)
	for _, access := range accesses {
		cluster, _ := h.store.GetCluster(access.ClusterID)
		clusterName := access.ClusterID
		if cluster != nil {
			clusterName = cluster.DisplayName
		}

		statusEmoji := map[models.AccessStatus]string{
			models.AccessStatusPending:  "⏳",
			models.AccessStatusApproved: "✅",
			models.AccessStatusDenied:   "❌",
			models.AccessStatusActive:   "🟢",
			models.AccessStatusExpired:  "⏰",
			models.AccessStatusRevoked:  "🚫",
		}

		fields = append(fields, map[string]interface{}{
			"title": clusterName,
			"value": fmt.Sprintf("%s %s\nRequested: %s\nDuration: %s",
				statusEmoji[access.Status], access.Status,
				access.RequestedAt.Format("2006-01-02 15:04"),
				access.Duration.String()),
			"short": true,
		})
	}

	response := map[string]interface{}{
		"response_type": "ephemeral",
		"text":          "Your access requests:",
		"attachments": []map[string]interface{}{
			{
				"color":  "good",
				"fields": fields,
			},
		},
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *CommandHandler) handleAdmin(w http.ResponseWriter, cmd SlackCommand, args []string) {
	if err := h.rbac.ValidatePermission(cmd.UserID, auth.PermissionManageClusters); err != nil {
		h.sendError(w, "You don't have admin permissions.")
		return
	}

	if len(args) == 0 {
		h.sendMessage(w, "Admin commands: `add-cluster`, `disable-cluster`, `enable-cluster`, `grant-role`")
		return
	}

	subcommand := args[0]
	switch subcommand {
	case "add-cluster":
		h.sendMessage(w, "Use the API endpoint `/api/v1/clusters` to add clusters programmatically.")
	case "grant-role":
		h.sendMessage(w, "Use the API endpoint `/api/v1/users/role` to manage user roles.")
	default:
		h.sendError(w, fmt.Sprintf("Unknown admin command: %s", subcommand))
	}
}

func (h *CommandHandler) sendHelp(w http.ResponseWriter) {
	help := `*JIT Access Commands:*
• ` + "`/jit request <cluster> <reason>`" + ` - Request access to a cluster
• ` + "`/jit list`" + ` - List available clusters
• ` + "`/jit status`" + ` - View your access requests
• ` + "`/jit admin`" + ` - Admin commands (admin only)
• ` + "`/jit help`" + ` - Show this help

*Examples:*
• ` + "`/jit request prod-cluster debugging issue #1234`" + `
• ` + "`/jit list`" + `
• ` + "`/jit status`"

	h.sendMessage(w, help)
}

func (h *CommandHandler) sendMessage(w http.ResponseWriter, text string) {
	response := map[string]interface{}{
		"response_type": "ephemeral",
		"text":          text,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

func (h *CommandHandler) sendError(w http.ResponseWriter, text string) {
	response := map[string]interface{}{
		"response_type": "ephemeral",
		"text":          "❌ " + text,
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}
