package collapseconfiguration

import (
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/registry"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
)

func NewREST(scheme *runtime.Scheme, storageImpl storage.Interface, optsGetter generic.RESTOptionsGetter) (*registry.REST, error) {
	strategy := NewStrategy(scheme)

	dryRunnableStorage := genericregistry.DryRunnableStorage{Codec: nil, Storage: storageImpl}

	store := &genericregistry.Store{
		NewFunc:                   func() runtime.Object { return &softwarecomposition.CollapseConfiguration{} },
		NewListFunc:               func() runtime.Object { return &softwarecomposition.CollapseConfigurationList{} },
		PredicateFunc:             MatchCollapseConfiguration,
		DefaultQualifiedResource:  softwarecomposition.Resource("collapseconfigurations"),
		SingularQualifiedResource: softwarecomposition.Resource("collapseconfiguration"),

		Storage: dryRunnableStorage,

		CreateStrategy: strategy,
		UpdateStrategy: strategy,
		DeleteStrategy: strategy,

		TableConvertor: rest.NewDefaultTableConvertor(softwarecomposition.Resource("collapseconfigurations")),
	}
	options := &generic.StoreOptions{RESTOptions: optsGetter, AttrFunc: GetAttrs}
	if err := store.CompleteWithOptions(options); err != nil {
		return nil, err
	}

	return &registry.REST{Store: store}, nil
}
