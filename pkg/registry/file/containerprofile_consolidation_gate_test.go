package file

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	helpersv1 "github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage"
)

type mergeHold struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int64
}

func newMergeHold() *mergeHold {
	return &mergeHold{entered: make(chan struct{}, 8), release: make(chan struct{})}
}

func (h *mergeHold) settings() dynamicpathdetector.CollapseSettings {
	h.calls.Add(1)
	select {
	case h.entered <- struct{}{}:
	default:
	}
	<-h.release
	return dynamicpathdetector.DefaultCollapseSettings()
}

func (h *mergeHold) waitEntered(t *testing.T) {
	t.Helper()
	select {
	case <-h.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("consolidation never reached the deflate")
	}
}

func withShortLockTimeout(t *testing.T, d time.Duration) {
	t.Helper()
	prev := lockTimeout
	lockTimeout = d
	t.Cleanup(func() { lockTimeout = prev })
}

func cpKeyOne(ns, name string) string {
	return "/spdx.softwarecomposition.kubescape.io/containerprofile/" + ns + "/" + name
}

func ingestParts(t *testing.T, e *duressEnv, ns string, parts []*softwarecomposition.ContainerProfile) {
	t.Helper()
	for _, p := range parts {
		require.NoError(t, e.s.Create(context.Background(), cpKeyOne(ns, p.Name), p, &softwarecomposition.ContainerProfile{}, 0))
	}
}

func rowsWithData(t *testing.T, e *duressEnv, key string) int {
	t.Helper()
	conn, err := e.pool.Take(context.Background())
	require.NoError(t, err)
	defer e.pool.Put(conn)
	series, err := ListTimeSeriesContainers(conn, key)
	require.NoError(t, err)
	n := 0
	for _, rows := range series {
		for _, r := range rows {
			if r.HasData {
				n++
			}
		}
	}
	return n
}

func TestConsolidation_APIWritesProceedDuringMerge(t *testing.T) {
	withShortLockTimeout(t, 500*time.Millisecond)
	e := newDuressEnv(t)
	hold := newMergeHold()
	e.processor.CollapseSettings = hold.settings

	const ns = "arc-systems"
	parts, err := duressParts(ns, "series-runner", 6)
	require.NoError(t, err)
	consolidatedName, _ := SplitProfileName(parts[0].Name)
	consolidatedKey := cpKeyOne(ns, consolidatedName)
	ingestParts(t, e, ns, parts)

	done := make(chan error, 1)
	go func() { done <- e.processor.ConsolidateTimeSeries(context.Background()) }()
	hold.waitEntered(t)

	release, free := gateForPool(e.pool).tryAcquire(nil)
	if free {
		release()
	}
	assert.True(t, free, "write gate must be free while the merge and deflate run")

	late, err := duressParts(ns, "series-runner", 7)
	require.NoError(t, err)
	latePart := late[6]
	latePart.Annotations[helpersv1.StatusMetadataKey] = helpersv1.Learning
	latePart.Annotations[helpersv1.CompletionMetadataKey] = helpersv1.Partial
	reqCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	lateErr := e.s.Create(reqCtx, cpKeyOne(ns, latePart.Name), latePart, &softwarecomposition.ContainerProfile{}, 0)
	assert.NoError(t, lateErr, "a part arriving while another key consolidates must be stored, not time out")
	assert.False(t, apierrors.IsServerTimeout(lateErr))

	userCP := &softwarecomposition.ContainerProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "redis-client",
			Namespace:   "redis",
			Annotations: map[string]string{helpersv1.ManagedByMetadataKey: helpersv1.ManagedByUserValue},
		},
		Spec: softwarecomposition.ContainerProfileSpec{Execs: []softwarecomposition.ExecCalls{{Path: "/usr/bin/redis-cli"}}},
	}
	userErr := e.s.Create(reqCtx, cpKeyOne("redis", "redis-client"), userCP, &softwarecomposition.ContainerProfile{}, 0)
	assert.NoError(t, userErr, "a user-managed profile written while a merge runs must be stored")

	updErr := e.s.GuaranteedUpdate(reqCtx, cpKeyOne("redis", "redis-client"), &softwarecomposition.ContainerProfile{}, false, nil,
		func(input runtime.Object, _ storage.ResponseMeta) (runtime.Object, *uint64, error) {
			cur := input.(*softwarecomposition.ContainerProfile).DeepCopy()
			cur.Spec.Execs = append(cur.Spec.Execs, softwarecomposition.ExecCalls{Path: "/usr/bin/id"})
			return cur, nil, nil
		}, nil)
	assert.NoError(t, updErr, "a user-managed profile re-applied while a merge runs must be stored")

	close(hold.release)
	select {
	case cerr := <-done:
		require.NoError(t, cerr)
	case <-time.After(20 * time.Second):
		t.Fatal("consolidation did not finish after the deflate was released")
	}

	var got softwarecomposition.ContainerProfile
	require.NoError(t, e.s.Get(context.Background(), consolidatedKey, storage.GetOptions{}, &got))
	for _, p := range parts {
		gerr := e.s.Get(context.Background(), cpKeyOne(ns, p.Name), storage.GetOptions{}, &softwarecomposition.ContainerProfile{})
		assert.True(t, storage.IsNotFound(gerr) || apierrors.IsNotFound(gerr), "merged part %s must be deleted", p.Name)
	}
	if lateErr == nil {
		require.NoError(t, e.s.Get(context.Background(), cpKeyOne(ns, latePart.Name), storage.GetOptions{}, &softwarecomposition.ContainerProfile{}),
			"the late part must survive the pass that did not include it")
		assert.Equal(t, 1, rowsWithData(t, e, consolidatedKey), "the late part stays queued for the next pass")
		require.NoError(t, e.processor.ConsolidateTimeSeries(context.Background()))
		assert.Equal(t, 0, rowsWithData(t, e, consolidatedKey))
		gerr := e.s.Get(context.Background(), cpKeyOne(ns, latePart.Name), storage.GetOptions{}, &softwarecomposition.ContainerProfile{})
		assert.True(t, storage.IsNotFound(gerr) || apierrors.IsNotFound(gerr), "the late part is merged on the next pass")
	}
	assertAgreement(t, e, []string{consolidatedKey, cpKeyOne("redis", "redis-client")})
}

