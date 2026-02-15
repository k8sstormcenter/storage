package dynamicpathdetectortests

import (
	"fmt"
	"testing"

	types "github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"github.com/stretchr/testify/assert"
)

const threshold = dynamicpathdetector.ExecArgDynamicThreshold

// ---------------------------------------------------------------------------
// DeduplicateExecs — exact-duplicate removal, no collapsing
// ---------------------------------------------------------------------------

func TestDeduplicateExecsRemovesDuplicates(t *testing.T) {
	input := []types.ExecCalls{
		{Path: "/usr/bin/curl", Args: []string{"http://example.com"}},
		{Path: "/usr/bin/curl", Args: []string{"http://example.com"}}, // exact dup
		{Path: "/usr/bin/curl", Args: []string{"http://example.org"}},
	}

	result := dynamicpathdetector.DeduplicateExecs(input)

	assert.Len(t, result, 2)
	assert.Equal(t, []string{"http://example.com"}, result[0].Args)
	assert.Equal(t, []string{"http://example.org"}, result[1].Args)
}

func TestDeduplicateExecsPreservesOrder(t *testing.T) {
	input := []types.ExecCalls{
		{Path: "/bin/mkdir", Args: []string{"-p", "/var/log"}},
		{Path: "/bin/rm", Args: []string{"-f", "/tmp/lock"}},
		{Path: "/bin/mkdir", Args: []string{"-p", "/var/log"}}, // exact dup
	}

	result := dynamicpathdetector.DeduplicateExecs(input)

	assert.Len(t, result, 2)
	assert.Equal(t, "/bin/mkdir", result[0].Path)
	assert.Equal(t, "/bin/rm", result[1].Path)
}

func TestDeduplicateExecsNil(t *testing.T) {
	assert.Nil(t, dynamicpathdetector.DeduplicateExecs(nil))
}

func TestDeduplicateExecsDistinguishesByParentPath(t *testing.T) {
	input := []types.ExecCalls{
		{Path: "/usr/bin/ls", Args: []string{"-l"}, ParentPath: "/bin/bash"},
		{Path: "/usr/bin/ls", Args: []string{"-l"}, ParentPath: "/bin/sh"},
	}

	result := dynamicpathdetector.DeduplicateExecs(input)

	// Same Path+Args but different ParentPath → not duplicates
	assert.Len(t, result, 2)
}

// ---------------------------------------------------------------------------
// CollapseExecArgs — argument-vector collapsing on already-deduped input
// ---------------------------------------------------------------------------

