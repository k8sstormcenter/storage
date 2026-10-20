package file

import (
	"context"
	"testing"
	"time"

	helpersv1 "github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/generated/clientset/versioned/scheme"
	"github.com/kubescape/storage/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	genericregistry "k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/storage"
)

func TestDryRunUpdate_CopiesWithoutPersisting(t *testing.T) {
	const (
		ns   = "arc-systems"
		name = "nnf-runner-noble-runner"
	)
	s, key := newGuardTestStorage(t, ns, name)
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	applied := &softwarecomposition.ContainerProfile{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns,
			Annotations: map[string]string{helpersv1.ManagedByMetadataKey: helpersv1.ManagedByUserValue}},
		Spec: softwarecomposition.ContainerProfileSpec{Execs: []softwarecomposition.ExecCalls{{Path: "/home/runner/run.sh"}}},
	}
	require.NoError(t, s.Create(ctx, key, applied, &softwarecomposition.ContainerProfile{}, 0))

	dry := genericregistry.DryRunnableStorage{Codec: registry.DryRunCodec(scheme.Scheme), Storage: s}
	out := &softwarecomposition.ContainerProfile{}
	require.NotPanics(t, func() {
		require.NoError(t, dry.GuaranteedUpdate(ctx, key, out, false, nil,
			func(input runtime.Object, _ storage.ResponseMeta) (runtime.Object, *uint64, error) {
				cur := input.(*softwarecomposition.ContainerProfile).DeepCopy()
				cur.Spec.Execs = append(cur.Spec.Execs, softwarecomposition.ExecCalls{Path: "/usr/bin/id"})
				return cur, nil, nil
			}, true, nil))
	}, "a server-side dry-run update must not panic")
	assert.Len(t, out.Spec.Execs, 2, "the dry-run result reflects the update")

	stored := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, stored))
	assert.Len(t, stored.Spec.Execs, 1, "nothing is persisted by a dry-run")
}
