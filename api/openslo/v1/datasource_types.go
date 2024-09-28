package v1

import (
	openslov1 "github.com/OpenSLO/OpenSLO/pkg/openslo/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatasourceStatus defines the observed state of Datasource
type DatasourceStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// Datasource is the Schema for the datasources API
type Datasource struct {
	metav1.TypeMeta   `json:",inline"`
	ObjectMetaOpenSLO `json:"metadata,omitempty"`

	Spec   openslov1.DataSourceSpec `json:"spec,omitempty"`
	Status DatasourceStatus         `json:"status,omitempty"`
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
