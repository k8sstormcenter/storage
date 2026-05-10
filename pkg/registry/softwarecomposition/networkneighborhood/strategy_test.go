package networkneighborhood

import (
	"context"
	"testing"

	"github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func TestPrepareForUpdate(t *testing.T) {
	tests := []struct {
		name           string
		oldAnnotations map[string]string
		newAnnotations map[string]string
		expected       map[string]string
	}{
		{
			name: "transition from complete (with status) to partial - rejected",
			oldAnnotations: map[string]string{
				helpers.CompletionMetadataKey: "complete",
				helpers.StatusMetadataKey:     "initializing",
			},
			newAnnotations: map[string]string{
				helpers.CompletionMetadataKey: "partial",
				helpers.StatusMetadataKey:     "ready",
			},
			expected: map[string]string{
				helpers.CompletionMetadataKey: "complete",
				helpers.StatusMetadataKey:     "initializing",
			},
		},
		{
			name: "transition from partial (with status) to complete - accepted",
			oldAnnotations: map[string]string{
				helpers.CompletionMetadataKey: "partial",
				helpers.StatusMetadataKey:     "initializing",
			},
			newAnnotations: map[string]string{
				helpers.CompletionMetadataKey: "partial",
				helpers.StatusMetadataKey:     "ready",
			},
			expected: map[string]string{
				helpers.CompletionMetadataKey: "partial",
				helpers.StatusMetadataKey:     "ready",
			},
		},
		{
			name: "transition from partial (without status) to complete - accepted",
			oldAnnotations: map[string]string{
				helpers.CompletionMetadataKey: "partial",
			},
			newAnnotations: map[string]string{
				helpers.CompletionMetadataKey: "complete",
				helpers.StatusMetadataKey:     "ready",
			},
			expected: map[string]string{
				helpers.CompletionMetadataKey: "complete",
				helpers.StatusMetadataKey:     "ready",
			},
		},
		{
			name: "transition from complete (without status) to partial - rejected",
			oldAnnotations: map[string]string{
				helpers.CompletionMetadataKey: "complete",
			},
			newAnnotations: map[string]string{
				helpers.CompletionMetadataKey: "partial",
				helpers.StatusMetadataKey:     "initializing",
			},
			expected: map[string]string{
				helpers.CompletionMetadataKey: "complete",
			},
		},
		{
			name: "transition from a final AP - all changes are rejected",
			oldAnnotations: map[string]string{
				helpers.CompletionMetadataKey: "complete",
				helpers.StatusMetadataKey:     "completed",
			},
			newAnnotations: map[string]string{
				helpers.CompletionMetadataKey: "partial",
				helpers.StatusMetadataKey:     "initializing",
			},
			expected: map[string]string{
				helpers.CompletionMetadataKey: "complete",
				helpers.StatusMetadataKey:     "completed",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NetworkNeighborhoodStrategy{}

			obj := &softwarecomposition.NetworkNeighborhood{ObjectMeta: metav1.ObjectMeta{Annotations: tt.newAnnotations}}
			old := &softwarecomposition.NetworkNeighborhood{ObjectMeta: metav1.ObjectMeta{Annotations: tt.oldAnnotations}}

			s.PrepareForUpdate(context.TODO(), obj, old)
			assert.Equal(t, tt.expected, obj.Annotations)
		})
	}
}

