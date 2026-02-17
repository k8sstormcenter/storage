package dynamicpathdetectortests

import (
	"sync"
	"testing"

	"github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultCollapseSettings(t *testing.T) {
	s := dynamicpathdetector.DefaultCollapseSettings()
	assert.Equal(t, dynamicpathdetector.OpenDynamicThreshold, s.OpenDynamicThreshold)
	assert.Equal(t, dynamicpathdetector.EndpointDynamicThreshold, s.EndpointDynamicThreshold)
	assert.Equal(t, dynamicpathdetector.DefaultCollapseConfigs, s.CollapseConfigs)
}

func TestParseCollapseSettings_Empty(t *testing.T) {
	s, err := dynamicpathdetector.ParseCollapseSettings(nil)
	require.NoError(t, err)
	assert.Equal(t, dynamicpathdetector.DefaultCollapseSettings(), s)

	s, err = dynamicpathdetector.ParseCollapseSettings([]byte{})
	require.NoError(t, err)
	assert.Equal(t, dynamicpathdetector.DefaultCollapseSettings(), s)
}

func TestParseCollapseSettings_Partial(t *testing.T) {
	// Only set openDynamicThreshold — others should get defaults
	data := []byte(`{"openDynamicThreshold": 99}`)
	s, err := dynamicpathdetector.ParseCollapseSettings(data)
	require.NoError(t, err)
	assert.Equal(t, 99, s.OpenDynamicThreshold)
	assert.Equal(t, dynamicpathdetector.EndpointDynamicThreshold, s.EndpointDynamicThreshold)
	assert.Equal(t, dynamicpathdetector.DefaultCollapseConfigs, s.CollapseConfigs)
}

func TestParseCollapseSettings_Full(t *testing.T) {
	data := []byte(`{
		"openDynamicThreshold": 10,
		"endpointDynamicThreshold": 20,
		"collapseConfigs": [
			{"prefix": "/tmp", "threshold": 3}
		]
	}`)
	s, err := dynamicpathdetector.ParseCollapseSettings(data)
	require.NoError(t, err)
	assert.Equal(t, 10, s.OpenDynamicThreshold)
	assert.Equal(t, 20, s.EndpointDynamicThreshold)
	require.Len(t, s.CollapseConfigs, 1)
	assert.Equal(t, "/tmp", s.CollapseConfigs[0].Prefix)
	assert.Equal(t, 3, s.CollapseConfigs[0].Threshold)
}

func TestParseCollapseSettings_InvalidJSON(t *testing.T) {
	data := []byte(`{invalid json}`)
	s, err := dynamicpathdetector.ParseCollapseSettings(data)
	assert.Error(t, err)
	// Should return defaults on error
	assert.Equal(t, dynamicpathdetector.DefaultCollapseSettings(), s)
}

func TestCollapseConfigProvider_GetAfterUpdate(t *testing.T) {
	p := dynamicpathdetector.NewCollapseConfigProvider()

	// Verify defaults
	s := p.Get()
	assert.Equal(t, dynamicpathdetector.OpenDynamicThreshold, s.OpenDynamicThreshold)

	// Update and verify
	custom := dynamicpathdetector.CollapseSettings{
		OpenDynamicThreshold:     7,
		EndpointDynamicThreshold: 14,
		CollapseConfigs: []dynamicpathdetector.CollapseConfig{
			{Prefix: "/foo", Threshold: 2},
		},
	}
	p.Update(custom)

	s = p.Get()
	assert.Equal(t, 7, s.OpenDynamicThreshold)
	assert.Equal(t, 14, s.EndpointDynamicThreshold)
	require.Len(t, s.CollapseConfigs, 1)
	assert.Equal(t, "/foo", s.CollapseConfigs[0].Prefix)
}

func TestCollapseConfigProvider_ConcurrentAccess(t *testing.T) {
	p := dynamicpathdetector.NewCollapseConfigProvider()

	var wg sync.WaitGroup
	// Readers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				s := p.Get()
				_ = s.OpenDynamicThreshold
			}
		}()
	}
	// Writer
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 1000; j++ {
			p.Update(dynamicpathdetector.CollapseSettings{
				OpenDynamicThreshold:     j,
				EndpointDynamicThreshold: j * 2,
				CollapseConfigs:          dynamicpathdetector.DefaultCollapseConfigs,
			})
		}
	}()

	wg.Wait()
	// If we get here without a data race, the test passes
}
