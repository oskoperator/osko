package v1

import (
	openslov1 "github.com/OpenSLO/OpenSLO/pkg/openslo/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SLOStatus defines the observed state of SLO
type SLOStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	CurrentSLO         string             `json:"currentSLO,omitempty"`
	LastEvaluationTime metav1.Time        `json:"lastEvaluationTime,omitempty"`
	Ready              string             `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type=string,JSONPath=.status.ready,description="The reason for the current status of the SLO resource"
//+kubebuilder:printcolumn:name="Window",type=string,JSONPath=.spec.timeWindow[0].duration,description="The time window for the SLO resource"
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=.metadata.creationTimestamp,description="The time when the SLO resource was created"

// SLO is the Schema for the slos API
type SLO struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   openslov1.SLOSpec `json:"spec,omitempty"`
	Status SLOStatus         `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// SLOList contains a list of SLO
type SLOList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SLO `json:"items"`
}

func init() {
	SchemeBuilder.Register(&SLO{}, &SLOList{})
}
