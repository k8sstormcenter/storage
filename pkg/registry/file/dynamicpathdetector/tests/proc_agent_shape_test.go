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
