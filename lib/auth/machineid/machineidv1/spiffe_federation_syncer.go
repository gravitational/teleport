/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package machineidv1

import (
	"context"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/spiffe/go-spiffe/v2/bundle/spiffebundle"
	"github.com/spiffe/go-spiffe/v2/federation"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"go.opentelemetry.io/otel"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/lib/backend"
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/auth/machineid/v1")

type spiffeFederationStore interface {
	ListSPIFFEFederations(ctx context.Context, limit int, token string) ([]*machineidv1.SPIFFEFederation, string, error)
	GetSPIFFEFederation(ctx context.Context, name string) (*machineidv1.SPIFFEFederation, error)
	UpdateSPIFFEFederation(ctx context.Context, federation *machineidv1.SPIFFEFederation) (*machineidv1.SPIFFEFederation, error)
}

type eventsWatcher interface {
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
}

func listAllTrustDomains(ctx context.Context, store spiffeFederationStore) ([]*machineidv1.SPIFFEFederation, error) {
	var trustDomains []*machineidv1.SPIFFEFederation
	var token string
	for {
		tds, nextToken, err := store.ListSPIFFEFederations(ctx, 100, token)
		if err != nil {
			return nil, trace.Wrap(err, "failed to list trust domains")
		}
		trustDomains = append(trustDomains, tds...)
		if nextToken == "" {
			break
		}
		token = nextToken
	}
	return trustDomains, nil
}

// SPIFFEFederationSyncerConfig is the configuration for the SPIFFE federation syncer.
type SPIFFEFederationSyncerConfig struct {
	Backend       backend.Backend
	Store         spiffeFederationStore
	EventsWatcher eventsWatcher
	Logger        *slog.Logger
	Clock         clockwork.Clock

	// MinSyncInterval is the minimum interval between syncs. If an upstream trust domain specifies a refresh hint
	// that is less than this value, this value will be used instead. This allows us to prevent a poorly configured
	// upstream trust domain from causing excessive load on the local cluster.
	MinSyncInterval time.Duration
	// MaxSyncInterval is the maximum interval between syncs. If an upstream trust domain specifies a refresh hint
	// that is greater than this value, this value will be used instead. This allows us to prevent a poorly configured
	// upstream trust domain from causing excessive staleness in the local cluster.
	MaxSyncInterval time.Duration
	// DefaultSyncInterval is the interval between syncs that will be used if an upstream trust domain does not specify
	// a refresh hint.
	DefaultSyncInterval time.Duration

	SPIFFEFetchOptions []federation.FetchOption
}

// SPIFFEFederationSyncer is a syncer that manages the trust bundles of federated clusters.
// It runs on a single elected auth server.
type SPIFFEFederationSyncer struct {
	cfg SPIFFEFederationSyncerConfig
}

const (
	minRefreshInterval     = time.Minute * 1
	maxRefreshInterval     = time.Hour * 24
	defaultRefreshInterval = time.Minute * 5
)

// NewSPIFFEFederationSyncer creates a new SPIFFEFederationSyncer.
func NewSPIFFEFederationSyncer(cfg SPIFFEFederationSyncerConfig) (*SPIFFEFederationSyncer, error) {
	switch {
	case cfg.Backend == nil:
		return nil, trace.BadParameter("backend: must be non-nil")
	case cfg.Store == nil:
		return nil, trace.BadParameter("store: must be non-nil")
	case cfg.Logger == nil:
		return nil, trace.BadParameter("logger: must be non-nil")
	case cfg.Clock == nil:
		return nil, trace.BadParameter("clock: must be non-nil")
	}
	if cfg.MinSyncInterval == 0 {
		cfg.MinSyncInterval = minRefreshInterval
	}
	if cfg.MaxSyncInterval == 0 {
		cfg.MaxSyncInterval = maxRefreshInterval
	}
	if cfg.DefaultSyncInterval == 0 {
		cfg.DefaultSyncInterval = defaultRefreshInterval
	}
	return &SPIFFEFederationSyncer{
		cfg: cfg,
	}, nil
}

func (s *SPIFFEFederationSyncer) Run(ctx context.Context) error {
	// TODO: Should this go into a loop?
	return backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
		// TODO: Evaluate sensible TTL/retry
		LockConfiguration: backend.LockConfiguration{
			Backend:       s.cfg.Backend,
			LockName:      "spiffe_federation_syncer",
			TTL:           time.Minute,
			RetryInterval: time.Minute,
		},
		RefreshLockInterval: time.Second * 15,
	}, s.runWhileLocked)
}

