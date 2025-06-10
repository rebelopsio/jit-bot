package controller

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// JITAccessRequest represents a request for just-in-time access to an EKS cluster
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="User",type=string,JSONPath=`.spec.userID`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.targetCluster.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Duration",type=string,JSONPath=`.spec.duration`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type JITAccessRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JITAccessRequestSpec   `json:"spec,omitempty"`
	Status JITAccessRequestStatus `json:"status,omitempty"`
}

type JITAccessRequestSpec struct {
	// UserID is the Slack user ID requesting access
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^U[A-Z0-9]{10}$`
	UserID string `json:"userID"`

	// UserEmail is the email address of the requesting user
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
	UserEmail string `json:"userEmail"`

	// TargetCluster specifies the EKS cluster to access
	// +kubebuilder:validation:Required
	TargetCluster TargetCluster `json:"targetCluster"`

	// Reason is the business justification for access
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=10
	// +kubebuilder:validation:MaxLength=500
	Reason string `json:"reason"`

	// Duration is the requested access duration
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^(\d+[dhms])+$`
	Duration string `json:"duration"`

	// Permissions are the requested permission levels
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Enum=view;edit;admin;cluster-admin;debug;logs;exec;port-forward
	Permissions []string `json:"permissions"`

	// Namespaces are the target namespaces (empty = cluster-wide)
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Namespaces []string `json:"namespaces,omitempty"`

	// Approvers are the required approvers for this request
	// +kubebuilder:validation:Optional
	Approvers []string `json:"approvers,omitempty"`

	// SlackChannel is where the request was made
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^C[A-Z0-9]{10}$`
	SlackChannel string `json:"slackChannel,omitempty"`

	// RequestedAt is when the request was created
	// +kubebuilder:validation:Required
	RequestedAt metav1.Time `json:"requestedAt"`
}

type TargetCluster struct {
	// Name is the EKS cluster name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=100
	Name string `json:"name"`

	// AWSAccount is the AWS account ID where cluster resides
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^\d{12}$`
	AWSAccount string `json:"awsAccount"`

	// Region is the AWS region
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z]{2}-[a-z]+-\d{1}$`
	Region string `json:"region"`

	// Endpoint is the EKS cluster endpoint URL
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Pattern=`^https://.*$`
	Endpoint string `json:"endpoint,omitempty"`
}

type JITAccessRequestStatus struct {
	// Phase represents the current phase of the access request
	Phase AccessPhase `json:"phase,omitempty"`

	// Approvals is the list of approvals received
	Approvals []Approval `json:"approvals,omitempty"`

	// AccessEntry contains details of the granted access
	AccessEntry *AccessEntryStatus `json:"accessEntry,omitempty"`

	// Conditions represent the current condition of the access request
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Message is a human readable message about current status
	Message string `json:"message,omitempty"`
}

type AccessPhase string

const (
	AccessPhasePending  AccessPhase = "Pending"
	AccessPhaseApproved AccessPhase = "Approved"
	AccessPhaseDenied   AccessPhase = "Denied"
	AccessPhaseActive   AccessPhase = "Active"
	AccessPhaseExpired  AccessPhase = "Expired"
	AccessPhaseRevoked  AccessPhase = "Revoked"
)

type Approval struct {
	// Approver is the user ID who approved
	Approver string `json:"approver"`

	// ApprovedAt is when the approval was given
	ApprovedAt metav1.Time `json:"approvedAt"`

	// Comment is an optional approval comment
	Comment string `json:"comment,omitempty"`
}

type AccessEntryStatus struct {
	// PrincipalArn is the ARN of the principal granted access
	PrincipalArn string `json:"principalArn"`

	// SessionName is the STS session name
	SessionName string `json:"sessionName"`

	// CreatedAt is when the access was granted
	CreatedAt metav1.Time `json:"createdAt"`

	// ExpiresAt is when the access expires
	ExpiresAt metav1.Time `json:"expiresAt"`
}

// JITAccessJob represents a Kubernetes job that manages the lifecycle of JIT access
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Request",type=string,JSONPath=`.spec.accessRequestRef.name`
// +kubebuilder:printcolumn:name="Cluster",type=string,JSONPath=`.spec.targetCluster.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Expires",type=date,JSONPath=`.status.expiryTime`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
type JITAccessJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JITAccessJobSpec   `json:"spec,omitempty"`
	Status JITAccessJobStatus `json:"status,omitempty"`
}

type JITAccessJobSpec struct {
	// AccessRequestRef references the JITAccessRequest
	AccessRequestRef ObjectReference `json:"accessRequestRef"`

	// TargetCluster specifies the EKS cluster
	TargetCluster TargetCluster `json:"targetCluster"`

	// Duration is the access duration
	Duration string `json:"duration"`

	// JITRoleArn is the ARN of the JIT role to assume
	JITRoleArn string `json:"jitRoleArn"`

	// Permissions are the requested permission levels
	Permissions []string `json:"permissions"`

	// Namespaces are the target namespaces
	Namespaces []string `json:"namespaces,omitempty"`

	// CleanupPolicy defines when to cleanup the access
	CleanupPolicy CleanupPolicy `json:"cleanupPolicy,omitempty"`
}

type ObjectReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type CleanupPolicy string

const (
	CleanupPolicyOnExpiry CleanupPolicy = "OnExpiry"
	CleanupPolicyOnDelete CleanupPolicy = "OnDelete"
	CleanupPolicyManual   CleanupPolicy = "Manual"
)

type JITAccessJobStatus struct {
	// Phase represents the current phase of the job
	Phase JobPhase `json:"phase,omitempty"`

	// StartTime is when the job started
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime is when the job completed
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// ExpiryTime is when the access expires
	ExpiryTime *metav1.Time `json:"expiryTime,omitempty"`

	// AccessEntry contains the created access entry details
	AccessEntry *JobAccessEntry `json:"accessEntry,omitempty"`

	// KubeConfigSecretRef references the generated kubeconfig secret
	KubeConfigSecretRef *ObjectReference `json:"kubeConfigSecretRef,omitempty"`

	// Conditions represent the current condition of the job
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type JobPhase string

const (
	JobPhasePending   JobPhase = "Pending"
	JobPhaseCreating  JobPhase = "Creating"
	JobPhaseActive    JobPhase = "Active"
	JobPhaseExpiring  JobPhase = "Expiring"
	JobPhaseCompleted JobPhase = "Completed"
	JobPhaseFailed    JobPhase = "Failed"
)

type JobAccessEntry struct {
	// PrincipalArn is the ARN of the principal
	PrincipalArn string `json:"principalArn"`

	// SessionName is the STS session name
	SessionName string `json:"sessionName"`

	// CredentialsSecretRef references the credentials secret
	CredentialsSecretRef *ObjectReference `json:"credentialsSecretRef,omitempty"`
}

// JITAccessRequestList contains a list of JITAccessRequest
// +kubebuilder:object:root=true
type JITAccessRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JITAccessRequest `json:"items"`
}

// JITAccessJobList contains a list of JITAccessJob
// +kubebuilder:object:root=true
type JITAccessJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []JITAccessJob `json:"items"`
}

// DeepCopyObject implements runtime.Object
func (in *JITAccessRequest) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyObject implements runtime.Object
func (in *JITAccessRequestList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyObject implements runtime.Object
func (in *JITAccessJob) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyObject implements runtime.Object
func (in *JITAccessJobList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
