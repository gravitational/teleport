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
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
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

func listAllTrustDomains(
	ctx context.Context, store spiffeFederationStore,
) ([]*machineidv1.SPIFFEFederation, error) {
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

// SPIFFEFederationSyncerConfig is the configuration for the SPIFFE federation
// syncer.
type SPIFFEFederationSyncerConfig struct {
	// Backend should be a backend.Backend which can be used for obtaining the
	// lock required to run the syncer.
	Backend backend.Backend
	// Store is where the SPIFFEFederation resources can be fetched and updated.
	Store spiffeFederationStore
	// EventsWatcher is used to watch for changes to SPIFFEFederations.
	EventsWatcher eventsWatcher
	// Logger is the logger that the syncer will use.
	Logger *slog.Logger
	// Clock is the clock that the syncer will use.
	Clock clockwork.Clock

	// MinSyncInterval is the minimum interval between syncs. If an upstream
	// trust domain specifies a refresh hint that is less than this value, this
	// value will be used instead. This allows us to prevent a poorly configured
	// upstream trust domain from causing excessive load on the local cluster.
	MinSyncInterval time.Duration
	// MaxSyncInterval is the maximum interval between syncs. If an upstream
	// trust domain specifies a refresh hint that is greater than this value,
	// this value will be used instead. This allows us to prevent a poorly
	// configured upstream trust domain from causing excessive staleness in the
	// local cluster.
	MaxSyncInterval time.Duration
	// DefaultSyncInterval is the interval between syncs that will be used if an
	// upstream trust domain does not specify a refresh hint.
	DefaultSyncInterval time.Duration

	// SyncTimeout is the maximum time that a sync operation is allowed to take.
	// If a sync operation takes longer than this value, it will be aborted and
	// retried.
	SyncTimeout time.Duration

	// SPIFFEFetchOptions are the options that will be used when fetching a
	// trust bundle from a remote cluster. These options will be passed to the
	// spiffebundle.FetchBundle function. This is usually used during testing
	// to override the Web PKI CAs.
	SPIFFEFetchOptions []federation.FetchOption
}

// CheckAndSetDefaults checks the configuration and sets defaults where
// necessary.
func (c *SPIFFEFederationSyncerConfig) CheckAndSetDefaults() error {
	switch {
	case c.Backend == nil:
		return trace.BadParameter("backend: must be non-nil")
	case c.Store == nil:
		return trace.BadParameter("store: must be non-nil")
	case c.Logger == nil:
		return trace.BadParameter("logger: must be non-nil")
	case c.Clock == nil:
		return trace.BadParameter("clock: must be non-nil")
	}
	if c.MinSyncInterval == 0 {
		c.MinSyncInterval = minRefreshInterval
	}
	if c.MaxSyncInterval == 0 {
		c.MaxSyncInterval = maxRefreshInterval
	}
	if c.DefaultSyncInterval == 0 {
		c.DefaultSyncInterval = defaultRefreshInterval
	}
	if c.SyncTimeout == 0 {
		c.SyncTimeout = defaultSyncTimeout
	}
	return nil
}

// SPIFFEFederationSyncer is a syncer that manages the trust bundles of
// federated clusters. It runs on a single elected auth server.
type SPIFFEFederationSyncer struct {
	cfg SPIFFEFederationSyncerConfig
}

const (
	minRefreshInterval = time.Minute * 1
	maxRefreshInterval = time.Hour * 24
	// SPIFFE Federation (4.1):
	// > If not set, a reasonably low default value should apply - five minutes
	// > is recommended
	defaultRefreshInterval = time.Minute * 5
	defaultSyncTimeout     = time.Second * 30
)

// NewSPIFFEFederationSyncer creates a new SPIFFEFederationSyncer.
func NewSPIFFEFederationSyncer(cfg SPIFFEFederationSyncerConfig) (*SPIFFEFederationSyncer, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err, "validating SPIFFE federation syncer config")
	}
	return &SPIFFEFederationSyncer{
		cfg: cfg,
	}, nil
}

