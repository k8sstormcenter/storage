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

	if sbomSet == nil {
		sbomSet = mapset.NewThreadUnsafeSet[string]()
	}

	dynamicOpens := make(map[string]types.OpenCalls)
	for _, open := range opens {
		_, _ = AnalyzeOpen(open.Path, analyzer)
	}

	for i := range opens {
		// sbomSet files have to be always present in the dynamicOpens
		if sbomSet.ContainsOne(opens[i].Path) {
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

// consolidateOpens drops any literal Open whose path is already covered
// by a wildcard / dynamic-identifier sibling in the same result set, and
// merges the dropped entry's Flags into that sibling. This is a
// post-trie cleanup pass: AnalyzeOpens may emit both a collapsed pattern
// (e.g. /etc/⋯) AND the original literals (/etc/passwd) when only some
// children at that node hit threshold. Without this pass, downstream
// matchers see both forms and the literal acts as redundant noise.
//
// Two invariants:
//
//  1. Patterns are always preserved — if a path contains either
//     WildcardIdentifier or DynamicIdentifier it counts as a pattern
//     and is never absorbed.
//  2. SBOM-listed paths are always preserved — they are part of the
//     image's manifest and must remain identifiable on their own,
//     even if a wildcard pattern would otherwise cover them.
//
// Subsumption check uses the same CompareDynamic the runtime matcher
// (CEL `ap.was_opened`) uses, so both sides agree on what "covered"
// means at every depth.
func consolidateOpens(opens []types.OpenCalls, sbomSet mapset.Set[string]) []types.OpenCalls {
	if len(opens) <= 1 {
		return opens
	}

	patternIdx := map[int]bool{}
	for i, o := range opens {
		if strings.Contains(o.Path, WildcardIdentifier) || strings.Contains(o.Path, DynamicIdentifier) {
			patternIdx[i] = true
		}
	}
	if len(patternIdx) == 0 {
		return opens
	}

	keep := make([]bool, len(opens))
	for i := range opens {
		keep[i] = true
	}

	for i, o := range opens {
		if patternIdx[i] {
			continue // patterns always survive
		}
		if sbomSet != nil && sbomSet.ContainsOne(o.Path) {
			continue // SBOM paths always survive
		}
		for pi := range patternIdx {
			if CompareDynamic(opens[pi].Path, o.Path) {
				// `o` is subsumed by the pattern at pi — fold its Flags
				// into the pattern entry so all observed access modes
				// remain represented.
				opens[pi].Flags = mapset.Sorted(mapset.NewThreadUnsafeSet(slices.Concat(opens[pi].Flags, o.Flags)...))
				keep[i] = false
				break
			}
		}
	}

	out := make([]types.OpenCalls, 0, len(opens))
	for i, o := range opens {
		if keep[i] {
			out = append(out, o)
		}
	}
	return out
}

func AnalyzeOpen(path string, analyzer *PathAnalyzer) (string, error) {
	return analyzer.AnalyzePath(path, "opens")
}
