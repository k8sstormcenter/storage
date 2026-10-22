package registry

import (
	"github.com/kubescape/storage/pkg/apis/softwarecomposition/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
)

// DryRunCodec returns the codec the generic registry's dry-run wrapper uses to
// copy an object without persisting it. The stores in this server supply
// their own storage, so CompleteWithOptions never sets the codec; a nil
// codec makes every server-side dry-run (and with it every server-side
// apply that dry-runs first) panic in runtime.Encode.
func DryRunCodec(scheme *runtime.Scheme) runtime.Codec {
	return serializer.NewCodecFactory(scheme).LegacyCodec(v1beta1.SchemeGroupVersion)
}
