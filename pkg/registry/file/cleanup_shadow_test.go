package file

import (
	"testing"

	helpersv1 "github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCleanup_ShadowProfilesAreImmortal(t *testing.T) {
	shadow := &metav1.ObjectMeta{
		Labels:      map[string]string{"kubescape.io/profile-role": "shadow", helpersv1.RelatedKindMetadataKey: "Deployment", helpersv1.TemplateHashKey: "gone"},
		Annotations: map[string]string{helpersv1.WlidMetadataKey: "wlid://cluster-kind-kind/namespace-shop/deployment-web"},
	}
	assert.True(t, isShadowProfile(shadow), "a learned profile of a bound workload is a shadow")
	assert.False(t, isUserManaged(shadow))

	learned := &metav1.ObjectMeta{Labels: map[string]string{helpersv1.RelatedKindMetadataKey: "Deployment"}}
	assert.False(t, isShadowProfile(learned), "an unbound workload's learned profile is not a shadow and keeps following the cleanup rules")
	assert.False(t, isShadowProfile(nil))
	assert.False(t, isShadowProfile(&metav1.ObjectMeta{Labels: map[string]string{"kubescape.io/profile-role": "suggestion"}}), "a suggestion is not a shadow")
}
