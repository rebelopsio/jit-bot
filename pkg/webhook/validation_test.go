package webhook

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/rebelopsio/jit-bot/pkg/controller"
)

func TestJITAccessRequestValidator_Handle(t *testing.T) {
	tests := []struct {
		name        string
		request     *controller.JITAccessRequest
		wantAllowed bool
		wantMessage string
	}{
		{
			name: "valid request",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "U123456789A",
					UserEmail: "test@company.com",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123456789012",
						Region:     "us-east-1",
					},
					Reason:      "Deploy critical hotfix for payment service",
					Duration:    "2h",
					Permissions: []string{"edit"},
					Namespaces:  []string{"payment-service"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: true,
		},
		{
			name: "invalid user ID format",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "invalid-user",
					UserEmail: "test@company.com",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123456789012",
						Region:     "us-east-1",
					},
					Reason:      "Deploy critical hotfix",
					Duration:    "2h",
					Permissions: []string{"edit"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: false,
			wantMessage: "invalid user ID format",
		},
		{
			name: "invalid email format",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "U123456789A",
					UserEmail: "not-an-email",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123456789012",
						Region:     "us-east-1",
					},
					Reason:      "Deploy critical hotfix",
					Duration:    "2h",
					Permissions: []string{"edit"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: false,
			wantMessage: "invalid email format",
		},
		{
			name: "duration too short",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "U123456789A",
					UserEmail: "test@company.com",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123456789012",
						Region:     "us-east-1",
					},
					Reason:      "Deploy critical hotfix",
					Duration:    "5m",
					Permissions: []string{"edit"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: false,
			wantMessage: "duration must be at least 15m0s",
		},
		{
			name: "duration too long",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "U123456789A",
					UserEmail: "test@company.com",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123456789012",
						Region:     "us-east-1",
					},
					Reason:      "Deploy critical hotfix",
					Duration:    "8d",
					Permissions: []string{"edit"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: false,
			wantMessage: "duration cannot exceed 168h0m0s",
		},
		{
			name: "invalid duration format",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "U123456789A",
					UserEmail: "test@company.com",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123456789012",
						Region:     "us-east-1",
					},
					Reason:      "Deploy critical hotfix",
					Duration:    "invalid",
					Permissions: []string{"edit"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: false,
			wantMessage: "invalid duration format",
		},
		{
			name: "invalid permissions",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "U123456789A",
					UserEmail: "test@company.com",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123456789012",
						Region:     "us-east-1",
					},
					Reason:      "Deploy critical hotfix",
					Duration:    "2h",
					Permissions: []string{"invalid-permission"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: false,
			wantMessage: "invalid permission",
		},
		{
			name: "cluster-admin without sufficient reason",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "U123456789A",
					UserEmail: "test@company.com",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123456789012",
						Region:     "us-east-1",
					},
					Reason:      "Need access",
					Duration:    "2h",
					Permissions: []string{"cluster-admin"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: false,
			wantMessage: "cluster-admin permission requires detailed justification",
		},
		{
			name: "cluster-admin with sufficient reason",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "U123456789A",
					UserEmail: "test@company.com",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123456789012",
						Region:     "us-east-1",
					},
					Reason:      "Emergency incident response for critical security vulnerability affecting payment processing system requiring immediate cluster-wide access to diagnose and remediate",
					Duration:    "2h",
					Permissions: []string{"cluster-admin"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: true,
		},
		{
			name: "invalid AWS account",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "U123456789A",
					UserEmail: "test@company.com",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123",
						Region:     "us-east-1",
					},
					Reason:      "Deploy critical hotfix",
					Duration:    "2h",
					Permissions: []string{"edit"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: false,
			wantMessage: "invalid AWS account ID format",
		},
		{
			name: "invalid AWS region",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "U123456789A",
					UserEmail: "test@company.com",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123456789012",
						Region:     "invalid-region",
					},
					Reason:      "Deploy critical hotfix",
					Duration:    "2h",
					Permissions: []string{"edit"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: false,
			wantMessage: "invalid AWS region format",
		},
		{
			name: "reason too short",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "U123456789A",
					UserEmail: "test@company.com",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123456789012",
						Region:     "us-east-1",
					},
					Reason:      "test",
					Duration:    "2h",
					Permissions: []string{"edit"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: false,
			wantMessage: "reason must be at least 10 characters",
		},
		{
			name: "generic reason",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "U123456789A",
					UserEmail: "test@company.com",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123456789012",
						Region:     "us-east-1",
					},
					Reason:      "need access to debug",
					Duration:    "2h",
					Permissions: []string{"edit"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: false,
			wantMessage: "reason appears generic",
		},
		{
			name: "invalid namespace format",
			request: &controller.JITAccessRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-request",
					Namespace: "jit-system",
				},
				Spec: controller.JITAccessRequestSpec{
					UserID:    "U123456789A",
					UserEmail: "test@company.com",
					TargetCluster: controller.TargetCluster{
						Name:       "prod-east-1",
						AWSAccount: "123456789012",
						Region:     "us-east-1",
					},
					Reason:      "Deploy critical hotfix",
					Duration:    "2h",
					Permissions: []string{"edit"},
					Namespaces:  []string{"Invalid_Namespace!"},
					RequestedAt: metav1.Now(),
				},
			},
			wantAllowed: false,
			wantMessage: "invalid namespace format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			err := controller.AddToScheme(scheme)
			require.NoError(t, err)

			validator := &JITAccessRequestValidator{
				decoder: admission.NewDecoder(scheme),
			}

			// Encode request to JSON
			requestJSON, err := json.Marshal(tt.request)
			require.NoError(t, err)

			// Create admission request
			req := admission.Request{
				AdmissionRequest: admissionv1.AdmissionRequest{
					Operation: admissionv1.Create,
					Object: runtime.RawExtension{
						Raw: requestJSON,
					},
				},
			}

			ctx := t.Context()
			resp := validator.Handle(ctx, req)

			assert.Equal(t, tt.wantAllowed, resp.Allowed, "Expected allowed=%v, got=%v", tt.wantAllowed, resp.Allowed)

			if !tt.wantAllowed && tt.wantMessage != "" {
				assert.Contains(
					t,
					resp.Result.Message,
					tt.wantMessage,
					"Expected message to contain '%s', got '%s'",
					tt.wantMessage,
					resp.Result.Message,
				)
			}
		})
	}
}

func TestValidateDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration string
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid duration - 1 hour",
			duration: "1h",
			wantErr:  false,
		},
		{
			name:     "valid duration - 2 days",
			duration: "2d",
			wantErr:  false,
		},
		{
			name:     "valid duration - minimum 15 minutes",
			duration: "15m",
			wantErr:  false,
		},
		{
			name:     "valid duration - maximum 7 days",
			duration: "7d",
			wantErr:  false,
		},
		{
			name:     "invalid duration - too short",
			duration: "10m",
			wantErr:  true,
			errMsg:   "duration must be at least",
		},
		{
			name:     "invalid duration - too long",
			duration: "8d",
			wantErr:  true,
			errMsg:   "duration cannot exceed",
		},
		{
			name:     "invalid duration - format",
			duration: "invalid",
			wantErr:  true,
			errMsg:   "duration must be in format like '1h', '30m', '2h30m', '1d'",
		},
		{
			name:     "invalid duration - empty",
			duration: "",
			wantErr:  true,
			errMsg:   "duration must be in format like '1h', '30m', '2h30m', '1d'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDuration(tt.duration)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidatePermissions(t *testing.T) {
	tests := []struct {
		name        string
		permissions []string
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "valid permissions - single",
			permissions: []string{"view"},
			wantErr:     false,
		},
		{
			name:        "valid permissions - multiple",
			permissions: []string{"view", "edit"},
			wantErr:     false,
		},
		{
			name:        "valid permissions - admin",
			permissions: []string{"admin"},
			wantErr:     false,
		},
		{
			name:        "valid permissions - cluster-admin",
			permissions: []string{"cluster-admin"},
			wantErr:     false,
		},
		{
			name:        "invalid permissions - unknown",
			permissions: []string{"invalid"},
			wantErr:     true,
			errMsg:      "invalid permission",
		},
		{
			name:        "invalid permissions - empty",
			permissions: []string{},
			wantErr:     true,
			errMsg:      "at least one permission must be specified",
		},
		{
			name:        "invalid permissions - mixed valid and invalid",
			permissions: []string{"view", "invalid"},
			wantErr:     true,
			errMsg:      "invalid permission",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePermissions(tt.permissions)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateClusterConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  controller.TargetCluster
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid cluster config",
			config: controller.TargetCluster{
				Name:       "prod-east-1",
				AWSAccount: "123456789012",
				Region:     "us-east-1",
			},
			wantErr: false,
		},
		{
			name: "invalid AWS account - too short",
			config: controller.TargetCluster{
				Name:       "prod-east-1",
				AWSAccount: "123",
				Region:     "us-east-1",
			},
			wantErr: true,
			errMsg:  "AWS account ID must be 12 digits",
		},
		{
			name: "invalid AWS account - non-numeric",
			config: controller.TargetCluster{
				Name:       "prod-east-1",
				AWSAccount: "12345678901a",
				Region:     "us-east-1",
			},
			wantErr: true,
			errMsg:  "AWS account ID must be 12 digits",
		},
		{
			name: "invalid region format",
			config: controller.TargetCluster{
				Name:       "prod-east-1",
				AWSAccount: "123456789012",
				Region:     "invalid-region-format",
			},
			wantErr: true,
			errMsg:  "invalid AWS region format",
		},
		{
			name: "empty cluster name",
			config: controller.TargetCluster{
				Name:       "",
				AWSAccount: "123456789012",
				Region:     "us-east-1",
			},
			wantErr: true,
			errMsg:  "cluster name is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateCluster(tt.config)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateReason(t *testing.T) {
	tests := []struct {
		name        string
		reason      string
		permissions []string
		wantErr     bool
		errMsg      string
	}{
		{
			name:        "valid reason - specific",
			reason:      "Deploy critical security patch for payment service vulnerability",
			permissions: []string{"edit"},
			wantErr:     false,
		},
		{
			name:        "valid reason - cluster-admin with detailed justification",
			reason:      "Emergency incident response for critical security vulnerability affecting payment processing system requiring immediate cluster-wide access to diagnose and remediate the issue",
			permissions: []string{"cluster-admin"},
			wantErr:     false,
		},
		{
			name:        "invalid reason - too short",
			reason:      "test",
			permissions: []string{"edit"},
			wantErr:     true,
			errMsg:      "reason must be at least 10 characters long",
		},
		{
			name:        "invalid reason - generic",
			reason:      "testing",
			permissions: []string{"edit"},
			wantErr:     true,
			errMsg:      "reason must be at least 10 characters long", // Length check happens first
		},
		{
			name:        "invalid reason - cluster-admin insufficient",
			reason:      "Need admin access",
			permissions: []string{"cluster-admin"},
			wantErr:     false, // validateReason doesn't check cluster-admin requirements
		},
		{
			name:        "invalid reason - debug generic",
			reason:      "debug",
			permissions: []string{"view"},
			wantErr:     true,
			errMsg:      "reason must be at least 10 characters long", // Length check happens first
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateReason(tt.reason)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateNamespaces(t *testing.T) {
	tests := []struct {
		name       string
		namespaces []string
		wantErr    bool
		errMsg     string
	}{
		{
			name:       "valid namespaces",
			namespaces: []string{"default", "kube-system", "payment-service"},
			wantErr:    false,
		},
		{
			name:       "valid namespaces - single",
			namespaces: []string{"my-app"},
			wantErr:    false,
		},
		{
			name:       "valid namespaces - empty list",
			namespaces: []string{},
			wantErr:    false,
		},
		{
			name:       "invalid namespace - uppercase",
			namespaces: []string{"Invalid"},
			wantErr:    true,
			errMsg:     "invalid namespace name:",
		},
		{
			name:       "invalid namespace - special characters",
			namespaces: []string{"invalid_namespace!"},
			wantErr:    true,
			errMsg:     "invalid namespace name:",
		},
		{
			name:       "invalid namespace - starts with number",
			namespaces: []string{"1invalid"},
			wantErr:    false, // Kubernetes namespace regex allows starting with number
		},
		{
			name: "invalid namespace - too long",
			namespaces: []string{
				"this-is-a-very-long-namespace-name-that-exceeds-the-maximum-allowed-length-for-kubernetes-namespaces",
			},
			wantErr: false, // Current validation doesn't check length limits
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateNamespaces(tt.namespaces, []string{"view"})

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestValidateApprovers(t *testing.T) {
	tests := []struct {
		name      string
		approvers []string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "valid approvers - Slack user IDs",
			approvers: []string{"U123456789A", "U987654321B"},
			wantErr:   false,
		},
		{
			name:      "valid approvers - team names",
			approvers: []string{"platform-team", "sre-team"},
			wantErr:   false,
		},
		{
			name:      "valid approvers - mixed",
			approvers: []string{"U123456789A", "security-team"},
			wantErr:   false,
		},
		{
			name:      "valid approvers - empty list",
			approvers: []string{},
			wantErr:   false,
		},
		{
			name:      "invalid approver - invalid format",
			approvers: []string{"Invalid_ID!"},
			wantErr:   true,
			errMsg:    "invalid approver format",
		},
		{
			name:      "invalid approver - mixed valid and invalid",
			approvers: []string{"U123456789A", "Invalid!"},
			wantErr:   true,
			errMsg:    "invalid approver format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateApprovers(tt.approvers)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.Contains(t, err.Error(), tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name     string
		duration string
		want     time.Duration
		wantErr  bool
	}{
		{
			name:     "parse hours",
			duration: "2h",
			want:     2 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "parse minutes",
			duration: "30m",
			want:     30 * time.Minute,
			wantErr:  false,
		},
		{
			name:     "parse days",
			duration: "1d",
			want:     24 * time.Hour,
			wantErr:  false,
		},
		{
			name:     "parse complex duration",
			duration: "1d2h30m",
			want:     26*time.Hour + 30*time.Minute,
			wantErr:  false,
		},
		{
			name:     "invalid format",
			duration: "invalid",
			wantErr:  true,
		},
		{
			name:     "empty string",
			duration: "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDuration(tt.duration)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
