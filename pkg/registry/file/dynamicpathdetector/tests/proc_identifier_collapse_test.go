package dynamicpathdetectortests

import (
	"fmt"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	types "github.com/kubescape/storage/pkg/apis/softwarecomposition"
	dp "github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A pid and a tid are kernel-assigned identifiers, not names. The threshold
// cannot collapse them because collapse fires on a node's child COUNT: a
// container with three threads never produces enough distinct tids to reach
// any sane threshold, so the tids freeze into the profile as literals and the
// next run of the same workload — which draws different ones — is unprofiled.
//
// These tests pin the structural rule that replaces that statistical one.

const dynSeg = dp.DynamicIdentifier

func firstSight(t *testing.T, path string) string {
	t.Helper()
	// A fresh analyzer per call: whatever comes back was decided without any
	// accumulated cardinality, which is the whole point of the rule.
	got, err := dp.AnalyzeOpen(path, dp.NewPathAnalyzer(dp.OpenDynamicThreshold))
	require.NoError(t, err)
	return got
}

func TestProcIdentifiersCollapseOnFirstSight(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		// the pid
		{"/proc/1", "/proc/" + dynSeg},
		{"/proc/1234", "/proc/" + dynSeg},
		{"/proc/007", "/proc/" + dynSeg},
		{"/proc/4294967296", "/proc/" + dynSeg},
		{"/proc/1234/status", "/proc/" + dynSeg + "/status"},
		{"/proc/1234/cmdline", "/proc/" + dynSeg + "/cmdline"},

		// the tid — the case the threshold could never reach
		{"/proc/1234/task", "/proc/" + dynSeg + "/task"},
		{"/proc/1234/task/5678", "/proc/" + dynSeg + "/task/" + dynSeg},
		{"/proc/1234/task/5678/status", "/proc/" + dynSeg + "/task/" + dynSeg + "/status"},
		{"/proc/1234/task/5678/comm", "/proc/" + dynSeg + "/task/" + dynSeg + "/comm"},
		{"/proc/1/task/1/stat", "/proc/" + dynSeg + "/task/" + dynSeg + "/stat"},

		// the pid position may already be dynamic: the node agent collapses
		// /proc/<pid> at report time, so the tid arrives under a ⋯ parent.
		{"/proc/" + dynSeg + "/task/5678/status", "/proc/" + dynSeg + "/task/" + dynSeg + "/status"},
		{"/proc/" + dynSeg + "/task/5678", "/proc/" + dynSeg + "/task/" + dynSeg},

		// ... or be a stable name, in which case only the tid collapses
		{"/proc/self/task/5678/status", "/proc/self/task/" + dynSeg + "/status"},
		{"/proc/thread-self/task/99/status", "/proc/thread-self/task/" + dynSeg + "/status"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			assert.Equal(t, tc.want, firstSight(t, tc.path))
		})
	}
}

// Everything under /proc that is a NAME must survive verbatim. A profile that
// wildcards /proc/self or /proc/net/tcp stops distinguishing the reads that
// matter, which is the opposite of the fix.
func TestProcNamesAreNotIdentifiers(t *testing.T) {
	for _, path := range []string{
		"/proc",
		"/proc/self",
		"/proc/self/status",
		"/proc/self/maps",
		"/proc/thread-self",
		"/proc/cpuinfo",
		"/proc/meminfo",
		"/proc/net/tcp",
		"/proc/sys/kernel/randomize_va_space",
		"/proc/sys/net/ipv4/ip_forward",
		"/proc/1234/task/self/status", // "self" in the tid slot is a name too
		"/proc/1234/net/dev",          // numeric parent, but "net" is a name
	} {
		t.Run(path, func(t *testing.T) {
			// Only the pid, where present, may change.
			want := path
			if path == "/proc/1234/task/self/status" {
				want = "/proc/" + dynSeg + "/task/self/status"
			}
			if path == "/proc/1234/net/dev" {
				want = "/proc/" + dynSeg + "/net/dev"
			}
			assert.Equal(t, want, firstSight(t, path))
		})
	}
}

