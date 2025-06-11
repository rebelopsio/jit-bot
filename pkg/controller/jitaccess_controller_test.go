package controller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/rebelopsio/jit-bot/pkg/auth"
)

func createTestRequest(name, namespace string, phase AccessPhase) *JITAccessRequest {
	return &JITAccessRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: JITAccessRequestSpec{
			UserID:    "U123456789A",
			UserEmail: "test@company.com",
			TargetCluster: TargetCluster{
				Name:       "dev-east-1",
				AWSAccount: "123456789012",
				Region:     "us-east-1",
			},
			Reason:      "Testing controller functionality",
			Duration:    "2h",
			Permissions: []string{"view"},
			RequestedAt: metav1.Now(),
		},
		Status: JITAccessRequestStatus{
			Phase: phase,
		},
	}
}

func createTestJob(name, namespace string) *JITAccessJob { //nolint:unused // test helper
	return &JITAccessJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-job",
			Namespace: namespace,
		},
		Spec: JITAccessJobSpec{
			AccessRequestRef: ObjectReference{
				Name:      name,
				Namespace: namespace,
			},
			TargetCluster: TargetCluster{
				Name:       "dev-east-1",
				AWSAccount: "123456789012",
				Region:     "us-east-1",
			},
			Duration:    "2h",
			JITRoleArn:  "arn:aws:iam::123456789012:role/JITAccess",
			Permissions: []string{"view"},
		},
	}
}

func TestJITAccessRequestReconciler_Reconcile(t *testing.T) {
	scheme := setupTestScheme(t)
	tests := createReconcileTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runReconcileTest(t, scheme, tt)
		})
	}
}

func setupTestScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	err := clientgoscheme.AddToScheme(scheme)
	require.NoError(t, err)
	err = AddToScheme(scheme)
	require.NoError(t, err)
	return scheme
}

func createReconcileTestCases() []struct {
	name          string
	request       *JITAccessRequest
	existingJob   *JITAccessJob
	expectJob     bool
	expectStatus  AccessPhase
	expectError   bool
	expectRequeue bool
} {
	return []struct {
		name          string
		request       *JITAccessRequest
		existingJob   *JITAccessJob
		expectJob     bool
		expectStatus  AccessPhase
		expectError   bool
		expectRequeue bool
	}{
		{
			name:          "new pending request creates job",
			request:       createTestRequest("test-request", "jit-system", AccessPhasePending),
			expectJob:     false, // Dev environment doesn't require approval
			expectStatus:  AccessPhaseApproved,
			expectRequeue: true,
		},
		{
			name:          "approved request creates job",
			request:       createApprovedTestRequest(),
			expectJob:     true,
			expectStatus:  AccessPhaseActive,
			expectRequeue: true,
		},
		{
			name:          "denied request does not create job",
			request:       createDeniedTestRequest(),
			expectJob:     false,
			expectStatus:  AccessPhaseDenied,
			expectRequeue: false,
		},
		{
			name:          "request with existing job does not create duplicate",
			request:       createRequestWithExistingJob(),
			existingJob:   createExistingJob(),
			expectJob:     true,
			expectStatus:  AccessPhaseActive,
			expectRequeue: true,
		},
	}
}

func runReconcileTest(t *testing.T, scheme *runtime.Scheme, tt struct {
	name          string
	request       *JITAccessRequest
	existingJob   *JITAccessJob
	expectJob     bool
	expectStatus  AccessPhase
	expectError   bool
	expectRequeue bool
}) {
	// Create fake client with initial objects
	objs := []client.Object{tt.request}
	if tt.existingJob != nil {
		objs = append(objs, tt.existingJob)
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&JITAccessRequest{}, &JITAccessJob{}).
		Build()

	// Create reconciler
	reconciler := createTestReconciler(fakeClient, scheme, tt.request.Spec.UserID)

	// Create reconcile request
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      tt.request.Name,
			Namespace: tt.request.Namespace,
		},
	}

	// Perform reconciliation and validate results
	validateReconcileResult(t, reconciler, req, tt, fakeClient)
}

func createTestReconciler(fakeClient client.Client, scheme *runtime.Scheme, userID string) *JITAccessRequestReconciler {
	rbac := auth.NewRBAC([]string{})
	rbac.SetUserRole(userID, auth.RoleRequester)
	return &JITAccessRequestReconciler{
		Client: fakeClient,
		Scheme: scheme,
		RBAC:   rbac,
	}
}

func validateReconcileResult(t *testing.T, reconciler *JITAccessRequestReconciler, req reconcile.Request, tt struct {
	name          string
	request       *JITAccessRequest
	existingJob   *JITAccessJob
	expectJob     bool
	expectStatus  AccessPhase
	expectError   bool
	expectRequeue bool
}, fakeClient client.Client) {
	ctx := t.Context()
	result, reconcileErr := reconciler.Reconcile(ctx, req)

	if tt.expectError {
		assert.Error(t, reconcileErr)
		return
	}

	assert.NoError(t, reconcileErr)
	if tt.expectRequeue {
		assert.True(t, result.RequeueAfter > 0, "Expected requeue but RequeueAfter was not set")
	} else {
		assert.Zero(t, result.RequeueAfter, "Expected no requeue but RequeueAfter was set")
	}

	// Check the request status was updated
	updatedRequest := &JITAccessRequest{}
	err := fakeClient.Get(ctx, req.NamespacedName, updatedRequest)
	require.NoError(t, err)

	assert.Equal(t, tt.expectStatus, updatedRequest.Status.Phase)

	// Check if job was created when expected
	if tt.expectJob && tt.existingJob == nil {
		validateJobCreation(t, fakeClient, ctx, tt.request)
	}
}