func TestConsolidation_DeflateRunsOncePerPass(t *testing.T) {
	e := newDuressEnv(t)
	hold := newMergeHold()
	close(hold.release)
	e.processor.CollapseSettings = hold.settings

	parts, err := duressParts("ns-once", "series-once", 3)
	require.NoError(t, err)
	ingestParts(t, e, "ns-once", parts)
	before := hold.calls.Load()
	require.NoError(t, e.processor.ConsolidateTimeSeries(context.Background()))
	assert.Equal(t, int64(1), hold.calls.Load()-before, "the consolidated profile is deflated exactly once per pass")
}

func TestConsolidation_FailedCommitKeepsParts(t *testing.T) {
	e := newDuressEnv(t)
	parts, err := duressParts("ns-fail", "series-fail", 4)
	require.NoError(t, err)
	consolidatedName, _ := SplitProfileName(parts[0].Name)
	consolidatedKey := cpKeyOne("ns-fail", consolidatedName)
	ingestParts(t, e, "ns-fail", parts)

	injected := errors.New("injected rename failure")
	var armed atomic.Bool
	armed.Store(true)
	e.s.renamePayloadFn = func(oldname, newname string) error {
		if armed.Load() {
			return injected
		}
		return e.fs.Rename(oldname, newname)
	}
	cerr := e.processor.ConsolidateTimeSeries(context.Background())
	require.Error(t, cerr)
	assert.ErrorIs(t, cerr, injected)

	for _, p := range parts {
		require.NoError(t, e.s.Get(context.Background(), cpKeyOne("ns-fail", p.Name), storage.GetOptions{}, &softwarecomposition.ContainerProfile{}),
			"a failed commit must leave every part in place")
	}
	assert.Equal(t, len(parts), rowsWithData(t, e, consolidatedKey), "a failed commit must leave every row queued")
	gerr := e.s.Get(context.Background(), consolidatedKey, storage.GetOptions{}, &softwarecomposition.ContainerProfile{})
	assert.True(t, storage.IsNotFound(gerr) || apierrors.IsNotFound(gerr), "no consolidated profile after a rolled-back commit")

	armed.Store(false)
	require.NoError(t, e.processor.ConsolidateTimeSeries(context.Background()))
	require.NoError(t, e.s.Get(context.Background(), consolidatedKey, storage.GetOptions{}, &softwarecomposition.ContainerProfile{}))
	assert.Equal(t, 0, rowsWithData(t, e, consolidatedKey))
	assertAgreement(t, e, []string{consolidatedKey})
}

