package controller

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/rebelopsio/jit-bot/pkg/kubernetes"
	"github.com/rebelopsio/jit-bot/pkg/models"
)

// JITAccessJobReconciler reconciles a JITAccessJob object
type JITAccessJobReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	AccessManager *kubernetes.AccessManager
}

//+kubebuilder:rbac:groups=jit.rebelops.io,resources=jitaccessjobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=jit.rebelops.io,resources=jitaccessjobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=jit.rebelops.io,resources=jitaccessjobs/finalizers,verbs=update
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles JITAccessJob lifecycle
func (r *JITAccessJobReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the JITAccessJob instance
	var job JITAccessJob
	if err := r.Get(ctx, req.NamespacedName, &job); err != nil {
		log.Error(err, "unable to fetch JITAccessJob")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle different phases
	switch job.Status.Phase {
	case "", JobPhasePending:
		return r.handlePendingJob(ctx, &job)
	case JobPhaseCreating:
		return r.handleCreatingJob(ctx, &job)
	case JobPhaseActive:
		return r.handleActiveJob(ctx, &job)
	case JobPhaseExpiring:
		return r.handleExpiringJob(ctx, &job)
	case JobPhaseCompleted, JobPhaseFailed:
		return r.handleCompletedJob(ctx, &job)
	default:
		log.Info("No action needed for current phase", "phase", job.Status.Phase)
		return ctrl.Result{}, nil
	}
}

func (r *JITAccessJobReconciler) handlePendingJob(ctx context.Context, job *JITAccessJob) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Initialize status
	job.Status.Phase = JobPhaseCreating
	now := metav1.Now()
	job.Status.StartTime = &now

	// Parse duration and set expiry time
	duration, err := time.ParseDuration(job.Spec.Duration)
	if err != nil {
		job.Status.Phase = JobPhaseFailed
		r.setJobCondition(job, metav1.Condition{
			Type:               "Failed",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "InvalidDuration",
			Message:            fmt.Sprintf("Failed to parse duration: %v", err),
		})
		return ctrl.Result{}, r.Status().Update(ctx, job)
	}

	expiryTime := metav1.NewTime(now.Add(duration))
	job.Status.ExpiryTime = &expiryTime

	r.setJobCondition(job, metav1.Condition{
		Type:               "Started",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "JobStarted",
		Message:            "JIT access job has started",
	})

	if err := r.Status().Update(ctx, job); err != nil {
		log.Error(err, "unable to update JITAccessJob status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{Requeue: true}, nil
}

func (r *JITAccessJobReconciler) handleCreatingJob(ctx context.Context, job *JITAccessJob) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get the original access request
	var accessReq JITAccessRequest
	if err := r.Get(ctx, client.ObjectKey{
		Name:      job.Spec.AccessRequestRef.Name,
		Namespace: job.Spec.AccessRequestRef.Namespace,
	}, &accessReq); err != nil {
		log.Error(err, "unable to fetch JITAccessRequest")
		job.Status.Phase = JobPhaseFailed
		r.setJobCondition(job, metav1.Condition{
			Type:               "Failed",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "AccessRequestNotFound",
			Message:            fmt.Sprintf("Failed to fetch access request: %v", err),
		})
		if updateErr := r.Status().Update(ctx, job); updateErr != nil {
			log.Error(updateErr, "unable to update JITAccessJob status")
		}
		return ctrl.Result{}, err
	}

	// Create AWS access
	grantReq := kubernetes.GrantAccessRequest{
		ClusterAccess: r.convertToClusterAccess(&accessReq),
		Cluster:       r.convertToCluster(&accessReq.Spec.TargetCluster),
		UserEmail:     accessReq.Spec.UserEmail,
		Permissions:   job.Spec.Permissions,
		Namespaces:    job.Spec.Namespaces,
		JITRoleArn:    job.Spec.JITRoleArn,
	}

	credentials, err := r.AccessManager.GrantAccess(ctx, grantReq)
	if err != nil {
		log.Error(err, "failed to grant access")
		job.Status.Phase = JobPhaseFailed
		r.setJobCondition(job, metav1.Condition{
			Type:               "Failed",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "AccessGrantFailed",
			Message:            fmt.Sprintf("Failed to grant access: %v", err),
		})
		if updateErr := r.Status().Update(ctx, job); updateErr != nil {
			log.Error(updateErr, "unable to update JITAccessJob status")
		}
		return ctrl.Result{}, err
	}

	// Create secrets for credentials and kubeconfig
	credentialsSecret, err := r.createCredentialsSecret(job, credentials)
	if err != nil {
		log.Error(err, "failed to create credentials secret")
		return ctrl.Result{}, err
	}

	kubeConfigSecret, err := r.createKubeConfigSecret(job, credentials.KubeConfig)
	if err != nil {
		log.Error(err, "failed to create kubeconfig secret")
		return ctrl.Result{}, err
	}

	// Update job status
	job.Status.Phase = JobPhaseActive
	job.Status.AccessEntry = &JobAccessEntry{
		PrincipalArn: "arn:aws:sts::" + job.Spec.TargetCluster.AWSAccount + ":assumed-role/JITAccessRole/" +
			fmt.Sprintf("jit-%s-%s-%d", accessReq.Spec.UserID, job.Spec.TargetCluster.Name, time.Now().Unix()),
		SessionName: fmt.Sprintf("jit-%s-%s", accessReq.Spec.UserID, job.Spec.TargetCluster.Name),
		CredentialsSecretRef: &ObjectReference{
			Name:      credentialsSecret.Name,
			Namespace: credentialsSecret.Namespace,
		},
	}
	job.Status.KubeConfigSecretRef = &ObjectReference{
		Name:      kubeConfigSecret.Name,
		Namespace: kubeConfigSecret.Namespace,
	}

	r.setJobCondition(job, metav1.Condition{
		Type:               "AccessGranted",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "AccessCreated",
		Message:            "JIT access has been successfully created",
	})

	if err := r.Status().Update(ctx, job); err != nil {
		log.Error(err, "unable to update JITAccessJob status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully granted JIT access", "user", accessReq.Spec.UserID, "cluster", job.Spec.TargetCluster.Name)

	// Check expiry periodically
	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

func (r *JITAccessJobReconciler) handleActiveJob(ctx context.Context, job *JITAccessJob) (ctrl.Result, error) {
	// Check if job has expired
	if job.Status.ExpiryTime != nil && time.Now().After(job.Status.ExpiryTime.Time) {
		job.Status.Phase = JobPhaseExpiring
		r.setJobCondition(job, metav1.Condition{
			Type:               "Expiring",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "AccessExpired",
			Message:            "JIT access has expired and is being cleaned up",
		})

		if err := r.Status().Update(ctx, job); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Continue monitoring
	return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
}

func (r *JITAccessJobReconciler) handleExpiringJob(ctx context.Context, job *JITAccessJob) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Get the original access request for cleanup
	var accessReq JITAccessRequest
	if err := r.Get(ctx, client.ObjectKey{
		Name:      job.Spec.AccessRequestRef.Name,
		Namespace: job.Spec.AccessRequestRef.Namespace,
	}, &accessReq); err != nil {
		log.Error(err, "unable to fetch JITAccessRequest for cleanup")
		// Continue with cleanup anyway
	} else {
		// Revoke access
		clusterAccess := r.convertToClusterAccess(&accessReq)
		cluster := r.convertToCluster(&accessReq.Spec.TargetCluster)

		if err := r.AccessManager.RevokeAccess(ctx, clusterAccess, cluster, job.Spec.JITRoleArn); err != nil {
			log.Error(err, "failed to revoke access")
			// Don't fail the job, just log the error
		}
	}

	// Clean up secrets
	if job.Status.AccessEntry != nil && job.Status.AccessEntry.CredentialsSecretRef != nil {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      job.Status.AccessEntry.CredentialsSecretRef.Name,
				Namespace: job.Status.AccessEntry.CredentialsSecretRef.Namespace,
			},
		}
		if err := r.Delete(ctx, secret); err != nil {
			log.Error(err, "failed to delete credentials secret")
		}
	}

	if job.Status.KubeConfigSecretRef != nil {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      job.Status.KubeConfigSecretRef.Name,
				Namespace: job.Status.KubeConfigSecretRef.Namespace,
			},
		}
		if err := r.Delete(ctx, secret); err != nil {
			log.Error(err, "failed to delete kubeconfig secret")
		}
	}

	// Mark as completed
	job.Status.Phase = JobPhaseCompleted
	now := metav1.Now()
	job.Status.CompletionTime = &now

	r.setJobCondition(job, metav1.Condition{
		Type:               "Completed",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "AccessRevoked",
		Message:            "JIT access has been successfully revoked and cleaned up",
	})

	if err := r.Status().Update(ctx, job); err != nil {
		log.Error(err, "unable to update JITAccessJob status")
		return ctrl.Result{}, err
	}

	log.Info("Successfully completed JIT access job")
	return ctrl.Result{}, nil
}

