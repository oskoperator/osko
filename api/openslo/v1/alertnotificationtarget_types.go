package v1

import (
	openslov1 "github.com/OpenSLO/OpenSLO/pkg/openslo/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AlertNotificationTargetStatus defines the observed state of AlertNotificationTarget
type AlertNotificationTargetStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AlertNotificationTarget is the Schema for the alertnotificationtargets API
type AlertNotificationTarget struct {
	metav1.TypeMeta   `json:",inline"`
	ObjectMetaOpenSLO `json:"metadata,omitempty"`

	Spec   openslov1.AlertNotificationTargetSpec `json:"spec,omitempty"`
	Status AlertNotificationTargetStatus         `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AlertNotificationTargetList contains a list of AlertNotificationTarget
type AlertNotificationTargetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AlertNotificationTarget `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AlertNotificationTarget{}, &AlertNotificationTargetList{})
}
