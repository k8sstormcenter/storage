package dynamicpathdetectortests

import (
	"fmt"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	types "github.com/kubescape/storage/pkg/apis/softwarecomposition"
	dp "github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The shape found in production: nginx proxy_temp derives its directory from a
// counter, so the whole path advances together and no part of it repeats.
// Exactly one temp file landed inside the learning window, so the completed
// profile froze that single literal — and because a completed profile admits
// nothing further, the running container went on writing 0000000002 and beyond
// against an allowlist that can never match them.
func TestNginxProxyTempCounterCollapses(t *testing.T) {
	for _, tc := range []struct{ path, want string }{
		{"/tmp/nginx/proxy_temp/1/00/0000000001", "/tmp/nginx/proxy_temp/1/00/" + dynSeg},
		{"/tmp/nginx/proxy_temp/2/00/0000000002", "/tmp/nginx/proxy_temp/2/00/" + dynSeg},
		{"/var/lib/nginx/fastcgi_temp/9/99/0000009999", "/var/lib/nginx/fastcgi_temp/9/99/" + dynSeg},
	} {
		t.Run(tc.path, func(t *testing.T) {
			assert.Equal(t, tc.want, firstSight(t, tc.path))
		})
	}
}

// A profile that learned one temp write must end up holding a pattern that
// matches the writes that come after it. This is the property the frozen
// literal broke, stated directly rather than as a shape comparison.
func TestFrozenCounterPatternMatchesLaterWrites(t *testing.T) {
	learned := firstSight(t, "/tmp/nginx/proxy_temp/1/00/0000000001")

	for i := 2; i <= 6; i++ {
		later := fmt.Sprintf("/tmp/nginx/proxy_temp/1/00/%010d", i)
		assert.True(t, dp.CompareDynamic(learned, later),
			"a profile holding %q must match the container's next temp write %q", learned, later)
	}
	// The cost, stated rather than hidden: ⋯ matches ANY single segment, so
	// collapsing the counter does admit any filename directly in that temp
	// directory. That is the standing trade for every ⋯ in a profile, and it
	// buys away a guaranteed false positive on every temp write after the
	// first. It is bounded in the two ways that matter — the pattern stays
	// pinned to this directory and to one segment.
	assert.True(t, dp.CompareDynamic(learned, "/tmp/nginx/proxy_temp/1/00/malicious"),
		"one segment in the same directory is admitted - the accepted cost of the collapse")
	assert.False(t, dp.CompareDynamic(learned, "/tmp/nginx/proxy_temp/1/00/0000000001/deeper"),
		"must not reach deeper: ⋯ is one segment, not a subtree")
	assert.False(t, dp.CompareDynamic(learned, "/tmp/nginx/proxy_temp/2/00/0000000002"),
		"must not reach a sibling directory")
	assert.False(t, dp.CompareDynamic(learned, "/etc/passwd"))
}

// The padding is the whole discriminator. A meaningful integer is never
// written 0000000001, and every one of these must survive untouched — this is
// what keeps the rule safe to apply outside /proc.
func TestMeaningfulIntegersSurvive(t *testing.T) {
	for _, path := range []string{
		"/data/pods/0/restart/3",     // ordinals and restart indices
		"/var/log/app/1",             // rotation suffix
		"/var/log/app/2",             //
		"/etc/rc0.d/S01service",      // not digits-only
		"/opt/app/007/config",        // padded but short: not a generated counter
		"/opt/app/0/config",          // a bare zero is an ordinal
		"/opt/app/00/config",         // still too short to be a sequence number
		"/srv/2026/09/10/report.txt", // dates are meaningful and unpadded
		"/mnt/disk1/1234/data",       // four digits, but no leading zero
	} {
		t.Run(path, func(t *testing.T) {
			assert.Equal(t, path, firstSight(t, path), "collapsed a meaningful integer")
		})
	}
}

// The boundary case, stated on its own because it is the one a future change
// is most likely to get wrong: four digits with a leading zero collapses,
// three does not.
func TestCounterLengthBoundary(t *testing.T) {
	assert.Equal(t, "/x/"+dynSeg, firstSight(t, "/x/0001"), "four padded digits is a counter")
	assert.Equal(t, "/x/001", firstSight(t, "/x/001"), "three digits stays literal")
	assert.Equal(t, "/x/1000", firstSight(t, "/x/1000"), "no leading zero stays literal")
}

// At profile scope: many temp writes fold to one entry rather than a tail of
// one-shot literals, and their flags merge.
func TestProxyTempWritesFoldToOneEntry(t *testing.T) {
	var opens []types.OpenCalls
	for i := 1; i <= 40; i++ {
		opens = append(opens, types.OpenCalls{
			Path:  fmt.Sprintf("/tmp/nginx/proxy_temp/1/00/%010d", i),
			Flags: []string{"O_WRONLY", "O_CREAT"},
		})
	}
	opens = append(opens, types.OpenCalls{Path: "/etc/nginx/nginx.conf", Flags: []string{"O_RDONLY"}})

	got, err := dp.AnalyzeOpens(opens, dp.NewPathAnalyzer(dp.OpenDynamicThreshold), nil)
	require.NoError(t, err)

	paths := mapset.NewThreadUnsafeSet[string]()
	for _, o := range got {
		paths.Add(o.Path)
	}
	assert.ElementsMatch(t, []string{
		"/etc/nginx/nginx.conf",
		"/tmp/nginx/proxy_temp/1/00/" + dynSeg,
	}, paths.ToSlice(), "40 temp writes must fold to one pattern, config read untouched")

	for _, o := range got {
		if o.Path == "/tmp/nginx/proxy_temp/1/00/"+dynSeg {
			assert.ElementsMatch(t, []string{"O_CREAT", "O_WRONLY"}, o.Flags, "flags of folded entries must merge")
		}
	}
}

// Re-analysing an emitted pattern must not move it: storage re-reads stored
// paths on every save.
func TestCounterCollapseIsIdempotent(t *testing.T) {
	for _, path := range []string{
		"/tmp/nginx/proxy_temp/1/00/0000000001",
		"/opt/app/0/config",
		"/var/log/app/1",
	} {
		t.Run(path, func(t *testing.T) {
			once := firstSight(t, path)
			assert.Equal(t, once, firstSight(t, once))
			assert.Equal(t, once, firstSight(t, firstSight(t, once)))
		})
	}
}
