package v1

// +kubebuilder:validation:MaxLength=1050
type Description string

// +kubebuilder:validation:Pattern=`^[1-9]\d*[s m h d]$`
type Duration string

type MetricSourceSpec struct {
	MetricSourceRef string `json:"metricSourceRef,omitempty"`
	Type            string `json:"type,omitempty"`
	Spec            string `json:"spec,omitempty"`
}
