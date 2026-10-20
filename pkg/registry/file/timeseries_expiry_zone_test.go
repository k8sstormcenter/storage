package file

import (
	"context"
	"testing"
	"time"

	helpersv1 "github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListTimeSeriesExpired_LocalZoneDoesNotExpireFreshRows(t *testing.T) {
	prev := time.Local
	time.Local = time.FixedZone("CEST", 2*60*60)
	t.Cleanup(func() { time.Local = prev })

	pool := NewTestPool(t.TempDir())
	require.NotNil(t, pool)
	t.Cleanup(func() { _ = pool.Close() })
	conn, err := pool.Take(context.Background())
	require.NoError(t, err)
	defer pool.Put(conn)

	fresh := time.Now().UTC().Add(-10 * time.Minute).String()
	stale := time.Now().UTC().Add(-3 * time.Hour).String()
	require.NoError(t, WriteTimeSeriesEntry(conn, "containerprofile", "ns", "fresh", "s", "1", fresh, helpersv1.Learning, helpersv1.Partial, "", true))
	require.NoError(t, WriteTimeSeriesEntry(conn, "containerprofile", "ns", "stale", "s", "1", stale, helpersv1.Learning, helpersv1.Partial, "", true))

	keys, err := ListTimeSeriesExpired(conn, time.Hour)
	require.NoError(t, err)
	assert.Equal(t, []string{K8sKeysToPath("", "spdx.softwarecomposition.kubescape.io", "containerprofile", "", "ns", "stale")}, keys,
		"only the row older than the threshold expires; a local zone ahead of UTC must not expire a fresh row")
}
