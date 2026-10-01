package file

import (
	"testing"

	wlidPkg "github.com/armosec/utils-k8s-go/wlid"
	mapset "github.com/deckarep/golang-set/v2"
	"github.com/goradd/maps"
	helpersv1 "github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRunningPodWlid_ListedPodWithoutTypeMetaStillNamesTheKind(t *testing.T) {
	listed := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "probe"}}
	assert.Empty(t, listed.Kind, "typed list items carry no TypeMeta")
	got := runningPodWlid(listed)
	want := wlidWithoutClusterName(wlidPkg.GetK8sWLID("", "shop", "Pod", "probe"))
	assert.Equal(t, want, got)
	assert.Contains(t, got, "/pod-probe", "the running set must name the bare pod the way its learned profile does, or cleanup deletes the profile every interval")
}

func TestCleanup_LiveWorkloadsKeepTheirLearnedProfiles(t *testing.T) {
	running := ResourceMaps{
		RunningTemplateHash:          mapset.NewSet[string]("7b754b5498"),
		RunningWlidsToContainerNames: new(maps.SafeMap[string, mapset.Set[string]]),
	}
	listedBarePod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "probe"}}
	running.RunningWlidsToContainerNames.Set(runningPodWlid(listedBarePod), mapset.NewSet[string]("probe"))

	deploymentShadow := &metav1.ObjectMeta{
		Labels:      map[string]string{helpersv1.TemplateHashKey: "7b754b5498", helpersv1.RelatedKindMetadataKey: "Deployment"},
		Annotations: map[string]string{helpersv1.WlidMetadataKey: "wlid://cluster-kind-kind/namespace-shop/deployment-web"},
	}
	assert.False(t, deleteByTemplateHashOrWlid("", "", deploymentShadow, running), "a live replica's shadow survives the cleanup interval")

	barePodShadow := &metav1.ObjectMeta{
		Labels:      map[string]string{helpersv1.RelatedKindMetadataKey: "Pod"},
		Annotations: map[string]string{helpersv1.WlidMetadataKey: "wlid://cluster-kind-kind/namespace-shop/pod-probe"},
	}
	assert.False(t, deleteByTemplateHashOrWlid("", "", barePodShadow, running), "a running bare pod's shadow survives: the running wlid names the kind the agent writes")

	rolledOut := &metav1.ObjectMeta{
		Labels:      map[string]string{helpersv1.TemplateHashKey: "0ld", helpersv1.RelatedKindMetadataKey: "Deployment"},
		Annotations: map[string]string{helpersv1.WlidMetadataKey: "wlid://cluster-kind-kind/namespace-shop/deployment-web"},
	}
	assert.True(t, deleteByTemplateHashOrWlid("", "", rolledOut, running), "a replaced template hash is the one legitimate deletion")
}
