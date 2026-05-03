/*
Copyright 2024 The Kubescape Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package file

import (
	"fmt"
	"testing"

	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/config"
	"github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"github.com/stretchr/testify/assert"
)

// TestContainerProfileProcessor_CollapseSettings_NilProviderFallsBack pins
// that PreSave doesn't panic when CollapseSettings is nil — the runtime
// equivalent of "the cluster has no CollapseConfiguration CR yet".
// The struct's CollapseSettings field is exported, so external callers
// can leave it unset; PreSave must handle that.
func TestContainerProfileProcessor_CollapseSettings_NilProviderFallsBack(t *testing.T) {
	c := NewContainerProfileProcessor(config.Config{
		DefaultNamespace:          "kubescape",
		MaxApplicationProfileSize: 40000,
	}, nil)
	// Force the field nil to simulate an external caller that bypassed the
	// constructor's defaulting (e.g. zero-value struct literal).
	c.CollapseSettings = nil

	// Direct deflate exercises the same path PreSave uses and must produce
	// a sensible result with default settings.
	spec := softwarecomposition.ContainerProfileSpec{
		Architectures: []string{"amd64"},
	}
	result := DeflateContainerProfileSpec(spec, nil, dynamicpathdetector.DefaultCollapseSettings())
	assert.Equal(t, []string{"amd64"}, result.Architectures)
}

// TestContainerProfileProcessor_CustomCollapseSettings_ReachDeflate pins
// that a custom CollapseSettings provider's threshold actually reaches
// the deflate path, end-to-end through the public DeflateContainerProfileSpec
// function. We build a spec with 4 /etc children and assert that with a
// threshold-3 override they collapse, while with the default (100) they
// stay distinct.
func TestContainerProfileProcessor_CustomCollapseSettings_ReachDeflate(t *testing.T) {
	custom := dynamicpathdetector.CollapseSettings{
		OpenDynamicThreshold:     50,
		EndpointDynamicThreshold: 100,
		CollapseConfigs: []dynamicpathdetector.CollapseConfig{
			{Prefix: "/etc", Threshold: 3},
		},
	}

	spec := softwarecomposition.ContainerProfileSpec{}
	for i := 0; i < 4; i++ {
		spec.Opens = append(spec.Opens, softwarecomposition.OpenCalls{
			Path:  fmt.Sprintf("/etc/file%d", i),
			Flags: []string{"O_RDONLY"},
		})
	}

	defResult := DeflateContainerProfileSpec(spec, nil, dynamicpathdetector.DefaultCollapseSettings())
	assert.Greater(t, len(defResult.Opens), 1, "default threshold 100: four /etc files should NOT collapse")

	customResult := DeflateContainerProfileSpec(spec, nil, custom)
	collapsed := false
	for _, o := range customResult.Opens {
		if o.Path == "/etc/"+dynamicpathdetector.DynamicIdentifier {
			collapsed = true
			break
		}
	}
	assert.True(t, collapsed, "custom threshold 3: four /etc files MUST collapse to /etc/⋯")
}

// TestContainerProfileProcessor_DefaultConstructorWiresProvider pins the
// constructor contract — a freshly-constructed processor must have a
// non-nil CollapseSettings provider that returns the compiled defaults.
func TestContainerProfileProcessor_DefaultConstructorWiresProvider(t *testing.T) {
	c := NewContainerProfileProcessor(config.Config{
		DefaultNamespace:          "kubescape",
		MaxApplicationProfileSize: 40000,
	}, nil)
	assert.NotNil(t, c.CollapseSettings, "constructor must wire a default provider")
	got := c.CollapseSettings()
	want := dynamicpathdetector.DefaultCollapseSettings()
	assert.Equal(t, want.OpenDynamicThreshold, got.OpenDynamicThreshold)
	assert.Equal(t, want.EndpointDynamicThreshold, got.EndpointDynamicThreshold)
	assert.Equal(t, len(want.CollapseConfigs), len(got.CollapseConfigs))
}
