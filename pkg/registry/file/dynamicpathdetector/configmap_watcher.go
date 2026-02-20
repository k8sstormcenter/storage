package dynamicpathdetector

import (
	"context"
	"time"

	"github.com/kubescape/go-logger"
	"github.com/kubescape/go-logger/helpers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

const collapseConfigKey = "collapseConfig.json"

// WatchCollapseConfigMap starts a Kubernetes informer that watches a single
// ConfigMap (by name) for the collapseConfig.json key. When the key is
// created or updated, the provider is updated with the parsed settings.
// When the ConfigMap or key is deleted, defaults are restored.
//
// This function blocks until ctx is cancelled.
func WatchCollapseConfigMap(ctx context.Context, client kubernetes.Interface, namespace, configMapName string, provider *CollapseConfigProvider) {
	listWatcher := cache.NewListWatchFromClient(
		client.CoreV1().RESTClient(),
		"configmaps",
		namespace,
		fields.OneTermEqualSelector("metadata.name", configMapName),
	)

	handleConfigMap := func(obj interface{}) {
		cm, ok := obj.(*corev1.ConfigMap)
		if !ok {
			return
		}
		raw, exists := cm.Data[collapseConfigKey]
		if !exists {
			// Key absent — use defaults
			provider.Update(DefaultCollapseSettings())
			logger.L().Info("collapse config: key absent in ConfigMap, using defaults",
				helpers.String("configMap", configMapName))
			return
		}
		settings, err := ParseCollapseSettings([]byte(raw))
		if err != nil {
			// Malformed JSON — log error and keep previous config
			logger.L().Error("collapse config: failed to parse, keeping previous config",
				helpers.Error(err), helpers.String("configMap", configMapName))
			return
		}
		provider.Update(settings)
		logger.L().Info("collapse config: updated from ConfigMap",
			helpers.String("configMap", configMapName),
			helpers.Int("openThreshold", settings.OpenDynamicThreshold),
			helpers.Int("endpointThreshold", settings.EndpointDynamicThreshold),
			helpers.Int("collapseConfigs", len(settings.CollapseConfigs)))
	}

	_, informer := cache.NewInformer(
		listWatcher,
		&corev1.ConfigMap{},
		30*time.Second, // resync period
		cache.ResourceEventHandlerFuncs{
			AddFunc: handleConfigMap,
			UpdateFunc: func(_, newObj interface{}) {
				handleConfigMap(newObj)
			},
			DeleteFunc: func(_ interface{}) {
				provider.Update(DefaultCollapseSettings())
				logger.L().Info("collapse config: ConfigMap deleted, reverting to defaults",
					helpers.String("configMap", configMapName))
			},
		},
	)

	informer.Run(ctx.Done())
}
