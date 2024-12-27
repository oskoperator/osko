package v1alpha1

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/common/model"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// MimirRuleSpec defines the desired state of MimirRule
type MimirRuleSpec struct {
	ConnectionDetails ConnectionDetails `json:"mimirConnectionDetails,omitempty"`
	// Groups is an example field of MimirRule. Edit mimirrule_types.go to remove/update
	Groups []RuleGroup `json:"groups"`
}

// MimirRuleStatus defines the observed state of MimirRule
type MimirRuleStatus struct {
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	LastEvaluationTime metav1.Time        `json:"lastEvaluationTime,omitempty"`
	Ready              string             `json:"ready,omitempty"`
}

type RuleGroup struct {
	Name                          string          `json:"name"`
	SourceTenants                 []string        `json:"source_tenants,omitempty"`
	Rules                         []Rule          `json:"rules"`
	Interval                      model.Duration  `json:"interval,omitempty"`
	EvaluationDelay               *model.Duration `json:"evaluation_delay,omitempty"`
	Limit                         int             `json:"limit,omitempty"`
	AlignEvaluationTimeOnInterval bool            `json:"align_evaluation_time_on_interval,omitempty"`
}

type Rule struct {
	Record        string                 `json:"record,omitempty"`
	Alert         string                 `json:"alert,omitempty"`
	Expr          string                 `json:"expr"`
	For           *monitoringv1.Duration `json:"for,omitempty"`
	KeepFiringFor model.Duration         `json:"keep_firing_for,omitempty"`
	Labels        map[string]string      `json:"labels,omitempty"`
	Annotations   map[string]string      `json:"annotations,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Ready",type=string,JSONPath=.status.ready,description="The reason for the current status of the MimirRule resource"
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

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
