package file

import (
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"k8s.io/apimachinery/pkg/runtime"
)

// gob flattens *int32 and omits zero values, so an explicit port 0 decodes as
// nil — a user-authored port-0 literal silently becomes an any-port stanza
// (observed: Test_28 port_zero fixture returned port:null after round-trip).
// stampPortZero marks explicit zeros before encode; restorePortZero rebuilds
// the pointer after decode. Legacy payloads carry no PortZero and are
// untouched: nonzero ports never need the marker, and their lost *0s were
// already nil at rest.

func stampPortZero(obj runtime.Object) {
	forEachNetworkPort(obj, func(p *softwarecomposition.NetworkPort) {
		if p.Port != nil && *p.Port == 0 {
			p.PortZero = true
		}
	})
}

func restorePortZero(obj runtime.Object) {
	forEachNetworkPort(obj, func(p *softwarecomposition.NetworkPort) {
		if p.Port == nil && p.PortZero {
			zero := int32(0)
			p.Port = &zero
		}
	})
}

func forEachNetworkPort(obj runtime.Object, fn func(*softwarecomposition.NetworkPort)) {
	cp, ok := obj.(*softwarecomposition.ContainerProfile)
	if !ok {
		return
	}
	visit := func(neighbors []softwarecomposition.NetworkNeighbor) {
		for i := range neighbors {
			for j := range neighbors[i].Ports {
				fn(&neighbors[i].Ports[j])
			}
		}
	}
	visit(cp.Spec.Ingress)
	visit(cp.Spec.Egress)
	for _, group := range [][]softwarecomposition.ContainerProfileContainer{cp.Spec.Containers, cp.Spec.InitContainers, cp.Spec.EphemeralContainers} {
		for i := range group {
			visit(group[i].Ingress)
			visit(group[i].Egress)
		}
	}
}