func TestCollapseExecArgsApacheStartup(t *testing.T) {
	// Already-deduped execs from a real Apache container.
	// 3 variants for mkdir and 3 for dirname — exceeds ExecArgDynamicThreshold.
	deduped := []types.ExecCalls{
		{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/lock/apache2"}},
		{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/log/apache2"}},
		{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/run/apache2"}},
		{Path: "/bin/rm", Args: []string{"/bin/rm", "-f", "/var/run/apache2/apache2.pid"}},
		{Path: "/usr/bin/dirname", Args: []string{"/usr/bin/dirname", "/var/lock/apache2"}},
		{Path: "/usr/bin/dirname", Args: []string{"/usr/bin/dirname", "/var/log/apache2"}},
		{Path: "/usr/bin/dirname", Args: []string{"/usr/bin/dirname", "/var/run/apache2"}},
	}

	result := dynamicpathdetector.CollapseExecArgs(deduped, 3)

	// 7 entries → 3 after collapsing:
	//   /bin/mkdir       [/bin/mkdir, -p, ⋯]
	//   /bin/rm          [/bin/rm, -f, /var/run/apache2/apache2.pid]  (only 1, unchanged)
	//   /usr/bin/dirname [/usr/bin/dirname, ⋯]
	assert.Len(t, result, 3, "got: %v", result)

	mkdirResults := filterByPath(result, "/bin/mkdir")
	assert.Len(t, mkdirResults, 1)
	assert.Equal(t, []string{"/bin/mkdir", "-p", dynamicpathdetector.DynamicIdentifier}, mkdirResults[0].Args)

	rmResults := filterByPath(result, "/bin/rm")
	assert.Len(t, rmResults, 1)
	assert.Equal(t, []string{"/bin/rm", "-f", "/var/run/apache2/apache2.pid"}, rmResults[0].Args)

	dirnameResults := filterByPath(result, "/usr/bin/dirname")
	assert.Len(t, dirnameResults, 1)
	assert.Equal(t, []string{"/usr/bin/dirname", dynamicpathdetector.DynamicIdentifier}, dirnameResults[0].Args)
}

func TestCollapseExecArgsPreservesStaticPositions(t *testing.T) {
	// curl -s <varying URL> — only the URL position varies
	var deduped []types.ExecCalls
	for i := 0; i < threshold+1; i++ {
		deduped = append(deduped, types.ExecCalls{
			Path: "/usr/bin/curl",
			Args: []string{"-s", fmt.Sprintf("http://service%d/api", i)},
		})
	}

	result := dynamicpathdetector.CollapseExecArgs(deduped, threshold)

	assert.Len(t, result, 1)
	assert.Equal(t, []string{"-s", dynamicpathdetector.DynamicIdentifier}, result[0].Args)
}

func TestCollapseExecArgsBelowThresholdNoCollapse(t *testing.T) {
	// Generate exactly threshold entries — at threshold means not exceeded, no collapse
	var deduped []types.ExecCalls
	for i := 0; i < threshold; i++ {
		deduped = append(deduped, types.ExecCalls{
			Path: "/bin/mkdir",
			Args: []string{"/bin/mkdir", "-p", fmt.Sprintf("/var/dir%d", i)},
		})
	}

	result := dynamicpathdetector.CollapseExecArgs(deduped, threshold)
	assert.Len(t, result, threshold, "at threshold, nothing should collapse")
}

func TestCollapseExecArgsDifferentBinariesIsolated(t *testing.T) {
	var deduped []types.ExecCalls

	// threshold+1 unique curl URLs → should collapse
	for i := 0; i < threshold+1; i++ {
		deduped = append(deduped, types.ExecCalls{
			Path: "/usr/bin/curl",
			Args: []string{fmt.Sprintf("http://service%d", i)},
		})
	}

	// Only 1 grep pattern → should NOT collapse (well below threshold)
	deduped = append(deduped,
		types.ExecCalls{Path: "/bin/grep", Args: []string{"pattern1"}},
	)

	result := dynamicpathdetector.CollapseExecArgs(deduped, threshold)

	assert.Len(t, filterByPath(result, "/usr/bin/curl"), 1)
	assert.Equal(t, []string{dynamicpathdetector.DynamicIdentifier}, filterByPath(result, "/usr/bin/curl")[0].Args)
	assert.Len(t, filterByPath(result, "/bin/grep"), 1)
}

func TestCollapseExecArgsEmptyArgs(t *testing.T) {
	deduped := []types.ExecCalls{
		{Path: "/usr/bin/ls", Args: []string{}},
		{Path: "/usr/bin/ls"},
	}

	result := dynamicpathdetector.CollapseExecArgs(deduped, threshold)

	assert.NotEmpty(t, result)
	for _, r := range result {
		assert.Equal(t, "/usr/bin/ls", r.Path)
	}
}

func TestCollapseExecArgsNil(t *testing.T) {
	assert.Nil(t, dynamicpathdetector.CollapseExecArgs(nil, threshold))
}

func TestCollapseExecArgsParentPathPreserved(t *testing.T) {
	var deduped []types.ExecCalls
	for i := 0; i < threshold+1; i++ {
		deduped = append(deduped, types.ExecCalls{
			Path:       "/usr/bin/curl",
			Args:       []string{fmt.Sprintf("http://service%d", i)},
			ParentPath: "/bin/bash",
		})
	}

	result := dynamicpathdetector.CollapseExecArgs(deduped, threshold)

	assert.Len(t, result, 1)
	assert.Equal(t, "/bin/bash", result[0].ParentPath)
	assert.Equal(t, []string{dynamicpathdetector.DynamicIdentifier}, result[0].Args)
}

// ---------------------------------------------------------------------------
// Pipeline — DeduplicateExecs then CollapseExecArgs (as used in processors)
// ---------------------------------------------------------------------------

func TestPipelineApacheStartupWithDuplicates(t *testing.T) {
	// Raw input with exact duplicates mixed in
	input := []types.ExecCalls{
		{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/lock/apache2"}},
		{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/log/apache2"}},
		{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/lock/apache2"}}, // exact dup
		{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/run/apache2"}},
		{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/log/apache2"}},  // exact dup
		{Path: "/usr/bin/dirname", Args: []string{"/usr/bin/dirname", "/var/lock/apache2"}},
		{Path: "/usr/bin/dirname", Args: []string{"/usr/bin/dirname", "/var/log/apache2"}},
		{Path: "/usr/bin/dirname", Args: []string{"/usr/bin/dirname", "/var/run/apache2"}},
		{Path: "/usr/bin/dirname", Args: []string{"/usr/bin/dirname", "/var/lock/apache2"}}, // exact dup
	}

	// Step 1: dedup removes 3 exact duplicates → 6 unique entries
	deduped := dynamicpathdetector.DeduplicateExecs(input)
	assert.Len(t, deduped, 6)

	// Step 2: collapse merges mkdir variants and dirname variants → 2
	result := dynamicpathdetector.CollapseExecArgs(deduped, 2)
	assert.Len(t, result, 2)

	mkdirResults := filterByPath(result, "/bin/mkdir")
	assert.Len(t, mkdirResults, 1)
	assert.Equal(t, []string{"/bin/mkdir", "-p", dynamicpathdetector.DynamicIdentifier}, mkdirResults[0].Args)

	dirnameResults := filterByPath(result, "/usr/bin/dirname")
	assert.Len(t, dirnameResults, 1)
	assert.Equal(t, []string{"/usr/bin/dirname", dynamicpathdetector.DynamicIdentifier}, dirnameResults[0].Args)
}

func TestPipelineVariableLengthArgs(t *testing.T) {
	var input []types.ExecCalls

	for i := 0; i < threshold+1; i++ {
		args := []string{fmt.Sprintf("http://service%d", i)}
		if i%2 == 0 {
			args = append(args, "--verbose")
		}
		input = append(input, types.ExecCalls{
			Path: "/usr/bin/curl",
			Args: args,
		})
	}

	deduped := dynamicpathdetector.DeduplicateExecs(input)
	result := dynamicpathdetector.CollapseExecArgs(deduped, threshold)

	for _, r := range result {
		assert.Equal(t, dynamicpathdetector.DynamicIdentifier, r.Args[0])
	}
}

// ---------------------------------------------------------------------------

func filterByPath(execs []types.ExecCalls, path string) []types.ExecCalls {
	var filtered []types.ExecCalls
	for _, e := range execs {
		if e.Path == path {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
