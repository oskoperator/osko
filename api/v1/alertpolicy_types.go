package v1

import (
	"github.com/SLO-Kubernetes-Operator/slo-kubernetes-operator/api/v1/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AlertPolicySpec defines the desired state of AlertPolicy
type AlertPolicySpec struct {
	Description        common.Description `json:"description,omitempty"`
	AlertWhenNoData    bool               `json:"alertWhenNoData,omitempty"`
	AlertWhenResolved  bool               `json:"alertWhenResolved,omitempty"`
	AlertWhenBreaching bool               `json:"alertWhenBreaching,omitempty"`
	// +kubebuilder:validation:MaxItems=1
	Conditions          []AlertCondition          `json:"conditions,omitempty"`
	NotificationTargets []AlertNotificationTarget `json:"notificationTargets,omitempty"`
}

// AlertPolicyStatus defines the observed state of AlertPolicy
type AlertPolicyStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// AlertPolicy is the Schema for the alertpolicies API
type AlertPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AlertPolicySpec   `json:"spec,omitempty"`
	Status AlertPolicyStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// AlertPolicyList contains a list of AlertPolicy
type AlertPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AlertPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AlertPolicy{}, &AlertPolicyList{})
}
