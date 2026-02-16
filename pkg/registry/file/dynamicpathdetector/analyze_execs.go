package dynamicpathdetector

import (
	"maps"
	"slices"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	types "github.com/kubescape/storage/pkg/apis/softwarecomposition"
)

// DeduplicateExecs removes exact-duplicate ExecCalls based on their
// string representation (Path + Args + Envs + ParentPath).
func DeduplicateExecs(execs []types.ExecCalls) []types.ExecCalls {
	if execs == nil {
		return nil
	}
	out := make([]types.ExecCalls, 0, len(execs))
	seen := mapset.NewThreadUnsafeSet[string]()
	for _, e := range execs {
		key := e.String()
		if seen.Contains(key) {
			continue
		}
		seen.Add(key)
		out = append(out, e)
	}
	return out
}

// CollapseExecArgs collapses argument positions that show high variability
// across execs sharing the same binary Path. Positions with more than
// threshold unique values are replaced with DynamicIdentifier (⋯).
// Since collapsing can turn previously-distinct entries into duplicates,
// a second dedup pass is applied to the result.
func CollapseExecArgs(execs []types.ExecCalls, threshold int) []types.ExecCalls {
	if execs == nil {
		return nil
	}

	analyzer := NewArgAnalyzer(threshold)

	// Build trie from all arg vectors, grouped by exec Path
	for _, exec := range execs {
		analyzer.AddArgs(exec.Args, exec.Path)
	}

	// Apply collapsing and dedup the result
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
