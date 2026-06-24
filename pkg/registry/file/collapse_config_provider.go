/*
Copyright 2024 The Kubescape Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

package file

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/kubescape/storage/pkg/apis/softwarecomposition"
	"github.com/kubescape/storage/pkg/registry/file/dynamicpathdetector"
	"k8s.io/apiserver/pkg/storage"
)

// DefaultCollapseConfigurationName is the cluster-scoped CR name the
// deflate path reads to learn effective collapse thresholds.
const DefaultCollapseConfigurationName = "default"

// collapseConfigGetTimeout bounds each storage Get so a slow or
// self-referential read on the apiserver's own storage can never stall the
// caller. On timeout/error the provider keeps the last known-good settings;
// it only falls back to defaults when the CR is genuinely absent.
const collapseConfigGetTimeout = 2 * time.Second

// collapseConfigRefreshInterval is how often the background refresher polls
// storage for CollapseConfiguration/default. Deflate runs per-profile under
// load; the previous per-call Get stalled the request path, so the Get now
// happens off the request path and operator edits go live within one tick.
const collapseConfigRefreshInterval = 10 * time.Second

func collapseConfigurationKey(name string) string {
	return K8sKeysToPath("", "spdx.softwarecomposition.kubescape.io", "collapseconfigurations", "", "", name)
}

// NewCRDCollapseSettingsProvider returns a CollapseSettingsProvider that
// reads the cluster-scoped CollapseConfiguration/<DefaultCollapseConfigurationName>
// and projects it via dynamicpathdetector.CollapseSettingsFromCRD. It falls
// back to dynamicpathdetector.DefaultCollapseSettings when storage is nil or
// the CR is genuinely absent; a transient read error keeps the last
// known-good snapshot rather than reverting an applied configuration.
//
// The storage read is NEVER performed on the deflate (request) path: it runs
// in a bounded-timeout background refresher, and the returned closure only
// reads an atomically-published snapshot. This prevents the self-referential
// per-call Get from stalling the apiserver under load while keeping operator
// edits live within collapseConfigRefreshInterval. The background refresher
// lives for the process lifetime (acceptable for a server-lifetime provider);
// the unexported variant takes a stop channel used by tests and available for
// a future graceful-shutdown hook.
func NewCRDCollapseSettingsProvider(s storage.Interface) dynamicpathdetector.CollapseSettingsProvider {
	return newCRDCollapseSettingsProvider(s, collapseConfigRefreshInterval, nil)
}

// newCRDCollapseSettingsProvider is the testable core: refreshInterval and the
// stop channel are injectable so tests can poll on a short interval and shut
// the refresher down deterministically. A nil stop channel never fires, so the
// exported constructor's goroutine runs for the process lifetime.
func newCRDCollapseSettingsProvider(s storage.Interface, refreshInterval time.Duration, stop <-chan struct{}) dynamicpathdetector.CollapseSettingsProvider {
	if s == nil {
		return dynamicpathdetector.DefaultCollapseSettings
	}
	key := collapseConfigurationKey(DefaultCollapseConfigurationName)

	var current atomic.Pointer[dynamicpathdetector.CollapseSettings]
	def := dynamicpathdetector.DefaultCollapseSettings()
	current.Store(&def)

	refresh := func() {
		ctx, cancel := context.WithTimeout(context.Background(), collapseConfigGetTimeout)
		defer cancel()
		crd := &softwarecomposition.CollapseConfiguration{}
		if err := s.Get(ctx, key, storage.GetOptions{IgnoreNotFound: true}, crd); err != nil {
			// Transient read error (timeout, etcd blip): keep the last
			// known-good snapshot instead of reverting an operator-applied
			// configuration to defaults. The next tick retries.
			return
		}
		var settings dynamicpathdetector.CollapseSettings
		if crd.Name == "" {
			// CR genuinely absent: IgnoreNotFound zeroed the out object.
			// CollapseSettingsFromCRD only defaults for a nil pointer, so the
			// empty-name guard is required to avoid all-zero thresholds here.
			settings = dynamicpathdetector.DefaultCollapseSettings()
		} else {
			settings = dynamicpathdetector.CollapseSettingsFromCRD(crd)
		}
		current.Store(&settings)
	}

	// Initial bounded refresh (<= collapseConfigGetTimeout) seeds the snapshot
	// before the provider is used. This is the only synchronous storage touch
	// and never happens on the deflate (request) path.
	refresh()
	go func() {
		t := time.NewTicker(refreshInterval)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				refresh()
			}
		}
	}()

	return func() dynamicpathdetector.CollapseSettings {
		return *current.Load()
	}
}