func (s *SPIFFEFederationSyncer) Run(ctx context.Context) error {
	// Loop to retry if acquiring lock fails, with some backoff to avoid pinning
	// the CPU.
	waitWithJitter := retryutils.SeventhJitter(time.Second * 10)
	for {
		err := backend.RunWhileLocked(ctx, backend.RunWhileLockedConfig{
			LockConfiguration: backend.LockConfiguration{
				Backend:            s.cfg.Backend,
				LockNameComponents: []string{"spiffe_federation_syncer"},
				TTL:                time.Minute,
				// It doesn't matter too much if the syncer isn't running for
				// a short period of time so we can take a relaxed approach to
				// retrying to grab the lock.
				RetryInterval: time.Second * 30,
			},
		}, s.syncTrustDomains)
		if err != nil {
			s.cfg.Logger.ErrorContext(
				ctx,
				"SPIFFEFederation syncer encountered a fatal error, it will restart after backoff",
				"error", err,
				"restart_after", waitWithJitter,
			)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(waitWithJitter):
		}
	}
}

type trustDomainSyncState struct {
	// putEventsCh is a channel for passing PUT events to a specific
	// SPIFFEFederations syncer.
	putEventsCh chan types.Event
	// stopCh is a channel for signaling a specific SPIFFEFederations
	// syncer to stop syncing. This is closed when the watcher detects that the
	// resource has been deleted.
	stopCh chan struct{}
}