type spiffeFederationState struct {
	// eventsCh is an unbuffered channel for passing events to a specific
	// SPIFFEFederations syncer.
	eventsCh chan types.Event
}

// runWhileLocked is the core loop of the syncer that runs on a single auth
// server.
//
// Its goal is to sync the contents of trust bundles from remote clusters to
// the local cluster. It does this by creating a goroutine that manages each
// federated cluster.
func (s *SPIFFEFederationSyncer) runWhileLocked(ctx context.Context) error {
	// This wg will track all active syncers. We'll wait here until we're done.
	wg := &sync.WaitGroup{}
	defer func() {
		wg.Wait()
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up state management that will hold a list of all active trust domain syncers.
	states := map[string]spiffeFederationState{}
	mu := &sync.Mutex{}
	startSyncingFederation := func(trustDomain string) {
		mu.Lock()
		defer mu.Unlock()

		// Don't start if we're already syncing this trust domain.
		_, ok := states[trustDomain]
		if ok {
			return
		}

		states[trustDomain] = spiffeFederationState{
			eventsCh: make(chan types.Event),
		}

		wg.Add(1)
		go func() {
			defer func() {
				mu.Lock()
				delete(states, trustDomain)
				mu.Unlock()
				wg.Done()
			}()
			s.syncFederationLoop(ctx, trustDomain, states[trustDomain].eventsCh)
		}()
	}

	// Establish our watcher, we'll use this to react instantly to changes to SPIFFEFederations.
	w, err := s.cfg.EventsWatcher.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{
			Kind: types.KindSPIFFEFederation,
		}},
	})
	if err != nil {
		return trace.Wrap(err, "failed to create watcher")
	}
	defer func(w types.Watcher) {
		err := w.Close()
		if err != nil {
			s.cfg.Logger.ErrorContext(ctx, "Failed to close watcher", "error", err)
		}
	}(w)

	// Wait for initial "Init" event to indicate we're now receiving events.
	select {
	case evt := <-w.Events():
		if evt.Type == types.OpInit {
			break
		}
		return trace.BadParameter("expected init event, got %v", evt.Type)
	case <-ctx.Done():
		return nil
	}

	// Fetch an initial list of all federations and start syncers for them.
	trustDomains, err := listAllTrustDomains(ctx, s.cfg.Store)
	if err != nil {
		return trace.Wrap(err, "initially listing trust domains")
	}
	for _, td := range trustDomains {
		startSyncingFederation(td.GetMetadata().GetName())
	}

	// Now we can start reacting to events, we'll want to start/stop syncers as needed.
	// We'll want to start a syncer for any new trust domain, and propagate events to existing syncers.
	for {
		select {
		case evt := <-w.Events():
			mu.Lock()
			existingState, ok := states[evt.Resource.GetName()]
			if ok {
				existingState.eventsCh <- evt
			} else {
				startSyncingFederation(evt.Resource.GetName())
			}
			mu.Unlock()
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *SPIFFEFederationSyncer) syncFederationLoop(
	ctx context.Context,
	name string,
	eventsCh <-chan types.Event,
) {
	log := s.cfg.Logger.With("trust_domain", name)
	log.InfoContext(ctx, "Starting to manage syncing of SPIFFEFederation", "trust_domain", name)
	defer func() {
		log.InfoContext(ctx, "Stopped managing syncing of SPIFFEFederation", "trust_domain", name)
	}()

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  time.Second,
		Step:   time.Second,
		Max:    time.Second * 10,
		Clock:  s.cfg.Clock,
		Jitter: retryutils.NewSeventhJitter(),
	})
	if err != nil {
		log.ErrorContext(
			ctx,
			"Failed to create retry strategy, federation syncer will not run",
			"error", err,
		)
		return
	}

	var synced *machineidv1.SPIFFEFederation
	nextSync := s.cfg.Clock.NewTimer(time.Minute)
	nextSync.Stop()
	defer nextSync.Stop()
	firstRun := make(chan struct{})
	firstRun <- struct{}{}
	for {
		// Default behaviour of retry is to return a closed channel if duration
		// is zero, but it makes more sense here to have a channel that will
		// block forever.
		var nextRetry <-chan time.Time
		if err != nil {
			nextRetry = retry.After()
		}
		select {
		case <-firstRun:
			// On the first run, we should sync immediately.
			log.DebugContext(ctx, "First run, trying sync immediately")
		case <-nextSync.Chan():
			log.DebugContext(ctx, "Next sync time has passed, trying sync")
		case <-nextRetry:
			log.InfoContext(ctx, "Wait for backoff complete, retrying sync")
		case evt := <-eventsCh:
			if evt.Type == types.OpDelete {
				log.DebugContext(ctx, "Resource has been deleted, stopping sync loop")
				// If we've been deleted, we should stop syncing.
				return
			}
			// If we've just synced, we can effectively expect an "echo" of our
			// last update. We can ignore this event safely.
			if synced != nil {
				if evt.Resource.GetRevision() == synced.GetMetadata().GetRevision() {
					continue
				}
				log.DebugContext(ctx, "Resource has been updated, trying sync immediately")
			}
			// Note, we explicitly don't use the resource within the event.
			// Instead, we will fetch the latest upon starting the sync. This
			// avoids completing multiple syncs if multiple changes are queued.
		case <-ctx.Done():
			return
		}

		synced, err = s.syncFederation(ctx, name)
		if err != nil {
			// If the resource has been deleted, there's no point retrying.
			// We should stop syncing.
			if trace.IsNotFound(err) {
				log.ErrorContext(
					ctx,
					"Resource has been deleted, stopping sync",
					"trust_domain", name,
					"error", err,
				)
				return
			}
			retry.Inc()
			log.ErrorContext(
				ctx,
				"Failed to sync federation, will retry after backoff",
				"error", err,
				"backoff", retry.Duration(),
			)
		} else {
			retry.Reset()
		}

		nextSync.Stop()
		if synced != nil {
			if nextSyncAt := synced.GetStatus().GetNextSyncAt().AsTime(); !nextSyncAt.IsZero() {
				timeUntil := nextSyncAt.Sub(s.cfg.Clock.Now())
				// Ensure the timer after /after/ the next sync time.
				timeUntil = timeUntil + time.Second
				nextSync.Reset(timeUntil)
				log.InfoContext(ctx, "Will try to sync again at", "next_sync_at", nextSyncAt)
			}
		}
	}
}