func (r *JITAccessJobReconciler) handleCompletedJob(ctx context.Context, job *JITAccessJob) (ctrl.Result, error) {
	// No action needed for completed jobs
	return ctrl.Result{}, nil
}

func (r *JITAccessJobReconciler) createCredentialsSecret(job *JITAccessJob, creds *kubernetes.AccessCredentials) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("jit-credentials-%s", job.Name),
			Namespace: job.Namespace,
			Labels: map[string]string{
				"jit.rebelops.io/job":  job.Name,
				"jit.rebelops.io/type": "credentials",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"aws-access-key-id":     []byte(creds.TemporaryCredentials.AccessKeyId),
			"aws-secret-access-key": []byte(creds.TemporaryCredentials.SecretAccessKey),
			"aws-session-token":     []byte(creds.TemporaryCredentials.SessionToken),
			"expires-at":            []byte(creds.ExpiresAt.Format(time.RFC3339)),
		},
	}

	return secret, r.Create(context.TODO(), secret)
}

func (r *JITAccessJobReconciler) createKubeConfigSecret(job *JITAccessJob, kubeConfig string) (*corev1.Secret, error) {
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("jit-kubeconfig-%s", job.Name),
			Namespace: job.Namespace,
			Labels: map[string]string{
				"jit.rebelops.io/job":  job.Name,
				"jit.rebelops.io/type": "kubeconfig",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			"kubeconfig": []byte(kubeConfig),
		},
	}

	return secret, r.Create(context.TODO(), secret)
}

