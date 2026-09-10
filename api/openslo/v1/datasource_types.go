package v1

import (
	osko "github.com/oskoperator/osko/api/osko/v1alpha1"
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
	Description Description `json:"description,omitempty"`

	// Type selects the metrics backend this Datasource points at.
	// Defaulted so that a Datasource written before this field existed keeps
	// resolving to a backend instead of failing to parse.
	// +kubebuilder:validation:Enum=prometheus;mimir;cortex;thanos;victoriametrics
	// +kubebuilder:default=mimir
	Type string `json:"type,omitempty"`

	ConnectionDetails osko.ConnectionDetails `json:"connectionDetails,omitempty"`
}

// DatasourceStatus defines the observed state of Datasource
type DatasourceStatus struct {
	// Conditions holds the latest observations of the Datasource state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Ready mirrors the status of the Ready condition so it can be surfaced
	// as a printer column.
	Ready string `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced
//+kubebuilder:printcolumn:name="Type",type=string,JSONPath=.spec.type,description="The metrics backend this Datasource points at"
//+kubebuilder:printcolumn:name="Ready",type=string,JSONPath=.status.ready,description="The reason for the current status of the Datasource resource"

// Datasource is the Schema for the datasources API
type Datasource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

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
