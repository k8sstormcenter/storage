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
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestRogueArtifact_RoundTrip(t *testing.T) {
	pool := NewTestPool(t.TempDir())
	require.NotNil(t, pool)
	defer func() { _ = pool.Close() }()
	sch := scheme.Scheme
	install.Install(sch)
	s := NewStorageImpl(afero.NewMemMapFs(), "/", pool, nil, sch)

	ctx, cancel := context.WithTimeout(context.TODO(), 5*time.Second)
	defer cancel()

	ra := &softwarecomposition.RogueArtifact{
		ObjectMeta: metav1.ObjectMeta{Name: "rogue-abc", Namespace: "ns1", Labels: map[string]string{"kubescape.io/rogue-state": "rogue"}},
		Spec:       softwarecomposition.RogueArtifactSpec{State: "rogue", ContainerName: "nginx"},
		Status:     softwarecomposition.RogueArtifactStatus{Phase: "Firing", FiredAt: metav1.NewTime(time.Now())},
	}
	key := "/spdx.softwarecomposition.kubescape.io/rogueartifacts/ns1/rogue-abc"
	require.NoError(t, s.Create(ctx, key, ra, &softwarecomposition.RogueArtifact{}, 0), "Create RogueArtifact")

	// Real serving path: clientset List → apiserver → StorageImpl.GetList,
	// keyed on the RESOURCE (rogueartifacts), matching the create key.
	list := &softwarecomposition.RogueArtifactList{}
	require.NoError(t, s.GetList(ctx, "/spdx.softwarecomposition.kubescape.io/rogueartifacts/ns1", storage.ListOptions{Recursive: true, Predicate: storage.Everything}, list), "GetList")
	require.Len(t, list.Items, 1, "the RogueArtifact must be listable")
	require.Equal(t, "rogue", list.Items[0].Labels["kubescape.io/rogue-state"], "state must ride in a label so it survives the metadata-only List")
}
