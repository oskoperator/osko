package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Ruler struct {
	// +kubebuilder:default=false
	Enabled bool   `json:"enabled,omitempty"`
	Subpath string `json:"subpath,omitempty"`
}

type Tenant struct {
	// +kubebuilder:default=false
	Enabled bool   `json:"enabled,omitempty"`
	ID      string `json:"ID,omitempty"`
}

// ConnectionDetails specify how to connect to your metrics data provider
type ConnectionDetails struct {
	Address string `json:"address,omitempty"`
	Ruler   Ruler  `json:"ruler,omitempty"`
	Tenant  Tenant `json:"tenant,omitempty"`
}

// DatasourceSpec defines the desired state of Datasource
type DatasourceSpec struct {
	Description       string            `json:"description,omitempty"`
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
