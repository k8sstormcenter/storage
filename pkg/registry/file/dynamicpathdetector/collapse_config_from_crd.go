package dynamicpathdetector

import (
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CollapseSettings holds all collapse thresholds in one place.
type CollapseSettings struct {
	OpenDynamicThreshold     int
	EndpointDynamicThreshold int
	CollapseConfigs          []CollapseConfig
}

// DefaultCollapseSettings returns the built-in defaults.
func DefaultCollapseSettings() CollapseSettings {
	return CollapseSettings{
		OpenDynamicThreshold:     OpenDynamicThreshold,
		EndpointDynamicThreshold: EndpointDynamicThreshold,
		CollapseConfigs:          append([]CollapseConfig{}, DefaultCollapseConfigs...),
	}
}

// CollapseSettingsFromCRD converts a CollapseConfiguration CRD into CollapseSettings.
func CollapseSettingsFromCRD(crd *softwarecomposition.CollapseConfiguration) CollapseSettings {
	configs := make([]CollapseConfig, len(crd.Spec.CollapseConfigs))
	for i, entry := range crd.Spec.CollapseConfigs {
		configs[i] = CollapseConfig{
			Prefix:    entry.Prefix,
			Threshold: entry.Threshold,
		}
	}
	return CollapseSettings{
		OpenDynamicThreshold:     crd.Spec.OpenDynamicThreshold,
		EndpointDynamicThreshold: crd.Spec.EndpointDynamicThreshold,
		CollapseConfigs:          configs,
	}
}

// CRDFromCollapseSettings converts CollapseSettings back into a CollapseConfiguration CRD.
func CRDFromCollapseSettings(name string, settings CollapseSettings) *softwarecomposition.CollapseConfiguration {
	entries := make([]softwarecomposition.CollapseConfigEntry, len(settings.CollapseConfigs))
	for i, cfg := range settings.CollapseConfigs {
		entries[i] = softwarecomposition.CollapseConfigEntry{
			Prefix:    cfg.Prefix,
			Threshold: cfg.Threshold,
		}
	}
	return &softwarecomposition.CollapseConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: softwarecomposition.CollapseConfigurationSpec{
			OpenDynamicThreshold:     settings.OpenDynamicThreshold,
			EndpointDynamicThreshold: settings.EndpointDynamicThreshold,
			CollapseConfigs:          entries,
		},
	}
}
