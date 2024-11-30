package v2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConditionSpec struct {
	// +kubebuilder:validation:Enum=Burnrate
	Kind string `json:"kind,omitempty"`
	// +kubebuilder:validation:Enum=lte;gte;lt;gt
	Op             string   `json:"op,omitempty"`
	Threshold      string   `json:"threshold,omitempty"`
	LookbackWindow Duration `json:"lookbackWindow,omitempty"`
	AlertAfter     Duration `json:"alertAfter,omitempty"`
}

// AlertConditionSpec defines the desired state of AlertCondition
type AlertConditionSpec struct {
	Description Description   `json:"description,omitempty"`
	Severity    string        `json:"severity,omitempty"`
	Condition   ConditionSpec `json:"condition,omitempty"`
}

// AlertConditionStatus defines the observed state of AlertCondition
type AlertConditionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AlertCondition is the Schema for the alertconditions API
type AlertCondition struct {
	metav1.TypeMeta   `json:",inline"`
	ObjectMetaOpenSLO `json:"metadata,omitempty"`

	Spec   AlertConditionSpec   `json:"spec,omitempty"`
	Status AlertConditionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AlertConditionList contains a list of AlertCondition
type AlertConditionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AlertCondition `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AlertCondition{}, &AlertConditionList{})
}
