package file

import (
	"context"
	"fmt"
	"testing"
	"time"

	helpersv1 "github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition/v1beta1"
	"github.com/kubescape/storage/pkg/config"
	"github.com/kubescape/storage/pkg/utils"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/client-go/kubernetes/scheme"
	"zombiezen.com/go/sqlite/sqlitemigration"
)

func newMergeCapStorage(t *testing.T) (*ContainerProfileProcessor, *StorageImpl, *sqlitemigration.Pool) {
	t.Helper()
	pool := NewTestPool(t.TempDir())
	require.NotNil(t, pool)
	t.Cleanup(func() { _ = pool.Close() })
	sch := scheme.Scheme
	require.NoError(t, softwarecomposition.AddToScheme(sch))
	require.NoError(t, v1beta1.AddToScheme(sch))
	processor := NewContainerProfileProcessor(config.Config{DefaultNamespace: "kubescape", MaxContainerProfileSize: 40000}, nil)
	processor.Interval = 0
	s := &StorageImpl{
		appFs:           afero.NewMemMapFs(),
		pool:            pool,
		locks:           utils.NewMapMutex[string](),
		processor:       processor,
		root:            DefaultStorageRoot,
		scheme:          sch,
		versioner:       storage.APIObjectVersioner{},
		watchDispatcher: NewWatchDispatcher(),
	}
	processor.SetStorage(NewContainerProfileStorageImpl(s, pool))
	return processor, s, pool
}

func createChunks(t *testing.T, s *StorageImpl, ns, base string, n int) string {
	t.Helper()
	ctx := context.TODO()
	start := time.Now().Add(-time.Hour)
	prev := time.Time{}
	for i := 0; i < n; i++ {
		rt := start.Add(time.Duration(i) * time.Minute)
		p := &softwarecomposition.ContainerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%032x", base, i+1),
				Namespace: ns,
				Annotations: map[string]string{
					helpersv1.InstanceIDMetadataKey:              "apiVersion-apps/v1/namespace-" + ns + "/kind-ReplicaSet/name-x/containerName-c",
					helpersv1.WlidMetadataKey:                    "wlid://cluster-t/namespace-" + ns + "/deployment-x",
					helpersv1.ReportSeriesIdMetadataKey:          "series-1",
					helpersv1.ReportTimestampMetadataKey:         rt.String(),
					helpersv1.PreviousReportTimestampMetadataKey: prev.String(),
					helpersv1.StatusMetadataKey:                  helpersv1.Learning,
					helpersv1.CompletionMetadataKey:              helpersv1.Partial,
				},
			},
			Spec: softwarecomposition.ContainerProfileSpec{Opens: []softwarecomposition.OpenCalls{{Path: fmt.Sprintf("/opt/app/f%d", i), Flags: []string{"O_RDONLY"}}}},
		}
		prev = rt
		require.NoError(t, s.Create(ctx, "/spdx.softwarecomposition.kubescape.io/containerprofile/"+ns+"/"+p.Name, p, nil, 0))
	}
	return "/spdx.softwarecomposition.kubescape.io/containerprofile/" + ns + "/" + base
}

func rowsWithData(t *testing.T, pool *sqlitemigration.Pool, key string) int {
	t.Helper()
	conn, err := pool.Take(context.TODO())
	require.NoError(t, err)
	defer pool.Put(conn)
	rows, err := ListTimeSeriesContainers(conn, key)
	require.NoError(t, err)
	n := 0
	for _, series := range rows {
		for _, r := range series {
			if r.HasData {
				n++
			}
		}
	}
	return n
}

func TestConsolidate_MergesAtMostMaxChunksPerMergePerTick(t *testing.T) {
	processor, s, pool := newMergeCapStorage(t)
	processor.MaxChunksPerMerge = 4
	key := createChunks(t, s, "ns-cap", "replicaset-x-c-aaaa-bbbb", 10)
	require.Equal(t, 10, rowsWithData(t, pool, key))

	ctx := context.TODO()
	require.NoError(t, processor.ConsolidateTimeSeries(ctx))
	assert.Equal(t, 6, rowsWithData(t, pool, key), "one tick folds four chunks and leaves the rest for the next")

	base := &softwarecomposition.ContainerProfile{}
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, base))
	assert.Len(t, base.Spec.Opens, 4)

	for i := 0; i < 3 && rowsWithData(t, pool, key) > 0; i++ {
		require.NoError(t, processor.ConsolidateTimeSeries(ctx))
	}
	assert.Equal(t, 0, rowsWithData(t, pool, key))
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, base))
	assert.Len(t, base.Spec.Opens, 10, "every chunk reaches the base within three ticks")
}

func TestConsolidate_UnlimitedWhenCapIsZero(t *testing.T) {
	processor, s, pool := newMergeCapStorage(t)
	processor.MaxChunksPerMerge = 0
	key := createChunks(t, s, "ns-nocap", "replicaset-y-c-aaaa-bbbb", 10)
	require.NoError(t, processor.ConsolidateTimeSeries(context.TODO()))
	assert.Equal(t, 0, rowsWithData(t, pool, key))
}
