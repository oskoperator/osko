package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AlertManagerConfigSpec defines the desired state of AlertManagerConfig
type AlertManagerConfigSpec struct {
	SecretRef v1.SecretReference `json:"secretRef,omitempty"`
}

// AlertManagerConfigStatus defines the observed state of AlertManagerConfig
type AlertManagerConfigStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	LastEvaluationTime metav1.Time        `json:"lastEvaluationTime,omitempty"`
	Ready              string             `json:"ready,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=.status.ready,description="The reason for the current status of the AlertmanagerConfig resource"
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=.metadata.creationTimestamp,description="The time when the AlertmanagerConfig resource was created"

// AlertManagerConfig is the Schema for the alertmanagerconfigs API
type AlertManagerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AlertManagerConfigSpec   `json:"spec,omitempty"`
	Status AlertManagerConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AlertManagerConfigList contains a list of AlertManagerConfig
type AlertManagerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AlertManagerConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AlertManagerConfig{}, &AlertManagerConfigList{})
}
