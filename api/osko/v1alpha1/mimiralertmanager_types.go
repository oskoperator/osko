package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MimirAlertManagerSpec defines the desired state of MimirAlertManager
type MimirAlertManagerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of MimirAlertManager. Edit mimiralertmanager_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// MimirAlertManagerStatus defines the observed state of MimirAlertManager
type MimirAlertManagerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MimirAlertManager is the Schema for the mimiralertmanagers API
type MimirAlertManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MimirAlertManagerSpec   `json:"spec,omitempty"`
	Status MimirAlertManagerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MimirAlertManagerList contains a list of MimirAlertManager
type MimirAlertManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MimirAlertManager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MimirAlertManager{}, &MimirAlertManagerList{})
}
