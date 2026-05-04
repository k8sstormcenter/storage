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
// Implementation is recursive backtracking. Argument vectors in real
// profiles are short (typically ≤ a dozen entries) and contain at most a
// handful of wildcards, so the worst case stays well below the cost of a
// regex compile.
func CompareExecArgs(profileArgs, runtimeArgs []string) bool {
	// Outer-level empty profile = "no argv constraint" — wildcard match.
	// The inner matcher keeps strict empty-empty semantics so anchoring
	// during recursion (`profile fully consumed but runtime has more`)
	// remains a mismatch.
	if len(profileArgs) == 0 {
		return true
	}
	return matchExecArgs(profileArgs, runtimeArgs)
}

// matchExecArgs is the strict recursive matcher. Both sides must consume
// fully for a match; this is what gives the function its anchored shape.
// Callers that want "no argv constraint" semantics on empty profile go
// through CompareExecArgs, which short-circuits before this is reached.
func matchExecArgs(profileArgs, runtimeArgs []string) bool {
	if len(profileArgs) == 0 {
		return len(runtimeArgs) == 0
	}

	head := profileArgs[0]

	if head == WildcardIdentifier {
		// Try absorbing 0..len(runtimeArgs) of the runtime into this *,
		// then match the remaining profile against the remaining runtime.
		for k := 0; k <= len(runtimeArgs); k++ {
			if matchExecArgs(profileArgs[1:], runtimeArgs[k:]) {
				return true
			}
		}
		return false
	}

	if len(runtimeArgs) == 0 {
		return false
	}

	if head == DynamicIdentifier || head == runtimeArgs[0] {
		return matchExecArgs(profileArgs[1:], runtimeArgs[1:])
	}
	return false
}
