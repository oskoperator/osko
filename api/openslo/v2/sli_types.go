package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MetricSpec struct {
	MetricSource MetricSource `json:"metricSource,omitempty"`
}

type RatioMetricSpec struct {
	Raw MetricSpec `json:"raw,omitempty"`
	// +kubebuilder:validation:Enum=success;failure
	RawType string     `json:"rawType,omitempty"`
	Good    MetricSpec `json:"good,omitempty"`
	Bad     MetricSpec `json:"bad,omitempty"`
	Total   MetricSpec `json:"total,omitempty"`
	Counter bool       `json:"counter,omitempty"`
}

type ThresholdMetricSpec struct {
	MetricSource MetricSource `json:"metricSource,omitempty"`
}

// SLISpec defines the desired state of SLI
type SLISpec struct {
	Description     Description         `json:"description,omitempty"`
	ThresholdMetric ThresholdMetricSpec `json:"thresholdMetric,omitempty"`
	RatioMetric     RatioMetricSpec     `json:"ratioMetric,omitempty"`
}

// SLIStatus defines the observed state of SLI
type SLIStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SLI is the Schema for the slis API
type SLI struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SLISpec   `json:"spec,omitempty"`
	Status SLIStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SLIList contains a list of SLI
type SLIList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SLI `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SLI{}, &SLIList{})
}
