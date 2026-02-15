package file

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition/consts"
	"github.com/kubescape/storage/pkg/config"
	"github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// openThreshold returns the collapse threshold used by deflateApplicationProfileContainer
// for file-open paths. NewPathAnalyzer uses a uniform threshold (OpenDynamicThreshold).
func openThreshold() int {
	return dynamicpathdetector.OpenDynamicThreshold
}

var ap = softwarecomposition.ApplicationProfile{
	ObjectMeta: v1.ObjectMeta{
		Annotations: map[string]string{},
	},
	Spec: softwarecomposition.ApplicationProfileSpec{
		Architectures: []string{"amd64", "arm64", "amd64"},
		EphemeralContainers: []softwarecomposition.ApplicationProfileContainer{
			{
				Name: "ephemeralContainer",
				Execs: []softwarecomposition.ExecCalls{
					{Path: "/bin/bash", Args: []string{"-c", "echo abc"}},
				},
			},
		},
		InitContainers: []softwarecomposition.ApplicationProfileContainer{
			{
				Name: "initContainer",
				Execs: []softwarecomposition.ExecCalls{
					{Path: "/bin/bash", Args: []string{"-c", "echo hello"}},
				},
			},
		},
		Containers: []softwarecomposition.ApplicationProfileContainer{
			{
				Name: "container1",
				Execs: []softwarecomposition.ExecCalls{
					{Path: "/usr/bin/ls", Args: []string{"-l", "/tmp"}},
					{Path: "/usr/bin/ls", Args: []string{"-l", "/home"}},
					{Path: "/usr/bin/ls", Args: []string{"-l", "/tmp"}},
				},
			},
			{
				Name: "container2",
				Execs: []softwarecomposition.ExecCalls{
					{Path: "/usr/bin/ping", Args: []string{"localhost"}},
				},
				Opens: []softwarecomposition.OpenCalls{
					{Path: "/etc/hosts", Flags: []string{"O_CLOEXEC", "O_RDONLY"}},
				},
				Endpoints: []softwarecomposition.HTTPEndpoint{
					{
						Endpoint:  ":443/abc",
						Methods:   []string{"GET"},
						Internal:  false,
						Direction: consts.Inbound,
						Headers:   []byte{},
					},
				},
			},
		},
	},
}

func TestApplicationProfileProcessor_PreSave(t *testing.T) {
	tests := []struct {
		name                      string
		maxApplicationProfileSize int
		object                    runtime.Object
		want                      runtime.Object
		wantErr                   assert.ErrorAssertionFunc
	}{
		{
			name:                      "ApplicationProfile with initContainers and ephemeralContainers",
			maxApplicationProfileSize: 40000,
			object:                    &ap,
			want: &softwarecomposition.ApplicationProfile{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						helpers.ResourceSizeMetadataKey: "7",
					},
				},
				SchemaVersion: 1,
				Spec: softwarecomposition.ApplicationProfileSpec{
					Architectures: []string{"amd64", "arm64"},
					EphemeralContainers: []softwarecomposition.ApplicationProfileContainer{
						{
							Name: "ephemeralContainer",
							Execs: []softwarecomposition.ExecCalls{
								{Path: "/bin/bash", Args: []string{"-c", "echo abc"}},
							},
						},
					},
					InitContainers: []softwarecomposition.ApplicationProfileContainer{
						{
							Name: "initContainer",
							Execs: []softwarecomposition.ExecCalls{
								{Path: "/bin/bash", Args: []string{"-c", "echo hello"}},
							},
						},
					},
					Containers: []softwarecomposition.ApplicationProfileContainer{
						{
							Name: "container1",
							Execs: []softwarecomposition.ExecCalls{
								{Path: "/usr/bin/ls", Args: []string{"-l", "/home"}},
								{Path: "/usr/bin/ls", Args: []string{"-l", "/tmp"}},
							},
						},
						{
							Name: "container2",
							Execs: []softwarecomposition.ExecCalls{
								{Path: "/usr/bin/ping", Args: []string{"localhost"}},
							},
							Opens: []softwarecomposition.OpenCalls{
								{Path: "/etc/hosts", Flags: []string{"O_CLOEXEC", "O_RDONLY"}},
							},
							Endpoints: []softwarecomposition.HTTPEndpoint{
								{
									Endpoint:  ":443/abc",
									Methods:   []string{"GET"},
									Internal:  false,
									Direction: consts.Inbound,
									Headers:   []byte{},
								},
							},
						},
					},
				},
			},
			wantErr: assert.NoError,
		},
		{
			name:                      "ApplicationProfile too big",
			maxApplicationProfileSize: 5,
			object:                    &ap,
			want:                      &ap,
			wantErr:                   assert.Error,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := NewApplicationProfileProcessor(config.Config{DefaultNamespace: "kubescape", MaxApplicationProfileSize: tt.maxApplicationProfileSize})
			tt.wantErr(t, a.PreSave(context.TODO(), tt.object), fmt.Sprintf("PreSave(%v)", tt.object))
			slices.Sort(tt.object.(*softwarecomposition.ApplicationProfile).Spec.Architectures)
			assert.Equal(t, tt.want, tt.object)
		})
	}
}

