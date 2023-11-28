//go:build !ignore_autogenerated
// +build !ignore_autogenerated

// Code generated by controller-gen. DO NOT EDIT.

package v1alpha1

import (
	"github.com/prometheus/common/model"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *ConnectionDetails) DeepCopyInto(out *ConnectionDetails) {
	*out = *in
	if in.SourceTenants != nil {
		in, out := &in.SourceTenants, &out.SourceTenants
		*out = make([]string, len(*in))
		copy(*out, *in)
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
func (in *Cortex) DeepCopyInto(out *Cortex) {
	*out = *in
	out.Ruler = in.Ruler
	in.Multitenancy.DeepCopyInto(&out.Multitenancy)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Cortex.
func (in *Cortex) DeepCopy() *Cortex {
	if in == nil {
		return nil
	}
	out := new(Cortex)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Mimir) DeepCopyInto(out *Mimir) {
	*out = *in
	out.Ruler = in.Ruler
	in.Multitenancy.DeepCopyInto(&out.Multitenancy)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Mimir.
func (in *Mimir) DeepCopy() *Mimir {
	if in == nil {
		return nil
	}
	out := new(Mimir)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MimirRule) DeepCopyInto(out *MimirRule) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MimirRule.
func (in *MimirRule) DeepCopy() *MimirRule {
	if in == nil {
		return nil
	}
	out := new(MimirRule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MimirRule) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MimirRuleList) DeepCopyInto(out *MimirRuleList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]MimirRule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MimirRuleList.
func (in *MimirRuleList) DeepCopy() *MimirRuleList {
	if in == nil {
		return nil
	}
	out := new(MimirRuleList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *MimirRuleList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MimirRuleSpec) DeepCopyInto(out *MimirRuleSpec) {
	*out = *in
	if in.Groups != nil {
		in, out := &in.Groups, &out.Groups
		*out = make([]RuleGroup, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MimirRuleSpec.
func (in *MimirRuleSpec) DeepCopy() *MimirRuleSpec {
	if in == nil {
		return nil
	}
	out := new(MimirRuleSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *MimirRuleStatus) DeepCopyInto(out *MimirRuleStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]v1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	in.LastEvaluationTime.DeepCopyInto(&out.LastEvaluationTime)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new MimirRuleStatus.
func (in *MimirRuleStatus) DeepCopy() *MimirRuleStatus {
	if in == nil {
		return nil
	}
	out := new(MimirRuleStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Multitenancy) DeepCopyInto(out *Multitenancy) {
	*out = *in
	if in.SourceTenants != nil {
		in, out := &in.SourceTenants, &out.SourceTenants
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Multitenancy.
func (in *Multitenancy) DeepCopy() *Multitenancy {
	if in == nil {
		return nil
	}
	out := new(Multitenancy)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Rule) DeepCopyInto(out *Rule) {
	*out = *in
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Rule.
func (in *Rule) DeepCopy() *Rule {
	if in == nil {
		return nil
	}
	out := new(Rule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *RuleGroup) DeepCopyInto(out *RuleGroup) {
	*out = *in
	if in.SourceTenants != nil {
		in, out := &in.SourceTenants, &out.SourceTenants
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Rules != nil {
		in, out := &in.Rules, &out.Rules
		*out = make([]Rule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.EvaluationDelay != nil {
		in, out := &in.EvaluationDelay, &out.EvaluationDelay
		*out = new(model.Duration)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new RuleGroup.
func (in *RuleGroup) DeepCopy() *RuleGroup {
	if in == nil {
		return nil
	}
	out := new(RuleGroup)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *Ruler) DeepCopyInto(out *Ruler) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new Ruler.
func (in *Ruler) DeepCopy() *Ruler {
	if in == nil {
		return nil
	}
	out := new(Ruler)
	in.DeepCopyInto(out)
	return out
}
