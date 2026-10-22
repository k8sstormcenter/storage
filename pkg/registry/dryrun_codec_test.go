package registry

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/apis/softwarecomposition/install"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestDryRunCodec_RoundTripsAnInternalObject(t *testing.T) {
	scheme := runtime.NewScheme()
	install.Install(scheme)
	codec := DryRunCodec(scheme)
	require.NotNil(t, codec)

	in := &softwarecomposition.ContainerProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "redis-client", Namespace: "redis", ResourceVersion: "7",
			Annotations: map[string]string{"kubescape.io/managed-by": "User"}},
		Spec: softwarecomposition.ContainerProfileSpec{Execs: []softwarecomposition.ExecCalls{{Path: "/usr/bin/redis-cli", Args: []string{"redis-cli", "ping"}}}},
	}
	data, err := runtime.Encode(codec, in)
	require.NoError(t, err)
	out := &softwarecomposition.ContainerProfile{}
	_, _, err = codec.Decode(data, nil, out)
	require.NoError(t, err)
	assert.Equal(t, in.Spec, out.Spec)
	assert.Equal(t, in.Name, out.Name)
	assert.Equal(t, in.Annotations, out.Annotations)
}

func TestEveryStoreWiresTheDryRunCodec(t *testing.T) {
	root := filepath.Join("softwarecomposition")
	nilCodec := regexp.MustCompile(`DryRunnableStorage\{Codec:\s*nil`)
	var offenders []string
	require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		b, rerr := os.ReadFile(path)
		if rerr != nil {
			return rerr
		}
		if nilCodec.Match(b) {
			offenders = append(offenders, path)
		}
		return nil
	}))
	assert.Empty(t, offenders, "a store with a nil dry-run codec panics on every server-side dry-run")
}
