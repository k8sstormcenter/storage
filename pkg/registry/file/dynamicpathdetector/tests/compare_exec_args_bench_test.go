package dynamicpathdetectortests

import (
	"testing"

	"github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
)

// BenchmarkCompareExecArgs covers the five representative shapes
// CompareExecArgs sees in the R0040 hot path:
//
//   - literal_match:   pure literal Args, runtime matches byte-for-byte
//   - literal_mismatch: pure literal Args, runtime mismatches partway
//   - ellipsis_one:    profile uses one `⋯` (DynamicIdentifier) — exactly one
//                      argv position
//   - star_trailing:   profile ends with `*` (WildcardIdentifier) — 0+ trailing
//                      positions
//   - mixed_tokens:    a mix of literals, `⋯`, and `*` — the realistic case
//   - star_redos:      adversarial multi-`*` input (memoised path)
//
// Reports allocs/op so the hot literal/single-token shapes can be
// pinned to zero-allocation in a follow-up bench gate.
func BenchmarkCompareExecArgs(b *testing.B) {
	cases := []struct {
		name    string
		profile []string
		runtime []string
	}{
		{
			name:    "literal_match",
			profile: []string{"/usr/sbin/apache2", "-DFOREGROUND"},
			runtime: []string{"/usr/sbin/apache2", "-DFOREGROUND"},
		},
		{
			name:    "literal_mismatch",
			profile: []string{"/usr/sbin/apache2", "-DFOREGROUND"},
			runtime: []string{"/usr/sbin/apache2", "-DBACKGROUND"},
		},
		{
			name:    "ellipsis_one",
			profile: []string{"--user", dynamicpathdetector.DynamicIdentifier, "--port", "8080"},
			runtime: []string{"--user", "alice", "--port", "8080"},
		},
		{
			name:    "star_trailing",
			profile: []string{"/usr/bin/curl", dynamicpathdetector.WildcardIdentifier},
			runtime: []string{"/usr/bin/curl", "-sS", "-o", "/tmp/out", "https://api.example.com"},
		},
		{
			name: "mixed_tokens",
			profile: []string{
				"/usr/bin/pg_dump",
				"-h", dynamicpathdetector.DynamicIdentifier,
				"-U", "postgres",
				dynamicpathdetector.WildcardIdentifier,
			},
			runtime: []string{
				"/usr/bin/pg_dump",
				"-h", "db.internal",
				"-U", "postgres",
				"-d", "main",
				"-f", "/tmp/main.sql",
			},
		},
		{
			name: "star_redos_adversarial",
			profile: []string{
				dynamicpathdetector.WildcardIdentifier,
				dynamicpathdetector.WildcardIdentifier,
				dynamicpathdetector.WildcardIdentifier,
				dynamicpathdetector.WildcardIdentifier,
				"sentinel",
			},
			runtime: []string{"a", "b", "c", "d", "e", "f", "g", "h", "sentinel"},
		},
	}
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_ = dynamicpathdetector.CompareExecArgs(c.profile, c.runtime)
			}
		})
	}
}