func shouldSyncFederation(
	ctx context.Context,
	log *slog.Logger,
	clock clockwork.Clock,
	in *machineidv1.SPIFFEFederation,
) string {
	if in.Status == nil {
		log.DebugContext(ctx, "No status, will sync")
		return "no_status"
	}
	if in.Status.CurrentBundle == "" {
		log.DebugContext(ctx, "No status.current_bundle, will sync")
		return "no_current_bundle"
	}
	if in.Status.CurrentBundleSyncedAt.AsTime().IsZero() {
		log.DebugContext(ctx, "No status.current_bundle_synced_at, will sync")
		return "no_current_bundle_synced_at"
	}
	// Check if we've passed the next sync time.
	nextSyncAt := in.Status.NextSyncAt.AsTime()
	now := clock.Now()
	if !nextSyncAt.IsZero() && now.After(nextSyncAt) {
		log.DebugContext(
			ctx,
			"status.next_sync_at has passed, will sync",
			"next_sync_at", nextSyncAt,
			"now", now,
		)
		return "next_sync_at_passed"
	}
	// Check to see if the configured bundle source has changed
	if in.Status.CurrentBundleSyncedFrom != nil {
		if !proto.Equal(in.Spec.BundleSource, in.Status.CurrentBundleSyncedFrom) {
			log.DebugContext(ctx, "status.current_bundle_synced_from has changed, will sync")
			return "bundle_source_changed"
		}
	}

	return ""
}

