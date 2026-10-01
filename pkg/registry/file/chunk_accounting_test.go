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
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/apiserver/pkg/storage"
)

func TestChunkAccounting_OneAddedOneDeletedPerChunkAndAStableShadow(t *testing.T) {
	s, processor, _, _ := newLifecycleStorage(t)
	ctx, cancel := context.WithTimeout(context.TODO(), 30*time.Second)
	defer cancel()
	const ns = "shop"
	const baseName = "replicaset-web-abcd-web-1111-2222"
	key := cpPrefixKey + "/" + ns + "/" + baseName
	chunk := func(suffix, reportTs, prevTs string, execs []softwarecomposition.ExecCalls) *softwarecomposition.ContainerProfile {
		return &softwarecomposition.ContainerProfile{
			ObjectMeta: metav1.ObjectMeta{Name: baseName + "-" + suffix, Namespace: ns,
				Annotations: map[string]string{
					helpersv1.CompletionMetadataKey:              helpersv1.Partial,
					helpersv1.InstanceIDMetadataKey:              "apiVersion-apps/v1/namespace-shop/kind-ReplicaSet/name-web-abcd/containerName-web",
					helpersv1.PreviousReportTimestampMetadataKey: prevTs,
					helpersv1.ReportSeriesIdMetadataKey:          "11111111-2222-3333-4444-555555555555",
					helpersv1.ReportTimestampMetadataKey:         reportTs,
					helpersv1.StatusMetadataKey:                  helpersv1.Learning,
					helpersv1.ContainerTypeMetadataKey:           "containers",
				}},
			Spec: softwarecomposition.ContainerProfileSpec{Execs: execs},
		}
	}
	w, err := s.Watch(ctx, cpPrefixKey, storage.ListOptions{ResourceVersion: softwarecomposition.ResourceVersionFullSpec})
	require.NoError(t, err)
	defer w.Stop()
	drain := func() map[watch.EventType][]string {
		got := map[watch.EventType][]string{}
		for {
			select {
			case ev := <-w.ResultChan():
				cp, ok := ev.Object.(*softwarecomposition.ContainerProfile)
				name := ""
				if ok {
					name = cp.Name
				}
				got[ev.Type] = append(got[ev.Type], name)
			case <-time.After(700 * time.Millisecond):
				return got
			}
		}
	}

	require.NoError(t, s.Create(ctx, key+"-aaaa1111", chunk("aaaa1111", "2025-06-24T10:00:00Z", "", []softwarecomposition.ExecCalls{{Path: "/bin/sh"}}), nil, 0))
	require.NoError(t, s.Create(ctx, key+"-bbbb2222", chunk("bbbb2222", "2025-06-24T10:10:00Z", "2025-06-24T10:00:00Z", []softwarecomposition.ExecCalls{{Path: "/usr/sbin/nginx"}}), nil, 0))
	got := drain()
	assert.Len(t, got[watch.Added], 2, "one Added per chunk")
	assert.Empty(t, got[watch.Deleted])

	require.NoError(t, processor.ConsolidateTimeSeries(ctx))
	got = drain()
	assert.ElementsMatch(t, []string{baseName + "-aaaa1111", baseName + "-bbbb2222"}, got[watch.Deleted], "every merged chunk is deleted exactly once")
	t.Logf("events after the first consolidation: %v", got)
	appearances := append(append([]string{}, got[watch.Added]...), got[watch.Modified]...)
	assert.Equal(t, []string{baseName}, appearances, "the merged shadow appears once, as Added or Modified")
	var shadow softwarecomposition.ContainerProfile
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, &shadow))
	rv := shadow.ResourceVersion
	require.Len(t, shadow.Spec.Execs, 2)

	require.NoError(t, processor.ConsolidateTimeSeries(ctx))
	got = drain()
	assert.Empty(t, got, "a consolidation pass with no new chunk emits nothing")
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, &shadow))
	assert.Equal(t, rv, shadow.ResourceVersion, "the shadow's ResourceVersion moves only when merged content changes")

	require.NoError(t, s.Create(ctx, key+"-cccc3333", chunk("cccc3333", "2025-06-24T10:20:00Z", "2025-06-24T10:10:00Z", []softwarecomposition.ExecCalls{{Path: "/bin/sh"}}), nil, 0))
	drain()
	require.NoError(t, processor.ConsolidateTimeSeries(ctx))
	got = drain()
	assert.Equal(t, []string{baseName + "-cccc3333"}, got[watch.Deleted])
	require.NoError(t, s.Get(ctx, key, storage.GetOptions{}, &shadow))
	assert.Len(t, shadow.Spec.Execs, 2, "a chunk that adds nothing new leaves the merged content as it was")
	if len(got[watch.Modified]) > 0 {
		t.Logf("consolidating an identical chunk emitted a Modified event; RV %s -> %s", rv, shadow.ResourceVersion)
	}
}
