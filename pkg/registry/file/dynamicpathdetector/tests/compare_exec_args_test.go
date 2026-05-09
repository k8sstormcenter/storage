package dynamicpathdetectortests

import (
	"testing"
	"time"

	"github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
)

// CompareExecArgs matches a runtime argument vector against a profile
// argument vector that may contain two wildcard tokens:
//
//	"⋯" (DynamicIdentifier)  — matches exactly ONE argument position.
//	"*" (WildcardIdentifier) — matches ZERO OR MORE consecutive args.
//
// Anything else is a literal string match. The match must be exact across
// the full vectors — extra runtime args after the profile is exhausted (and
// no trailing wildcard absorbs them) is a non-match.

func TestCompareExecArgs_LiteralMatch(t *testing.T) {
	cases := []struct {
		name    string
		profile []string
		runtime []string
		want    bool
	}{
		// Empty profileArgs = "no argv constraint" — matches any runtime.
		// Pinned this way so path-only Execs entries in user-defined
		// ApplicationProfiles don't silently trigger R0040 when the rule
		// consults was_executed_with_args. See storage/node-agent issue
		// where Test_28 (and others using path-only entries) failed because
		// the strict empty-empty match was firing R0040 on every legit exec.
		{"both empty", nil, nil, true},
		{"empty profile, non-empty runtime", nil, []string{"a"}, true},
		{"empty profile, multi-arg runtime", nil, []string{"a", "b", "c"}, true},
		{"non-empty profile, empty runtime", []string{"a"}, nil, false},
		{"single literal match", []string{"--help"}, []string{"--help"}, true},
		{"single literal mismatch", []string{"--help"}, []string{"--version"}, false},
		{"profile longer than runtime", []string{"a", "b"}, []string{"a"}, false},
		{"runtime longer than profile (no wildcard)", []string{"a"}, []string{"a", "b"}, false},
		{"multi-literal match", []string{"-l", "-a", "/tmp"}, []string{"-l", "-a", "/tmp"}, true},
		{"multi-literal mismatch in middle", []string{"-l", "-a", "/tmp"}, []string{"-l", "-z", "/tmp"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dynamicpathdetector.CompareExecArgs(tc.profile, tc.runtime); got != tc.want {
				t.Errorf("CompareExecArgs(%v, %v) = %v, want %v", tc.profile, tc.runtime, got, tc.want)
			}
		})
	}
}

func TestCompareExecArgs_DynamicIdentifier(t *testing.T) {
	cases := []struct {
		name    string
		profile []string
		runtime []string
		want    bool
	}{
		{"⋯ matches one arg", []string{"⋯"}, []string{"anything"}, true},
		{"⋯ does NOT match zero args", []string{"⋯"}, nil, false},
		{"⋯ does NOT match two args", []string{"⋯"}, []string{"a", "b"}, false},
		{"⋯ in middle, full vector matches", []string{"--user", "⋯", "--port", "8080"}, []string{"--user", "alice", "--port", "8080"}, true},
		{"⋯ in middle, surrounding literal mismatch", []string{"--user", "⋯", "--port", "8080"}, []string{"--user", "alice", "--port", "9090"}, false},
		{"adjacent ⋯⋯ matches exactly two args", []string{"⋯", "⋯"}, []string{"a", "b"}, true},
		{"adjacent ⋯⋯ rejects one arg", []string{"⋯", "⋯"}, []string{"a"}, false},
		{"adjacent ⋯⋯ rejects three args", []string{"⋯", "⋯"}, []string{"a", "b", "c"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dynamicpathdetector.CompareExecArgs(tc.profile, tc.runtime); got != tc.want {
				t.Errorf("CompareExecArgs(%v, %v) = %v, want %v", tc.profile, tc.runtime, got, tc.want)
			}
		})
	}
}

func TestCompareExecArgs_WildcardIdentifier(t *testing.T) {
	cases := []struct {
		name    string
		profile []string
		runtime []string
		want    bool
	}{
		{"* matches empty runtime", []string{"*"}, nil, true},
		{"* matches one arg", []string{"*"}, []string{"a"}, true},
		{"* matches many args", []string{"*"}, []string{"a", "b", "c", "d"}, true},
		{"trailing * with prefix match", []string{"-c", "*"}, []string{"-c", "echo hi"}, true},
		{"trailing * absorbs nothing when runtime exact-prefix length", []string{"-c", "*"}, []string{"-c"}, true},
		{"trailing * mismatch in literal prefix", []string{"-c", "*"}, []string{"-x", "echo hi"}, false},
		{"middle * matches and re-anchors on literal", []string{"sh", "*", "exit"}, []string{"sh", "-c", "echo hi", "exit"}, true},
		{"middle * with literal that does not appear", []string{"sh", "*", "exit"}, []string{"sh", "-c", "echo hi"}, false},
		{"middle * matches when zero args between anchors", []string{"sh", "*", "exit"}, []string{"sh", "exit"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dynamicpathdetector.CompareExecArgs(tc.profile, tc.runtime); got != tc.want {
				t.Errorf("CompareExecArgs(%v, %v) = %v, want %v", tc.profile, tc.runtime, got, tc.want)
			}
		})
	}
}

