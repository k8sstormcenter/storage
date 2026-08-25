package v1beta1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// RogueArtifact is the in-cluster, discoverable record of a container that is
// not covered by a verified authored profile. One object per firing container.
type RogueArtifact struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   RogueArtifactSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status RogueArtifactStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

type RogueArtifactSpec struct {
	State         string `json:"state,omitempty" protobuf:"bytes,1,opt,name=state"`
	Learning      bool   `json:"learning,omitempty" protobuf:"varint,2,opt,name=learning"`
	Reason        string `json:"reason,omitempty" protobuf:"bytes,3,opt,name=reason"`
	WorkloadName  string `json:"workloadName,omitempty" protobuf:"bytes,4,opt,name=workloadName"`
	WorkloadKind  string `json:"workloadKind,omitempty" protobuf:"bytes,5,opt,name=workloadKind"`
	WorkloadUID   string `json:"workloadUID,omitempty" protobuf:"bytes,6,opt,name=workloadUID"`
	ContainerName string `json:"containerName,omitempty" protobuf:"bytes,7,opt,name=containerName"`
	ContainerID   string `json:"containerID,omitempty" protobuf:"bytes,8,opt,name=containerID"`
	PodName       string `json:"podName,omitempty" protobuf:"bytes,9,opt,name=podName"`
	PodUID        string `json:"podUID,omitempty" protobuf:"bytes,10,opt,name=podUID"`
	NodeName      string `json:"nodeName,omitempty" protobuf:"bytes,11,opt,name=nodeName"`
}

type RogueArtifactStatus struct {
	Phase    string      `json:"phase,omitempty" protobuf:"bytes,1,opt,name=phase"`
	FiredAt  metav1.Time `json:"firedAt,omitempty" protobuf:"bytes,2,opt,name=firedAt"`
	HealedAt metav1.Time `json:"healedAt,omitempty" protobuf:"bytes,3,opt,name=healedAt"`
}

type RogueArtifactList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []RogueArtifact `json:"items" protobuf:"bytes,2,rep,name=items"`
}
