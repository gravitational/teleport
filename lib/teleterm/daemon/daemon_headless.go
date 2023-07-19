// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package daemon

import (
	"context"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	"github.com/gravitational/teleport/lib/utils"
)

// UpdateHeadlessAuthenticationState updates a headless authentication state.
func (s *Service) UpdateHeadlessAuthenticationState(ctx context.Context, clusterURI, headlessID string, state api.HeadlessAuthenticationState) error {
	cluster, err := s.ResolveCluster(clusterURI)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := cluster.UpdateHeadlessAuthenticationState(ctx, headlessID, types.HeadlessAuthenticationState(state)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// StartHeadlessHandlers starts a headless watcher for the given cluster URI.
func (s *Service) StartHeadlessWatcher(uri string) error {
	s.headlessWatcherClosersMu.Lock()
	defer s.headlessWatcherClosersMu.Unlock()

	cluster, err := s.ResolveCluster(uri)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.startHeadlessWatcher(cluster)
	return trace.Wrap(err)
}

// StartHeadlessWatchers starts headless watchers for all connected clusters.
func (s *Service) StartHeadlessWatchers() error {
	s.headlessWatcherClosersMu.Lock()
	defer s.headlessWatcherClosersMu.Unlock()

	clusters, err := s.cfg.Storage.ReadAll()
	if err != nil {
		return trace.Wrap(err)
	}

	for _, c := range clusters {
		if c.Connected() {
			if err := s.startHeadlessWatcher(c); err != nil {
				return trace.Wrap(err)
			}
		}
	}

	return nil
}

// startHeadlessWatcher starts a background process to watch for pending headless
// authentications for this cluster.
func (s *Service) startHeadlessWatcher(cluster *clusters.Cluster) error {
	// If there is already a watcher for this cluster, close and replace it.
	// This may occur after relogin, for example.
	if err := s.stopHeadlessWatcher(cluster.URI.String()); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  utils.FullJitter(defaults.HighResPollingPeriod / 10),
		Step:   defaults.HighResPollingPeriod / 5,
		Max:    defaults.HighResPollingPeriod,
		Jitter: retryutils.NewHalfJitter(),
		Clock:  s.cfg.Storage.Clock,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	watchCtx, watchCancel := context.WithCancel(s.closeContext)
	s.headlessWatcherClosers[cluster.URI.String()] = watchCancel

	watch := func() error {
		watcher, closeWatcher, err := cluster.WatchPendingHeadlessAuthentications(watchCtx)
		if err != nil {
			return trace.Wrap(err)
		}
		retry.Reset()

		defer closeWatcher()
		for {
			select {
			case event := <-watcher.Events():
				// Ignore non-put events.
				if event.Type != types.OpPut {
					continue
				}

				ha, ok := event.Resource.(*types.HeadlessAuthentication)
				if !ok {
					return trace.Errorf("headless watcher returned an unexpected resource type %T", event.Resource)
				}

				// Notify the Electron App of the pending headless authentication to handle resolution.
				req := &api.SendPendingHeadlessAuthenticationRequest{
					RootClusterUri:                 cluster.URI.String(),
					HeadlessAuthenticationId:       ha.GetName(),
					HeadlessAuthenticationClientIp: ha.ClientIpAddress,
				}

				if _, err := s.tshdEventsClient.SendPendingHeadlessAuthentication(watchCtx, req); err != nil {
					return trace.Wrap(err)
				}
			case <-watcher.Done():
				return trace.Wrap(err)
			case <-watchCtx.Done():
				return nil
			}
		}
	}

	log := s.cfg.Log.WithField("cluster", cluster.URI.String())
	log.Debugf("Starting headless watch loop.")
	go func() {
		defer watchCancel()
		for {
			if !cluster.Connected() {
				log.Debugf("Not connected to cluster. Returning from headless watch loop.")
				if err := s.StopHeadlessWatcher(cluster.URI.String()); err != nil {
					log.WithError(err).Debugf("Failed to remove headless watcher.")
				}
				return
			}

			err := watch()

			startedWaiting := s.cfg.Storage.Clock.Now()
			select {
			case t := <-retry.After():
				log.WithError(err).Debugf("Restarting watch on error after waiting %v.", t.Sub(startedWaiting))
				retry.Inc()
			case <-watchCtx.Done():
				log.WithError(watchCtx.Err()).Debugf("Context closed with err. Returning from headless watch loop.")
				return
			}
		}
	}()

	return nil
}

// StopHeadlessWatcher stops the headless watcher for the given cluster URI.
func (s *Service) StopHeadlessWatcher(uri string) error {
	s.headlessWatcherClosersMu.Lock()
	defer s.headlessWatcherClosersMu.Unlock()

	return trace.Wrap(s.stopHeadlessWatcher(uri))
}

// StopHeadlessWatchers stops all headless watchers.
func (s *Service) StopHeadlessWatchers() {
	s.headlessWatcherClosersMu.Lock()
	defer s.headlessWatcherClosersMu.Unlock()

	for uri := range s.headlessWatcherClosers {
		if err := s.stopHeadlessWatcher(uri); err != nil {
			s.cfg.Log.WithField("cluster", uri).WithError(err).Debug("Encountered unexpected error closing headless watcher")
		}
	}
}

func (s *Service) stopHeadlessWatcher(uri string) error {
	if _, ok := s.headlessWatcherClosers[uri]; !ok {
		return trace.NotFound("no headless watcher for cluster %v", uri)
	}

	s.headlessWatcherClosers[uri]()
	delete(s.headlessWatcherClosers, uri)
	return nil
}
