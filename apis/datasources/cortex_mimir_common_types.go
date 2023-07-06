package v1alpha1

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
