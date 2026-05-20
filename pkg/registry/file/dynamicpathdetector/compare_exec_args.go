package dynamicpathdetector

// CompareExecArgs reports whether a runtime exec argument vector matches a
// profile argument vector. The profile vector may contain two wildcard
// tokens:
//
//	DynamicIdentifier  ("⋯") — matches exactly one argument position.
//	WildcardIdentifier ("*") — matches zero or more consecutive arguments.
//
// Anything else is a literal-equality match. The match is anchored at both
// ends: every runtime argument must be consumed by the profile vector,
// either by a literal, a DynamicIdentifier, or absorbed into a
// WildcardIdentifier run.
//
// Empty profileArgs is treated as "no argv constraint" — i.e. matches any
// runtime arg vector. This keeps path-only Execs entries (the common case
// in user-defined ApplicationProfiles, which omit the Args field) from
// silently triggering R0040 just because the rule started consulting
// was_executed_with_args. A user that wants to assert "this exec must have
// no args" can write Args: []string{} in their profile and the empty
// runtime vector still matches by virtue of the wildcard semantics.
//
// Implementation is index-based recursive backtracking with memoisation
// on (profileIndex, runtimeIndex) state pairs. The naive backtracking
// form would degrade to exponential time on adversarial inputs like
// `[*, *, *, …, x]` against a long literal vector — every prefix `*`
// has multiple split choices and the suffix mismatch only surfaces
// at the very end, so each path gets re-explored. Memoisation bounds
// the work at O(len(profile) * len(runtime)) — i.e. quadratic in the
// vector lengths, the standard wildcard-match complexity. CodeRabbit
// flagged this as a Major on PR #27.
func CompareExecArgs(profileArgs, runtimeArgs []string) bool {
	// Outer-level empty profile = "no argv constraint" — wildcard match.
	if len(profileArgs) == 0 {
		return true
	}
	// Dispatch by `*` count.
	//
	//   0 or 1 `*`  → linear forward / split-anchored walk
	//                 (compareExecArgsLinear). Zero allocations: no map,
	//                 no closure, no recursion. R0040 fires on every
	//                 execve event in clusters running the args-aware
	//                 rule; the previous code allocated 2 maps + a
	//                 closure per call (~912 B / 6 allocs on literal
	//                 args; up to ~6.5 KB on multi-`*` shapes).
	//
	//   2+ `*`      → memoised recursive backtracker
	//                 (compareExecArgsMemo). The memo absorbs the
	//                 exponential re-entry of multi-`*` patterns
	//                 (CodeRabbit upstream PR #27 finding). Allocation
	//                 cost is acceptable here because multi-`*`
	//                 patterns are author-supplied and rare on the
	//                 per-event hot path.
	//
	// Matthias's upstream PR #323 perf review on CompareDynamic
	// motivates the analogous split here.
	if hasMultipleStarsArgs(profileArgs) {
		return compareExecArgsMemo(profileArgs, runtimeArgs)
	}
	return compareExecArgsLinear(profileArgs, runtimeArgs)
}

// hasMultipleStarsArgs reports whether the profile argv contains two or
// more WildcardIdentifier tokens. Zero-allocation scan.
func hasMultipleStarsArgs(profileArgs []string) bool {
	n := 0
	for _, a := range profileArgs {
		if a == WildcardIdentifier {
			n++
			if n >= 2 {
				return true
			}
		}
	}
	return false
}

// compareExecArgsLinear is the zero-allocation core for 0 or 1 `*`
// argv patterns. Caller MUST have verified hasMultipleStarsArgs is
// false. Semantics are identical to compareExecArgsMemo.
//
// For 0 `*`: position-by-position with `⋯` (DynamicIdentifier) matching
// any single token; lengths must agree exactly (anchored at both ends).
//
// For 1 `*`: prefix segments (left of `*`) match runtime prefix; suffix
// segments (right of `*`) match runtime suffix; `*` absorbs the middle.
// Mid `*` admits zero or more runtime args, trailing `*` admits zero or
// more remaining args (still anchored — every runtime arg consumed by
// either a literal, a `⋯`, or the `*`-run).
func compareExecArgsLinear(p, r []string) bool {
	// Locate the `*` segment, if any.
	starIdx := -1
	for i, a := range p {
		if a == WildcardIdentifier {
			starIdx = i
			break
		}
	}
	if starIdx < 0 {
		// 0 `*` — anchored position-by-position match.
		if len(p) != len(r) {
			return false
		}
		for i := range p {
			if p[i] != DynamicIdentifier && p[i] != r[i] {
				return false
			}
		}
		return true
	}
	// 1 `*` — split prefix / suffix at the `*` index.
	prefixLen := starIdx
	suffixLen := len(p) - starIdx - 1
	if len(r) < prefixLen+suffixLen {
		return false
	}
	// Prefix walks forward from runtime[0].
	for i := 0; i < prefixLen; i++ {
		if p[i] != DynamicIdentifier && p[i] != r[i] {
			return false
		}
	}
	// Suffix walks forward from runtime[len(r)-suffixLen:].
	rSuffixStart := len(r) - suffixLen
	for i := 0; i < suffixLen; i++ {
		pSeg := p[starIdx+1+i]
		if pSeg != DynamicIdentifier && pSeg != r[rSuffixStart+i] {
			return false
		}
	}
	return true
}

// compareExecArgsMemo is the memoised recursive backtracker, reached
// only when the profile argv has 2+ `*` tokens. The memo bounds work
// at O(len(profile) * len(runtime)) on adversarial inputs like
// `[*, *, *, …, sentinel]` against a long literal vector — every
// prefix `*` has multiple split choices and the suffix mismatch only
// surfaces at the very end. CodeRabbit upstream PR #27 finding.
func compareExecArgsMemo(profileArgs, runtimeArgs []string) bool {
	type state struct{ pi, ri int }
	memo := make(map[state]bool, (len(profileArgs)+1)*(len(runtimeArgs)+1))
	seen := make(map[state]bool, (len(profileArgs)+1)*(len(runtimeArgs)+1))

	var match func(pi, ri int) bool
	match = func(pi, ri int) bool {
		s := state{pi: pi, ri: ri}
		if seen[s] {
			return memo[s]
		}
		seen[s] = true

		if pi == len(profileArgs) {
			memo[s] = ri == len(runtimeArgs)
			return memo[s]
		}

		head := profileArgs[pi]

		if head == WildcardIdentifier {
			for k := ri; k <= len(runtimeArgs); k++ {
				if match(pi+1, k) {
					memo[s] = true
					return true
				}
			}
			memo[s] = false
			return false
		}

		if ri == len(runtimeArgs) {
			memo[s] = false
			return false
		}

		if head == DynamicIdentifier || head == runtimeArgs[ri] {
			memo[s] = match(pi+1, ri+1)
			return memo[s]
		}

		memo[s] = false
		return false
	}

	return match(0, 0)
}
