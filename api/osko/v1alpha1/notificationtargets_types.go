package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
)

// +kubebuilder:object:generate=true
type NotificationTargets struct {
	OpsGenie OpsGenie `json:"opsgenie,omitempty"`
}

type OpsGenie struct {
	APIURL   string                `json:"apiUrl,omitempty"`
	APIKey   *v1.SecretKeySelector `json:"apiKey,omitempty"`
	Priority string                `json:"priority,omitempty"`
}
