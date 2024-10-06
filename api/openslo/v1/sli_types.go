package v1

import (
	openslov1 "github.com/OpenSLO/OpenSLO/pkg/openslo/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SLIStatus defines the observed state of SLI
type SLIStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status

// SLI is the Schema for the slis API
type SLI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   openslov1.SLISpec `json:"spec,omitempty"`
	Status SLIStatus         `json:"status,omitempty"`
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