func TestCompareExecArgs_MixedTokens(t *testing.T) {
	cases := []struct {
		name    string
		profile []string
		runtime []string
		want    bool
	}{
		{"⋯ then * — needs at least one arg before the *",
			[]string{"⋯", "*"}, []string{"a"}, true},
		{"⋯ then * — empty runtime fails (⋯ needs one)",
			[]string{"⋯", "*"}, nil, false},
		{"⋯ then * — many args ok",
			[]string{"⋯", "*"}, []string{"a", "b", "c"}, true},
		{"* then ⋯ — needs at least one arg for ⋯",
			[]string{"*", "⋯"}, []string{"x"}, true},
		{"* then ⋯ — empty runtime fails",
			[]string{"*", "⋯"}, nil, false},
		{"literal, ⋯, *  — typical user pattern",
			[]string{"--user", "⋯", "*"}, []string{"--user", "alice", "--verbose", "--out", "/tmp"}, true},
		{"literal, ⋯, *  — runtime too short for ⋯",
			[]string{"--user", "⋯", "*"}, []string{"--user"}, false},
		{"only ⋯, runtime empty — fails (⋯ requires exactly one)",
			[]string{"⋯"}, []string{}, false},
		{"only *, runtime empty — passes",
			[]string{"*"}, []string{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dynamicpathdetector.CompareExecArgs(tc.profile, tc.runtime); got != tc.want {
				t.Errorf("CompareExecArgs(%v, %v) = %v, want %v", tc.profile, tc.runtime, got, tc.want)
			}
		})
	}
}

func TestCompareExecArgs_RealisticPatterns(t *testing.T) {
	cases := []struct {
		name    string
		profile []string
		runtime []string
		want    bool
	}{
		{"curl with any URL", []string{"-s", "⋯"}, []string{"-s", "https://example.com"}, true},
		{"sh -c with any command",
			[]string{"-c", "*"},
			[]string{"-c", "while true; do sleep 1; done"},
			true,
		},
		{"echo with any number of words",
			[]string{"hello", "*"},
			[]string{"hello", "world", "from", "test"},
			true,
		},
		{"ls -l in arbitrary directory",
			[]string{"-l", "⋯"},
			[]string{"-l", "/var/log"},
			true,
		},
		{"ls without args fails wildcard arg pattern",
			[]string{"-l", "⋯"},
			[]string{"-l"},
			false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dynamicpathdetector.CompareExecArgs(tc.profile, tc.runtime); got != tc.want {
				t.Errorf("CompareExecArgs(%v, %v) = %v, want %v", tc.profile, tc.runtime, got, tc.want)
			}
		})
	}
}

// TestCompareExecArgs_ReDoSResistance pins that the matcher handles
// adversarial wildcard-heavy inputs in bounded time. The classic
// catastrophic-backtracking case is `[*, *, *, …, "literal"]` vs a
// long literal-runtime vector that mismatches the trailing literal
// — every prefix * has multiple split choices and the suffix
// mismatch only surfaces at the very end, so each path gets
// re-explored. With memoisation this is O(P*R); without it, naïve
// recursion would be exponential.
//
// CodeRabbit flagged the unmemoised version on PR #27 (Major).
func TestCompareExecArgs_ReDoSResistance(t *testing.T) {
	// 20 leading wildcards + a literal that won't match. Without
	// memoisation, the naïve matcher tries roughly 2^20 path splits
	// before failing — observable as a many-second test. The
	// memoised version completes in microseconds.
	profile := make([]string, 0, 21)
	for i := 0; i < 20; i++ {
		profile = append(profile, dynamicpathdetector.WildcardIdentifier)
	}
	profile = append(profile, "needle-that-does-not-exist")

	runtime := make([]string, 0, 50)
	for i := 0; i < 50; i++ {
		runtime = append(runtime, "a")
	}

	start := time.Now()
	got := dynamicpathdetector.CompareExecArgs(profile, runtime)
	elapsed := time.Since(start)

	if got {
		t.Errorf("expected mismatch for trailing-literal that isn't in runtime")
	}
	// Memoised matcher: 21 * 51 = ~1100 states, each O(R) work for
	// the wildcard split → total bound ~50K ops. Generous budget of
	// 100ms catches any regression to the unmemoised form (which
	// would be measured in seconds, not milliseconds, on this input).
	if elapsed > 100*time.Millisecond {
		t.Errorf("matcher took %v on adversarial input — memoisation regression?", elapsed)
	}
}