func TestConsolidation_RunnerBacklogDoesNotStarveUserWrites(t *testing.T) {
	const (
		runnerKeys   = 40
		partsPerKey  = 3
		userProfiles = 13
		deflateCost  = 150 * time.Millisecond
		requestLimit = 100 * time.Millisecond
	)
	e := newDuressEnv(t)
	e.processor.Workers = 2
	e.processor.CollapseSettings = func() dynamicpathdetector.CollapseSettings {
		time.Sleep(deflateCost)
		return dynamicpathdetector.DefaultCollapseSettings()
	}

	const ns = "arc-systems"
	runnerConsolidated := make([]string, 0, runnerKeys)
	for i := 0; i < runnerKeys; i++ {
		parts, err := duressParts(ns, "series-runner", partsPerKey)
		require.NoError(t, err)
		for _, p := range parts {
			p.Name = "replicaset-runner-" + string(rune('a'+i%26)) + string(rune('a'+i/26)) + p.Name[len("replicaset-coredns-5d78c9869d"):]
			p.Annotations[helpersv1.StatusMetadataKey] = helpersv1.Learning
			p.Annotations[helpersv1.CompletionMetadataKey] = helpersv1.Partial
		}
		name, _ := SplitProfileName(parts[0].Name)
		runnerConsolidated = append(runnerConsolidated, cpKeyOne(ns, name))
		ingestParts(t, e, ns, parts)
	}

	userKeys := make([]string, 0, userProfiles)
	for i := 0; i < userProfiles; i++ {
		name := "user-cp-" + string(rune('a'+i))
		key := cpKeyOne("redis", name)
		userKeys = append(userKeys, key)
		cp := &softwarecomposition.ContainerProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: "redis",
				Annotations: map[string]string{helpersv1.ManagedByMetadataKey: helpersv1.ManagedByUserValue},
			},
			Spec: softwarecomposition.ContainerProfileSpec{Execs: []softwarecomposition.ExecCalls{{Path: "/usr/bin/redis-server"}}},
		}
		require.NoError(t, e.s.Create(context.Background(), key, cp, &softwarecomposition.ContainerProfile{}, 0))
	}

	stop := make(chan struct{})
	var timedOut, failed, applied atomic.Int64
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			key := userKeys[i%len(userKeys)]
			reqCtx, cancel := context.WithTimeout(context.Background(), requestLimit)
			err := e.s.GuaranteedUpdate(reqCtx, key, &softwarecomposition.ContainerProfile{}, false, nil,
				func(input runtime.Object, _ storage.ResponseMeta) (runtime.Object, *uint64, error) {
					cur := input.(*softwarecomposition.ContainerProfile).DeepCopy()
					cur.Annotations["reconciler/pass"] = strconv.Itoa(i)
					return cur, nil, nil
				}, nil)
			cancel()
			switch {
			case err == nil:
				applied.Add(1)
			case apierrors.IsServerTimeout(err) || errors.Is(err, context.DeadlineExceeded):
				timedOut.Add(1)
			default:
				failed.Add(1)
				t.Logf("user write failed: %v", err)
			}
			time.Sleep(5 * time.Millisecond)
		}
	}()

	passCtx, passCancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer passCancel()
	perr := e.processor.ConsolidateTimeSeries(passCtx)
	close(stop)
	wg.Wait()
	require.NoError(t, perr, "the backlog pass must complete")

	assert.Zero(t, timedOut.Load(), "user-managed writes must never lose the write gate to a merge")
	assert.Zero(t, failed.Load())
	assert.Greater(t, applied.Load(), int64(userProfiles), "the reconciler kept applying throughout the pass")
	for _, key := range runnerConsolidated {
		require.NoError(t, e.s.Get(context.Background(), key, storage.GetOptions{}, &softwarecomposition.ContainerProfile{}), "runner profile %s consolidated", key)
		assert.Equal(t, 0, rowsWithData(t, e, key))
	}
	assertAgreement(t, e, append(runnerConsolidated, userKeys...))
}
