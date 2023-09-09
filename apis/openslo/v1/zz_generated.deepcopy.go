//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v1

import (
	"github.com/oskoperator/osko/apis/osko/v1alpha1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertCondition) DeepCopyInto(out *AlertCondition) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMetaOpenSLO.DeepCopyInto(&out.ObjectMetaOpenSLO)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertCondition.
func (in *AlertCondition) DeepCopy() *AlertCondition {
	if in == nil {
		return nil
	}
	out := new(AlertCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AlertCondition) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertConditionList) DeepCopyInto(out *AlertConditionList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AlertCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertConditionList.
func (in *AlertConditionList) DeepCopy() *AlertConditionList {
	if in == nil {
		return nil
	}
	out := new(AlertConditionList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AlertConditionList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertConditionSpec) DeepCopyInto(out *AlertConditionSpec) {
	*out = *in
	out.Condition = in.Condition
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertConditionSpec.
func (in *AlertConditionSpec) DeepCopy() *AlertConditionSpec {
	if in == nil {
		return nil
	}
	out := new(AlertConditionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertConditionStatus) DeepCopyInto(out *AlertConditionStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertConditionStatus.
func (in *AlertConditionStatus) DeepCopy() *AlertConditionStatus {
	if in == nil {
		return nil
	}
	out := new(AlertConditionStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertNotificationTarget) DeepCopyInto(out *AlertNotificationTarget) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMetaOpenSLO.DeepCopyInto(&out.ObjectMetaOpenSLO)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertNotificationTarget.
func (in *AlertNotificationTarget) DeepCopy() *AlertNotificationTarget {
	if in == nil {
		return nil
	}
	out := new(AlertNotificationTarget)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AlertNotificationTarget) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertNotificationTargetList) DeepCopyInto(out *AlertNotificationTargetList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AlertNotificationTarget, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertNotificationTargetList.
func (in *AlertNotificationTargetList) DeepCopy() *AlertNotificationTargetList {
	if in == nil {
		return nil
	}
	out := new(AlertNotificationTargetList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AlertNotificationTargetList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertNotificationTargetSpec) DeepCopyInto(out *AlertNotificationTargetSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertNotificationTargetSpec.
func (in *AlertNotificationTargetSpec) DeepCopy() *AlertNotificationTargetSpec {
	if in == nil {
		return nil
	}
	out := new(AlertNotificationTargetSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertNotificationTargetStatus) DeepCopyInto(out *AlertNotificationTargetStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertNotificationTargetStatus.
func (in *AlertNotificationTargetStatus) DeepCopy() *AlertNotificationTargetStatus {
	if in == nil {
		return nil
	}
	out := new(AlertNotificationTargetStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertPolicy) DeepCopyInto(out *AlertPolicy) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMetaOpenSLO.DeepCopyInto(&out.ObjectMetaOpenSLO)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertPolicy.
func (in *AlertPolicy) DeepCopy() *AlertPolicy {
	if in == nil {
		return nil
	}
	out := new(AlertPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AlertPolicy) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertPolicyCondition) DeepCopyInto(out *AlertPolicyCondition) {
	*out = *in
	in.Metadata.DeepCopyInto(&out.Metadata)
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(AlertConditionSpec)
		**out = **in
	}
	if in.ConditionRef != nil {
		in, out := &in.ConditionRef, &out.ConditionRef
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertPolicyCondition.
func (in *AlertPolicyCondition) DeepCopy() *AlertPolicyCondition {
	if in == nil {
		return nil
	}
	out := new(AlertPolicyCondition)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertPolicyList) DeepCopyInto(out *AlertPolicyList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]AlertPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertPolicyList.
func (in *AlertPolicyList) DeepCopy() *AlertPolicyList {
	if in == nil {
		return nil
	}
	out := new(AlertPolicyList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *AlertPolicyList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertPolicyNotificationTarget) DeepCopyInto(out *AlertPolicyNotificationTarget) {
	*out = *in
	in.Metadata.DeepCopyInto(&out.Metadata)
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(AlertNotificationTargetSpec)
		**out = **in
	}
	if in.TargetRef != nil {
		in, out := &in.TargetRef, &out.TargetRef
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertPolicyNotificationTarget.
func (in *AlertPolicyNotificationTarget) DeepCopy() *AlertPolicyNotificationTarget {
	if in == nil {
		return nil
	}
	out := new(AlertPolicyNotificationTarget)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertPolicySpec) DeepCopyInto(out *AlertPolicySpec) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]AlertPolicyCondition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.NotificationTargets != nil {
		in, out := &in.NotificationTargets, &out.NotificationTargets
		*out = make([]AlertPolicyNotificationTarget, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertPolicySpec.
func (in *AlertPolicySpec) DeepCopy() *AlertPolicySpec {
	if in == nil {
		return nil
	}
	out := new(AlertPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *AlertPolicyStatus) DeepCopyInto(out *AlertPolicyStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new AlertPolicyStatus.
func (in *AlertPolicyStatus) DeepCopy() *AlertPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(AlertPolicyStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *CalendarSpec) DeepCopyInto(out *CalendarSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new CalendarSpec.
func (in *CalendarSpec) DeepCopy() *CalendarSpec {
	if in == nil {
		return nil
	}
	out := new(CalendarSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConditionSpec) DeepCopyInto(out *ConditionSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConditionSpec.
func (in *ConditionSpec) DeepCopy() *ConditionSpec {
	if in == nil {
		return nil
	}
	out := new(ConditionSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConnectionDetails) DeepCopyInto(out *ConnectionDetails) {
	*out = *in
	if in.Mimir != nil {
		in, out := &in.Mimir, &out.Mimir
		*out = new(v1alpha1.Mimir)
		(*in).DeepCopyInto(*out)
	}
	if in.Cortex != nil {
		in, out := &in.Cortex, &out.Cortex
		*out = new(v1alpha1.Cortex)
		(*in).DeepCopyInto(*out)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ConnectionDetails.
func (in *ConnectionDetails) DeepCopy() *ConnectionDetails {
	if in == nil {
		return nil
	}
	out := new(ConnectionDetails)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Datasource) DeepCopyInto(out *Datasource) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMetaOpenSLO.DeepCopyInto(&out.ObjectMetaOpenSLO)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Datasource.
func (in *Datasource) DeepCopy() *Datasource {
	if in == nil {
		return nil
	}
	out := new(Datasource)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Datasource) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DatasourceList) DeepCopyInto(out *DatasourceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Datasource, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DatasourceList.
func (in *DatasourceList) DeepCopy() *DatasourceList {
	if in == nil {
		return nil
	}
	out := new(DatasourceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *DatasourceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DatasourceSpec) DeepCopyInto(out *DatasourceSpec) {
	*out = *in
	in.ConnectionDetails.DeepCopyInto(&out.ConnectionDetails)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DatasourceSpec.
func (in *DatasourceSpec) DeepCopy() *DatasourceSpec {
	if in == nil {
		return nil
	}
	out := new(DatasourceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *DatasourceStatus) DeepCopyInto(out *DatasourceStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new DatasourceStatus.
func (in *DatasourceStatus) DeepCopy() *DatasourceStatus {
	if in == nil {
		return nil
	}
	out := new(DatasourceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Indicator) DeepCopyInto(out *Indicator) {
	*out = *in
	in.Metadata.DeepCopyInto(&out.Metadata)
	out.Spec = in.Spec
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Indicator.
func (in *Indicator) DeepCopy() *Indicator {
	if in == nil {
		return nil
	}
	out := new(Indicator)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricSourceSpec) DeepCopyInto(out *MetricSourceSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricSourceSpec.
func (in *MetricSourceSpec) DeepCopy() *MetricSourceSpec {
	if in == nil {
		return nil
	}
	out := new(MetricSourceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MetricSpec) DeepCopyInto(out *MetricSpec) {
	*out = *in
	out.MetricSource = in.MetricSource
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MetricSpec.
func (in *MetricSpec) DeepCopy() *MetricSpec {
	if in == nil {
		return nil
	}
	out := new(MetricSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ObjectMetaOpenSLO) DeepCopyInto(out *ObjectMetaOpenSLO) {
	*out = *in
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ObjectMetaOpenSLO.
func (in *ObjectMetaOpenSLO) DeepCopy() *ObjectMetaOpenSLO {
	if in == nil {
		return nil
	}
	out := new(ObjectMetaOpenSLO)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ObjectivesSpec) DeepCopyInto(out *ObjectivesSpec) {
	*out = *in
	if in.Indicator != nil {
		in, out := &in.Indicator, &out.Indicator
		*out = new(Indicator)
		(*in).DeepCopyInto(*out)
	}
	if in.IndicatorRef != nil {
		in, out := &in.IndicatorRef, &out.IndicatorRef
		*out = new(string)
		**out = **in
	}
	out.CompositeWeight = in.CompositeWeight.DeepCopy()
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ObjectivesSpec.
func (in *ObjectivesSpec) DeepCopy() *ObjectivesSpec {
	if in == nil {
		return nil
	}
	out := new(ObjectivesSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RatioMetricSpec) DeepCopyInto(out *RatioMetricSpec) {
	*out = *in
	out.Raw = in.Raw
	out.Good = in.Good
	out.Bad = in.Bad
	out.Total = in.Total
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RatioMetricSpec.
func (in *RatioMetricSpec) DeepCopy() *RatioMetricSpec {
	if in == nil {
		return nil
	}
	out := new(RatioMetricSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SLI) DeepCopyInto(out *SLI) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMetaOpenSLO.DeepCopyInto(&out.ObjectMetaOpenSLO)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SLI.
func (in *SLI) DeepCopy() *SLI {
	if in == nil {
		return nil
	}
	out := new(SLI)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SLI) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SLIList) DeepCopyInto(out *SLIList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SLI, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SLIList.
func (in *SLIList) DeepCopy() *SLIList {
	if in == nil {
		return nil
	}
	out := new(SLIList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SLIList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SLISpec) DeepCopyInto(out *SLISpec) {
	*out = *in
	out.ThresholdMetric = in.ThresholdMetric
	out.RatioMetric = in.RatioMetric
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SLISpec.
func (in *SLISpec) DeepCopy() *SLISpec {
	if in == nil {
		return nil
	}
	out := new(SLISpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SLIStatus) DeepCopyInto(out *SLIStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SLIStatus.
func (in *SLIStatus) DeepCopy() *SLIStatus {
	if in == nil {
		return nil
	}
	out := new(SLIStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SLO) DeepCopyInto(out *SLO) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMetaOpenSLO.DeepCopyInto(&out.ObjectMetaOpenSLO)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SLO.
func (in *SLO) DeepCopy() *SLO {
	if in == nil {
		return nil
	}
	out := new(SLO)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SLO) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SLOAlertPolicy) DeepCopyInto(out *SLOAlertPolicy) {
	*out = *in
	in.Metadata.DeepCopyInto(&out.Metadata)
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(AlertPolicySpec)
		(*in).DeepCopyInto(*out)
	}
	if in.AlertPolicyRef != nil {
		in, out := &in.AlertPolicyRef, &out.AlertPolicyRef
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SLOAlertPolicy.
func (in *SLOAlertPolicy) DeepCopy() *SLOAlertPolicy {
	if in == nil {
		return nil
	}
	out := new(SLOAlertPolicy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SLOList) DeepCopyInto(out *SLOList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]SLO, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SLOList.
func (in *SLOList) DeepCopy() *SLOList {
	if in == nil {
		return nil
	}
	out := new(SLOList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *SLOList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SLOSpec) DeepCopyInto(out *SLOSpec) {
	*out = *in
	if in.Indicator != nil {
		in, out := &in.Indicator, &out.Indicator
		*out = new(SLISpec)
		**out = **in
	}
	if in.IndicatorRef != nil {
		in, out := &in.IndicatorRef, &out.IndicatorRef
		*out = new(string)
		**out = **in
	}
	if in.TimeWindow != nil {
		in, out := &in.TimeWindow, &out.TimeWindow
		*out = make([]TimeWindowSpec, len(*in))
		copy(*out, *in)
	}
	if in.Objectives != nil {
		in, out := &in.Objectives, &out.Objectives
		*out = make([]ObjectivesSpec, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.AlertPolicies != nil {
		in, out := &in.AlertPolicies, &out.AlertPolicies
		*out = make([]SLOAlertPolicy, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SLOSpec.
func (in *SLOSpec) DeepCopy() *SLOSpec {
	if in == nil {
		return nil
	}
	out := new(SLOSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *SLOStatus) DeepCopyInto(out *SLOStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new SLOStatus.
func (in *SLOStatus) DeepCopy() *SLOStatus {
	if in == nil {
		return nil
	}
	out := new(SLOStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Service) DeepCopyInto(out *Service) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMetaOpenSLO.DeepCopyInto(&out.ObjectMetaOpenSLO)
	out.Spec = in.Spec
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Service.
func (in *Service) DeepCopy() *Service {
	if in == nil {
		return nil
	}
	out := new(Service)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *Service) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceList) DeepCopyInto(out *ServiceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Service, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceList.
func (in *ServiceList) DeepCopy() *ServiceList {
	if in == nil {
		return nil
	}
	out := new(ServiceList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *ServiceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceSpec) DeepCopyInto(out *ServiceSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceSpec.
func (in *ServiceSpec) DeepCopy() *ServiceSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ServiceStatus) DeepCopyInto(out *ServiceStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ServiceStatus.
func (in *ServiceStatus) DeepCopy() *ServiceStatus {
	if in == nil {
		return nil
	}
	out := new(ServiceStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ThresholdMetricSpec) DeepCopyInto(out *ThresholdMetricSpec) {
	*out = *in
	out.MetricSource = in.MetricSource
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new ThresholdMetricSpec.
func (in *ThresholdMetricSpec) DeepCopy() *ThresholdMetricSpec {
	if in == nil {
		return nil
	}
	out := new(ThresholdMetricSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *TimeWindowSpec) DeepCopyInto(out *TimeWindowSpec) {
	*out = *in
	out.Calendar = in.Calendar
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new TimeWindowSpec.
func (in *TimeWindowSpec) DeepCopy() *TimeWindowSpec {
	if in == nil {
		return nil
	}
	out := new(TimeWindowSpec)
	in.DeepCopyInto(out)
	return out
}
