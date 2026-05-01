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
// Implementation is recursive backtracking. Argument vectors in real
// profiles are short (typically ≤ a dozen entries) and contain at most a
// handful of wildcards, so the worst case stays well below the cost of a
// regex compile.
func CompareExecArgs(profileArgs, runtimeArgs []string) bool {
	if len(profileArgs) == 0 {
		return len(runtimeArgs) == 0
	}

	head := profileArgs[0]

	if head == WildcardIdentifier {
		// Try absorbing 0..len(runtimeArgs) of the runtime into this *,
		// then match the remaining profile against the remaining runtime.
		for k := 0; k <= len(runtimeArgs); k++ {
			if CompareExecArgs(profileArgs[1:], runtimeArgs[k:]) {
				return true
			}
		}
		return false
	}

	if len(runtimeArgs) == 0 {
		return false
	}

	if head == DynamicIdentifier || head == runtimeArgs[0] {
		return CompareExecArgs(profileArgs[1:], runtimeArgs[1:])
	}
	return false
}
