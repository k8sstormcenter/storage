package dynamicpathdetectortests

import (
	"fmt"
	"testing"

	types "github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"github.com/stretchr/testify/assert"
)

func TestAnalyzeExecsNoCollapse(t *testing.T) {
	threshold := 10
	input := []types.ExecCalls{
		{Path: "/usr/bin/curl", Args: []string{"http://example.com"}},
		{Path: "/usr/bin/curl", Args: []string{"http://example.org"}},
		{Path: "/usr/bin/curl", Args: []string{"http://example.com"}}, // duplicate
	}

	result := dynamicpathdetector.AnalyzeExecs(input, threshold)

	// Should dedup but not collapse (only 2 unique values < threshold)
	assert.Len(t, result, 2)
}

func TestAnalyzeExecsArgPositionCollapse(t *testing.T) {
	threshold := 10
	var input []types.ExecCalls
	for i := 0; i < threshold+1; i++ {
		input = append(input, types.ExecCalls{
			Path: "/usr/bin/curl",
			Args: []string{fmt.Sprintf("http://service%d/api", i)},
		})
	}

	result := dynamicpathdetector.AnalyzeExecs(input, threshold)

	assert.Len(t, result, 1)
	assert.Equal(t, "/usr/bin/curl", result[0].Path)
	assert.Equal(t, []string{dynamicpathdetector.DynamicIdentifier}, result[0].Args)
}

func TestAnalyzeExecsDifferentBinariesIsolated(t *testing.T) {
	threshold := 10
	var input []types.ExecCalls

	// 11 unique curl URLs → should collapse
	for i := 0; i < threshold+1; i++ {
		input = append(input, types.ExecCalls{
			Path: "/usr/bin/curl",
			Args: []string{fmt.Sprintf("http://service%d", i)},
		})
	}

	// Only 2 unique grep patterns → should NOT collapse
	input = append(input,
		types.ExecCalls{Path: "/bin/grep", Args: []string{"pattern1"}},
		types.ExecCalls{Path: "/bin/grep", Args: []string{"pattern2"}},
	)

	result := dynamicpathdetector.AnalyzeExecs(input, threshold)

	curlResults := filterByPath(result, "/usr/bin/curl")
	grepResults := filterByPath(result, "/bin/grep")

	assert.Len(t, curlResults, 1)
	assert.Equal(t, []string{dynamicpathdetector.DynamicIdentifier}, curlResults[0].Args)

	assert.Len(t, grepResults, 2)
}

func TestAnalyzeExecsPreservesStaticArgs(t *testing.T) {
	threshold := 10
	var input []types.ExecCalls

	// curl -s <dynamic URL> — only the URL varies
	for i := 0; i < threshold+1; i++ {
		input = append(input, types.ExecCalls{
			Path: "/usr/bin/curl",
			Args: []string{"-s", fmt.Sprintf("http://service%d/api", i)},
		})
	}

	result := dynamicpathdetector.AnalyzeExecs(input, threshold)

	assert.Len(t, result, 1)
	assert.Equal(t, "/usr/bin/curl", result[0].Path)
	assert.Equal(t, []string{"-s", dynamicpathdetector.DynamicIdentifier}, result[0].Args)
}

func TestAnalyzeExecsVariableLengthArgs(t *testing.T) {
	threshold := 10
	var input []types.ExecCalls

	// Some have 1 arg, some have 2
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

	result := dynamicpathdetector.AnalyzeExecs(input, threshold)

	// First arg position should collapse; results may vary by length
	for _, r := range result {
		assert.Equal(t, dynamicpathdetector.DynamicIdentifier, r.Args[0])
	}
}

func TestAnalyzeExecsEmptyArgs(t *testing.T) {
	input := []types.ExecCalls{
		{Path: "/usr/bin/ls", Args: []string{}},
		{Path: "/usr/bin/ls"},
	}

	result := dynamicpathdetector.AnalyzeExecs(input, 10)

	// Both have no args — should dedup to a small set
	assert.NotEmpty(t, result)
	for _, r := range result {
		assert.Equal(t, "/usr/bin/ls", r.Path)
	}
}

func TestAnalyzeExecsNilInput(t *testing.T) {
	result := dynamicpathdetector.AnalyzeExecs(nil, 10)
	assert.Nil(t, result)
}

func TestAnalyzeExecsThreshold1(t *testing.T) {
	input := []types.ExecCalls{
		{Path: "/usr/bin/echo", Args: []string{"hello"}},
		{Path: "/usr/bin/echo", Args: []string{"world"}},
	}

	result := dynamicpathdetector.AnalyzeExecs(input, 1)

	// Threshold 1: any position with >1 unique value collapses
	assert.Len(t, result, 1)
	assert.Equal(t, []string{dynamicpathdetector.DynamicIdentifier}, result[0].Args)
}

func TestAnalyzeExecsParentPathPreserved(t *testing.T) {
	threshold := 10
	var input []types.ExecCalls
	for i := 0; i < threshold+1; i++ {
		input = append(input, types.ExecCalls{
			Path:       "/usr/bin/curl",
			Args:       []string{fmt.Sprintf("http://service%d", i)},
			ParentPath: "/bin/bash",
		})
	}

	result := dynamicpathdetector.AnalyzeExecs(input, threshold)

	assert.Len(t, result, 1)
	assert.Equal(t, "/bin/bash", result[0].ParentPath)
	assert.Equal(t, []string{dynamicpathdetector.DynamicIdentifier}, result[0].Args)
}

func filterByPath(execs []types.ExecCalls, path string) []types.ExecCalls {
	var filtered []types.ExecCalls
	for _, e := range execs {
		if e.Path == path {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