func (s *SPIFFEFederationSyncer) syncFederation(
	ctx context.Context, name string,
) (out *machineidv1.SPIFFEFederation, err error) {
	ctx, span := tracer.Start(ctx, "SPIFFEFederationSyncer/syncFederation")
	defer func() {
		tracing.EndSpan(span, err)
	}()

	current, err := s.cfg.Store.GetSPIFFEFederation(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get SPIFFE federation")
	}

	log := s.cfg.Logger.With(
		"current_revision", current.GetMetadata().GetRevision(),
		"trust_domain", current.GetMetadata().GetName(),
	)

	td, err := spiffeid.TrustDomainFromString(current.GetMetadata().GetName())
	if err != nil {
		log.ErrorContext(ctx, "Invalid trust domain name", "error", err)
		return nil, trace.Wrap(err)
	}

	// Determine - should we refresh...
	syncReason := shouldSyncFederation(ctx, log, s.cfg.Clock, current)
	if syncReason == "" {
		log.DebugContext(ctx, "Skipping sync as is not required")
		return current, nil
	}
	log.InfoContext(ctx, "Sync starting", "reason", syncReason)

	// Clone the persisted resource so we can compare to it.
	out = proto.Clone(current).(*machineidv1.SPIFFEFederation)

	// Refresh...
	if out.Status == nil {
		out.Status = &machineidv1.SPIFFEFederationStatus{}
	}

	var bundle *spiffebundle.Bundle
	var nextSyncIn time.Duration
	switch {
	case current.Spec.BundleSource.HttpsWeb != nil:
		url := current.Spec.BundleSource.HttpsWeb.BundleEndpointUrl
		log.DebugContext(
			ctx,
			"Fetching bundle using https_web profile",
			"url", url,
		)
		bundle, err = federation.FetchBundle(ctx, td, url, s.cfg.SPIFFEFetchOptions...)
		if err != nil {
			return nil, trace.Wrap(err, "fetching bundle using https_web profile")
		}

		// Calculate the duration before we should next sync, applying any sensible upper and lower bounds.
		nextSyncIn = s.cfg.DefaultSyncInterval
		if refreshHint, ok := bundle.RefreshHint(); ok {
			if refreshHint < s.cfg.MinSyncInterval {
				log.InfoContext(ctx, "Refresh hint is less than MinSyncInterval, using MinSyncInterval", "refresh_hint", refreshHint)
				nextSyncIn = s.cfg.MinSyncInterval
			} else if refreshHint > s.cfg.MaxSyncInterval {
				log.InfoContext(ctx, "Refresh hint is greater than MaxSyncInterval, using MaxSyncInterval", "refresh_hint", refreshHint)
				nextSyncIn = s.cfg.MaxSyncInterval
			} else {
				nextSyncIn = refreshHint
			}
		}
	case current.Spec.BundleSource.Static != nil:
		log.DebugContext(ctx, "Fetching bundle using spec.bundle_source.static.bundle")
		bundle, err = spiffebundle.Parse(td, []byte(current.Spec.BundleSource.Static.Bundle))
		if err != nil {
			return nil, trace.Wrap(err, "parsing bundle from static profile")
		}
	default:
		return nil, trace.BadParameter("spec.bundle_source: at least one of https_web or static must be set")
	}

	bundleBytes, err := bundle.Marshal()
	if err != nil {
		return nil, trace.Wrap(err, "marshalling bundle")
	}
	out.Status.CurrentBundle = string(bundleBytes)
	out.Status.CurrentBundleSyncedFrom = current.Spec.BundleSource

	syncedAt := s.cfg.Clock.Now().UTC()
	out.Status.CurrentBundleSyncedAt = timestamppb.New(syncedAt)
	// For certain sources, we need to set a next sync time.
	if nextSyncIn > 0 {
		out.Status.NextSyncAt = timestamppb.New(syncedAt.Add(nextSyncIn))
	}

	out, err = s.cfg.Store.UpdateSPIFFEFederation(ctx, out)
	if err != nil {
		return nil, trace.Wrap(err, "persisting updated SPIFFE federation")
	}
	log.InfoContext(
		ctx,
		"Sync finished",
		"new_revision", out.Metadata.Revision,
	)

	return out, nil
}
