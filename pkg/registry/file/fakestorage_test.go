package file

import (
	"context"
	"time"

	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
)

// fakeStorage implements ContainerProfileStorage with minimal behavior for tests.
// Only GetContainerProfileMetadata is used by ComputeAggregatedData; other methods are stubs.
type fakeStorage struct {
	profiles map[string]softwarecomposition.ContainerProfile
}

func (f *fakeStorage) WithConnection(ctx context.Context) (context.Context, func(), error) {
	return ctx, func() {}, nil
}
func (f *fakeStorage) BeginTransaction(ctx context.Context) (func(*error), error) {
	return func(*error) {}, nil
}

// TimeSeriesOperations
func (f *fakeStorage) ListTimeSeriesExpired(ctx context.Context, threshold time.Duration) ([]string, error) {
	return nil, nil
}
func (f *fakeStorage) ListTimeSeriesWithData(ctx context.Context) ([]string, error) {
	return nil, nil
}
func (f *fakeStorage) ListTimeSeriesContainers(ctx context.Context, key string) (map[string][]softwarecomposition.TimeSeriesContainers, error) {
	return nil, nil
}
func (f *fakeStorage) DeleteTimeSeriesContainerEntries(ctx context.Context, key string) error {
	return nil
}
func (f *fakeStorage) ReplaceTimeSeriesContainerEntries(ctx context.Context, key, seriesID string, deleteTimeSeries []string, newTimeSeries []softwarecomposition.TimeSeriesContainers) error {
	return nil
}

// ContainerProfileStorage methods (stubs except GetContainerProfileMetadata)
func (f *fakeStorage) DeleteContainerProfile(ctx context.Context, key string) error {
	return nil
}
func (f *fakeStorage) GetContainerProfile(ctx context.Context, key string) (softwarecomposition.ContainerProfile, error) {
	var p softwarecomposition.ContainerProfile
	return p, nil
}
func (f *fakeStorage) GetContainerProfileMetadata(ctx context.Context, key string) (softwarecomposition.ContainerProfile, error) {
	if p, ok := f.profiles[key]; ok {
		return p, nil
	}
	var empty softwarecomposition.ContainerProfile
	return empty, nil
}
func (f *fakeStorage) GetContainerProfileMetadataNoLock(ctx context.Context, key string) (softwarecomposition.ContainerProfile, error) {
	return f.GetContainerProfileMetadata(ctx, key)
}
func (f *fakeStorage) GetSbom(ctx context.Context, key string) (softwarecomposition.SBOMSyft, error) {
	return softwarecomposition.SBOMSyft{}, nil
}
func (f *fakeStorage) GetTsContainerProfile(ctx context.Context, key string) (softwarecomposition.ContainerProfile, error) {
	return softwarecomposition.ContainerProfile{}, nil
}
func (f *fakeStorage) SaveContainerProfile(ctx context.Context, key string, profile *softwarecomposition.ContainerProfile) error {
	return nil
}
func (f *fakeStorage) GetStorageImpl() *StorageImpl {
	return nil
}
func (f *fakeStorage) WriteTimeSeriesEntry(ctx context.Context, kind, namespace, name, seriesID, tsSuffix, reportTimestamp, status, completion, previousReportTimestamp string, hasData bool) error {
	return nil
}
