package v1

import (
	openslov1 "github.com/OpenSLO/OpenSLO/pkg/openslo/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AlertConditionStatus defines the observed state of AlertCondition
type AlertConditionStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:storageversion
//+kubebuilder:subresource:status

// AlertCondition is the Schema for the alertconditions API
type AlertCondition struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   openslov1.AlertConditionSpec `json:"spec,omitempty"`
	Status AlertConditionStatus         `json:"status,omitempty"`
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
