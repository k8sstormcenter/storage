package file

import (
	"context"
	"testing"
	"time"

	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition/install"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/client-go/kubernetes/scheme"
)

// TestRogueArtifact_LabelOnlyUpdatePersists pins the exact sequence behind the
// frozen-artifact symptom: an external label edit followed by the writer's
// label-only revert must persist and be visible to a subsequent Get.
func TestRogueArtifact_LabelOnlyUpdatePersists(t *testing.T) {
	pool := NewTestPool(t.TempDir())
	require.NotNil(t, pool)
	defer func() { _ = pool.Close() }()
	sch := scheme.Scheme
	install.Install(sch)
	s := NewStorageImpl(afero.NewMemMapFs(), "/", pool, nil, sch)

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()
	key := "/spdx.softwarecomposition.kubescape.io/rogueartifacts/ns1/rogue-x"

	mk := func(state string) *softwarecomposition.RogueArtifact {
		return &softwarecomposition.RogueArtifact{
			ObjectMeta: metav1.ObjectMeta{Name: "rogue-x", Namespace: "ns1",
				Labels: map[string]string{"kubescape.io/rogue-state": state, "kubescape.io/container-name": "app"}},
			Spec:   softwarecomposition.RogueArtifactSpec{State: state, ContainerName: "app"},
			Status: softwarecomposition.RogueArtifactStatus{Phase: "Firing"},
		}
	}
	require.NoError(t, s.Create(ctx, key, mk("rogue"), &softwarecomposition.RogueArtifact{}, 0))

	setLabels := func(state string) {
		out := &softwarecomposition.RogueArtifact{}
		err := s.GuaranteedUpdate(ctx, key, out, false, nil,
			func(input runtime.Object, _ storage.ResponseMeta) (runtime.Object, *uint64, error) {
				cur := input.(*softwarecomposition.RogueArtifact).DeepCopy()
				cur.Labels["kubescape.io/rogue-state"] = state
				return cur, nil, nil
			}, nil)
		require.NoError(t, err, "label-only update to %s", state)
	}

	// external edit → healed; writer revert → rogue
	setLabels("healed")
	got := &softwarecomposition.RogueArtifact{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, got))
	require.Equal(t, "healed", got.Labels["kubescape.io/rogue-state"], "external label edit must persist")

	setLabels("rogue")
	got2 := &softwarecomposition.RogueArtifact{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, got2))
	require.Equal(t, "rogue", got2.Labels["kubescape.io/rogue-state"], "the writer's label-only revert must persist and be readable")
}
