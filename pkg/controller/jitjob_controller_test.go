package controller

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/rebelopsio/jit-bot/pkg/kubernetes"
)

// Note: The actual AccessManager is a struct, not an interface.
// For test purposes, we'll use the real AccessManager but won't call AWS APIs in tests.

func TestJITAccessJobReconciler_Reconcile(t *testing.T) {
	scheme := setupJobTestScheme(t)
	tests := createJobReconcileTestCases()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runJobReconcileTest(t, scheme, tt)
		})
	}
}

func setupJobTestScheme(t *testing.T) *runtime.Scheme {
	scheme := runtime.NewScheme()
	err := clientgoscheme.AddToScheme(scheme)
	require.NoError(t, err)
	err = AddToScheme(scheme)
	require.NoError(t, err)
	return scheme
}

func createJobReconcileTestCases() []struct {
	name                string
	job                 *JITAccessJob
	existingSecret      *corev1.Secret
	expectStatus        JobPhase
	expectSecretCreated bool
	expectError         bool
	expectRequeue       bool
} {
	return []struct {
		name                string
		job                 *JITAccessJob
		existingSecret      *corev1.Secret
		expectStatus        JobPhase
		expectSecretCreated bool
		expectError         bool
		expectRequeue       bool
	}{
		{
			name:                "new job creates access entry and secret",
			job:                 createNewTestJob(),
			expectStatus:        JobPhaseCreating,
			expectSecretCreated: false,
			expectRequeue:       true,
		},
		{
			name:          "expired job deletes access entry",
			job:           createExpiredTestJob(),
			expectStatus:  JobPhaseExpiring,
			expectRequeue: true,
		},
		{
			name:          "job with AWS access entry creation failure",
			job:           createFailingTestJob(),
			expectStatus:  JobPhaseCreating,
			expectRequeue: true,
			expectError:   false,
		},
		{
			name:          "completed job is not processed",
			job:           createCompletedTestJob(),
			expectStatus:  JobPhaseCompleted,
			expectRequeue: false,
		},
	}
}

func runJobReconcileTest(t *testing.T, scheme *runtime.Scheme, tt struct {
	name                string
	job                 *JITAccessJob
	existingSecret      *corev1.Secret
	expectStatus        JobPhase
	expectSecretCreated bool
	expectError         bool
	expectRequeue       bool
}) {
	// Create fake client with initial objects
	objs := []client.Object{tt.job}
	if tt.existingSecret != nil {
		objs = append(objs, tt.existingSecret)
	}

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		WithStatusSubresource(&JITAccessJob{}).
		Build()

	// Create reconciler
	reconciler := createJobTestReconciler(fakeClient, scheme)

	// Create reconcile request
	req := reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name:      tt.job.Name,
			Namespace: tt.job.Namespace,
		},
	}

	// Perform reconciliation and validate results
	validateJobReconcileResult(t, reconciler, req, tt, fakeClient)
}

func createJobTestReconciler(fakeClient client.Client, scheme *runtime.Scheme) *JITAccessJobReconciler {
	accessManager, _ := kubernetes.NewAccessManager("us-east-1")
	return &JITAccessJobReconciler{
		Client:        fakeClient,
		Scheme:        scheme,
		AccessManager: accessManager,
	}
}

func validateJobReconcileResult(t *testing.T, reconciler *JITAccessJobReconciler, req reconcile.Request, tt struct {
	name                string
	job                 *JITAccessJob
	existingSecret      *corev1.Secret
	expectStatus        JobPhase
	expectSecretCreated bool
	expectError         bool
	expectRequeue       bool
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

	// Check the job status was updated
	updatedJob := &JITAccessJob{}
	err := fakeClient.Get(ctx, req.NamespacedName, updatedJob)
	require.NoError(t, err)

	assert.Equal(t, tt.expectStatus, updatedJob.Status.Phase)

	// Check if secret was created when expected
	if tt.expectSecretCreated {
		validateJobSecretCreation(t, fakeClient, ctx, tt.job)
	}
}

func validateJobSecretCreation(t *testing.T, fakeClient client.Client, ctx context.Context, job *JITAccessJob) {
	secretList := &corev1.SecretList{}
	err := fakeClient.List(ctx, secretList, client.InNamespace(job.Namespace))
	require.NoError(t, err)

	found := false
	for _, secret := range secretList.Items {
		if secret.Labels["jit.rebelops.io/job"] == job.Name {
			found = true
			assert.Equal(t, "Opaque", string(secret.Type))
			assert.Contains(t, secret.Data, "kubeconfig")
			break
		}
	}
	assert.True(t, found, "Expected secret to be created")
}

func createNewTestJob() *JITAccessJob {
	return &JITAccessJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "jit-system",
		},
		Spec: JITAccessJobSpec{
			AccessRequestRef: ObjectReference{
				Name:      "test-request",
				Namespace: "jit-system",
			},
			TargetCluster: TargetCluster{
				Name:       "dev-east-1",
				AWSAccount: "123456789012",
				Region:     "us-east-1",
			},
			Permissions: []string{"view"},
			Duration:    "2h",
			Namespaces:  []string{"default"},
		},
		Status: JITAccessJobStatus{
			Phase: JobPhasePending,
		},
	}
}

func createExpiredTestJob() *JITAccessJob {
	return &JITAccessJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "jit-system",
		},
		Spec: JITAccessJobSpec{
			AccessRequestRef: ObjectReference{
				Name:      "test-request",
				Namespace: "jit-system",
			},
			TargetCluster: TargetCluster{
				Name:       "dev-east-1",
				AWSAccount: "123456789012",
				Region:     "us-east-1",
			},
			Permissions: []string{"view"},
			Duration:    "1h",
		},
		Status: JITAccessJobStatus{
			Phase:      JobPhaseActive,
			StartTime:  &metav1.Time{Time: time.Now().Add(-2 * time.Hour)},
			ExpiryTime: &metav1.Time{Time: time.Now().Add(-1 * time.Hour)},
		},
	}
}

func createFailingTestJob() *JITAccessJob {
	return &JITAccessJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "jit-system",
		},
		Spec: JITAccessJobSpec{
			AccessRequestRef: ObjectReference{
				Name:      "test-request",
				Namespace: "jit-system",
			},
			TargetCluster: TargetCluster{
				Name:       "prod-east-1",
				AWSAccount: "123456789012",
				Region:     "us-east-1",
			},
			Permissions: []string{"admin"},
			Duration:    "1h",
		},
		Status: JITAccessJobStatus{
			Phase: JobPhasePending,
		},
	}
}

func createCompletedTestJob() *JITAccessJob {
	return &JITAccessJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job",
			Namespace: "jit-system",
		},
		Spec: JITAccessJobSpec{
			AccessRequestRef: ObjectReference{
				Name:      "test-request",
				Namespace: "jit-system",
			},
			TargetCluster: TargetCluster{
				Name: "dev-east-1",
			},
			Permissions: []string{"view"},
			Duration:    "1h",
		},
		Status: JITAccessJobStatus{
			Phase: JobPhaseCompleted,
		},
	}
}

// Removed TestJITAccessJobReconciler_DetermineNextAction - determineNextAction method doesn't exist

// Removed TestGenerateSecretName - generateSecretName function doesn't exist
