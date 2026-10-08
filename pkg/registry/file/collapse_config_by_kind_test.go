package file

import (
	"testing"

	helpersv1 "github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/config"
	"github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func collapseCR(name string, open int32, prefix string, threshold int32) *softwarecomposition.CollapseConfiguration {
	return &softwarecomposition.CollapseConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: softwarecomposition.CollapseConfigurationSpec{
			OpenDynamicThreshold: open,
			CollapseConfigs:      []softwarecomposition.CollapseConfigEntry{{Prefix: prefix, Threshold: threshold}},
		},
	}
}

func TestCollapseSettingsForLabels_NodeProfileUsesNodeCR(t *testing.T) {
	s := &fakeCollapseStorage{stored: map[string]runtime.Object{
		collapseConfigurationKey(DefaultCollapseConfigurationName): collapseCR("default", 50, "/etc", 100),
		collapseConfigurationKey("node"):                           collapseCR("node", 50, "/sys/fs/cgroup", 1),
	}}
	settingsFor := NewCRDCollapseSettingsForProvider(s)

	node := settingsFor(map[string]string{helpersv1.ArtifactTypeMetadataKey: "node"})
	require.Len(t, node.CollapseConfigs, 1)
	assert.Equal(t, "/sys/fs/cgroup", node.CollapseConfigs[0].Prefix)

	container := settingsFor(map[string]string{"app": "nginx"})
	require.Len(t, container.CollapseConfigs, 1)
	assert.Equal(t, "/etc", container.CollapseConfigs[0].Prefix)

	assert.Equal(t, container, settingsFor(nil))
}

func TestCollapseSettingsForLabels_MissingKindCRFallsBackToDefault(t *testing.T) {
	s := &fakeCollapseStorage{stored: map[string]runtime.Object{
		collapseConfigurationKey(DefaultCollapseConfigurationName): collapseCR("default", 50, "/etc", 100),
	}}
	settingsFor := NewCRDCollapseSettingsForProvider(s)
	got := settingsFor(map[string]string{helpersv1.ArtifactTypeMetadataKey: "node"})
	require.Len(t, got.CollapseConfigs, 1)
	assert.Equal(t, "/etc", got.CollapseConfigs[0].Prefix)

	none := NewCRDCollapseSettingsForProvider(&fakeCollapseStorage{stored: map[string]runtime.Object{}})
	assert.Equal(t, dynamicpathdetector.DefaultCollapseSettings(), none(map[string]string{helpersv1.ArtifactTypeMetadataKey: "node"}))
	assert.Equal(t, dynamicpathdetector.DefaultCollapseSettings(), NewCRDCollapseSettingsForProvider(nil)(nil))
}

func TestPreSave_NodeProfileDeflatesWithNodeCR(t *testing.T) {
	c := NewContainerProfileProcessor(config.Config{DefaultNamespace: "kubescape", MaxContainerProfileSize: 40000}, nil)
	c.CollapseSettingsFor = func(labels map[string]string) dynamicpathdetector.CollapseSettings {
		if labels[helpersv1.ArtifactTypeMetadataKey] == "node" {
			return dynamicpathdetector.CollapseSettings{OpenDynamicThreshold: 50, EndpointDynamicThreshold: 100, CollapseConfigs: []dynamicpathdetector.CollapseConfig{{Prefix: "/sys/fs/cgroup", Threshold: 1}}}
		}
		return dynamicpathdetector.DefaultCollapseSettings()
	}
	c.ContainerProfileStorage = &fakeStorage{}

	mk := func(labels map[string]string) *softwarecomposition.ContainerProfile {
		p := &softwarecomposition.ContainerProfile{ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "kubescape", Labels: labels, Annotations: map[string]string{}}}
		for _, n := range []string{"a", "b", "c"} {
			p.Spec.Opens = append(p.Spec.Opens, softwarecomposition.OpenCalls{Path: "/sys/fs/cgroup/kubepods.slice/" + n, Flags: []string{"O_RDONLY"}})
		}
		return p
	}
	node := mk(map[string]string{helpersv1.ArtifactTypeMetadataKey: "node"})
	require.NoError(t, c.PreSave(t.Context(), node))
	require.Len(t, node.Spec.Opens, 1)
	assert.Equal(t, "/sys/fs/cgroup/"+dynamicpathdetector.WildcardIdentifier, node.Spec.Opens[0].Path, "threshold 1 folds the whole subtree to a wildcard")

	pod := mk(map[string]string{"app": "nginx"})
	require.NoError(t, c.PreSave(t.Context(), pod))
	assert.Len(t, pod.Spec.Opens, 3, "container profiles keep the default thresholds")
}