func TestDeflateRulePolicies(t *testing.T) {
	tests := []struct {
		name string
		in   map[string]softwarecomposition.RulePolicy
		want map[string]softwarecomposition.RulePolicy
	}{
		{
			name: "nil map",
			in:   nil,
			want: nil,
		},
		{
			name: "empty map",
			in:   map[string]softwarecomposition.RulePolicy{},
			want: map[string]softwarecomposition.RulePolicy{},
		},
		{
			name: "single rule with unsorted processes",
			in: map[string]softwarecomposition.RulePolicy{
				"rule1": {
					AllowedProcesses: []string{"cat", "bash", "ls"},
					AllowedContainer: true,
				},
			},
			want: map[string]softwarecomposition.RulePolicy{
				"rule1": {
					AllowedProcesses: []string{"bash", "cat", "ls"},
					AllowedContainer: true,
				},
			},
		},
		{
			name: "multiple rules with duplicate processes",
			in: map[string]softwarecomposition.RulePolicy{
				"rule1": {
					AllowedProcesses: []string{"cat", "bash", "ls", "bash"},
					AllowedContainer: true,
				},
				"rule2": {
					AllowedProcesses: []string{"nginx", "nginx", "python"},
					AllowedContainer: false,
				},
			},
			want: map[string]softwarecomposition.RulePolicy{
				"rule1": {
					AllowedProcesses: []string{"bash", "cat", "ls"},
					AllowedContainer: true,
				},
				"rule2": {
					AllowedProcesses: []string{"nginx", "python"},
					AllowedContainer: false,
				},
			},
		},
		{
			name: "rule with empty processes",
			in: map[string]softwarecomposition.RulePolicy{
				"rule1": {
					AllowedProcesses: []string{},
					AllowedContainer: true,
				},
			},
			want: map[string]softwarecomposition.RulePolicy{
				"rule1": {
					AllowedProcesses: []string{},
					AllowedContainer: true,
				},
			},
		},
		{
			name: "rule with nil processes",
			in: map[string]softwarecomposition.RulePolicy{
				"rule1": {
					AllowedProcesses: nil,
					AllowedContainer: true,
				},
			},
			want: map[string]softwarecomposition.RulePolicy{
				"rule1": {
					AllowedProcesses: nil,
					AllowedContainer: true,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeflateRulePolicies(tt.in)
			assert.Equal(t, tt.want, got)
		})
	}
}

// ---------------------------------------------------------------------------
// Exec dedup + collapse through the actual deflateApplicationProfileContainer
// ---------------------------------------------------------------------------

// TestDeflateApplicationProfileContainer_ExecDedup verifies that exact-duplicate
// execs are removed by the deflate pipeline.
func TestDeflateApplicationProfileContainer_ExecDedup(t *testing.T) {
	container := softwarecomposition.ApplicationProfileContainer{
		Name: "test",
		Execs: []softwarecomposition.ExecCalls{
			{Path: "/usr/bin/ls", Args: []string{"-l", "/tmp"}},
			{Path: "/usr/bin/ls", Args: []string{"-l", "/tmp"}}, // exact dup
			{Path: "/usr/bin/ls", Args: []string{"-l", "/tmp"}}, // exact dup
			{Path: "/usr/bin/ls", Args: []string{"-l", "/home"}},
		},
	}

	result := deflateApplicationProfileContainer(container, nil)

	assert.Len(t, result.Execs, 2, "exact duplicates should be removed, got %v", result.Execs)
}

// TestDeflateApplicationProfileContainer_ExecCollapseApache uses the exact exec
// data from a real Apache container ApplicationProfile. The pipeline must:
//  1. Deduplicate exact duplicates
//  2. Collapse varying arg positions to ⋯
func TestDeflateApplicationProfileContainer_ExecCollapseApache(t *testing.T) {
	container := softwarecomposition.ApplicationProfileContainer{
		Name: "apache",
		Execs: []softwarecomposition.ExecCalls{
			// mkdir called with 3 different dirs (+ duplicates of each)
			{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/lock/apache2"}},
			{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/lock/apache2"}},
			{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/log/apache2"}},
			{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/log/apache2"}},
			{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/run/apache2"}},
			{Path: "/bin/mkdir", Args: []string{"/bin/mkdir", "-p", "/var/run/apache2"}},
			// rm called once
			{Path: "/bin/rm", Args: []string{"/bin/rm", "-f", "/var/run/apache2/apache2.pid"}},
			{Path: "/bin/rm", Args: []string{"/bin/rm", "-f", "/var/run/apache2/apache2.pid"}},
			// dirname called with 3 different dirs (+ duplicates)
			{Path: "/usr/bin/dirname", Args: []string{"/usr/bin/dirname", "/var/lock/apache2"}},
			{Path: "/usr/bin/dirname", Args: []string{"/usr/bin/dirname", "/var/lock/apache2"}},
			{Path: "/usr/bin/dirname", Args: []string{"/usr/bin/dirname", "/var/log/apache2"}},
			{Path: "/usr/bin/dirname", Args: []string{"/usr/bin/dirname", "/var/log/apache2"}},
			{Path: "/usr/bin/dirname", Args: []string{"/usr/bin/dirname", "/var/run/apache2"}},
		},
	}

	result := deflateApplicationProfileContainer(container, nil)

	// After dedup: 7 unique. After collapse (threshold=10, 3 variants per binary):
	// With threshold=10 and only 3 variants, collapsing does NOT kick in.
	// This test documents the current behavior at ExecArgDynamicThreshold.
	t.Logf("ExecArgDynamicThreshold=%d", dynamicpathdetector.ExecArgDynamicThreshold)
	t.Logf("result execs (%d):", len(result.Execs))
	for _, e := range result.Execs {
		t.Logf("  path=%s args=%v", e.Path, e.Args)
	}

	// Duplicates must always be removed regardless of threshold
	assert.Less(t, len(result.Execs), len(container.Execs),
		"duplicates were not removed")

	// After dedup we expect 7 unique entries
	// (3 mkdir + 1 rm + 3 dirname)
	// Whether they further collapse depends on the threshold
	if dynamicpathdetector.ExecArgDynamicThreshold < 3 {
		// Collapsing kicks in: 3 variants > threshold
		assert.Len(t, result.Execs, 3,
			"with threshold < 3, mkdir/dirname variants should collapse")

		for _, e := range result.Execs {
			if e.Path == "/bin/mkdir" {
				assert.Equal(t, []string{"/bin/mkdir", "-p", dynamicpathdetector.DynamicIdentifier}, e.Args)
			}
			if e.Path == "/usr/bin/dirname" {
				assert.Equal(t, []string{"/usr/bin/dirname", dynamicpathdetector.DynamicIdentifier}, e.Args)
			}
			if e.Path == "/bin/rm" {
				assert.Equal(t, []string{"/bin/rm", "-f", "/var/run/apache2/apache2.pid"}, e.Args)
			}
		}
	} else {
		// Threshold too high for 3 variants — only dedup, no collapse
		assert.Len(t, result.Execs, 7,
			"with threshold >= 3, 3 variants should not collapse (only dedup)")
	}
}

// TestDeflateApplicationProfileContainer_ExecCollapseHighVariability generates
// enough exec variants to exceed ExecArgDynamicThreshold and verifies collapsing.
func TestDeflateApplicationProfileContainer_ExecCollapseHighVariability(t *testing.T) {
	n := dynamicpathdetector.ExecArgDynamicThreshold + 1
	var execs []softwarecomposition.ExecCalls
	for i := 0; i < n; i++ {
		execs = append(execs, softwarecomposition.ExecCalls{
			Path: "/usr/bin/curl",
			Args: []string{"-s", fmt.Sprintf("http://service%d/api", i)},
		})
	}
	// Add duplicates
	execs = append(execs, execs[0], execs[1], execs[2])

	container := softwarecomposition.ApplicationProfileContainer{
		Name:  "curl-heavy",
		Execs: execs,
	}

	result := deflateApplicationProfileContainer(container, nil)

	// All entries share path=/usr/bin/curl, static arg "-s", varying URL
	assert.Len(t, result.Execs, 1,
		"all curl variants should collapse to one entry, got %v", result.Execs)
	assert.Equal(t, "/usr/bin/curl", result.Execs[0].Path)
	assert.Equal(t, []string{"-s", dynamicpathdetector.DynamicIdentifier}, result.Execs[0].Args,
		"static -s preserved, dynamic URL collapsed")
}

// TestDeflateApplicationProfileContainer_PreSaveExecEndToEnd runs the full
// PreSave path with exec data to verify dedup+collapse in the real pipeline.
func TestDeflateApplicationProfileContainer_PreSaveExecEndToEnd(t *testing.T) {
	n := dynamicpathdetector.ExecArgDynamicThreshold + 1
	var execs []softwarecomposition.ExecCalls
	for i := 0; i < n; i++ {
		// Each exec appears twice (exact dup)
		exec := softwarecomposition.ExecCalls{
			Path: "/usr/bin/wget",
			Args: []string{"/usr/bin/wget", "-q", fmt.Sprintf("http://backend%d:8080/health", i)},
		}
		execs = append(execs, exec, exec)
	}

	profile := &softwarecomposition.ApplicationProfile{
		ObjectMeta: v1.ObjectMeta{Annotations: map[string]string{}},
		Spec: softwarecomposition.ApplicationProfileSpec{
			Containers: []softwarecomposition.ApplicationProfileContainer{
				{Name: "sidecar", Execs: execs},
			},
		},
	}

	processor := NewApplicationProfileProcessor(config.Config{
		DefaultNamespace:          "kubescape",
		MaxApplicationProfileSize: 100000,
	})

	err := processor.PreSave(context.TODO(), profile)
	assert.NoError(t, err)

	resultExecs := profile.Spec.Containers[0].Execs
	t.Logf("input: %d execs, output: %d execs", len(execs), len(resultExecs))
	for _, e := range resultExecs {
		t.Logf("  path=%s args=%v", e.Path, e.Args)
	}

	assert.Len(t, resultExecs, 1,
		"all wget variants should dedup+collapse to 1 entry")
	assert.Equal(t, "/usr/bin/wget", resultExecs[0].Path)
	assert.Equal(t, []string{"/usr/bin/wget", "-q", dynamicpathdetector.DynamicIdentifier}, resultExecs[0].Args)
}

// generateSOOpens creates N unique .so OpenCalls under /usr/lib/x86_64-linux-gnu/
func generateSOOpens(n int) []softwarecomposition.OpenCalls {
	opens := make([]softwarecomposition.OpenCalls, n)
	for i := 0; i < n; i++ {
		opens[i] = softwarecomposition.OpenCalls{
			Path:  fmt.Sprintf("/usr/lib/x86_64-linux-gnu/lib%d.so.%d", i, i%5),
			Flags: []string{"O_RDONLY", "O_CLOEXEC"},
		}
	}
	return opens
}

func TestDeflateApplicationProfileContainer_CollapsesManyOpens(t *testing.T) {
	// Generate enough opens to exceed the uniform threshold used by NewPathAnalyzer
	numOpens := openThreshold() + 1
	opens := generateSOOpens(numOpens)

	container := softwarecomposition.ApplicationProfileContainer{
		Name:  "test-container",
		Opens: opens,
	}

	result := deflateApplicationProfileContainer(container, nil)

	assert.Less(t, len(result.Opens), numOpens,
		"%d .so files should be collapsed, got %d opens", numOpens, len(result.Opens))

	// Verify collapsed paths contain dynamic or wildcard segments
	for _, open := range result.Opens {
		if strings.HasPrefix(open.Path, "/usr/lib/x86_64-linux-gnu/") {
			assert.True(t,
				strings.Contains(open.Path, "\u22ef") || strings.Contains(open.Path, "*"),
				"path %q should contain a dynamic or wildcard segment", open.Path)
		}
	}

	// Flags should be preserved and merged
	for _, open := range result.Opens {
		assert.NotEmpty(t, open.Flags, "flags should be preserved after collapse")
	}
}

func TestDeflateApplicationProfileContainer_CollapsesWithSbomSet(t *testing.T) {
	numOpens := openThreshold() + 1
	opens := generateSOOpens(numOpens)

	// Build sbomSet containing ALL the .so paths (realistic scenario)
	sbomSet := mapset.NewSet[string]()
	for _, open := range opens {
		sbomSet.Add(open.Path)
	}

	container := softwarecomposition.ApplicationProfileContainer{
		Name:  "test-container",
		Opens: opens,
	}

	result := deflateApplicationProfileContainer(container, sbomSet)

	// Even though all paths are in SBOM, they should still be collapsed
	assert.Less(t, len(result.Opens), numOpens,
		"SBOM paths should be collapsed too, got %d opens", len(result.Opens))
}

func TestDeflateApplicationProfileContainer_MixedPathsCollapse(t *testing.T) {
	var opens []softwarecomposition.OpenCalls

	// /usr/lib uses the uniform threshold from NewPathAnalyzer(OpenDynamicThreshold)
	usrLibThreshold := openThreshold()
	for i := 0; i < usrLibThreshold+1; i++ {
		opens = append(opens, softwarecomposition.OpenCalls{
			Path:  fmt.Sprintf("/usr/lib/lib%d.so", i),
			Flags: []string{"O_RDONLY"},
		})
	}

	// /etc also uses the same uniform threshold
	etcThreshold := openThreshold()
	for i := 0; i < etcThreshold+1; i++ {
		opens = append(opens, softwarecomposition.OpenCalls{
			Path:  fmt.Sprintf("/etc/conf%d.cfg", i),
			Flags: []string{"O_RDONLY"},
		})
	}

	opens = append(opens,
		softwarecomposition.OpenCalls{Path: "/tmp/file1.txt", Flags: []string{"O_RDWR"}},
		softwarecomposition.OpenCalls{Path: "/tmp/file2.txt", Flags: []string{"O_RDWR"}},
	)

	container := softwarecomposition.ApplicationProfileContainer{
		Name:  "test-container",
		Opens: opens,
	}

	result := deflateApplicationProfileContainer(container, nil)

	// Count paths by prefix
	var usrLibPaths, etcPaths, tmpPaths int
	for _, open := range result.Opens {
		switch {
		case strings.HasPrefix(open.Path, "/usr/lib/"):
			usrLibPaths++
		case strings.HasPrefix(open.Path, "/etc/"):
			etcPaths++
		case strings.HasPrefix(open.Path, "/tmp/"):
			tmpPaths++
		}
	}

	assert.LessOrEqual(t, usrLibPaths, 1, "/usr/lib/ paths should collapse to 1, got %d", usrLibPaths)
	assert.LessOrEqual(t, etcPaths, 1, "/etc/ paths should collapse to 1, got %d", etcPaths)
	assert.Equal(t, 2, tmpPaths, "/tmp/ paths should remain individual (below threshold)")
}

// TestDeflateApplicationProfileContainer_NilSbomNoError verifies that nil sbomSet
// with a small number of opens (below threshold) works without error.
func TestDeflateApplicationProfileContainer_NilSbomNoError(t *testing.T) {
	container := softwarecomposition.ApplicationProfileContainer{
		Name: "test-container",
		Opens: []softwarecomposition.OpenCalls{
			{Path: "/etc/hosts", Flags: []string{"O_RDONLY"}},
			{Path: "/etc/resolv.conf", Flags: []string{"O_RDONLY"}},
			{Path: "/usr/lib/libc.so.6", Flags: []string{"O_RDONLY", "O_CLOEXEC"}},
		},
	}

	result := deflateApplicationProfileContainer(container, nil)

	// All 3 paths should remain (below any threshold)
	assert.Equal(t, 3, len(result.Opens), "paths below threshold should not collapse")
	// Paths should be sorted
	for i := 1; i < len(result.Opens); i++ {
		assert.True(t, result.Opens[i-1].Path <= result.Opens[i].Path,
			"opens should be sorted, got %q before %q", result.Opens[i-1].Path, result.Opens[i].Path)
	}
}

// TestDeflateApplicationProfileContainer_PreSaveEndToEnd verifies the full
// PreSave flow with an ApplicationProfile containing many opens that should collapse.
func TestDeflateApplicationProfileContainer_PreSaveEndToEnd(t *testing.T) {
	numOpens := openThreshold() + 1
	opens := generateSOOpens(numOpens)

	profile := &softwarecomposition.ApplicationProfile{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{},
		},
		Spec: softwarecomposition.ApplicationProfileSpec{
			Containers: []softwarecomposition.ApplicationProfileContainer{
				{
					Name:  "main",
					Opens: opens,
				},
			},
		},
	}

	processor := NewApplicationProfileProcessor(config.Config{
		DefaultNamespace:          "kubescape",
		MaxApplicationProfileSize: 100000,
	})

	err := processor.PreSave(context.TODO(), profile)
	assert.NoError(t, err)

	resultOpens := profile.Spec.Containers[0].Opens
	assert.Less(t, len(resultOpens), numOpens,
		"PreSave should collapse %d .so files, got %d opens", numOpens, len(resultOpens))

	// The collapsed path should contain dynamic or wildcard segments
	hasCollapsed := false
	for _, open := range resultOpens {
		if strings.Contains(open.Path, "\u22ef") || strings.Contains(open.Path, "*") {
			hasCollapsed = true
			break
		}
	}
	assert.True(t, hasCollapsed, "at least one path should contain a dynamic/wildcard segment after PreSave")
}
