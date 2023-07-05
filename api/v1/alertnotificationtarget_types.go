package v1

import (
	common "github.com/SLO-Kubernetes-Operator/slo-kubernetes-operator/api/v1/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AlertNotificationTargetSpec defines the desired state of AlertNotificationTarget
type AlertNotificationTargetSpec struct {
	Description common.Description `json:"description,omitempty"`
	Target      string             `json:"target,omitempty"`
}

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
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AlertNotificationTargetSpec   `json:"spec,omitempty"`
	Status AlertNotificationTargetStatus `json:"status,omitempty"`
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
