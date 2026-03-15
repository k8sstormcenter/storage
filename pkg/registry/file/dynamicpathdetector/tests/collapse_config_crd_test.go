package dynamicpathdetectortests

import (
	"testing"

	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDefaultCollapseSettings(t *testing.T) {
	s := dynamicpathdetector.DefaultCollapseSettings()

	assert.Equal(t, dynamicpathdetector.OpenDynamicThreshold, s.OpenDynamicThreshold)
	assert.Equal(t, dynamicpathdetector.EndpointDynamicThreshold, s.EndpointDynamicThreshold)
	assert.Equal(t, len(dynamicpathdetector.DefaultCollapseConfigs), len(s.CollapseConfigs))

	// Modifying returned slice must not affect the package-level defaults
	s.CollapseConfigs[0].Threshold = 999
	assert.NotEqual(t, 999, dynamicpathdetector.DefaultCollapseConfigs[0].Threshold,
		"DefaultCollapseSettings must return a copy, not a reference")
}

func TestCollapseSettingsFromCRD(t *testing.T) {
	crd := &softwarecomposition.CollapseConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config"},
		Spec: softwarecomposition.CollapseConfigurationSpec{
			OpenDynamicThreshold:     42,
			EndpointDynamicThreshold: 200,
			CollapseConfigs: []softwarecomposition.CollapseConfigEntry{
				{Prefix: "/etc", Threshold: 100},
				{Prefix: "/opt", Threshold: 5},
			},
		},
	}

	s := dynamicpathdetector.CollapseSettingsFromCRD(crd)

	assert.Equal(t, 42, s.OpenDynamicThreshold)
	assert.Equal(t, 200, s.EndpointDynamicThreshold)
	require.Len(t, s.CollapseConfigs, 2)
	assert.Equal(t, "/etc", s.CollapseConfigs[0].Prefix)
	assert.Equal(t, 100, s.CollapseConfigs[0].Threshold)
	assert.Equal(t, "/opt", s.CollapseConfigs[1].Prefix)
	assert.Equal(t, 5, s.CollapseConfigs[1].Threshold)
}

func TestCollapseSettingsFromCRD_EmptyConfigs(t *testing.T) {
	crd := &softwarecomposition.CollapseConfiguration{
		Spec: softwarecomposition.CollapseConfigurationSpec{
			OpenDynamicThreshold:     10,
			EndpointDynamicThreshold: 20,
			CollapseConfigs:          nil,
		},
	}

	s := dynamicpathdetector.CollapseSettingsFromCRD(crd)

	assert.Equal(t, 10, s.OpenDynamicThreshold)
	assert.Equal(t, 20, s.EndpointDynamicThreshold)
	assert.Empty(t, s.CollapseConfigs)
}

func TestCRDFromCollapseSettings(t *testing.T) {
	settings := dynamicpathdetector.CollapseSettings{
		OpenDynamicThreshold:     50,
		EndpointDynamicThreshold: 100,
		CollapseConfigs: []dynamicpathdetector.CollapseConfig{
			{Prefix: "/etc", Threshold: 100},
			{Prefix: "/var/run", Threshold: 3},
		},
	}

	crd := dynamicpathdetector.CRDFromCollapseSettings("my-config", settings)

	assert.Equal(t, "my-config", crd.Name)
	assert.Equal(t, 50, crd.Spec.OpenDynamicThreshold)
	assert.Equal(t, 100, crd.Spec.EndpointDynamicThreshold)
	require.Len(t, crd.Spec.CollapseConfigs, 2)
	assert.Equal(t, "/etc", crd.Spec.CollapseConfigs[0].Prefix)
	assert.Equal(t, 100, crd.Spec.CollapseConfigs[0].Threshold)
	assert.Equal(t, "/var/run", crd.Spec.CollapseConfigs[1].Prefix)
	assert.Equal(t, 3, crd.Spec.CollapseConfigs[1].Threshold)
}

