package v1

import (
	common "github.com/SLO-Kubernetes-Operator/slo-kubernetes-operator/api/v1/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type CalendarSpec struct {
	// Date with time in 24h format, format without time zone
	// +kubebuilder:example="2020-01-21 12:30:00"
	StartTime string `json:"startTime,omitempty"`

	// Name as in IANA Time Zone Database
	// +kubebuilder:example="America/New_York"
	TimeZone string `json:"timezone,omitempty"`
}

type TimeWindowSpec struct {
	Duration  common.Duration `json:"duration,omitempty"`
	IsRolling bool            `json:"isRolling,omitempty"`
	Calendar  CalendarSpec    `json:"calendar,omitempty"`
}

// SLOSpec defines the desired state of SLO
type SLOSpec struct {
	Description  common.Description `json:"description,omitempty"`
	Indicator    SLISpec            `json:"indicator,omitempty"`
	IndicatorRef string             `json:"indicatorRef,omitempty"`
	// +kubebuilder:validation:MaxItems=1
	TimeWindow []TimeWindowSpec `json:"timeWindow,omitempty"`
	// +kubebuilder:validation:Enum=Occurrences;Timeslices;RatioTimeslices
	BudgetingMethod string `json:"budgetingMethod,omitempty"`
}

// SLOStatus defines the observed state of SLO
type SLOStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// SLO is the Schema for the slos API
type SLO struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SLOSpec   `json:"spec,omitempty"`
	Status SLOStatus `json:"status,omitempty"`
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