func TestPrepareForUpdateFullObj(t *testing.T) {
	tests := []struct {
		name     string
		old      *softwarecomposition.NetworkNeighborhood
		new      *softwarecomposition.NetworkNeighborhood
		expected *softwarecomposition.NetworkNeighborhood
	}{
		{
			name: "transition from initializing to ready - changes are accepted",
			old: &softwarecomposition.NetworkNeighborhood{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						helpers.CompletionMetadataKey: "complete",
						helpers.StatusMetadataKey:     "initializing",
					},
				},
				Spec: softwarecomposition.NetworkNeighborhoodSpec{
					Containers: []softwarecomposition.NetworkNeighborhoodContainer{
						{
							Name:   "container1",
							Egress: []softwarecomposition.NetworkNeighbor{},
							Ingress: []softwarecomposition.NetworkNeighbor{
								{
									IPAddress: "154.53.46.32",
									Ports: []softwarecomposition.NetworkPort{
										{
											Port:     ptr.To(int32(80)),
											Protocol: softwarecomposition.ProtocolTCP,
											Name:     "TCP-80",
										},
									},
								},
							},
						},
					},
				},
			},
			new: &softwarecomposition.NetworkNeighborhood{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						helpers.CompletionMetadataKey: "complete",
						helpers.StatusMetadataKey:     "ready",
					},
				},
				Spec: softwarecomposition.NetworkNeighborhoodSpec{
					Containers: []softwarecomposition.NetworkNeighborhoodContainer{
						{
							Name:   "container1",
							Egress: []softwarecomposition.NetworkNeighbor{},
							Ingress: []softwarecomposition.NetworkNeighbor{
								{
									IPAddress: "154.53.46.32",
									Ports: []softwarecomposition.NetworkPort{
										{
											Port:     ptr.To(int32(80)),
											Protocol: softwarecomposition.ProtocolTCP,
											Name:     "TCP-80",
										},
									},
								},
							},
						},
						{
							Name:   "container2",
							Egress: []softwarecomposition.NetworkNeighbor{},
							Ingress: []softwarecomposition.NetworkNeighbor{
								{
									IPAddress: "154.53.46.32",
									Ports: []softwarecomposition.NetworkPort{
										{
											Port:     ptr.To(int32(80)),
											Protocol: softwarecomposition.ProtocolTCP,
											Name:     "TCP-80",
										},
									},
								},
							},
						},
					},
				},
			},
			expected: &softwarecomposition.NetworkNeighborhood{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						helpers.CompletionMetadataKey: "complete",
						helpers.StatusMetadataKey:     "ready",
					},
				},
				Spec: softwarecomposition.NetworkNeighborhoodSpec{
					Containers: []softwarecomposition.NetworkNeighborhoodContainer{
						{
							Name:   "container1",
							Egress: []softwarecomposition.NetworkNeighbor{},
							Ingress: []softwarecomposition.NetworkNeighbor{
								{
									IPAddress: "154.53.46.32",
									Ports: []softwarecomposition.NetworkPort{
										{
											Port:     ptr.To(int32(80)),
											Protocol: softwarecomposition.ProtocolTCP,
											Name:     "TCP-80",
										},
									},
								},
							},
						},
						{
							Name:   "container2",
							Egress: []softwarecomposition.NetworkNeighbor{},
							Ingress: []softwarecomposition.NetworkNeighbor{
								{
									IPAddress: "154.53.46.32",
									Ports: []softwarecomposition.NetworkPort{
										{
											Port:     ptr.To(int32(80)),
											Protocol: softwarecomposition.ProtocolTCP,
											Name:     "TCP-80",
										},
									},
								},
							},
						},
					},
				},
			},
		},
		{
			name: "transition from a final AP - all changes are rejected",
			old: &softwarecomposition.NetworkNeighborhood{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						helpers.CompletionMetadataKey: "complete",
						helpers.StatusMetadataKey:     "completed",
					},
				},
				Spec: softwarecomposition.NetworkNeighborhoodSpec{
					Containers: []softwarecomposition.NetworkNeighborhoodContainer{
						{
							Name:   "container1",
							Egress: []softwarecomposition.NetworkNeighbor{},
							Ingress: []softwarecomposition.NetworkNeighbor{
								{
									IPAddress: "154.53.46.32",
									Ports: []softwarecomposition.NetworkPort{
										{
											Port:     ptr.To(int32(80)),
											Protocol: softwarecomposition.ProtocolTCP,
											Name:     "TCP-80",
										},
									},
								},
							},
						},
					},
				},
			},
			new: &softwarecomposition.NetworkNeighborhood{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						helpers.CompletionMetadataKey: "partial",
						helpers.StatusMetadataKey:     "initializing",
					},
				},
				Spec: softwarecomposition.NetworkNeighborhoodSpec{
					Containers: []softwarecomposition.NetworkNeighborhoodContainer{
						{
							Name:   "container1",
							Egress: []softwarecomposition.NetworkNeighbor{},
							Ingress: []softwarecomposition.NetworkNeighbor{
								{
									IPAddress: "154.53.46.32",
									Ports: []softwarecomposition.NetworkPort{
										{
											Port:     ptr.To(int32(80)),
											Protocol: softwarecomposition.ProtocolTCP,
											Name:     "TCP-80",
										},
									},
								},
							},
						},
						{
							Name:   "container2",
							Egress: []softwarecomposition.NetworkNeighbor{},
							Ingress: []softwarecomposition.NetworkNeighbor{
								{
									IPAddress: "154.53.46.32",
									Ports: []softwarecomposition.NetworkPort{
										{
											Port:     ptr.To(int32(80)),
											Protocol: softwarecomposition.ProtocolTCP,
											Name:     "TCP-80",
										},
									},
								},
							},
						},
					},
				},
			},
			expected: &softwarecomposition.NetworkNeighborhood{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						helpers.CompletionMetadataKey: "complete",
						helpers.StatusMetadataKey:     "completed",
					},
				},
				Spec: softwarecomposition.NetworkNeighborhoodSpec{
					Containers: []softwarecomposition.NetworkNeighborhoodContainer{
						{
							Name:   "container1",
							Egress: []softwarecomposition.NetworkNeighbor{},
							Ingress: []softwarecomposition.NetworkNeighbor{
								{
									IPAddress: "154.53.46.32",
									Ports: []softwarecomposition.NetworkPort{
										{
											Port:     ptr.To(int32(80)),
											Protocol: softwarecomposition.ProtocolTCP,
											Name:     "TCP-80",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NetworkNeighborhoodStrategy{}
			s.PrepareForUpdate(context.TODO(), tt.new, tt.old)
			assert.Equal(t, tt.expected, tt.new)
		})
	}
}

// TestValidate_NetworkProfileEntries pins the v0.0.2 admission contract:
// malformed IPAddresses[] / DNSNames[] entries cause Validate to return
// field errors that the apiserver translates into a 400 to the client.
//
// Runtime matchers tolerate malformed entries (silently skip), but
// admission rejects them so the next person reviewing the profile sees
// a clean document — and so the user gets fast feedback at write time.
func TestValidate_NetworkProfileEntries(t *testing.T) {
	makeNN := func(neighbor softwarecomposition.NetworkNeighbor) *softwarecomposition.NetworkNeighborhood {
		return &softwarecomposition.NetworkNeighborhood{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "test-nn",
				Namespace:   "default",
				Annotations: map[string]string{helpers.CompletionMetadataKey: "complete", helpers.StatusMetadataKey: "ready"},
			},
			Spec: softwarecomposition.NetworkNeighborhoodSpec{
				Containers: []softwarecomposition.NetworkNeighborhoodContainer{
					{Name: "c", Egress: []softwarecomposition.NetworkNeighbor{neighbor}},
				},
			},
		}
	}

	cases := []struct {
		name       string
		neighbor   softwarecomposition.NetworkNeighbor
		wantErrors int
	}{
		{
			name:       "all valid IPs and DNSNames",
			neighbor:   softwarecomposition.NetworkNeighbor{IPAddresses: []string{"10.0.0.0/8", "*", "1.2.3.4"}, DNSNames: []string{"*.example.com.", "api.partner.io."}},
			wantErrors: 0,
		},
		{
			name:       "single malformed IP",
			neighbor:   softwarecomposition.NetworkNeighbor{IPAddresses: []string{"not-an-ip"}},
			wantErrors: 1,
		},
		{
			name:       "single malformed CIDR",
			neighbor:   softwarecomposition.NetworkNeighbor{IPAddresses: []string{"10.0.0.0/40"}},
			wantErrors: 1,
		},
		{
			name:       "recursive DNS wildcard rejected",
			neighbor:   softwarecomposition.NetworkNeighbor{DNSNames: []string{"**"}},
			wantErrors: 1,
		},
		{
			name:       "mid-position bare star rejected (must use ⋯)",
			neighbor:   softwarecomposition.NetworkNeighbor{DNSNames: []string{"foo.*.bar."}},
			wantErrors: 1,
		},
		{
			name:       "mixed: some good, some bad",
			neighbor:   softwarecomposition.NetworkNeighbor{IPAddresses: []string{"10.1.2.3", "garbage", "192.168.0.0/16"}, DNSNames: []string{"api.example.com.", "**", "*.example.com."}},
			wantErrors: 2,
		},
	}

	s := NetworkNeighborhoodStrategy{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := s.Validate(context.TODO(), makeNN(tc.neighbor))
			if len(errs) != tc.wantErrors {
				t.Errorf("Validate returned %d errors, want %d. errs: %v", len(errs), tc.wantErrors, errs)
			}
		})
	}
}
