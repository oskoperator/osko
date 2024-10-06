package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ObjectMetaOpenSLO struct {
	metav1.ObjectMeta `json:",inline"`
	DisplayName       string `json:"displayName,omitempty"`
}

type Ruler struct {
	// +kubebuilder:default=false
	Enabled bool   `json:"enabled,omitempty"`
	Subpath string `json:"subpath,omitempty"`
}

type Multitenancy struct {
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`
	// +kubebuilder:MinItems=1
	SourceTenants []string `json:"sourceTenants,omitempty"`
	TargetTenant  string   `json:"targetTenant,omitempty"`
}
