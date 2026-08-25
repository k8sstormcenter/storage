package rogueartifact

import (
	"context"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"

	"github.com/kubescape/storage/pkg/registry"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/storage"
)

// NewREST returns a RESTStorage object for RogueArtifact.
func NewREST(scheme *runtime.Scheme, storageImpl storage.Interface, optsGetter generic.RESTOptionsGetter) (*registry.REST, error) {
	strategy := NewStrategy(scheme)

	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &softwarecomposition.RogueArtifact{} },
		NewListFunc:               func() runtime.Object { return &softwarecomposition.RogueArtifactList{} },
		PredicateFunc:             MatchRogueArtifact,
		DefaultQualifiedResource:  softwarecomposition.Resource("rogueartifact"),
		SingularQualifiedResource: softwarecomposition.Resource("rogueartifacts"),

		Storage: genericregistry.DryRunnableStorage{Codec: nil, Storage: storageImpl},

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,

		TableConvertor: rogueArtifactTableConvertor{},
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}
	return &registry.REST{Store: store}, nil
}

// rogueArtifactTableConvertor renders `kubectl get rogueartifacts` columns.
type rogueArtifactTableConvertor struct{}

func (rogueArtifactTableConvertor) ConvertToTable(_ context.Context, obj runtime.Object, _ runtime.Object) (*metav1.Table, error) {
	table := &metav1.Table{ColumnDefinitions: []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string"},
		{Name: "State", Type: "string"},
		{Name: "Learning", Type: "boolean"},
		{Name: "Workload", Type: "string"},
		{Name: "Kind", Type: "string"},
		{Name: "Phase", Type: "string"},
		{Name: "Since", Type: "date"},
	}}
	addRow := func(ra *softwarecomposition.RogueArtifact) {
		table.Rows = append(table.Rows, metav1.TableRow{
			Cells:  []interface{}{ra.Name, ra.Spec.State, ra.Spec.Learning, ra.Spec.WorkloadName, ra.Spec.WorkloadKind, ra.Status.Phase, ra.Status.FiredAt},
			Object: runtime.RawExtension{Object: ra},
		})
	}
	switch t := obj.(type) {
	case *softwarecomposition.RogueArtifact:
		addRow(t)
	case *softwarecomposition.RogueArtifactList:
		for i := range t.Items {
			addRow(&t.Items[i])
		}
	}
	return table, nil
}
