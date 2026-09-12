package file

import (
	"bytes"
	"encoding/gob"
	"testing"

	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ptr32(v int32) *int32 { return &v }

func portZeroCP() *softwarecomposition.ContainerProfile {
	return &softwarecomposition.ContainerProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns"},
		Spec: softwarecomposition.ContainerProfileSpec{
			Egress: []softwarecomposition.NetworkNeighbor{{
				Identifier: "wildcard-zero-port",
				IPAddress:  "9.9.9.9",
				Ports:      []softwarecomposition.NetworkPort{{Name: "TCP-any", Protocol: "TCP", Port: ptr32(0)}},
			}},
			Containers: []softwarecomposition.ContainerProfileContainer{{
				Name: "app",
				Egress: []softwarecomposition.NetworkNeighbor{{
					Identifier: "dns",
					IPAddress:  "1.1.1.1",
					Ports: []softwarecomposition.NetworkPort{
						{Name: "TCP-0", Protocol: "TCP", Port: ptr32(0)},
						{Name: "UDP-53", Protocol: "UDP", Port: ptr32(53)},
					},
				}},
			}},
		},
	}
}

// gob flattens *int32 and omits zeros: without the stamp/restore pair an
// explicit port-0 literal decodes as nil and becomes an any-port stanza
// (observed live: Test_28 fixture returned port:null after round-trip).
func TestGobRoundTrip_PortZeroSurvives(t *testing.T) {
	in := portZeroCP()
	stampPortZero(in)
	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(in))
	out := &softwarecomposition.ContainerProfile{}
	require.NoError(t, gob.NewDecoder(&buf).Decode(out))
	restorePortZero(out)

	require.NotNil(t, out.Spec.Egress[0].Ports[0].Port, "spec-level explicit port 0 must survive the payload round-trip")
	assert.Equal(t, int32(0), *out.Spec.Egress[0].Ports[0].Port)
	require.NotNil(t, out.Spec.Containers[0].Egress[0].Ports[0].Port, "section-level explicit port 0 must survive")
	assert.Equal(t, int32(0), *out.Spec.Containers[0].Egress[0].Ports[0].Port)
	require.NotNil(t, out.Spec.Containers[0].Egress[0].Ports[1].Port, "nonzero port unaffected")
	assert.Equal(t, int32(53), *out.Spec.Containers[0].Egress[0].Ports[1].Port)
}

// Legacy payloads carry no PortZero marker: nonzero ports round-trip as
// before, an absent port stays absent — restore must not invent pointers.
func TestGobRoundTrip_LegacyPayloadUntouched(t *testing.T) {
	legacy := &softwarecomposition.ContainerProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "old", Namespace: "ns"},
		Spec: softwarecomposition.ContainerProfileSpec{
			Egress: []softwarecomposition.NetworkNeighbor{{
				Identifier: "mixed",
				IPAddress:  "8.8.8.8",
				Ports: []softwarecomposition.NetworkPort{
					{Name: "TCP-443", Protocol: "TCP", Port: ptr32(443)},
					{Name: "TCP-any", Protocol: "TCP"}, // port already lost at rest pre-fix
				},
			}},
		},
	}
	var buf bytes.Buffer
	require.NoError(t, gob.NewEncoder(&buf).Encode(legacy)) // no stamp: legacy writer
	out := &softwarecomposition.ContainerProfile{}
	require.NoError(t, gob.NewDecoder(&buf).Decode(out))
	restorePortZero(out)

	require.NotNil(t, out.Spec.Egress[0].Ports[0].Port)
	assert.Equal(t, int32(443), *out.Spec.Egress[0].Ports[0].Port)
	assert.Nil(t, out.Spec.Egress[0].Ports[1].Port, "restore must not invent a port where no marker exists")
}