func TestRoundTrip_DefaultSettings(t *testing.T) {
	// Default → CRD → Settings must be identical to the original
	original := dynamicpathdetector.DefaultCollapseSettings()
	crd := dynamicpathdetector.CRDFromCollapseSettings("default", original)
	roundTripped := dynamicpathdetector.CollapseSettingsFromCRD(crd)

	assert.Equal(t, original.OpenDynamicThreshold, roundTripped.OpenDynamicThreshold)
	assert.Equal(t, original.EndpointDynamicThreshold, roundTripped.EndpointDynamicThreshold)
	require.Equal(t, len(original.CollapseConfigs), len(roundTripped.CollapseConfigs))
	for i := range original.CollapseConfigs {
		assert.Equal(t, original.CollapseConfigs[i].Prefix, roundTripped.CollapseConfigs[i].Prefix)
		assert.Equal(t, original.CollapseConfigs[i].Threshold, roundTripped.CollapseConfigs[i].Threshold)
	}
}

func TestRoundTrip_CustomSettings(t *testing.T) {
	custom := dynamicpathdetector.CollapseSettings{
		OpenDynamicThreshold:     7,
		EndpointDynamicThreshold: 15,
		CollapseConfigs: []dynamicpathdetector.CollapseConfig{
			{Prefix: "/app/data", Threshold: 2},
			{Prefix: "/tmp", Threshold: 50},
			{Prefix: "/usr/lib", Threshold: 200},
		},
	}
	crd := dynamicpathdetector.CRDFromCollapseSettings("custom", custom)
	roundTripped := dynamicpathdetector.CollapseSettingsFromCRD(crd)

	assert.Equal(t, custom.OpenDynamicThreshold, roundTripped.OpenDynamicThreshold)
	assert.Equal(t, custom.EndpointDynamicThreshold, roundTripped.EndpointDynamicThreshold)
	require.Equal(t, len(custom.CollapseConfigs), len(roundTripped.CollapseConfigs))
	for i := range custom.CollapseConfigs {
		assert.Equal(t, custom.CollapseConfigs[i].Prefix, roundTripped.CollapseConfigs[i].Prefix)
		assert.Equal(t, custom.CollapseConfigs[i].Threshold, roundTripped.CollapseConfigs[i].Threshold)
	}
}

// TestCollapseSettingsProduceWorkingAnalyzer verifies that settings from a CRD
// create a functional PathAnalyzer that respects the configured thresholds.
func TestCollapseSettingsProduceWorkingAnalyzer(t *testing.T) {
	crd := &softwarecomposition.CollapseConfiguration{
		Spec: softwarecomposition.CollapseConfigurationSpec{
			OpenDynamicThreshold:     3,
			EndpointDynamicThreshold: 100,
			CollapseConfigs: []softwarecomposition.CollapseConfigEntry{
				{Prefix: "/etc", Threshold: 2},
			},
		},
	}

	s := dynamicpathdetector.CollapseSettingsFromCRD(crd)
	analyzer := dynamicpathdetector.NewPathAnalyzerWithConfigs(s.OpenDynamicThreshold, s.CollapseConfigs)

	// Add paths under /etc — threshold is 2, so 3 children should collapse
	analyzer.AddPath("/etc/hosts")
	analyzer.AddPath("/etc/resolv.conf")
	analyzer.AddPath("/etc/passwd")

	paths := analyzer.GetStoredPaths()
	// After collapse, /etc/ children should be a single dynamic node
	hasDynamic := false
	for _, p := range paths {
		if p == "/etc/\u22ef" || p == "/etc/*" {
			hasDynamic = true
		}
	}
	assert.True(t, hasDynamic, "3 children under /etc with threshold=2 should collapse, got: %v", paths)
}

// TestCollapseSettingsHighThresholdNoCollapse verifies that a high threshold
// prevents collapsing.
func TestCollapseSettingsHighThresholdNoCollapse(t *testing.T) {
	crd := &softwarecomposition.CollapseConfiguration{
		Spec: softwarecomposition.CollapseConfigurationSpec{
			OpenDynamicThreshold:     1000,
			EndpointDynamicThreshold: 1000,
			CollapseConfigs:          nil,
		},
	}

	s := dynamicpathdetector.CollapseSettingsFromCRD(crd)
	analyzer := dynamicpathdetector.NewPathAnalyzerWithConfigs(s.OpenDynamicThreshold, s.CollapseConfigs)

	// Add 10 paths — way below threshold of 1000
	for i := 0; i < 10; i++ {
		analyzer.AddPath("/usr/lib/libfoo" + string(rune('a'+i)) + ".so")
	}

	paths := analyzer.GetStoredPaths()
	assert.Equal(t, 10, len(paths), "10 paths below threshold=1000 should not collapse, got: %v", paths)
}