func (r *JITAccessJobReconciler) setJobCondition(job *JITAccessJob, condition metav1.Condition) {
	// Find existing condition of the same type
	for i, existing := range job.Status.Conditions {
		if existing.Type == condition.Type {
			job.Status.Conditions[i] = condition
			return
		}
	}

	// Add new condition
	job.Status.Conditions = append(job.Status.Conditions, condition)
}

// Helper functions to convert between types
func (r *JITAccessJobReconciler) convertToClusterAccess(req *JITAccessRequest) *models.ClusterAccess {
	duration, _ := time.ParseDuration(req.Spec.Duration)
	return &models.ClusterAccess{
		ID:          req.Name,
		ClusterID:   req.Spec.TargetCluster.Name,
		UserID:      req.Spec.UserID,
		UserEmail:   req.Spec.UserEmail,
		Reason:      req.Spec.Reason,
		Duration:    duration,
		Status:      models.AccessStatusActive,
		RequestedAt: req.Spec.RequestedAt.Time,
	}
}

func (r *JITAccessJobReconciler) convertToCluster(target *TargetCluster) *models.Cluster {
	duration, _ := time.ParseDuration("8h") // Default max duration
	return &models.Cluster{
		ID:          target.Name,
		Name:        target.Name,
		DisplayName: target.Name,
		AWSAccount:  target.AWSAccount,
		Region:      target.Region,
		MaxDuration: duration,
		Enabled:     true,
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *JITAccessJobReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&JITAccessJob{}).
		Owns(&corev1.Secret{}). // Watch owned secrets
		Complete(r)
}