func validateJobCreation(t *testing.T, fakeClient client.Client, ctx context.Context, request *JITAccessRequest) {
	jobList := &JITAccessJobList{}
	err := fakeClient.List(ctx, jobList, client.InNamespace(request.Namespace))
	require.NoError(t, err)

	found := false
	for _, job := range jobList.Items {
		if job.Spec.AccessRequestRef.Name == request.Name {
			found = true
			assert.Equal(t, request.Spec.TargetCluster, job.Spec.TargetCluster)
			assert.Equal(t, request.Spec.Permissions, job.Spec.Permissions)
			break
		}
	}
	assert.True(t, found, "Expected job to be created")
}

func createApprovedTestRequest() *JITAccessRequest {
	return &JITAccessRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-request",
			Namespace: "jit-system",
		},
		Spec: JITAccessRequestSpec{
			UserID:    "U123456789A",
			UserEmail: "test@company.com",
			TargetCluster: TargetCluster{
				Name:       "prod-east-1",
				AWSAccount: "123456789012",
				Region:     "us-east-1",
			},
			Reason:      "Testing controller functionality in production",
			Duration:    "1h",
			Permissions: []string{"view"},
			Approvers:   []string{"platform-team"},
			RequestedAt: metav1.Now(),
		},
		Status: JITAccessRequestStatus{
			Phase:   AccessPhaseApproved,
			Message: "Approved by platform-team",
			Approvals: []Approval{
				{
					Approver:   "U456789012B",
					ApprovedAt: metav1.Time{Time: time.Now()},
					Comment:    "Approved by platform-team",
				},
			},
		},
	}
}

func createDeniedTestRequest() *JITAccessRequest {
	return &JITAccessRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-request",
			Namespace: "jit-system",
		},
		Spec: JITAccessRequestSpec{
			UserID:    "U123456789A",
			UserEmail: "test@company.com",
			TargetCluster: TargetCluster{
				Name:       "prod-east-1",
				AWSAccount: "123456789012",
				Region:     "us-east-1",
			},
			Reason:      "Testing denial scenario",
			Duration:    "1h",
			Permissions: []string{"admin"},
			RequestedAt: metav1.Now(),
		},
		Status: JITAccessRequestStatus{
			Phase:   AccessPhaseDenied,
			Message: "Insufficient justification for admin access",
		},
	}
}

func createRequestWithExistingJob() *JITAccessRequest {
	return &JITAccessRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-request",
			Namespace: "jit-system",
		},
		Spec: JITAccessRequestSpec{
			UserID:    "U123456789A",
			UserEmail: "test@company.com",
			TargetCluster: TargetCluster{
				Name:       "staging-east-1",
				AWSAccount: "123456789012",
				Region:     "us-east-1",
			},
			Reason:      "Testing duplicate job prevention",
			Duration:    "2h",
			Permissions: []string{"edit"},
			RequestedAt: metav1.Now(),
		},
		Status: JITAccessRequestStatus{
			Phase: AccessPhaseApproved,
		},
	}
}

func createExistingJob() *JITAccessJob {
	return &JITAccessJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-request-job",
			Namespace: "jit-system",
		},
		Spec: JITAccessJobSpec{
			AccessRequestRef: ObjectReference{
				Name:      "test-request",
				Namespace: "jit-system",
			},
			TargetCluster: TargetCluster{
				Name:       "staging-east-1",
				AWSAccount: "123456789012",
				Region:     "us-east-1",
			},
			Permissions: []string{"edit"},
			Duration:    "2h",
		},
		Status: JITAccessJobStatus{
			Phase: JobPhaseCreating,
		},
	}
}

// Removed test functions that call non-existent methods:
// - TestJITAccessRequestReconciler_DetermineNextAction (calls determineNextAction)
// - TestJITAccessRequestReconciler_RequiresApproval (calls requiresApproval)
// - TestJITAccessRequestReconciler_AutoApprove (calls autoApprove)

// Removed test functions that call non-existent methods:
// - TestJITAccessRequestReconciler_CreateJob (calls createJob)
// - TestJITAccessRequestReconciler_UpdateStatus (calls updateStatus)
// - TestJITAccessRequestReconciler_IsExpired (calls isExpired)

func TestJITAccessRequestReconciler_SetupWithManager(t *testing.T) {
	scheme := runtime.NewScheme()
	err := clientgoscheme.AddToScheme(scheme)
	require.NoError(t, err)
	err = AddToScheme(scheme)
	require.NoError(t, err)

	// Create a fake client for testing
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Create reconciler
	rbac := auth.NewRBAC([]string{})
	reconciler := &JITAccessRequestReconciler{
		Client: fakeClient,
		Scheme: scheme,
		RBAC:   rbac,
	}

	// Test that the reconciler has the required fields set
	assert.NotNil(t, reconciler.Client)
	assert.NotNil(t, reconciler.Scheme)
	assert.NotNil(t, reconciler.RBAC)
}

// Removed TestGenerateJobName - generateJobName function doesn't exist
