package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// CompassManagerSpec defines the desired state of CompassManager
type CompassManagerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of CompassManager. Edit compassmanager_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// CompassManagerStatus defines the observed state of CompassManager
type CompassManagerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CompassManager is the Schema for the compassmanagers API
type CompassManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CompassManagerSpec   `json:"spec,omitempty"`
	Status CompassManagerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// CompassManagerList contains a list of CompassManager
type CompassManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CompassManager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CompassManager{}, &CompassManagerList{})
}
