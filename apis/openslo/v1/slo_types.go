package v1

import (
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type SLOAlertPolicy struct {
	// +kubebuilder:validation:Enum=AlertPolicy
	Kind string `json:"kind,omitempty"`
	// +kubebuilder:crd:generateEmbeddedObjectMeta=true
	Metadata       metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec           *AlertPolicySpec  `json:"spec,omitempty"`
	AlertPolicyRef *string           `json:"alertPolicyRef,omitempty"`
}

type Indicator struct {
	Metadata metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec     SLISpec           `json:"spec,omitempty"`
}

type ObjectivesSpec struct {
	// +optional
	DisplayName string `json:"displayName,omitempty"`
	// +kubebuilder:validation:Enum=lte;gte;lt;gt
	Op              string            `json:"op,omitempty"`
	Value           string            `json:"value,omitempty"`
	Target          string            `json:"target,omitempty"`
	TargetPercent   string            `json:"targetPercent,omitempty"`
	TimeSliceTarget string            `json:"timeSliceTarget,omitempty"`
	TimeSliceWindow Duration          `json:"timeSliceWindow,omitempty"`
	Indicator       *Indicator        `json:"indicator,omitempty"`
	IndicatorRef    *string           `json:"indicatorRef,omitempty"`
	CompositeWeight resource.Quantity `json:"compositeWeight,omitempty"`
}

type CalendarSpec struct {
	// Date with time in 24h format, format without time zone
	// +kubebuilder:example="2020-01-21 12:30:00"
	StartTime string `json:"startTime,omitempty"`

	// Name as in IANA Time Zone Database
	// +kubebuilder:example="America/New_York"
	TimeZone string `json:"timeZone,omitempty"`
}

type TimeWindowSpec struct {
	Duration  Duration     `json:"duration,omitempty"`
	IsRolling bool         `json:"isRolling,omitempty"`
	Calendar  CalendarSpec `json:"calendar,omitempty"`
}

// SLOSpec defines the desired state of SLO
type SLOSpec struct {
	Description  Description `json:"description,omitempty"`
	Service      string      `json:"service,omitempty"`
	Indicator    *Indicator  `json:"indicator,omitempty"`
	IndicatorRef *string     `json:"indicatorRef,omitempty"`
	// +kubebuilder:validation:MaxItems=1
	TimeWindow []TimeWindowSpec `json:"timeWindow,omitempty"`
	// +kubebuilder:validation:Enum=Occurrences;Timeslices;RatioTimeslices
	BudgetingMethod string           `json:"budgetingMethod,omitempty"`
	Objectives      []ObjectivesSpec `json:"objectives,omitempty"`
	AlertPolicies   []SLOAlertPolicy `json:"alertPolicies,omitempty"`
}

// SLOStatus defines the observed state of SLO
type SLOStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	// TODO: Maybe we should use something like []corev1.ObejctReference here?
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	CurrentSLO         string             `json:"currentSLO,omitempty"`
	LastEvaluationTime metav1.Time        `json:"lastEvaluationTime,omitempty"`
	Ready              string             `json:"ready,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=.status.ready,description="The reason for the current status of the SLO resource"
//+kubebuilder:printcolumn:name="Window",type=string,JSONPath=.spec.timeWindow[0].duration,description="The time window for the SLO resource"
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=.metadata.creationTimestamp,description="The time when the SLO resource was created"

// SLO is the Schema for the slos API
type SLO struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              SLOSpec   `json:"spec,omitempty"`
	Status            SLOStatus `json:"status,omitempty"`
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
