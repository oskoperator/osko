package v1

import (
	openslov1 "github.com/OpenSLO/OpenSLO/pkg/openslo/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ServiceStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Service is the Schema for the services API
type Service struct {
	metav1.TypeMeta   `json:",inline"`
	ObjectMetaOpenSLO `json:"metadata,omitempty"`

	Spec   openslov1.ServiceSpec `json:"spec,omitempty"`
	Status ServiceStatus         `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ServiceList contains a list of Service
type ServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Service `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Service{}, &ServiceList{})
}
