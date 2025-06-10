package controller

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "jit.rebelops.io", Version: "v1"}
)

var (
	// SchemeBuilder is used to add go types to the GroupVersionScheme
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&JITAccessRequest{},
		&JITAccessRequestList{},
		&JITAccessJob{},
		&JITAccessJobList{},
	)
	return nil
}
