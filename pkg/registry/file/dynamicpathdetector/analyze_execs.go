package dynamicpathdetector

import (
	"maps"
	"slices"
	"strings"

	types "github.com/kubescape/storage/pkg/apis/softwarecomposition"
)

// AnalyzeExecs collapses exec argument vectors using a trie-based approach.
// Argument positions with more than threshold unique values are replaced with DynamicIdentifier (⋯).
// Results are deduplicated by their collapsed string representation and sorted.
func AnalyzeExecs(execs []types.ExecCalls, threshold int) []types.ExecCalls {
	if execs == nil {
		return nil
	}

	analyzer := NewArgAnalyzer(threshold)

	// First pass: build trie from all arg vectors, grouped by exec Path
	for _, exec := range execs {
		analyzer.AddArgs(exec.Args, exec.Path)
	}

	// Second pass: read collapsed arg vectors, dedup by collapsed string
	dedupMap := make(map[string]types.ExecCalls)
	for _, exec := range execs {
		collapsed := analyzer.AnalyzeArgs(exec.Args, exec.Path)
		collapsedExec := types.ExecCalls{
			Path:       exec.Path,
			Args:       collapsed,
			Envs:       exec.Envs,
			ParentPath: exec.ParentPath,
		}
		key := collapsedExec.String()
		if _, ok := dedupMap[key]; !ok {
			dedupMap[key] = collapsedExec
		}
	}

	return slices.SortedFunc(maps.Values(dedupMap), func(a, b types.ExecCalls) int {
		return strings.Compare(a.String(), b.String())
	})
}
