package controller

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/rebelopsio/jit-bot/pkg/auth"
)

// JITAccessRequestReconciler reconciles a JITAccessRequest object
type JITAccessRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	RBAC   *auth.RBAC
}

//+kubebuilder:rbac:groups=jit.rebelops.io,resources=jitaccessrequests,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=jit.rebelops.io,resources=jitaccessrequests/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=jit.rebelops.io,resources=jitaccessrequests/finalizers,verbs=update
//+kubebuilder:rbac:groups=jit.rebelops.io,resources=jitaccessjobs,verbs=get;list;watch;create;update;patch;delete

// Reconcile handles JITAccessRequest lifecycle
func (r *JITAccessRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch the JITAccessRequest instance
	var jitReq JITAccessRequest
	if err := r.Get(ctx, req.NamespacedName, &jitReq); err != nil {
		log.Error(err, "unable to fetch JITAccessRequest")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle different phases
	switch jitReq.Status.Phase {
	case "", AccessPhasePending:
		return r.handlePendingRequest(ctx, &jitReq)
	case AccessPhaseApproved:
		return r.handleApprovedRequest(ctx, &jitReq)
	case AccessPhaseActive:
		return r.handleActiveRequest(ctx, &jitReq)
	case AccessPhaseExpired, AccessPhaseRevoked:
		return r.handleExpiredRequest(ctx, &jitReq)
	default:
		log.Info("No action needed for current phase", "phase", jitReq.Status.Phase)
		return ctrl.Result{}, nil
	}
}

func (r *JITAccessRequestReconciler) handlePendingRequest(ctx context.Context, jitReq *JITAccessRequest) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Initialize status if empty
	if jitReq.Status.Phase == "" {
		jitReq.Status.Phase = AccessPhasePending
		jitReq.Status.Message = "Request pending approval"
		jitReq.Status.Conditions = []metav1.Condition{
			{
				Type:               "Submitted",
				Status:             metav1.ConditionTrue,
				LastTransitionTime: metav1.Now(),
				Reason:             "RequestSubmitted",
				Message:            "JIT access request has been submitted",
			},
		}
		
		if err := r.Status().Update(ctx, jitReq); err != nil {
			log.Error(err, "unable to update JITAccessRequest status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{RequeueAfter: time.Second * 30}, nil
	}

	// Check if auto-approval is possible or if approvals are sufficient
	if r.shouldAutoApprove(jitReq) || r.hasRequiredApprovals(jitReq) {
		jitReq.Status.Phase = AccessPhaseApproved
		jitReq.Status.Message = "Request approved"
		
		// Add approval condition
		r.setCondition(jitReq, metav1.Condition{
			Type:               "Approved",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "RequiredApprovalsReceived",
			Message:            "JIT access request has been approved",
		})

		if err := r.Status().Update(ctx, jitReq); err != nil {
			log.Error(err, "unable to update JITAccessRequest status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Request is still pending, check periodically
	return ctrl.Result{RequeueAfter: time.Minute * 5}, nil
}

func (r *JITAccessRequestReconciler) handleApprovedRequest(ctx context.Context, jitReq *JITAccessRequest) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Create JITAccessJob to handle the actual access provisioning
	job := r.createJITAccessJob(jitReq)
	
	// Set owner reference for cleanup
	if err := controllerutil.SetControllerReference(jitReq, job, r.Scheme); err != nil {
		log.Error(err, "unable to set owner reference")
		return ctrl.Result{}, err
	}

	if err := r.Create(ctx, job); err != nil {
		log.Error(err, "unable to create JITAccessJob")
		jitReq.Status.Message = fmt.Sprintf("Failed to create access job: %v", err)
		r.Status().Update(ctx, jitReq)
		return ctrl.Result{}, err
	}

	// Update status to active
	jitReq.Status.Phase = AccessPhaseActive
	jitReq.Status.Message = "Access provisioning in progress"
	
	r.setCondition(jitReq, metav1.Condition{
		Type:               "Provisioning",
		Status:             metav1.ConditionTrue,
		LastTransitionTime: metav1.Now(),
		Reason:             "JobCreated",
		Message:            "JIT access job has been created",
	})

	if err := r.Status().Update(ctx, jitReq); err != nil {
		log.Error(err, "unable to update JITAccessRequest status")
		return ctrl.Result{}, err
	}

	log.Info("Created JITAccessJob", "job", job.Name)
	return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
}

func (r *JITAccessRequestReconciler) handleActiveRequest(ctx context.Context, jitReq *JITAccessRequest) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Check if access has expired
	if r.isRequestExpired(jitReq) {
		jitReq.Status.Phase = AccessPhaseExpired
		jitReq.Status.Message = "Access has expired"
		
		r.setCondition(jitReq, metav1.Condition{
			Type:               "Expired",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "AccessExpired",
			Message:            "JIT access has expired",
		})

		if err := r.Status().Update(ctx, jitReq); err != nil {
			log.Error(err, "unable to update JITAccessRequest status")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Check associated JITAccessJob status
	// This would involve fetching the job and updating accordingly
	return r.syncWithJob(ctx, jitReq)
}

func (r *JITAccessRequestReconciler) handleExpiredRequest(ctx context.Context, jitReq *JITAccessRequest) (ctrl.Result, error) {
	// Ensure cleanup is complete
	// The JITAccessJob controller will handle the actual cleanup
	return ctrl.Result{}, nil
}

func (r *JITAccessRequestReconciler) shouldAutoApprove(jitReq *JITAccessRequest) bool {
	// Implement auto-approval logic based on:
	// - User role
	// - Requested permissions
	// - Target cluster policies
	// - Time restrictions
	
	// For now, auto-approve "view" permissions for known users
	if len(jitReq.Spec.Permissions) == 1 && jitReq.Spec.Permissions[0] == "view" {
		if r.RBAC.UserHasPermission(jitReq.Spec.UserID, auth.PermissionCreateRequests) {
			return true
		}
	}
	
	return false
}

func (r *JITAccessRequestReconciler) hasRequiredApprovals(jitReq *JITAccessRequest) bool {
	if len(jitReq.Spec.Approvers) == 0 {
		return true // No approvers required
	}

	// Count valid approvals
	validApprovals := 0
	for _, approval := range jitReq.Status.Approvals {
		for _, requiredApprover := range jitReq.Spec.Approvers {
			if approval.Approver == requiredApprover {
				validApprovals++
				break
			}
		}
	}

	// For now, require all approvers to approve
	return validApprovals >= len(jitReq.Spec.Approvers)
}

func (r *JITAccessRequestReconciler) isRequestExpired(jitReq *JITAccessRequest) bool {
	if jitReq.Status.AccessEntry == nil {
		return false
	}

	return time.Now().After(jitReq.Status.AccessEntry.ExpiresAt.Time)
}

func (r *JITAccessRequestReconciler) createJITAccessJob(jitReq *JITAccessRequest) *JITAccessJob {
	return &JITAccessJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("jit-%s-%s", jitReq.Spec.UserID, jitReq.Name),
			Namespace: jitReq.Namespace,
			Labels: map[string]string{
				"jit.rebelops.io/request": jitReq.Name,
				"jit.rebelops.io/user":    jitReq.Spec.UserID,
				"jit.rebelops.io/cluster": jitReq.Spec.TargetCluster.Name,
			},
		},
		Spec: JITAccessJobSpec{
			AccessRequestRef: ObjectReference{
				Name:      jitReq.Name,
				Namespace: jitReq.Namespace,
			},
			TargetCluster:   jitReq.Spec.TargetCluster,
			Duration:        jitReq.Spec.Duration,
			JITRoleArn:      r.getJITRoleArn(jitReq.Spec.TargetCluster),
			Permissions:     jitReq.Spec.Permissions,
			Namespaces:      jitReq.Spec.Namespaces,
			CleanupPolicy:   CleanupPolicyOnExpiry,
		},
	}
}

func (r *JITAccessRequestReconciler) syncWithJob(ctx context.Context, jitReq *JITAccessRequest) (ctrl.Result, error) {
	// Fetch associated JITAccessJob
	jobName := fmt.Sprintf("jit-%s-%s", jitReq.Spec.UserID, jitReq.Name)
	var job JITAccessJob
	if err := r.Get(ctx, client.ObjectKey{Name: jobName, Namespace: jitReq.Namespace}, &job); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Update request status based on job status
	if job.Status.AccessEntry != nil && jitReq.Status.AccessEntry == nil {
		jitReq.Status.AccessEntry = &AccessEntryStatus{
			PrincipalArn: job.Status.AccessEntry.PrincipalArn,
			SessionName:  job.Status.AccessEntry.SessionName,
			CreatedAt:    *job.Status.StartTime,
			ExpiresAt:    *job.Status.ExpiryTime,
		}

		r.setCondition(jitReq, metav1.Condition{
			Type:               "AccessGranted",
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "AccessProvisioned",
			Message:            "JIT access has been successfully provisioned",
		})

		if err := r.Status().Update(ctx, jitReq); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Check again in 2 minutes
	return ctrl.Result{RequeueAfter: time.Minute * 2}, nil
}

func (r *JITAccessRequestReconciler) getJITRoleArn(cluster TargetCluster) string {
	// This would be configurable per cluster or environment
	return fmt.Sprintf("arn:aws:iam::%s:role/JITAccessRole", cluster.AWSAccount)
}

func (r *JITAccessRequestReconciler) setCondition(jitReq *JITAccessRequest, condition metav1.Condition) {
	// Find existing condition of the same type
	for i, existing := range jitReq.Status.Conditions {
		if existing.Type == condition.Type {
			jitReq.Status.Conditions[i] = condition
			return
		}
	}
	
	// Add new condition
	jitReq.Status.Conditions = append(jitReq.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *JITAccessRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&JITAccessRequest{}).
		Owns(&JITAccessJob{}). // Watch owned JITAccessJobs
		Complete(r)
}