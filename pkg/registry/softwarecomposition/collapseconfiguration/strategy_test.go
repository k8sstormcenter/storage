package collapseconfiguration

import (
	"context"
	"testing"

	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func validCC() *softwarecomposition.CollapseConfiguration {
	return &softwarecomposition.CollapseConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "default"},
		Spec: softwarecomposition.CollapseConfigurationSpec{
			OpenDynamicThreshold:     50,
			EndpointDynamicThreshold: 100,
			CollapseConfigs: []softwarecomposition.CollapseConfigEntry{
				{Prefix: "/etc", Threshold: 100},
				{Prefix: "/opt", Threshold: 5},
			},
		},
	}
}

func TestValidate_Valid(t *testing.T) {
	errs := validateCollapseConfiguration(validCC())
	assert.Empty(t, errs)
}

func TestValidate_ZeroOpenThreshold(t *testing.T) {
	cc := validCC()
	cc.Spec.OpenDynamicThreshold = 0
	errs := validateCollapseConfiguration(cc)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Field, "openDynamicThreshold")
}

func TestValidate_NegativeEndpointThreshold(t *testing.T) {
	cc := validCC()
	cc.Spec.EndpointDynamicThreshold = -1
	errs := validateCollapseConfiguration(cc)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Field, "endpointDynamicThreshold")
}

func TestValidate_EmptyPrefix(t *testing.T) {
	cc := validCC()
	cc.Spec.CollapseConfigs = append(cc.Spec.CollapseConfigs,
		softwarecomposition.CollapseConfigEntry{Prefix: "", Threshold: 10})
	errs := validateCollapseConfiguration(cc)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Field, "prefix")
}

func TestValidate_ZeroEntryThreshold(t *testing.T) {
	cc := validCC()
	cc.Spec.CollapseConfigs[0].Threshold = 0
	errs := validateCollapseConfiguration(cc)
	assert.Len(t, errs, 1)
	assert.Contains(t, errs[0].Field, "threshold")
}

func TestValidate_MultipleErrors(t *testing.T) {
	cc := &softwarecomposition.CollapseConfiguration{
		Spec: softwarecomposition.CollapseConfigurationSpec{
			OpenDynamicThreshold:     0,
			EndpointDynamicThreshold: 0,
			CollapseConfigs: []softwarecomposition.CollapseConfigEntry{
				{Prefix: "", Threshold: 0},
			},
		},
	}
	errs := validateCollapseConfiguration(cc)
	assert.Len(t, errs, 4, "expected 4 errors: openThreshold, endpointThreshold, prefix, threshold")
}

func TestValidate_EmptyCollapseConfigs(t *testing.T) {
	cc := validCC()
	cc.Spec.CollapseConfigs = nil
	errs := validateCollapseConfiguration(cc)
	assert.Empty(t, errs, "nil CollapseConfigs is valid")
}

func TestStrategy_ClusterScoped(t *testing.T) {
	s := NewStrategy(nil)
	assert.False(t, s.NamespaceScoped(), "CollapseConfiguration must be cluster-scoped")
}

func TestStrategy_Validate(t *testing.T) {
	s := NewStrategy(nil)
	errs := s.Validate(context.Background(), validCC())
	assert.Empty(t, errs)
}

func TestStrategy_ValidateUpdate(t *testing.T) {
	s := NewStrategy(nil)
	old := validCC()
	updated := validCC()
	updated.Spec.OpenDynamicThreshold = 25
	errs := s.ValidateUpdate(context.Background(), updated, old)
	assert.Empty(t, errs)
}