// The rule is anchored at the /proc root, and only there. Outside it an
// integer is routinely meaningful — a StatefulSet ordinal, a restart index, a
// rotation suffix — so it stays literal and the threshold decides.
func TestProcRuleIsAnchoredAtTheRoot(t *testing.T) {
	for _, path := range []string{
		"/procfs/1234/status",   // prefix boundary, not /proc
		"/var/proc/1234/status", // /proc not at the root
		"/proc2/1234",
		"/var/log/1234",
		"/etc/1234/conf",
		"/data/pods/0/restart/3",
	} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, path, firstSight(t, path), "collapsed a number outside /proc")
		})
	}
}

// Under /proc, EVERY digits-only segment is an identifier — not just the pid
// and the tid. A file descriptor, an fdinfo entry and an irq number are as
// per-run as a tid, and none of them is worth a special case.
func TestEveryIntegerUnderProcCollapses(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"/proc/1234/fd/3", "/proc/" + dynSeg + "/fd/" + dynSeg},
		{"/proc/1234/fdinfo/7", "/proc/" + dynSeg + "/fdinfo/" + dynSeg},
		{"/proc/1234/task/5678/fd/3", "/proc/" + dynSeg + "/task/" + dynSeg + "/fd/" + dynSeg},
		{"/proc/irq/24/smp_affinity", "/proc/irq/" + dynSeg + "/smp_affinity"},
		{"/proc/1234/tasks/5678", "/proc/" + dynSeg + "/tasks/" + dynSeg},
		{"/proc/self/fd/9", "/proc/self/fd/" + dynSeg},
	} {
		t.Run(tc.path, func(t *testing.T) {
			assert.Equal(t, tc.want, firstSight(t, tc.path))
		})
	}
}

// The failing case before the fix: a handful of threads, far below any
// threshold, each freezing a literal tid into the profile.
func TestFewThreadsStillCollapse(t *testing.T) {
	analyzer := dp.NewPathAnalyzer(dp.OpenDynamicThreshold)
	results := mapset.NewThreadUnsafeSet[string]()
	for _, tid := range []int{7, 8, 9} {
		got, err := dp.AnalyzeOpen(fmt.Sprintf("/proc/1234/task/%d/status", tid), analyzer)
		require.NoError(t, err)
		results.Add(got)
	}
	assert.Equal(t, 1, results.Cardinality(), "three threads must not leave three literals: %v", results.ToSlice())
	assert.True(t, results.ContainsOne("/proc/"+dynSeg+"/task/"+dynSeg+"/status"))
}

// The same thing at profile scope: AnalyzeOpens is what runs on save, so this
// is the shape that actually reaches storage.
func TestAnalyzeOpensFoldsRecordedTids(t *testing.T) {
	var opens []types.OpenCalls
	for pid := 100; pid < 104; pid++ {
		for tid := pid; tid < pid+3; tid++ {
			opens = append(opens,
				types.OpenCalls{Path: fmt.Sprintf("/proc/%d/task/%d/status", pid, tid), Flags: []string{"O_RDONLY"}},
				types.OpenCalls{Path: fmt.Sprintf("/proc/%d/task/%d/comm", pid, tid), Flags: []string{"O_RDONLY"}},
			)
		}
	}
	opens = append(opens, types.OpenCalls{Path: "/proc/self/status", Flags: []string{"O_RDONLY"}})

	got, err := dp.AnalyzeOpens(opens, dp.NewPathAnalyzer(dp.OpenDynamicThreshold), nil)
	require.NoError(t, err)

	paths := mapset.NewThreadUnsafeSet[string]()
	for _, o := range got {
		paths.Add(o.Path)
	}
	assert.ElementsMatch(t, []string{
		"/proc/" + dynSeg + "/task/" + dynSeg + "/comm",
		"/proc/" + dynSeg + "/task/" + dynSeg + "/status",
		"/proc/self/status",
	}, paths.ToSlice(), "24 recorded opens must fold to two patterns plus the named read")
}

// A profile already carrying literal tids — written by an agent that predates
// this rule — folds on the next save rather than staying poisoned.
func TestPreexistingLiteralsFoldOnResave(t *testing.T) {
	stored := []types.OpenCalls{
		{Path: "/proc/8/task/8/status", Flags: []string{"O_RDONLY"}},
		{Path: "/proc/8/task/9/status", Flags: []string{"O_RDONLY"}},
		{Path: "/proc/9/task/10/status", Flags: []string{"O_RDONLY", "O_CLOEXEC"}},
	}
	got, err := dp.AnalyzeOpens(stored, dp.NewPathAnalyzer(dp.OpenDynamicThreshold), nil)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "/proc/"+dynSeg+"/task/"+dynSeg+"/status", got[0].Path)
	assert.ElementsMatch(t, []string{"O_CLOEXEC", "O_RDONLY"}, got[0].Flags, "flags of the folded entries must merge, not be lost")
}

