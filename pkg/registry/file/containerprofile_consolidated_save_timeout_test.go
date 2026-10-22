package file

import (
	"context"
	"testing"
	"time"

	helpersv1 "github.com/kubescape/k8s-interface/instanceidhandler/v1/helpers"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

type saveDeadlineObserver struct {
	Processor
	observe func(context.Context)
}

func (p *saveDeadlineObserver) PreSave(ctx context.Context, obj runtime.Object) error {
	p.observe(ctx)
	return p.Processor.PreSave(ctx, obj)
}

func TestConsolidatedSaveBudgetAcrossBackends(t *testing.T) {
	for _, native := range []bool{false, true} {
		name := "legacy"
		if native {
			name = "objectstore"
		}
		t.Run(name, func(t *testing.T) {
			for _, parentBudget := range []time.Duration{0, 30 * time.Second} {
				t.Run(parentBudget.String(), func(t *testing.T) {
					calls := 0
					wrap := func(p Processor) Processor {
						return &saveDeadlineObserver{Processor: p, observe: func(ctx context.Context) {
							calls++
							deadline, ok := ctx.Deadline()
							require.True(t, ok)
							remaining := time.Until(deadline)
							if parentBudget == 0 {
								require.Greater(t, remaining, 90*time.Second)
								require.LessOrEqual(t, remaining, 2*time.Minute)
							} else {
								require.Greater(t, remaining, 20*time.Second)
								require.LessOrEqual(t, remaining, parentBudget)
							}
						}}
					}
					var b *backend
					if native {
						b = newObjectBackend(t, wrap)
					} else {
						b = newLegacyBackend(t, wrap)
					}
					defer b.watcher.Stop()
					ctx := context.Background()
					if parentBudget > 0 {
						var cancel context.CancelFunc
						ctx, cancel = context.WithTimeout(ctx, parentBudget)
						defer cancel()
					}
					ctx, release, err := b.processor.ContainerProfileStorage.WithConnection(ctx)
					require.NoError(t, err)
					defer release()
					p := newCollapsibleProfile(map[string]string{helpersv1.ManagedByMetadataKey: helpersv1.ManagedByUserValue})
					require.NoError(t, b.processor.ContainerProfileStorage.SaveContainerProfile(ctx, testCPPrefix+"redis/redis-client", p))
					require.Positive(t, calls)
				})
			}
		})
	}
}
