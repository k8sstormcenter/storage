package softwarecomposition

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// RogueArtifact is the in-cluster, discoverable record of a container that is
// not covered by a verified authored profile. One object per firing container;
// node-agent creates it on fire and transitions it to Healed on governance.
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RogueArtifact struct {
	metav1.TypeMeta
	metav1.ObjectMeta

	Spec   RogueArtifactSpec
	Status RogueArtifactStatus
}

type RogueArtifactSpec struct {
	// State is why the container is uncovered: rogue (no profile label at all),
	// unbound (label set but resolves to no profile), unsigned (a profile
	// resolves but is unsigned while signature verification is on).
	State string
	// Learning is true while the container is in kubescape.io/learning mode:
	// the effective profile is empty (deny-all, alerting) while node-agent
	// records the real behaviour in the background.
	Learning bool
	// Reason is a human-readable detail (the diverging binding name, verify
	// error class, etc.).
	Reason        string
	WorkloadName  string
	WorkloadKind  string
	WorkloadUID   string
	ContainerName string
	ContainerID   string
	PodName       string
	PodUID        string
	NodeName      string
}

type RogueArtifactStatus struct {
	// Phase is Firing while the container remains uncovered, Healed once it is
	// governed by a verified profile. A departed (e.g. completed Job) object
	// keeps its last Phase until the TTL sweep.
	Phase    string
	FiredAt  metav1.Time
	HealedAt metav1.Time
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type RogueArtifactList struct {
	metav1.TypeMeta
	metav1.ListMeta

	Items []RogueArtifact
}