// Collapsing must be a fixed point: re-analyzing an emitted pattern returns it
// unchanged. Storage re-feeds stored paths on every save, so a rule that drifts
// one step per save would erode the profile to /proc/* over time.
func TestProcCollapseIsIdempotent(t *testing.T) {
	for _, path := range []string{
		"/proc/1234/task/5678/status",
		"/proc/self/task/5678/status",
		"/proc/1234/status",
		"/proc/net/tcp",
	} {
		t.Run(path, func(t *testing.T) {
			once := firstSight(t, path)
			assert.Equal(t, once, firstSight(t, once), "second pass moved the pattern")
			assert.Equal(t, once, firstSight(t, firstSight(t, once)), "third pass moved the pattern")
		})
	}
}

// ⋯ must not become * here. * absorbs every remaining segment, so
// /proc/⋯/task/⋯/status degrading to /proc/* would stop the profile
// distinguishing a thread's status read from any other procfs read at all.
func TestProcCollapseDoesNotWidenToWildcard(t *testing.T) {
	for _, path := range []string{
		"/proc/1234/task/5678",
		"/proc/1234/task/5678/status",
		"/proc/1234/task/5678/children",
	} {
		got := firstSight(t, path)
		assert.NotContains(t, got, dp.WildcardIdentifier, "%s widened to a zero-or-more wildcard", path)
	}
}

// The emitted pattern has to keep matching live events, or the fix trades
// false positives for a profile nothing satisfies.
func TestEmittedProcPatternStillMatchesLiveEvents(t *testing.T) {
	pattern := firstSight(t, "/proc/1234/task/5678/status")

	for _, path := range []string{
		"/proc/1/task/1/status",
		"/proc/99999/task/12345/status",
		"/proc/1234/task/5678/status",
	} {
		assert.True(t, dp.CompareDynamic(pattern, path), "%q must match %q", pattern, path)
	}
	for _, path := range []string{
		"/proc/1234/task/5678/comm",    // different leaf
		"/proc/1234/task/5678/fd/3",    // deeper: ⋯ is one segment
		"/proc/1234/status",            // shallower
		"/proc/1234/tasks/5678/status", // different infix
		"/etc/1234/task/5678/status",   // different root
	} {
		assert.False(t, dp.CompareDynamic(pattern, path), "%q must not match %q", pattern, path)
	}
}

// An operator who has configured /proc as noise still gets the wildcard: the
// structural rule refines the trie, it does not override a threshold-1 prefix.
func TestWildcardedProcPrefixWins(t *testing.T) {
	analyzer := dp.NewPathAnalyzerWithConfigs(dp.OpenDynamicThreshold, []dp.CollapseConfig{
		{Prefix: "/proc", Threshold: 1},
	})
	got, err := dp.AnalyzeOpen("/proc/1234/task/5678/status", analyzer)
	require.NoError(t, err)
	assert.Equal(t, "/proc/"+dp.WildcardIdentifier, got)

	// and a second, different task keeps landing on the same wildcard
	again, err := dp.AnalyzeOpen("/proc/9/task/9/comm", analyzer)
	require.NoError(t, err)
	assert.Equal(t, "/proc/"+dp.WildcardIdentifier, again)
}

// Named segments under /proc still accumulate and collapse on COUNT. The
// structural rule takes the numbers; it does not switch the threshold off for
// everything else at those levels.
func TestThresholdStillAppliesToNamesUnderProc(t *testing.T) {
	analyzer := dp.NewPathAnalyzer(3)
	var last string
	for i := 0; i < 10; i++ {
		got, err := dp.AnalyzeOpen(fmt.Sprintf("/proc/1/attr/name%d/current", i), analyzer)
		require.NoError(t, err)
		last = got
	}
	assert.Equal(t, "/proc/"+dynSeg+"/attr/"+dynSeg+"/current", last,
		"non-numeric siblings must still collapse once their count passes the threshold")
}

func BenchmarkAnalyzeProcTaskPath(b *testing.B) {
	analyzer := dp.NewPathAnalyzer(dp.OpenDynamicThreshold)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = dp.AnalyzeOpen("/proc/1234/task/5678/status", analyzer)
	}
}
