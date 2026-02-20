package dynamicpathdetector

import (
	"encoding/json"
	"fmt"
	"sync/atomic"
)

// CollapseSettings holds the runtime-tunable collapse configuration.
// It is read on every PreSave via CollapseConfigProvider.Get().
type CollapseSettings struct {
	CollapseConfigs          []CollapseConfig `json:"collapseConfigs"`
	OpenDynamicThreshold     int              `json:"openDynamicThreshold"`
	EndpointDynamicThreshold int              `json:"endpointDynamicThreshold"`
}

// CollapseConfigProvider provides lock-free access to the current CollapseSettings.
// It is safe for concurrent use: reads via Get() and writes via Update() use atomic.Pointer.
type CollapseConfigProvider struct {
	settings atomic.Pointer[CollapseSettings]
}

// NewCollapseConfigProvider returns a provider initialized with DefaultCollapseSettings().
func NewCollapseConfigProvider() *CollapseConfigProvider {
	p := &CollapseConfigProvider{}
	defaults := DefaultCollapseSettings()
	p.settings.Store(&defaults)
	return p
}

// Get returns the current CollapseSettings. Lock-free; safe for concurrent calls.
func (p *CollapseConfigProvider) Get() CollapseSettings {
	return *p.settings.Load()
}

// Update replaces the current settings atomically.
func (p *CollapseConfigProvider) Update(s CollapseSettings) {
	p.settings.Store(&s)
}

// DefaultCollapseSettings returns a CollapseSettings populated from the
// package-level constants and DefaultCollapseConfigs.
func DefaultCollapseSettings() CollapseSettings {
	configs := make([]CollapseConfig, len(DefaultCollapseConfigs))
	copy(configs, DefaultCollapseConfigs)
	return CollapseSettings{
		CollapseConfigs:          configs,
		OpenDynamicThreshold:     OpenDynamicThreshold,
		EndpointDynamicThreshold: EndpointDynamicThreshold,
	}
}

// ParseCollapseSettings parses JSON into CollapseSettings, falling back to
// defaults for any zero-value fields. Returns the settings and any parse error.
func ParseCollapseSettings(data []byte) (CollapseSettings, error) {
	defaults := DefaultCollapseSettings()
	if len(data) == 0 {
		return defaults, nil
	}

	var s CollapseSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return defaults, fmt.Errorf("parse collapse settings: %w", err)
	}

	// Fall back to defaults for zero-value fields
	if s.OpenDynamicThreshold == 0 {
		s.OpenDynamicThreshold = defaults.OpenDynamicThreshold
	}
	if s.EndpointDynamicThreshold == 0 {
		s.EndpointDynamicThreshold = defaults.EndpointDynamicThreshold
	}
	if s.CollapseConfigs == nil {
		s.CollapseConfigs = defaults.CollapseConfigs
	}

	return s, nil
}
