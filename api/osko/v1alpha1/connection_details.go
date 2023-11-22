package v1alpha1

// +kubebuilder:object:generate=true
type ConnectionDetails struct {
	Address             string   `json:"address,omitempty"`
	TargetTenant        string   `json:"targetTenant,omitempty"`
	SourceTenants       []string `json:"sourceTenants,omitempty"`
	SyncPrometheusRules bool     `json:"syncPrometheusRules,omitempty"`
}
