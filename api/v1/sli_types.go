package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// SLISpec defines the desired state of SLI
type SLISpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// Foo is an example field of SLI. Edit sli_types.go to remove/update
	Foo string `json:"foo,omitempty"`
}

// SLIStatus defines the observed state of SLI
type SLIStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SLI is the Schema for the slis API
type SLI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SLISpec   `json:"spec,omitempty"`
	Status SLIStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SLIList contains a list of SLI
type SLIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SLI `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SLI{}, &SLIList{})
}
