package file

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apiserver/pkg/storage"
)

func TestConsolidation_SlowSaveOfALargeProfileStillCommits(t *testing.T) {
	e := newDuressEnv(t)
	parts, err := duressParts("honey", "series-host", 4)
	require.NoError(t, err)
	consolidatedName, _ := SplitProfileName(parts[0].Name)
	consolidatedKey := cpKeyOne("honey", consolidatedName)
	ingestParts(t, e, "honey", parts)

	var slow atomic.Bool
	slow.Store(true)
	e.s.renamePayloadFn = func(oldname, newname string) error {
		if slow.Load() {
			time.Sleep(6 * time.Second)
		}
		return e.fs.Rename(oldname, newname)
	}

	require.NoError(t, e.processor.ConsolidateTimeSeries(context.Background()), "a save that takes longer than an API request budget must still commit")
	slow.Store(false)

	got := &softwarecomposition.ContainerProfile{}
	require.NoError(t, e.s.Get(context.Background(), consolidatedKey, storage.GetOptions{}, got))
	assert.Equal(t, 0, rowsWithData(t, e, consolidatedKey), "every part merged")
	for _, p := range parts {
		gerr := e.s.Get(context.Background(), cpKeyOne("honey", p.Name), storage.GetOptions{}, &softwarecomposition.ContainerProfile{})
		assert.True(t, storage.IsNotFound(gerr) || apierrors.IsNotFound(gerr), "merged part %s is deleted", p.Name)
	}
}
