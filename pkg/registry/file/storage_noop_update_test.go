package file

import (
	"context"
	"testing"
	"time"

	helpersv1 "github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage"
)

func reconcilerShapedUpdate(in runtime.Object, pass int) runtime.Object {
	cur := in.(*softwarecomposition.ContainerProfile).DeepCopy()
	cur.TypeMeta = metav1.TypeMeta{Kind: "ContainerProfile", APIVersion: "spdx.softwarecomposition.kubescape.io/v1beta1"}
	cur.ManagedFields = []metav1.ManagedFieldsEntry{{
		Manager:    "gardener-resource-manager",
		Operation:  metav1.ManagedFieldsOperationUpdate,
		APIVersion: "spdx.softwarecomposition.kubescape.io/v1beta1",
		Time:       &metav1.Time{Time: time.Now().Add(time.Duration(pass) * time.Second)},
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(`{"f:spec":{}}`)},
	}}
	cur.Spec.Ingress = []softwarecomposition.NetworkNeighbor{}
	cur.Spec.Syscalls = []string{}
	cur.Spec.IdentifiedCallStacks = []softwarecomposition.IdentifiedCallStack{}
	for i := range cur.Spec.Execs {
		if cur.Spec.Execs[i].Args == nil {
			cur.Spec.Execs[i].Args = []string{}
		}
	}
	return cur
}

func TestGuaranteedUpdate_IdenticalReapplyIsNoop(t *testing.T) {
	const (
		ns   = "redis"
		name = "redis-client"
	)
	s, key := newGuardTestStorage(t, ns, name)
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	applied := &softwarecomposition.ContainerProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Annotations: map[string]string{
				helpersv1.ManagedByMetadataKey: helpersv1.ManagedByUserValue,
			},
		},
		Spec: softwarecomposition.ContainerProfileSpec{
			Execs:     []softwarecomposition.ExecCalls{{Path: "/usr/bin/redis-cli", Args: []string{}}, {Path: "/usr/bin/id"}},
			Endpoints: []softwarecomposition.HTTPEndpoint{},
			Ingress:   []softwarecomposition.NetworkNeighbor{},
		},
	}
	require.NoError(t, s.Create(ctx, key, applied, &softwarecomposition.ContainerProfile{}, 0))

	stored := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, stored))
	rvAfterCreate := stored.ResourceVersion
	assert.Empty(t, stored.ManagedFields, "storage does not persist managedFields")

	for pass := 1; pass <= 3; pass++ {
		out := &softwarecomposition.ContainerProfile{}
		require.NoError(t, s.GuaranteedUpdate(ctx, key, out, false, nil,
			func(input runtime.Object, _ storage.ResponseMeta) (runtime.Object, *uint64, error) {
				return reconcilerShapedUpdate(input, pass), nil, nil
			}, nil))
		again := &softwarecomposition.ContainerProfile{}
		require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, again))
		assert.Equal(t, rvAfterCreate, again.ResourceVersion, "pass %d: an identical re-apply must not bump the resourceVersion", pass)
		assert.Equal(t, stored.Spec, again.Spec)
		assert.Equal(t, stored.Annotations, again.Annotations)
	}

	out := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.GuaranteedUpdate(ctx, key, out, false, nil,
		func(input runtime.Object, _ storage.ResponseMeta) (runtime.Object, *uint64, error) {
			cur := reconcilerShapedUpdate(input, 9).(*softwarecomposition.ContainerProfile)
			cur.Spec.Execs = append(cur.Spec.Execs, softwarecomposition.ExecCalls{Path: "/bin/sh"})
			return cur, nil, nil
		}, nil))
	changed := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, changed))
	assert.NotEqual(t, rvAfterCreate, changed.ResourceVersion, "a real change still writes")
	assert.Len(t, changed.Spec.Execs, 3)
	assert.Empty(t, changed.ManagedFields)
}

func TestGuaranteedUpdate_LearnedProfileIdenticalReapplyIsNoop(t *testing.T) {
	const (
		ns   = "kube-system"
		name = "replicaset-coredns-5d78c9869d-coredns-185f-129c"
	)
	s, key := newGuardTestStorage(t, ns, name)
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	learned := &softwarecomposition.ContainerProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Annotations: map[string]string{
				helpersv1.StatusMetadataKey:     helpersv1.Completed,
				helpersv1.CompletionMetadataKey: helpersv1.Full,
			},
		},
		Spec: softwarecomposition.ContainerProfileSpec{
			Execs: []softwarecomposition.ExecCalls{{Path: "/coredns", Args: []string{"-conf", "/etc/coredns/Corefile"}}},
			Opens: []softwarecomposition.OpenCalls{{Path: "/etc/coredns/Corefile", Flags: []string{"O_RDONLY"}}},
		},
	}
	require.NoError(t, s.Create(ctx, key, learned, &softwarecomposition.ContainerProfile{}, 0))
	stored := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, stored))

	out := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.GuaranteedUpdate(ctx, key, out, false, nil,
		func(input runtime.Object, _ storage.ResponseMeta) (runtime.Object, *uint64, error) {
			return reconcilerShapedUpdate(input, 1), nil, nil
		}, nil))
	again := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, again))
	assert.Equal(t, stored.ResourceVersion, again.ResourceVersion, "an identical re-apply of a learned profile must not bump the resourceVersion")
	assert.Equal(t, stored.Spec, again.Spec)
}
