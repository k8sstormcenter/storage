package v1beta1

import runtime "k8s.io/apimachinery/pkg/runtime"

func (in *CollapseConfiguration) DeepCopyInto(out *CollapseConfiguration) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
}

func (in *CollapseConfiguration) DeepCopy() *CollapseConfiguration {
	if in == nil {
		return nil
	}
	out := new(CollapseConfiguration)
	in.DeepCopyInto(out)
	return out
}

func (in *CollapseConfiguration) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}

func (in *CollapseConfigurationSpec) DeepCopyInto(out *CollapseConfigurationSpec) {
	*out = *in
	if in.CollapseConfigs != nil {
		in, out := &in.CollapseConfigs, &out.CollapseConfigs
		*out = make([]CollapseConfigEntry, len(*in))
		copy(*out, *in)
	}
}

func (in *CollapseConfigurationList) DeepCopyInto(out *CollapseConfigurationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]CollapseConfiguration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *CollapseConfigurationList) DeepCopy() *CollapseConfigurationList {
	if in == nil {
		return nil
	}
	out := new(CollapseConfigurationList)
	in.DeepCopyInto(out)
	return out
}

func (in *CollapseConfigurationList) DeepCopyObject() runtime.Object {
	return in.DeepCopy()
}
