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
	"github.com/gravitational/teleport/lib/backend"
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/auth/machineid/v1")

type spiffeFederationStore interface {
	ListSPIFFEFederations(ctx context.Context, limit int, token string) ([]*machineidv1.SPIFFEFederation, string, error)
	GetSPIFFEFederation(ctx context.Context, name string) (*machineidv1.SPIFFEFederation, error)
	// TODO: Replace with Update()
	UpsertSPIFFEFederation(ctx context.Context, federation *machineidv1.SPIFFEFederation) (*machineidv1.SPIFFEFederation, error)
}

// SPIFFEFederationSyncerConfig is the configuration for the SPIFFE federation syncer.
type SPIFFEFederationSyncerConfig struct {
	Backend backend.Backend
	Store   spiffeFederationStore
	Logger  *slog.Logger
	Clock   clockwork.Clock

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
}

type SPIFFEFederationSyncer struct {
	cfg SPIFFEFederationSyncerConfig
}

const (
	minRefreshInterval     = time.Minute * 1
	maxRefreshInterval     = time.Hour * 24
	defaultRefreshInterval = time.Minute * 5
)

func NewSPIFFEFederationSyncer(cfg SPIFFEFederationSyncerConfig) (*SPIFFEFederationSyncer, error) {
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

// runWhileLocked is the core loop of the syncer that runs on a single auth
// server.
//
// Its goal is to sync the contents of trust bundles from remote clusters to
// the local cluster. It does this by creating a goroutine that manages each
// federated cluster.
func (s *SPIFFEFederationSyncer) runWhileLocked(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	trustDomains := []*machineidv1.SPIFFEFederation{}
	wg := &sync.WaitGroup{}
	for _, trustDomain := range trustDomains {
		wg.Add(1)
		go s.refreshFederationLoop(ctx, wg, trustDomain)
	}
	wg.Wait()
	return nil
}

func (s *SPIFFEFederationSyncer) refreshFederationLoop(
	ctx context.Context,
	wg *sync.WaitGroup,
	in *machineidv1.SPIFFEFederation,
) {
	defer wg.Done()
	for {

		s.syncFederation(ctx, in)

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func shouldSyncFederation(
	ctx context.Context,
	log *slog.Logger,
	clock clockwork.Clock,
	in *machineidv1.SPIFFEFederation,
) bool {
	if in.Status == nil {
		log.DebugContext(ctx, "No status, will sync")
		return true
	}
	if in.Status.CurrentBundle == "" {
		log.DebugContext(ctx, "No status.current_bundle, will sync")
		return true
	}
	if in.Status.CurrentBundleSyncedAt.AsTime().IsZero() {
		log.DebugContext(ctx, "No status.current_bundle_synced_at, will sync")
		return true
	}
	nextSyncAt := in.Status.NextSyncAt.AsTime()
	now := clock.Now()
	if !nextSyncAt.IsZero() && now.After(nextSyncAt) {
		log.DebugContext(
			ctx,
			"status.next_sync_at has passed, will sync",
			"next_sync_at", nextSyncAt,
			"now", now,
		)
		return true
	}
	return false
}

func (s *SPIFFEFederationSyncer) syncFederation(
	ctx context.Context, in *machineidv1.SPIFFEFederation,
) (out *machineidv1.SPIFFEFederation, err error) {
	ctx, span := tracer.Start(ctx, "SPIFFEFederationSyncer/syncFederation")
	defer func() {
		tracing.EndSpan(span, err)
	}()
	log := s.cfg.Logger.With("trust_domain", in.GetMetadata().GetName())
	// Clone the input so we can compare it to the persisted version later
	out = proto.Clone(in).(*machineidv1.SPIFFEFederation)

	td, err := spiffeid.TrustDomainFromString(in.GetMetadata().GetName())
	if err != nil {
		log.ErrorContext(ctx, "Invalid trust domain name", "error", err)
		return nil, trace.Wrap(err)
	}

	// Determine - should we refresh...
	if !shouldSyncFederation(ctx, log, s.cfg.Clock, in) {
		return out, nil
	}
	log.InfoContext(ctx, "SPIFFEFederation sync triggered")

	// Refresh...
	if out.Status == nil {
		out.Status = &machineidv1.SPIFFEFederationStatus{}
	}

	var bundle *spiffebundle.Bundle
	var nextSyncIn time.Duration
	switch {
	case in.Spec.BundleSource.HttpsWeb != nil:
		url := in.Spec.BundleSource.HttpsWeb.BundleEndpointUrl
		log.DebugContext(
			ctx,
			"Fetching bundle using https_web profile",
			"url", url,
		)
		bundle, err = federation.FetchBundle(ctx, td, url)
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
	case in.Spec.BundleSource.Static != nil:
		log.DebugContext(ctx, "Fetching bundle using spec.bundle_source.static.bundle")
		bundle, err = spiffebundle.Parse(td, []byte(in.Spec.BundleSource.Static.Bundle))
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
	out.Status.CurrentBundleSyncedAt = timestamppb.New(s.cfg.Clock.Now())
	out.Status.CurrentBundleSyncedFrom = in.Spec.BundleSource
	// For certain sources, we need to set a next sync time.
	if nextSyncIn > 0 {
		out.Status.NextSyncAt = timestamppb.New(s.cfg.Clock.Now().Add(nextSyncIn))
	}

	return out, nil
}
