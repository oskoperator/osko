package v1alpha1

// +kubebuilder:object:generate=true
type MetricSourceSpec struct {
	Query string `json:"query,omitempty"`
}
