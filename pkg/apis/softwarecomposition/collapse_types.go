package softwarecomposition

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CollapseConfiguration defines cluster-wide thresholds for dynamic path collapsing.
type CollapseConfiguration struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec CollapseConfigurationSpec
}

type CollapseConfigurationSpec struct {
	OpenDynamicThreshold     int
	EndpointDynamicThreshold int
	CollapseConfigs          []CollapseConfigEntry
}

type CollapseConfigEntry struct {
	Prefix    string
	Threshold int
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CollapseConfigurationList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []CollapseConfiguration
}
