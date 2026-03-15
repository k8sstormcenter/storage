package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CollapseConfiguration defines cluster-wide thresholds for dynamic path
// collapsing in ApplicationProfiles. It is signable and auditable.
type CollapseConfiguration struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec CollapseConfigurationSpec `json:"spec" protobuf:"bytes,2,req,name=spec"`
}

// CollapseConfigurationSpec holds the collapse thresholds.
type CollapseConfigurationSpec struct {
	// OpenDynamicThreshold is the default threshold for open-path collapsing.
	OpenDynamicThreshold int `json:"openDynamicThreshold" protobuf:"varint,1,req,name=openDynamicThreshold"`
	// EndpointDynamicThreshold is the default threshold for endpoint collapsing.
	EndpointDynamicThreshold int `json:"endpointDynamicThreshold" protobuf:"varint,2,req,name=endpointDynamicThreshold"`
	// CollapseConfigs defines per-prefix threshold overrides.
	CollapseConfigs []CollapseConfigEntry `json:"collapseConfigs,omitempty" protobuf:"bytes,3,rep,name=collapseConfigs"`
}

// CollapseConfigEntry defines a per-prefix collapse threshold.
type CollapseConfigEntry struct {
	// Prefix is the path prefix to match (e.g. "/etc", "/opt").
	Prefix string `json:"prefix" protobuf:"bytes,1,req,name=prefix"`
	// Threshold is the max unique children before collapsing.
	Threshold int `json:"threshold" protobuf:"varint,2,req,name=threshold"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CollapseConfigurationList is a list of CollapseConfiguration resources.
type CollapseConfigurationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []CollapseConfiguration `json:"items" protobuf:"bytes,2,rep,name=items"`
}
