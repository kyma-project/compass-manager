package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CompassManagerMappingSpec defines the desired state of CompassManagerMapping
type CompassManagerMappingSpec struct{}

// CompassManagerMappingStatus defines the observed state of CompassManagerMapping
type CompassManagerMappingStatus struct {
	Registered bool   `json:"registered"`
	Configured bool   `json:"configured"`
	State      string `json:"state,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// CompassManagerMapping is the Schema for the compassmanagermappings API
type CompassManagerMapping struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Status CompassManagerMappingStatus `json:"status,omitempty"`
	Spec   CompassManagerMappingSpec   `json:"spec,omitempty"`
}

//+kubebuilder:object:root=true

// CompassManagerMappingList contains a list of CompassManagerMapping
type CompassManagerMappingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CompassManagerMapping `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CompassManagerMapping{}, &CompassManagerMappingList{})
}
