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
// caller. On timeout/error the provider keeps the last known settings (or
// the default). Addresses CodeRabbit's "bound storage reads with a timeout
// to prevent request-path stalls" finding.
const collapseConfigGetTimeout = 2 * time.Second

// collapseConfigRefreshInterval is how often the background refresher polls
// storage for CollapseConfiguration/default. Deflate runs per-profile under
// load; the previous per-call Get stalled the request path, so the Get now
// happens off the request path and operator edits go live within one tick.
const collapseConfigRefreshInterval = 10 * time.Second

// collapseConfigurationKey is the in-storage key for the cluster-scoped
// CollapseConfiguration/default CR. It must match exactly the key the
// apiserver's REST endpoint writes the CR under, otherwise the provider's
// Get misses the applied CR and silently falls back to defaults.
//
// CollapseConfiguration is cluster-scoped (NamespaceScoped() == false in
// pkg/registry/softwarecomposition/collapseconfiguration/strategy.go), so
// the genericregistry NoNamespaceKeyFunc keys it as
// /<root>/<resource>/<name> with NO namespace segment. We use the
// cluster-scoped key helper rather than K8sKeysToPath, whose unconditional
// namespace segment would yield a stray empty segment for a cluster-scoped
// kind (/<root>/<resource>//<name>) that does not match the stored key.
func collapseConfigurationKey(name string) string {
	return K8sClusterScopedKeysToPath("", "spdx.softwarecomposition.kubescape.io", "collapseconfigurations", name)
}

// NewCRDCollapseSettingsProvider returns a CollapseSettingsProvider that
// reads the cluster-scoped CollapseConfiguration/<DefaultCollapseConfigurationName>
// and projects it via dynamicpathdetector.CollapseSettingsFromCRD, falling
// back to dynamicpathdetector.DefaultCollapseSettings when the CR is missing,
// unreadable, or storage is nil.
//
// The storage read is NEVER performed on the deflate (request) path: it runs
// in a bounded-timeout background refresher, and the returned closure only
// reads an atomically-published snapshot. This prevents the self-referential
// per-call Get from stalling the apiserver under load while keeping operator
// edits live within collapseConfigRefreshInterval.
func NewCRDCollapseSettingsProvider(s storage.Interface) dynamicpathdetector.CollapseSettingsProvider {
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
		err := s.Get(ctx, key, storage.GetOptions{IgnoreNotFound: true}, crd)
		var settings dynamicpathdetector.CollapseSettings
		if err != nil || crd.Name == "" {
			settings = dynamicpathdetector.DefaultCollapseSettings()
		} else {
			settings = dynamicpathdetector.CollapseSettingsFromCRD(crd)
		}
		current.Store(&settings)
	}

	refresh() // initial, bounded; falls back to default on timeout
	go func() {
		t := time.NewTicker(collapseConfigRefreshInterval)
		defer t.Stop()
		for range t.C {
			refresh()
		}
	}()

	return func() dynamicpathdetector.CollapseSettings {
		return *current.Load()
	}
}