// syncTrustDomains is the core loop of the syncer that runs on a single auth
// server.
//
// Its goal is to sync the contents of trust bundles from remote clusters to
// the local cluster. It does this by creating a goroutine that manages each
// federated cluster.
func (s *SPIFFEFederationSyncer) syncTrustDomains(ctx context.Context) error {
	s.cfg.Logger.InfoContext(
		ctx,
		"Obtained lock, SPIFFEFederation syncer is starting",
	)
	defer func() {
		s.cfg.Logger.InfoContext(
			ctx, "SPIFFEFederation syncer has stopped",
		)
	}()

	// This wg will track all active syncers. We'll wait here until we're done.
	wg := &sync.WaitGroup{}
	defer func() {
		wg.Wait()
	}()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Set up state management that will hold a list of all active trust domain
	// syncers.
	states := map[string]trustDomainSyncState{}
	mu := &sync.Mutex{}
	startSyncingFederation := func(trustDomain string) {
		mu.Lock()
		defer mu.Unlock()

		// Don't start if we're already syncing this trust domain.
		_, ok := states[trustDomain]
		if ok {
			return
		}

		eventsCh := make(chan types.Event, 1)
		stopCh := make(chan struct{})
		states[trustDomain] = trustDomainSyncState{
			putEventsCh: eventsCh,
			stopCh:      stopCh,
		}

		wg.Add(1)
		go func() {
			defer func() {
				mu.Lock()
				delete(states, trustDomain)
				mu.Unlock()
				wg.Done()
			}()
			s.syncTrustDomainLoop(ctx, trustDomain, eventsCh, stopCh)
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
	defer func() {
		err := w.Close()
		if err != nil {
			s.cfg.Logger.ErrorContext(
				ctx, "Failed to close watcher", "error", err,
			)
		}
	}()

	// Wait for initial "Init" event to indicate we're now receiving events.
	select {
	case <-w.Done():
		if err := w.Error(); err != nil {
			return trace.Wrap(err, "watcher failed")
		}
		return trace.BadParameter("watcher closed unexpectedly")
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

	// Now we can start reacting to events, we'll want to start/stop syncers as
	// needed. We'll want to start a syncer for any new trust domain, and
	// propagate events to existing syncers.
	for {
		select {
		case evt := <-w.Events():
			s.cfg.Logger.DebugContext(
				ctx,
				"Received event from SPIFFEFederation watcher",
				"evt_type", evt.Type,
			)
			switch evt.Type {
			case types.OpPut:
				mu.Lock()
				existingState, ok := states[evt.Resource.GetName()]
				mu.Unlock()
				// If it already exists, we can just pass the event along. If
				// there's already a sync queued due to an event, we don't need to
				// queue another since it fetches the latest resource anyway.
				if ok {
					select {
					case existingState.putEventsCh <- evt:
					default:
						s.cfg.Logger.DebugContext(
							ctx,
							"Sync already queued for trust domain, ignoring event",
						)
					}
					continue
				}
				// If it doesn't exist, we should kick off a goroutine to start
				// managing it. That routine will sync automatically on first
				// run so we don't need to pass the event along.
				startSyncingFederation(evt.Resource.GetName())
			case types.OpDelete:
				mu.Lock()
				existingState, ok := states[evt.Resource.GetName()]
				// If it exists, close the stopCh to tell it to exit and remove
				// it from the states map.
				if ok {
					close(existingState.stopCh)
					delete(states, evt.Resource.GetName())
				}
				mu.Unlock()
			default:
			}
		case <-w.Done():
			if err := w.Error(); err != nil {
				return trace.Wrap(err, "watcher failed")
			}
			return trace.BadParameter("watcher closed unexpectedly")
		case <-ctx.Done():
			return nil
		}
	}
}

func (s *SPIFFEFederationSyncer) syncTrustDomainLoop(
	ctx context.Context,
	name string,
	putEventsCh <-chan types.Event,
	stopCh <-chan struct{},
) {
	log := s.cfg.Logger.With("trust_domain", name)
	log.InfoContext(ctx, "Starting sync loop for trust domain")
	defer func() {
		log.InfoContext(ctx, "Stopped sync loop for trust domain")
	}()

	retry, err := retryutils.NewLinear(retryutils.LinearConfig{
		First:  time.Second,
		Step:   time.Second,
		Max:    time.Second * 10,
		Clock:  s.cfg.Clock,
		Jitter: retryutils.SeventhJitter,
	})
	if err != nil {
		log.ErrorContext(
			ctx,
			"Failed to create retry strategy, trust domain sync loop will not run",
			"error", err,
		)
		return
	}

	var synced *machineidv1.SPIFFEFederation
	var nextRetry <-chan time.Time
	nextSync := s.cfg.Clock.NewTimer(time.Minute)
	nextSync.Stop()
	defer nextSync.Stop()
	firstRun := make(chan struct{}, 1)
	firstRun <- struct{}{}
	for {
		select {
		case <-firstRun:
			// On the first run, we should sync immediately.
			log.DebugContext(ctx, "First run, trying sync immediately")
		case <-nextSync.Chan():
			log.DebugContext(ctx, "Next sync time has passed, trying sync")
		case <-nextRetry:
			log.InfoContext(ctx, "Wait for backoff complete, retrying sync")
		case evt := <-putEventsCh:
			// If we've just synced, we can effectively expect an "echo" of our
			// last update. We can ignore this event safely.
			if synced != nil {
				if evt.Resource.GetRevision() == synced.GetMetadata().GetRevision() {
					continue
				}
				log.DebugContext(
					ctx,
					"Resource has been updated, trying to sync trust domain immediately",
				)
			}
		// Note, we explicitly don't use the resource within the event.
		// Instead, we will fetch the latest upon starting the sync. This
		// avoids completing multiple syncs if multiple changes are queued.
		case <-stopCh:
			log.DebugContext(ctx, "Stop signal received, stopping sync loop")
			return
		case <-ctx.Done():
			return
		}
		// Stop our sync timer, we'll only set it if we successfully sync.
		nextSync.Stop()

		syncCtx, cancel := context.WithTimeout(ctx, s.cfg.SyncTimeout)
		synced, err = s.syncTrustDomain(syncCtx, name)
		cancel()

		if err != nil {
			// If the resource has been deleted, there's no point retrying.
			// We should stop syncing.
			if trace.IsNotFound(err) {
				log.ErrorContext(
					ctx,
					"Resource has been deleted, stopping sync loop for trust domain",
					"error", err,
				)
				return
			}
			retry.Inc()
			log.ErrorContext(
				ctx,
				"Failed to sync trust domain, will retry after backoff",
				"error", err,
				"backoff", retry.Duration(),
			)
			nextRetry = retry.After()
			continue
		}
		retry.Reset()
		nextRetry = nil

		// If we've successfully synced, set the timer up for our next sync.
		if nextSyncAt := synced.GetStatus().GetNextSyncAt().AsTime(); !nextSyncAt.IsZero() {
			timeUntil := nextSyncAt.Sub(s.cfg.Clock.Now())
			// Ensure the timer will tick /after/ the next sync time.
			timeUntil = timeUntil + time.Second
			nextSync.Reset(timeUntil)
			log.InfoContext(
				ctx,
				"Waiting to sync again",
				"next_sync_at", nextSyncAt,
			)
		}
	}
}

func (s *SPIFFEFederationSyncer) shouldSyncTrustDomain(
	ctx context.Context,
	log *slog.Logger,
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
	now := s.cfg.Clock.Now()
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

func (s *SPIFFEFederationSyncer) syncTrustDomain(
	ctx context.Context, name string,
) (out *machineidv1.SPIFFEFederation, err error) {
	ctx, span := tracer.Start(ctx, "SPIFFEFederationSyncer/syncTrustDomain")
	defer func() {
		tracing.EndSpan(span, err)
	}()

	current, err := s.cfg.Store.GetSPIFFEFederation(ctx, name)
	if err != nil {
		return nil, trace.Wrap(err, "failed to get SPIFFEFederation")
	}

	log := s.cfg.Logger.With(
		"current_revision", current.GetMetadata().GetRevision(),
		"trust_domain", current.GetMetadata().GetName(),
	)

	td, err := spiffeid.TrustDomainFromString(current.GetMetadata().GetName())
	if err != nil {
		return nil, trace.Wrap(err, "parsing metadata.name as trust domain name")
	}

	// Determine - should we sync...
	syncReason := s.shouldSyncTrustDomain(ctx, log, current)
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
	case current.GetSpec().GetBundleSource().GetHttpsWeb() != nil:
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

		// Calculate the duration before we should next sync, applying any
		// sensible upper and lower bounds.
		nextSyncIn = s.cfg.DefaultSyncInterval
		if refreshHint, ok := bundle.RefreshHint(); ok {
			if refreshHint < s.cfg.MinSyncInterval {
				log.DebugContext(
					ctx,
					"Refresh hint is less than MinSyncInterval, using MinSyncInterval",
					"refresh_hint", refreshHint,
					"min_sync_interval", s.cfg.MinSyncInterval,
				)
				nextSyncIn = s.cfg.MinSyncInterval
			} else if refreshHint > s.cfg.MaxSyncInterval {
				log.DebugContext(
					ctx,
					"Refresh hint is greater than MaxSyncInterval, using MaxSyncInterval",
					"refresh_hint", refreshHint,
					"max_sync_interval", s.cfg.MaxSyncInterval,
				)
				nextSyncIn = s.cfg.MaxSyncInterval
			} else {
				nextSyncIn = refreshHint
			}
		}
	case current.GetSpec().GetBundleSource().GetStatic() != nil:
		log.DebugContext(
			ctx, "Fetching bundle using spec.bundle_source.static.bundle",
		)
		bundle, err = spiffebundle.Parse(
			td, []byte(current.Spec.BundleSource.Static.Bundle),
		)
		if err != nil {
			return nil, trace.Wrap(
				err, "parsing bundle from static profile",
			)
		}
	default:
		return nil, trace.BadParameter(
			"spec.bundle_source: at least one of https_web or static must be set",
		)
	}

	bundleBytes, err := bundle.Marshal()
	if err != nil {
		return nil, trace.Wrap(err, "marshaling bundle")
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
		return nil, trace.Wrap(
			err, "persisting updated SPIFFEFederation",
		)
	}
	log.InfoContext(
		ctx,
		"Sync succeeded, new SPIFFEFederation persisted",
		"new_revision", out.Metadata.Revision,
	)

	return out, nil
}
