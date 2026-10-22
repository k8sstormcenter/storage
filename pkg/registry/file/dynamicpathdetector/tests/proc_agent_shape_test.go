package dynamicpathdetectortests

import (
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	types "github.com/kubescape/storage/pkg/apis/softwarecomposition"
	dp "github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shape the node agent actually emits, which is not the shape the rest of
// this package's tests feed in. The agent rewrites /proc/<pid> to ⋯ at report
// time, unconditionally and independent of any threshold, so storage receives
// a path whose pid slot is ALREADY dynamic and whose tid slot is still a
// literal number.
//
// That input is what made the level-wide dynamic child so costly here. A ⋯
// child sets IsNextDynamic, which routes every sibling at that level through
// it, so the agent's own pid rewrite was enough to pull /proc/self,
// /proc/cpuinfo and /proc/sys into the same pattern — a profile that no longer
// distinguishes a named procfs read from a task-directory walk, and that
// admits paths like /proc/<pid>/kernel/randomize_va_space which cannot exist.
//
// Routing pid and tid through the reserved key instead keeps the named
// siblings intact while still folding the identifiers.
func TestAgentEmittedShapeKeepsNamedProcReads(t *testing.T) {
	opens := []types.OpenCalls{
		{Path: "/proc/" + dynSeg + "/task/5678/status", Flags: []string{"O_RDONLY"}},
		{Path: "/proc/" + dynSeg + "/task/5679/status", Flags: []string{"O_RDONLY"}},
		{Path: "/proc/" + dynSeg + "/task/5680/status", Flags: []string{"O_RDONLY"}},
		{Path: "/proc/self/status", Flags: []string{"O_RDONLY"}},
		{Path: "/proc/cpuinfo", Flags: []string{"O_RDONLY"}},
		{Path: "/proc/sys/kernel/randomize_va_space", Flags: []string{"O_RDONLY"}},
	}

	got, err := dp.AnalyzeOpens(opens, dp.NewPathAnalyzer(dp.OpenDynamicThreshold), nil)
	require.NoError(t, err)

	paths := mapset.NewThreadUnsafeSet[string]()
	for _, o := range got {
		paths.Add(o.Path)
	}

	assert.ElementsMatch(t, []string{
		"/proc/cpuinfo",
		"/proc/self/status",
		"/proc/sys/kernel/randomize_va_space",
		"/proc/" + dynSeg + "/task/" + dynSeg + "/status",
	}, paths.ToSlice())

	// Stated separately from the set comparison so a failure names the actual
	// harm rather than just a diff: these three reads must remain their own
	// entries, and nothing may admit a task path that cannot exist.
	for _, named := range []string{"/proc/cpuinfo", "/proc/self/status", "/proc/sys/kernel/randomize_va_space"} {
		assert.True(t, paths.ContainsOne(named), "%s was swallowed by the pid wildcard", named)
	}
	assert.False(t, paths.ContainsOne("/proc/"+dynSeg+"/kernel/randomize_va_space"),
		"a sysctl read must not become a per-task path that cannot exist")
}

// The damaged shapes as they were actually found in stored profiles, taken
// from a per-object GET census of a live three-node cluster running the
// pre-fix analyzer: 31 profiles, 1310 recorded opens, 16 carrying /proc paths.
// Both halves of the defect were present in ordinary nginx and redis profiles.
//
// Each case names the read the container really performed and the entry the
// profile really ended up holding. This is the regression in the form it takes
// in production, rather than in the form it is convenient to construct.
func TestObservedDamagedProfileShapes(t *testing.T) {
	for _, tc := range []struct {
		what   string
		read   string // what the container opened
		stored string // what the pre-fix analyzer stored
		want   string // what it must store now
	}{
		{
			what:   "main thread's tid survives as a literal",
			read:   "/proc/1/task/1/fd",
			stored: "/proc/" + dynSeg + "/task/1/fd",
			want:   "/proc/" + dynSeg + "/task/" + dynSeg + "/fd",
		},
		{
			what:   "sysctl read swallowed by the pid",
			read:   "/proc/sys/kernel/randomize_va_space",
			stored: "/proc/" + dynSeg + "/kernel/randomize_va_space",
			want:   "/proc/sys/kernel/randomize_va_space",
		},
		{
			what:   "another sysctl read swallowed by the pid",
			read:   "/proc/sys/kernel/ngroups_max",
			stored: "/proc/" + dynSeg + "/kernel/ngroups_max",
			want:   "/proc/sys/kernel/ngroups_max",
		},
		{
			what:   "self read swallowed by the pid",
			read:   "/proc/self/setgroups",
			stored: "/proc/" + dynSeg + "/setgroups",
			want:   "/proc/self/setgroups",
		},
	} {
		t.Run(tc.what, func(t *testing.T) {
			// The analyzer is primed with a numeric pid before the case's own
			// read, in the form the node agent really emits it: already
			// rewritten to the dynamic identifier. That is what the damage
			// requires -- a dynamic node at the /proc level of the SAME
			// analyzer. A literal pid does not do it, because below the
			// threshold a literal simply stays a literal.
			// Analysed alone, on a fresh analyzer, all three swallowed cases
			// come out correct even on the pre-fix code — so a table without
			// this priming step would pass before the fix and prove nothing.
			analyzer := dp.NewPathAnalyzer(dp.OpenDynamicThreshold)
			_, err := dp.AnalyzeOpen("/proc/"+dynSeg+"/stat", analyzer)
			require.NoError(t, err)

			got, err := dp.AnalyzeOpen(tc.read, analyzer)
			require.NoError(t, err)

			assert.Equal(t, tc.want, got)
			assert.NotEqual(t, tc.stored, got, "still stores the damaged shape observed in production")
		})
	}
}

// The four damaged reads above, analysed together the way a real profile is,
// rather than one at a time. Sharing one analyzer is what let the pid's
// dynamic node reach across siblings in the first place, so the isolated
// cases would not have caught it.
func TestObservedDamagedProfileShapesTogether(t *testing.T) {
	var opens []types.OpenCalls
	for _, p := range []string{
		"/proc/1/task/1/fd",
		"/proc/2/task/2/fd",
		"/proc/sys/kernel/randomize_va_space",
		"/proc/sys/kernel/ngroups_max",
		"/proc/self/setgroups",
		"/proc/self/mountinfo",
		"/proc/1/cgroup",
	} {
		opens = append(opens, types.OpenCalls{Path: p, Flags: []string{"O_RDONLY"}})
	}

	got, err := dp.AnalyzeOpens(opens, dp.NewPathAnalyzer(dp.OpenDynamicThreshold), nil)
	require.NoError(t, err)

	paths := mapset.NewThreadUnsafeSet[string]()
	for _, o := range got {
		paths.Add(o.Path)
	}
	assert.ElementsMatch(t, []string{
		"/proc/" + dynSeg + "/task/" + dynSeg + "/fd",
		"/proc/" + dynSeg + "/cgroup",
		"/proc/self/mountinfo",
		"/proc/self/setgroups",
		"/proc/sys/kernel/ngroups_max",
		"/proc/sys/kernel/randomize_va_space",
	}, paths.ToSlice())
}
