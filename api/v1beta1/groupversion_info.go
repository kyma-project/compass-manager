// Package v1beta1 contains API Schema definitions for the operator v1beta1 API group
// +kubebuilder:object:generate=true
// +groupName=operator.kyma-project.io
package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "operator.kyma-project.io", Version: "v1beta1"} //nolint:gochecknoglobals

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes) //nolint:gochecknoglobals

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme //nolint:gochecknoglobals
)

// addKnownTypes registers the group version metadata to the scheme
func addKnownTypes(scheme *runtime.Scheme) error {
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}
