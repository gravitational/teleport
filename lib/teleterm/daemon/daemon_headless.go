/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package daemon

import (
	"context"
	"strings"
	"sync"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	api "github.com/gravitational/teleport/gen/proto/go/teleport/lib/teleterm/v1"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/teleterm/clusters"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// UpdateHeadlessAuthenticationState updates a headless authentication state.
func (s *Service) UpdateHeadlessAuthenticationState(ctx context.Context, rootClusterURI, headlessID string, state api.HeadlessAuthenticationState) error {
	cluster, _, err := s.ResolveCluster(rootClusterURI)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyClient, err := s.GetCachedClient(ctx, cluster.URI)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := cluster.UpdateHeadlessAuthenticationState(ctx, proxyClient.CurrentCluster(), headlessID, types.HeadlessAuthenticationState(state)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// StartHeadlessWatcher starts a headless watcher for the given cluster URI.
//
// If waitInit is true, this method will wait for the watcher to connect to the
// Auth Server and receive an OpInit event to indicate that the watcher is fully
// initialized and ready to catch headless events.
func (s *Service) StartHeadlessWatcher(rootClusterURI string, waitInit bool) error {
	s.headlessWatcherClosersMu.Lock()
	defer s.headlessWatcherClosersMu.Unlock()

	cluster, _, err := s.ResolveCluster(rootClusterURI)
	if err != nil {
		return trace.Wrap(err)
	}

	err = s.startHeadlessWatcher(cluster, waitInit)
	return trace.Wrap(err)
}

// startHeadlessWatcher starts a headless watcher for the given cluster.
//
// If waitInit is true, this method will wait for the watcher to connect to the
// Auth Server and receive an OpInit event to indicate that the watcher is fully
// initialized and ready to catch headless events.
func (s *Service) startHeadlessWatcher(rootCluster *clusters.Cluster, waitInit bool) error {
	// If there is already a watcher for this cluster, close and replace it.
	// This may occur after relogin, for example.
	if err := s.stopHeadlessWatcher(rootCluster.URI.String()); err != nil && !trace.IsNotFound(err) {
		return trace.Wrap(err)
	}

	maxBackoffDuration := defaults.MaxWatcherBackoff
	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  retryutils.FullJitter(maxBackoffDuration / 10),
		Step:   maxBackoffDuration / 5,
		Max:    maxBackoffDuration,
		Jitter: retryutils.HalfJitter,
		Clock:  s.cfg.Clock,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	watchCtx, watchCancel := context.WithCancel(s.closeContext)
	s.headlessWatcherClosers[rootCluster.URI.String()] = watchCancel

	log := s.cfg.Logger.With("cluster", logutils.StringerAttr(rootCluster.URI))

	pendingRequests := make(map[string]context.CancelFunc)
	pendingRequestsMu := sync.Mutex{}

	cancelPendingRequest := func(name string) {
		pendingRequestsMu.Lock()
		defer pendingRequestsMu.Unlock()
		if cancel, ok := pendingRequests[name]; ok {
			cancel()
		}
	}

	addPendingRequest := func(name string, cancel context.CancelFunc) {
		pendingRequestsMu.Lock()
		defer pendingRequestsMu.Unlock()
		pendingRequests[name] = cancel
	}

	pendingWatcherInitialized := make(chan struct{})
	pendingWatcherInitializedOnce := sync.Once{}

	watch := func() error {
		proxyClient, err := s.GetCachedClient(watchCtx, rootCluster.URI)
		if err != nil {
			return trace.Wrap(err)
		}
		authClient := proxyClient.CurrentCluster()

		pendingWatcher, closePendingWatcher, err := rootCluster.WatchPendingHeadlessAuthentications(watchCtx, authClient)
		if err != nil {
			return trace.Wrap(err)
		}
		defer closePendingWatcher()

		resolutionWatcher, closeResolutionWatcher, err := rootCluster.WatchHeadlessAuthentications(watchCtx, authClient)
		if err != nil {
			return trace.Wrap(err)
		}
		defer closeResolutionWatcher()

		// Wait for the pending watcher to finish initializing. the resolution watcher is not as critical,
		// so we skip waiting for it.

		select {
		case event := <-pendingWatcher.Events():
			if event.Type != types.OpInit {
				return trace.BadParameter("expected init event, got %v instead", event.Type)
			}
			pendingWatcherInitializedOnce.Do(func() { close(pendingWatcherInitialized) })
		case <-pendingWatcher.Done():
			return trace.Wrap(pendingWatcher.Error())
		case <-watchCtx.Done():
			return trace.Wrap(watchCtx.Err())
		}

		retry.Reset()

		for {
			select {
			case event := <-pendingWatcher.Events():
				// Ignore non-put events.
				if event.Type != types.OpPut {
					continue
				}

				ha, ok := event.Resource.(*types.HeadlessAuthentication)
				if !ok {
					return trace.Errorf("headless watcher returned an unexpected resource type %T", event.Resource)
				}

				// headless authentication requests will timeout after 3 minutes, so we can close the
				// Electron modal once this time is up.
				sendCtx, cancelSend := context.WithTimeout(s.closeContext, defaults.HeadlessLoginTimeout)

				// Add the pending request to the map so it is canceled early upon resolution.
				addPendingRequest(ha.GetName(), cancelSend)

				// Notify the Electron App of the pending headless authentication to handle resolution.
				// We do this in a goroutine so the watch loop can continue and cancel resolved requests.
				go func() {
					defer cancelSend()
					if err := s.sendPendingHeadlessAuthentication(sendCtx, ha, rootCluster.URI.String()); err != nil {
						if !strings.Contains(err.Error(), context.Canceled.Error()) && !strings.Contains(err.Error(), context.DeadlineExceeded.Error()) {
							log.DebugContext(sendCtx, "sendPendingHeadlessAuthentication resulted in unexpected error", "error", err)
						}
					}
				}()
			case event := <-resolutionWatcher.Events():
				// Watch for pending headless authentications to be approved, denied, or deleted (canceled/timeout).
				switch event.Type {
				case types.OpPut:
					ha, ok := event.Resource.(*types.HeadlessAuthentication)
					if !ok {
						return trace.Errorf("headless watcher returned an unexpected resource type %T", event.Resource)
					}

					switch ha.State {
					case types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_APPROVED, types.HeadlessAuthenticationState_HEADLESS_AUTHENTICATION_STATE_DENIED:
						cancelPendingRequest(ha.GetName())
					}
				case types.OpDelete:
					cancelPendingRequest(event.Resource.GetName())
				}
			case <-pendingWatcher.Done():
				return trace.Wrap(pendingWatcher.Error(), "pending watcher error")
			case <-resolutionWatcher.Done():
				return trace.Wrap(resolutionWatcher.Error(), "resolution watcher error")
			case <-watchCtx.Done():
				return nil
			}
		}
	}

	log.DebugContext(watchCtx, "Starting headless watch loop")
	go func() {
		defer func() {
			s.headlessWatcherClosersMu.Lock()
			defer s.headlessWatcherClosersMu.Unlock()

			select {
			case <-watchCtx.Done():
				// watcher was canceled by an outside call to stopHeadlessWatcher.
			default:
				// watcher closed due to error or cluster disconnect.
				if err := s.stopHeadlessWatcher(rootCluster.URI.String()); err != nil {
					log.DebugContext(watchCtx, "Failed to remove headless watcher", "error", err)
				}
			}
		}()

		for {
			if !rootCluster.Connected() {
				log.DebugContext(watchCtx, "Not connected to cluster, terminating headless watch loop")
				return
			}

			err := watch()
			if trace.IsNotImplemented(err) {
				// Don't retry watch if we are connecting to an old Auth Server.
				log.DebugContext(watchCtx, "Headless watcher not supported", "error", err)
				return
			}

			startedWaiting := s.cfg.Clock.Now()
			select {
			case t := <-retry.After():
				log.DebugContext(watchCtx, "Restarting watch on error",
					"backoff", t.Sub(startedWaiting),
					"error", err,
				)
				retry.Inc()
			case <-watchCtx.Done():
				log.DebugContext(watchCtx, "Context closed with error, ending headless watch loop",
					"error", watchCtx.Err(),
				)
				return
			}
		}
	}()

	if waitInit {
		select {
		case <-pendingWatcherInitialized:
		case <-watchCtx.Done():
			return trace.Wrap(watchCtx.Err())
		}
	}

	return nil
}

// sendPendingHeadlessAuthentication notifies the Electron App of a pending headless authentication.
func (s *Service) sendPendingHeadlessAuthentication(ctx context.Context, ha *types.HeadlessAuthentication, rootClusterURI string) error {
	req := &api.SendPendingHeadlessAuthenticationRequest{
		RootClusterUri:                 rootClusterURI,
		HeadlessAuthenticationId:       ha.GetName(),
		HeadlessAuthenticationClientIp: ha.ClientIpAddress,
	}

	if err := s.headlessAuthSemaphore.Acquire(ctx); err != nil {
		return trace.Wrap(err)
	}
	defer s.headlessAuthSemaphore.Release()

	_, err := s.tshdEventsClient.SendPendingHeadlessAuthentication(ctx, req)
	return trace.Wrap(err)
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
			s.cfg.Logger.DebugContext(s.closeContext, "Encountered unexpected error closing headless watcher",
				"error", err,
				"cluster", uri,
			)
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
