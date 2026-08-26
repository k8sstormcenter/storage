package file

import (
	"context"
	"errors"
	"os"
	"strings"
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
	"zombiezen.com/go/sqlite"
)

// TestSaveObject_MetadataFailureLeavesPayloadUntouched pins issue #44: a write
// whose metadata step fails must not have replaced the payload — GET keeps
// serving the previous object and no divergence forms.
func TestSaveObject_MetadataFailureLeavesPayloadUntouched(t *testing.T) {
	pool := NewTestPool(t.TempDir())
	require.NotNil(t, pool)
	defer func() { _ = pool.Close() }()
	sch := scheme.Scheme
	install.Install(sch)
	si := NewStorageImpl(afero.NewMemMapFs(), "/", pool, nil, sch).(*StorageImpl)

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()
	key := "/spdx.softwarecomposition.kubescape.io/rogueartifacts/ns1/rogue-a"

	mk := func(state string) *softwarecomposition.RogueArtifact {
		return &softwarecomposition.RogueArtifact{
			ObjectMeta: metav1.ObjectMeta{Name: "rogue-a", Namespace: "ns1",
				Labels: map[string]string{"kubescape.io/rogue-state": state}},
			Spec: softwarecomposition.RogueArtifactSpec{State: "rogue"},
		}
	}
	require.NoError(t, si.Create(ctx, key, mk("rogue"), &softwarecomposition.RogueArtifact{}, 0))

	// Sabotage the metadata step exactly where a lock/interrupt bites in
	// production — after the payload stage.
	si.writeMetadataFn = func(*sqlite.Conn, string, runtime.Object) error {
		return errors.New("sqlite: step: interrupted (injected)")
	}

	out := &softwarecomposition.RogueArtifact{}
	uerr := si.GuaranteedUpdate(ctx, key, out, false, nil,
		func(input runtime.Object, _ storage.ResponseMeta) (runtime.Object, *uint64, error) {
			cur := input.(*softwarecomposition.RogueArtifact).DeepCopy()
			cur.Labels["kubescape.io/rogue-state"] = "healed"
			return cur, nil, nil
		}, nil)
	require.Error(t, uerr, "update with a failing metadata step must fail")

	// Repair the seam and verify the payload was untouched: GET still serves
	// the pre-update object, and a normal update now succeeds and is visible.
	si.writeMetadataFn = writeMetadata
	got := &softwarecomposition.RogueArtifact{}
	require.NoError(t, si.Get(ctx, key, storage.GetOptions{}, got))
	require.Equal(t, "rogue", got.Labels["kubescape.io/rogue-state"],
		"a failed write must not leak a new payload (issue #44 torn write)")

	require.NoError(t, si.GuaranteedUpdate(ctx, key, out, false, nil,
		func(input runtime.Object, _ storage.ResponseMeta) (runtime.Object, *uint64, error) {
			cur := input.(*softwarecomposition.RogueArtifact).DeepCopy()
			cur.Labels["kubescape.io/rogue-state"] = "healed"
			return cur, nil, nil
		}, nil))
	got2 := &softwarecomposition.RogueArtifact{}
	require.NoError(t, si.Get(ctx, key, storage.GetOptions{}, got2))
	require.Equal(t, "healed", got2.Labels["kubescape.io/rogue-state"], "recovered write must land normally")
}

// TestSaveObject_RenameFailureRollsBackMetadata pins the rollback contract: if
// the payload rename fails after the metadata write, the metadata is rolled
// back (no inverse divergence) and the staged temp file is removed.
func TestSaveObject_RenameFailureRollsBackMetadata(t *testing.T) {
	pool := NewTestPool(t.TempDir())
	require.NotNil(t, pool)
	defer func() { _ = pool.Close() }()
	sch := scheme.Scheme
	install.Install(sch)
	fs := afero.NewMemMapFs()
	si := NewStorageImplWithCollector(fs, "/", pool, nil, sch, DefaultProcessor{}).(*StorageImpl)

	ctx, cancel := context.WithTimeout(context.TODO(), 10*time.Second)
	defer cancel()
	key := "/spdx.softwarecomposition.kubescape.io/rogueartifacts/ns1/rogue-r"

	mk := func(state string) *softwarecomposition.RogueArtifact {
		return &softwarecomposition.RogueArtifact{
			ObjectMeta: metav1.ObjectMeta{Name: "rogue-r", Namespace: "ns1",
				Labels: map[string]string{"kubescape.io/rogue-state": state}},
			Spec: softwarecomposition.RogueArtifactSpec{State: "rogue"},
		}
	}
	require.NoError(t, si.Create(ctx, key, mk("rogue"), &softwarecomposition.RogueArtifact{}, 0))

	si.renamePayloadFn = func(oldpath, newpath string) error {
		return errors.New("rename failed (injected)")
	}
	out := &softwarecomposition.RogueArtifact{}
	uerr := si.GuaranteedUpdate(ctx, key, out, false, nil,
		func(input runtime.Object, _ storage.ResponseMeta) (runtime.Object, *uint64, error) {
			cur := input.(*softwarecomposition.RogueArtifact).DeepCopy()
			cur.Labels["kubescape.io/rogue-state"] = "healed"
			return cur, nil, nil
		}, nil)
	require.Error(t, uerr, "update must fail when the payload rename fails")
	si.renamePayloadFn = nil

	// Payload unchanged.
	got := &softwarecomposition.RogueArtifact{}
	require.NoError(t, si.Get(ctx, key, storage.GetOptions{}, got))
	require.Equal(t, "rogue", got.Labels["kubescape.io/rogue-state"], "payload must be unchanged")

	// METADATA rolled back too: the namespace-scoped List reads metadata.
	list := &softwarecomposition.RogueArtifactList{}
	require.NoError(t, si.GetList(ctx, "/spdx.softwarecomposition.kubescape.io/rogueartifacts/ns1", storage.ListOptions{Recursive: true, Predicate: storage.Everything}, list))
	require.Len(t, list.Items, 1)
	require.Equal(t, "rogue", list.Items[0].Labels["kubescape.io/rogue-state"],
		"metadata must be ROLLED BACK when the rename fails — no inverse divergence")

	// No staged temp file left behind.
	leaked := false
	_ = afero.Walk(fs, "/", func(path string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() && strings.HasSuffix(path, ".t") {
			leaked = true
		}
		return nil
	})
	require.False(t, leaked, "no staged .t payload file may remain after a failed write")
}
