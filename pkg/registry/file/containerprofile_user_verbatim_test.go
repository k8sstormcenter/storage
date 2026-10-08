package file

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	helpersv1 "github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage"
)

func kubeletOpens(n int) []softwarecomposition.OpenCalls {
	opens := make([]softwarecomposition.OpenCalls, 0, n)
	for i := 0; i < n; i++ {
		opens = append(opens, softwarecomposition.OpenCalls{
			Path:  fmt.Sprintf("/var/lib/kubelet/pods/pod-%04d/volumes/kubernetes.io~projected/kube-api-access/token", i),
			Flags: []string{"O_RDONLY"},
		})
	}
	return opens
}

func openPaths(opens []softwarecomposition.OpenCalls) []string {
	out := make([]string, 0, len(opens))
	for _, o := range opens {
		out = append(out, o.Path)
	}
	sort.Strings(out)
	return out
}

func userProfile(ns, name string, opens []softwarecomposition.OpenCalls) *softwarecomposition.ContainerProfile {
	p := newGuardProfile(ns, name, helpersv1.Completed)
	p.Annotations[helpersv1.ManagedByMetadataKey] = helpersv1.ManagedByUserValue
	p.Spec.Opens = opens
	p.Spec.Execs = []softwarecomposition.ExecCalls{{Path: "/usr/local/bin/k3s", Args: []string{"k3s", "agent"}}}
	return p
}

func TestPreSave_UserProfileStoredVerbatimOnCreate(t *testing.T) {
	const ns, name = "honey", "node-k3s-2-worker-1-host-3fd7-aed3"
	s, key := newGuardTestStorage(t, ns, name)
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	submitted := userProfile(ns, name, kubeletOpens(120))
	want := openPaths(submitted.Spec.Opens)
	require.NoError(t, s.Create(ctx, key, submitted.DeepCopy(), &softwarecomposition.ContainerProfile{}, 0))

	got := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, got))
	assert.Equal(t, want, openPaths(got.Spec.Opens), "an authored profile is stored byte-exact; no path is folded")
	assert.Equal(t, submitted.Spec.Execs, got.Spec.Execs)
}

func TestPreSave_UserProfileStoredVerbatimOnUpdate(t *testing.T) {
	const ns, name = "honey", "node-k3s-2-worker-2-host-b38e-53c7"
	s, key := newGuardTestStorage(t, ns, name)
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	require.NoError(t, s.Create(ctx, key, userProfile(ns, name, kubeletOpens(10)), &softwarecomposition.ContainerProfile{}, 0))
	extra := kubeletOpens(120)
	want := openPaths(extra)

	out := &softwarecomposition.ContainerProfile{}
	done := make(chan error, 1)
	go func() {
		done <- s.GuaranteedUpdate(ctx, key, out, false, nil,
			func(input runtime.Object, _ storage.ResponseMeta) (runtime.Object, *uint64, error) {
				cur := input.(*softwarecomposition.ContainerProfile).DeepCopy()
				cur.Spec.Opens = extra
				return cur, nil, nil
			}, nil)
	}()
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatal("GuaranteedUpdate did not return")
	}

	got := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, got))
	assert.Equal(t, want, openPaths(got.Spec.Opens), "an update of an authored profile keeps every submitted path")
}

func TestPreSave_LearnedProfileStillCollapses(t *testing.T) {
	const ns, name = "honey", "replicaset-arc-runner-abc-runner-1a2b-3c4d"
	s, key := newGuardTestStorage(t, ns, name)
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()

	learned := newGuardProfile(ns, name, helpersv1.Learning)
	learned.Spec.Opens = kubeletOpens(120)
	require.NoError(t, s.Create(ctx, key, learned, &softwarecomposition.ContainerProfile{}, 0))

	got := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, got))
	assert.Less(t, len(got.Spec.Opens), 120, "a learned profile over the threshold is still folded by the collapser")
}
