package webhook

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/rebelopsio/jit-bot/pkg/controller"
)

// SetupWebhookWithManager sets up the webhook server with the manager
func SetupWebhookWithManager(mgr ctrl.Manager) error {
	// Setup webhook server
	hookServer := mgr.GetWebhookServer()
	
	// Register validation webhook for JITAccessRequest
	validator := &JITAccessRequestValidator{
		Client: mgr.GetClient(),
	}
	hookServer.Register("/validate-jit-rebelops-io-v1alpha1-jitaccessrequest", 
		&webhook.Admission{Handler: validator})

	// Register mutation webhook for JITAccessRequest
	mutator := &JITAccessRequestMutator{
		Client: mgr.GetClient(),
	}
	hookServer.Register("/mutate-jit-rebelops-io-v1alpha1-jitaccessrequest",
		&webhook.Admission{Handler: mutator})

	// Register mutation webhook for JITAccessJob
	jobMutator := &JITAccessJobMutator{
		Client: mgr.GetClient(),
	}
	hookServer.Register("/mutate-jit-rebelops-io-v1alpha1-jitaccessjob",
		&webhook.Admission{Handler: jobMutator})

	return nil
}


// SetupCRDValidation sets up OpenAPI schema validation in CRDs
func SetupCRDValidation(scheme *runtime.Scheme) error {
	// Add JITAccessRequest to scheme with validation
	if err := controller.AddToScheme(scheme); err != nil {
		return fmt.Errorf("failed to add controller types to scheme: %w", err)
	}

	return nil
}