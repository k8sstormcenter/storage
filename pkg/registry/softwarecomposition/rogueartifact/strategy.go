package rogueartifact

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

// NewStrategy creates and returns a RogueArtifactStrategy instance.
func NewStrategy(typer runtime.ObjectTyper) RogueArtifactStrategy {
	return RogueArtifactStrategy{typer, names.SimpleNameGenerator}
}

// GetAttrs returns labels.Set, fields.Set for a RogueArtifact.
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	ra, ok := obj.(*softwarecomposition.RogueArtifact)
	if !ok {
		return nil, nil, fmt.Errorf("given object is not a RogueArtifact")
	}
	return ra.ObjectMeta.Labels, SelectableFields(ra), nil
}

// MatchRogueArtifact is the filter used by the generic etcd backend.
func MatchRogueArtifact(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{Label: label, Field: field, GetAttrs: GetAttrs}
}

func SelectableFields(obj *softwarecomposition.RogueArtifact) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, true)
}

type RogueArtifactStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

func (RogueArtifactStrategy) NamespaceScoped() bool                                   { return true }
func (RogueArtifactStrategy) PrepareForCreate(_ context.Context, _ runtime.Object)    {}
func (RogueArtifactStrategy) PrepareForUpdate(_ context.Context, _, _ runtime.Object) {}
func (RogueArtifactStrategy) Validate(_ context.Context, _ runtime.Object) field.ErrorList {
	return field.ErrorList{}
}
func (RogueArtifactStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}
func (RogueArtifactStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}
func (RogueArtifactStrategy) AllowCreateOnUpdate() bool      { return true }
func (RogueArtifactStrategy) AllowUnconditionalUpdate() bool { return true }
func (RogueArtifactStrategy) Canonicalize(_ runtime.Object)  {}
func (RogueArtifactStrategy) ValidateUpdate(_ context.Context, _, _ runtime.Object) field.ErrorList {
	return field.ErrorList{}
}
