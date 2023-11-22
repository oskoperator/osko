package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MimirRuleSpec defines the desired state of MimirRule
type MimirRuleSpec struct {
	// Groups is an example field of MimirRule. Edit mimirrule_types.go to remove/update
	Groups []RuleGroup `json:"groups"`
}

// MimirRuleStatus defines the observed state of MimirRule
type MimirRuleStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type RuleGroup struct {
	Name          string   `json:"name"`
	SourceTenants []string `json:"source_tenants,omitempty"`
	Rules         []Rule   `json:"rules"`
}

type Rule struct {
	Record      string            `json:"record,omitempty"`
	Alert       string            `json:"alert,omitempty"`
	Expr        string            `json:"expr"`
	For         string            `json:"for,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// MimirRule is the Schema for the mimirrules API
type MimirRule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MimirRuleSpec   `json:"spec,omitempty"`
	Status MimirRuleStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MimirRuleList contains a list of MimirRule
type MimirRuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MimirRule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MimirRule{}, &MimirRuleList{})
}
