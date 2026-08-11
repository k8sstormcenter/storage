package file

import (
	"context"
	"fmt"

	"k8s.io/apiserver/pkg/storage"
)

type virtualStorageBase struct {
	immutableStorage
	realStore StorageQuerier
}

func (virtualStorageBase) EnableResourceSizeEstimation(storage.KeysFunc) error {
	return nil
}

func (virtualStorageBase) Stats(context.Context) (storage.Stats, error) {
	return storage.Stats{}, fmt.Errorf("unimplemented")
}

func (virtualStorageBase) SetKeysFunc(storage.KeysFunc) {}

func (virtualStorageBase) CompactRevision() int64 {
	return 0
}

func (virtualStorageBase) GetCurrentResourceVersion(context.Context) (uint64, error) {
	return 0, nil
}
