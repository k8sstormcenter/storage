package file

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/armosec/armoapi-go/armotypes"
	helpersv1 "github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage"
)

func newCollapsibleProfile(annotations map[string]string) *softwarecomposition.ContainerProfile {
	ingress := make([]softwarecomposition.NetworkNeighbor, 0, 64)
	for i := 0; i < 64; i++ {
		ingress = append(ingress, softwarecomposition.NetworkNeighbor{
			Identifier: fmt.Sprintf("external-%d", i),
			Type:       "external",
			IPAddress:  fmt.Sprintf("10.0.0.%d", i),
			Ports:      []softwarecomposition.NetworkPort{{Name: "80"}},
		})
	}
	return &softwarecomposition.ContainerProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "redis-client",
			Namespace:   "redis",
			Annotations: annotations,
		},
		Spec: softwarecomposition.ContainerProfileSpec{
			Execs:   []softwarecomposition.ExecCalls{{Path: "/usr/bin/redis-cli", Args: []string{"redis-cli", "ping"}}},
			Ingress: ingress,
		},
	}
}

func newPreSaveProcessor() ContainerProfileProcessor {
	return ContainerProfileProcessor{
		HostType:                armotypes.HostTypeKubernetes,
		MaxContainerProfileSize: 40000,
		ContainerProfileStorage: &fakeStorage{profiles: map[string]softwarecomposition.ContainerProfile{}},
		CollapseSettings: func() dynamicpathdetector.CollapseSettings {
			return dynamicpathdetector.CollapseSettings{
				NetworkIPGroupThreshold: 10,
				NetworkCIDRFloorBits:    24,
			}
		},
	}
}

func TestPreSave_UserManagedProfileStoredVerbatim(t *testing.T) {
	annotations := map[string]string{
		helpersv1.ManagedByMetadataKey: helpersv1.ManagedByUserValue,
		"kubescape.io/signed-by":       "operator",
	}
	profile := newCollapsibleProfile(annotations)
	want := profile.DeepCopy()

	processor := newPreSaveProcessor()
	require.NoError(t, processor.PreSave(context.TODO(), profile))

	assert.Equal(t, want.Spec, profile.Spec, "user-managed spec must not be deflated")
	assert.Equal(t, want.Annotations, profile.Annotations, "user-managed annotations must not be touched")
	_, hasSize := profile.Annotations[helpersv1.ResourceSizeMetadataKey]
	assert.False(t, hasSize, "no resource-size annotation on user-managed profile")
}

func TestPreSave_LearnedProfileStillDeflated(t *testing.T) {
	profile := newCollapsibleProfile(map[string]string{
		helpersv1.StatusMetadataKey: helpersv1.Learning,
	})

	processor := newPreSaveProcessor()
	require.NoError(t, processor.PreSave(context.TODO(), profile))

	require.Len(t, profile.Spec.Ingress, 1, "learned ingress collapses to one CIDR entry")
	assert.Equal(t, []string{"10.0.0.0/26"}, profile.Spec.Ingress[0].IPAddresses)
	assert.Equal(t, "2", profile.Annotations[helpersv1.ResourceSizeMetadataKey])
}

func TestPreSave_ManagedByOtherValueStillDeflated(t *testing.T) {
	profile := newCollapsibleProfile(map[string]string{
		helpersv1.ManagedByMetadataKey: "Kubescape",
	})

	processor := newPreSaveProcessor()
	require.NoError(t, processor.PreSave(context.TODO(), profile))

	require.Len(t, profile.Spec.Ingress, 1)
	assert.Contains(t, profile.Annotations, helpersv1.ResourceSizeMetadataKey)
}

func TestPreSave_UserManagedProfile_RealStorageRoundTrip(t *testing.T) {
	const (
		ns   = "redis"
		name = "redis-client"
	)
	s, key := newGuardTestStorage(t, ns, name)
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	applied := newCollapsibleProfile(map[string]string{
		helpersv1.ManagedByMetadataKey: helpersv1.ManagedByUserValue,
	})
	applied.Name = name
	applied.Namespace = ns
	want := applied.DeepCopy()

	require.NoError(t, s.Create(ctx, key, applied, &softwarecomposition.ContainerProfile{}, 0))

	got := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, got))
	assert.Equal(t, want.Spec, got.Spec, "created user-managed spec is stored as applied")
	assert.NotContains(t, got.Annotations, helpersv1.ResourceSizeMetadataKey)

	out := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.GuaranteedUpdate(ctx, key, out, false, nil,
		func(input runtime.Object, _ storage.ResponseMeta) (runtime.Object, *uint64, error) {
			cur := input.(*softwarecomposition.ContainerProfile).DeepCopy()
			cur.Spec = want.Spec
			cur.Spec.Execs = append(cur.Spec.Execs, softwarecomposition.ExecCalls{Path: "/usr/bin/id", Args: []string{"id"}})
			return cur, nil, nil
		}, nil))

	again := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, again))
	assert.Len(t, again.Spec.Ingress, 64, "updated user-managed ingress is not collapsed")
	assert.Len(t, again.Spec.Execs, 2)
	assert.NotContains(t, again.Annotations, helpersv1.ResourceSizeMetadataKey)
}

func TestPreSave_LearnedProfile_RealStorageStillDeflates(t *testing.T) {
	const (
		ns   = "redis"
		name = "replicaset-redis-client-6cb7c977c8-client-f2b6-07d5"
	)
	s, key := newGuardTestStorage(t, ns, name)
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	applied := newCollapsibleProfile(map[string]string{
		helpersv1.StatusMetadataKey: helpersv1.Learning,
	})
	applied.Name = name
	applied.Namespace = ns
	require.NoError(t, s.Create(ctx, key, applied, &softwarecomposition.ContainerProfile{}, 0))

	got := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, got))
	require.Len(t, got.Spec.Ingress, 1)
	assert.Equal(t, []string{"10.0.0.0/26"}, got.Spec.Ingress[0].IPAddresses)
	assert.Equal(t, "2", got.Annotations[helpersv1.ResourceSizeMetadataKey])
}
