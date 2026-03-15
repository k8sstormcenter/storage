package dynamicpathdetector

import (
	"maps"
	"slices"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"
	types "github.com/kubescape/storage/pkg/apis/softwarecomposition"
)

func AnalyzeOpens(opens []types.OpenCalls, analyzer *PathAnalyzer, sbomSet mapset.Set[string]) ([]types.OpenCalls, error) {
	if opens == nil {
		return nil, nil
	}

	// First pass: build trie from all paths
	dynamicOpens := make(map[string]types.OpenCalls)
	for _, open := range opens {
		_, _ = AnalyzeOpen(open.Path, analyzer)
	}

	// Second pass: read collapsed paths and merge
	for i := range opens {
		// sbomSet files must always be preserved uncollapsed to ensure
		// reproducible vulnerability scanning results.
		if sbomSet != nil && sbomSet.ContainsOne(opens[i].Path) {
			dynamicOpens[opens[i].Path] = opens[i]
			continue
		}

		result, err := AnalyzeOpen(opens[i].Path, analyzer)
		if err != nil {
			continue
		}

		if result != opens[i].Path {
			if existing, ok := dynamicOpens[result]; ok {
				existing.Flags = mapset.Sorted(mapset.NewThreadUnsafeSet(slices.Concat(existing.Flags, opens[i].Flags)...))
				dynamicOpens[result] = existing
			} else {
				dynamicOpen := types.OpenCalls{Path: result, Flags: opens[i].Flags}
				dynamicOpens[result] = dynamicOpen
			}
		} else {
			dynamicOpens[opens[i].Path] = opens[i]
		}
	}

	result := slices.SortedFunc(maps.Values(dynamicOpens), func(a, b types.OpenCalls) int {
		return strings.Compare(a.Path, b.Path)
	})

	return consolidateOpens(result, sbomSet), nil
}

// consolidateOpens removes paths that are subsumed by a wildcard or dynamic
// identifier already present in the result. For example, if "/etc/*" is present,
// "/etc/hosts" and "/etc/nginx/conf.d" are removed because they are already covered.
func consolidateOpens(opens []types.OpenCalls, sbomSet mapset.Set[string]) []types.OpenCalls {
	if len(opens) <= 1 {
		return opens
	}

	// Collect indices of paths that contain wildcards or dynamic identifiers
	patternIdx := map[int]bool{}
	for i, o := range opens {
		if strings.Contains(o.Path, WildcardIdentifier) || strings.Contains(o.Path, DynamicIdentifier) {
			patternIdx[i] = true
		}
	}
	if len(patternIdx) == 0 {
		return opens
	}

	// Track which entries to keep and accumulate merged flags into patterns
	keep := make([]bool, len(opens))
	for i := range opens {
		keep[i] = true
	}

	for i, o := range opens {
		if patternIdx[i] {
			continue // patterns always kept
		}
		// SBOM paths must never be consolidated away
		if sbomSet != nil && sbomSet.ContainsOne(o.Path) {
			continue
		}
		for pi := range patternIdx {
			if CompareDynamic(opens[pi].Path, o.Path) {
				// o is subsumed by pattern at pi — merge flags into the pattern
				opens[pi].Flags = mapset.Sorted(mapset.NewThreadUnsafeSet(slices.Concat(opens[pi].Flags, o.Flags)...))
				keep[i] = false
				break
			}
		}
	}

	var result []types.OpenCalls
	for i, o := range opens {
		if keep[i] {
			result = append(result, o)
		}
	}
	return result
}

func AnalyzeOpen(path string, analyzer *PathAnalyzer) (string, error) {
	return analyzer.AnalyzePath(path, "opens")
}
