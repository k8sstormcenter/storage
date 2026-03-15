package collapseconfiguration

import (
	"context"
	"fmt"

	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
)

func NewStrategy(typer runtime.ObjectTyper) CollapseConfigurationStrategy {
	return CollapseConfigurationStrategy{typer, names.SimpleNameGenerator}
}

func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	cc, ok := obj.(*softwarecomposition.CollapseConfiguration)
	if !ok {
		return nil, nil, fmt.Errorf("given object is not a CollapseConfiguration")
	}
	return cc.ObjectMeta.Labels, SelectableFields(cc), nil
}

func MatchCollapseConfiguration(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

func SelectableFields(obj *softwarecomposition.CollapseConfiguration) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, false)
}

type CollapseConfigurationStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

func (CollapseConfigurationStrategy) NamespaceScoped() bool {
	return false
}

func (CollapseConfigurationStrategy) PrepareForCreate(_ context.Context, _ runtime.Object) {
}

func (CollapseConfigurationStrategy) PrepareForUpdate(_ context.Context, _, _ runtime.Object) {
}

func (CollapseConfigurationStrategy) Validate(_ context.Context, obj runtime.Object) field.ErrorList {
	return validateCollapseConfiguration(obj.(*softwarecomposition.CollapseConfiguration))
}

func (CollapseConfigurationStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

func (CollapseConfigurationStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (CollapseConfigurationStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (CollapseConfigurationStrategy) Canonicalize(_ runtime.Object) {
}

func (CollapseConfigurationStrategy) ValidateUpdate(_ context.Context, obj, _ runtime.Object) field.ErrorList {
	return validateCollapseConfiguration(obj.(*softwarecomposition.CollapseConfiguration))
}

func (CollapseConfigurationStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}

func validateCollapseConfiguration(cc *softwarecomposition.CollapseConfiguration) field.ErrorList {
	allErrs := field.ErrorList{}
	specPath := field.NewPath("spec")

	if cc.Spec.OpenDynamicThreshold < 1 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("openDynamicThreshold"), cc.Spec.OpenDynamicThreshold, "must be >= 1"))
	}
	if cc.Spec.EndpointDynamicThreshold < 1 {
		allErrs = append(allErrs, field.Invalid(specPath.Child("endpointDynamicThreshold"), cc.Spec.EndpointDynamicThreshold, "must be >= 1"))
	}
	for i, entry := range cc.Spec.CollapseConfigs {
		entryPath := specPath.Child("collapseConfigs").Index(i)
		if entry.Prefix == "" {
			allErrs = append(allErrs, field.Required(entryPath.Child("prefix"), "prefix must not be empty"))
		}
		if entry.Threshold < 1 {
			allErrs = append(allErrs, field.Invalid(entryPath.Child("threshold"), entry.Threshold, "must be >= 1"))
		}
	}
	return allErrs
}
