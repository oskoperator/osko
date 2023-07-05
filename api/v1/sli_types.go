package v1

import (
	common "github.com/SLO-Kubernetes-Operator/slo-kubernetes-operator/api/v1/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type RatioMetricSpec struct {
	Raw map[string]common.MetricSourceSpec `json:"raw,omitempty"`
	// +kubebuilder:validation:Enum=success;failure
	RawType string                             `json:"rawType,omitempty"`
	Good    map[string]common.MetricSourceSpec `json:"good,omitempty"`
	Bad     map[string]common.MetricSourceSpec `json:"bad,omitempty"`
	Total   map[string]common.MetricSourceSpec `json:"total,omitempty"`
	Counter bool                               `json:"counter,omitempty"`
}

type ThresholdMetricSpec struct {
	MetricSource common.MetricSourceSpec `json:"metricSource,omitempty"`
}

// SLISpec defines the desired state of SLI
type SLISpec struct {
	Description     common.Description  `json:"description,omitempty"`
	ThresholdMetric ThresholdMetricSpec `json:"thresholdMetrics,omitempty"`
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
