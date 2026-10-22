package file

import (
	"context"
	"testing"
	"time"

	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apiserver/pkg/storage"
)

// Pins the consolidation livelock (goroutine-dump-proven): a gate holder's
// synchronous Get of an ABSENT key must not block on the gate — the old
// unconditional gated self-heal parked the read for the full lockTimeout on
// the very gate its caller held on another connection, sacrificing one
// consolidation pass per maintenance cycle, forever.
func TestGet_AbsentKey_DoesNotBlockOnHeldGate(t *testing.T) {
	env := newDuressEnv(t)

	holderConn, putHolder, err := env.s.takeConn(context.Background())
	require.NoError(t, err)
	defer putHolder()
	releaseGate, err := gateForPool(env.pool).acquire(context.Background(), holderConn)
	require.NoError(t, err)
	defer releaseGate()

	readerConn, putReader, err := env.s.takeConn(context.Background())
	require.NoError(t, err)
	defer putReader()

	// The provider's bootstrap shape: Get of a never-created key while the
	// gate is held by a DIFFERENT connection.
	start := time.Now()
	obj := &softwarecomposition.ContainerProfile{}
	err = env.s.GetWithConn(context.Background(), readerConn, cpKey("ns", "never-created"),
		storage.GetOptions{IgnoreNotFound: true}, obj)
	elapsed := time.Since(start)

	assert.NoError(t, err, "IgnoreNotFound Get of an absent key returns the zero value")
	assert.Less(t, elapsed, 2*time.Second,
		"a read must never wait on the write gate (pre-fix: parked ~lockTimeout=5s in the gated self-heal)")
}

// The heal itself still works when it is actually needed and the gate is
// free: metadata exists, payload missing → metadata row is reclaimed.
func TestSelfHeal_MetadataWithoutPayload_StillHeals(t *testing.T) {
	env := newDuressEnv(t)
	key := cpKey("ns", "torn")

	conn, put, err := env.s.takeConn(context.Background())
	require.NoError(t, err)
	defer put()
	require.NoError(t, WriteJSON(conn, key, []byte(`{"metadata":{"name":"torn","namespace":"ns"}}`)))

	obj := &softwarecomposition.ContainerProfile{}
	err = env.s.GetWithConn(context.Background(), conn, key, storage.GetOptions{IgnoreNotFound: true}, obj)
	assert.NoError(t, err)
	_, merr := ReadMetadata(conn, key)
	assert.ErrorIs(t, merr, ErrMetadataNotFound, "orphaned metadata must be reclaimed by the free-gate heal")
}
