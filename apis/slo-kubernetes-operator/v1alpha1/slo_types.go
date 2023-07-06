package v1alpha1

import (
	openslov1 "github.com/SLO-Kubernetes-Operator/slo-kubernetes-operator/apis/openslo/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SLOSpec defines the desired state of SLO
type SLOSpec struct {
	Spec     openslov1.SLOSpec `json:"spec,omitempty"`
	DataSink string            `json:"dataSink,omitempty"`
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
