package v1

import (
	osko "github.com/oskoperator/osko/apis/osko/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConnectionDetails specify how to connect to your metrics data provider
// +kubebuilder:validation:MinProperties=1
// +kubebuilder:validation:MaxProperties=1
type ConnectionDetails struct {
	Mimir  *osko.Mimir  `json:"mimir,omitempty"`
	Cortex *osko.Cortex `json:"cortex,omitempty"`
}

// DatasourceSpec defines the desired state of Datasource
type DatasourceSpec struct {
	Description       Description       `json:"description,omitempty"`
	Type              string            `json:"type,omitempty"`
	ConnectionDetails ConnectionDetails `json:"connectionDetails,omitempty"`
}

// DatasourceStatus defines the observed state of Datasource
type DatasourceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced

// Datasource is the Schema for the datasources API
type Datasource struct {
	metav1.TypeMeta   `json:",inline"`
	ObjectMetaOpenSLO `json:"metadata,omitempty"`

	Spec   DatasourceSpec   `json:"spec,omitempty"`
	Status DatasourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// DatasourceList contains a list of Datasource
type DatasourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Datasource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Datasource{}, &DatasourceList{})
}
