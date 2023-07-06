package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SLOSpec defines the desired state of SLO
type SLOSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of SLO. Edit slo_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// SLOStatus defines the observed state of SLO
type SLOStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SLO is the Schema for the slos API
type SLO struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SLOSpec   `json:"spec,omitempty"`
	Status SLOStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SLOList contains a list of SLO
type SLOList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SLO `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SLO{}, &SLOList{})
}
