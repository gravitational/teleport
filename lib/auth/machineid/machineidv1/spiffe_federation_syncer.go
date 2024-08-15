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
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/lib/backend"
)

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
}

type SPIFFEFederationSyncer struct {
	cfg SPIFFEFederationSyncerConfig
}

func NewSPIFFEFederationSyncer(cfg SPIFFEFederationSyncerConfig) (*SPIFFEFederationSyncer, error) {
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

const minRefreshInterval = time.Minute * 1
const maxRefreshInterval = time.Hour * 24
const defaultRefreshInterval = time.Minute * 5

func (s *SPIFFEFederationSyncer) refreshFederationLoop(
	ctx context.Context,
	wg *sync.WaitGroup,
	in *machineidv1.SPIFFEFederation,
) {
	defer wg.Done()
	for {

		s.refreshFederation(ctx, in)

		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

func shouldRefresh(
	ctx context.Context,
	log *slog.Logger,
	clock clockwork.Clock,
	td spiffeid.TrustDomain,
	in *machineidv1.SPIFFEFederation,
) bool {
	if in.Status == nil {
		log.DebugContext(ctx, "No status, will refresh")
		return true
	}
	if in.Status.CurrentBundle == "" {
		log.DebugContext(ctx, "No status.current_bundle, will refresh")
		return true
	}
	if in.Status.CurrentBundleSyncedAt.AsTime().IsZero() {
		log.DebugContext(ctx, "No status.current_bundle_synced_at, will refresh")
		return true
	}
	parsedBundle, err := spiffebundle.Parse(td, []byte(in.Status.CurrentBundle))
	if err != nil {
		log.ErrorContext(ctx, "Failed to parse current bundle, will refresh", "error", err)
		return true
	}
	// Looked at ParsedBundle refresh hint...

}

func (s *SPIFFEFederationSyncer) refreshFederation(ctx context.Context, in *machineidv1.SPIFFEFederation) (*machineidv1.SPIFFEFederation, error) {
	log := s.cfg.Logger.With("trust_domain", in.GetMetadata().GetName())
	out := proto.Clone(in).(*machineidv1.SPIFFEFederation)

	td, err := spiffeid.TrustDomainFromString(in.GetMetadata().GetName())
	if err != nil {
		log.ErrorContext(ctx, "Invalid trust domain name", "error", err)
		return nil, trace.Wrap(err)
	}

	// Determine - should we refresh...
	if !shouldRefresh(ctx, log, s.cfg.Clock, td, out) {
		return out, nil
	}

	// Refresh...
	if out.Status == nil {
		out.Status = &machineidv1.SPIFFEFederationStatus{}
	}

	switch {
	case out.Spec.BundleSource.HttpsWeb != nil:
		// If there's an existing bundle, let's check we're due to refresh
		// it.
		if out.Status.CurrentBundle != "" {
			if out.Status.CurrentBundleSyncedAt.AsTime().IsZero() {
				log.WarnContext(ctx, "current_bundle_synced_at is zero when current_bundle is not empty, this is unexpected. Will refresh bundle anyway.")
				// TODO(refresh anyway)
			}
			refreshHint := defaultRefreshInterval
			if out.Status.CurrentBundleRefreshHint != nil {
				requestedRefreshHint := out.Status.CurrentBundleRefreshHint.AsDuration()
				// Enforce some sensible limits on the requested refresh.
				if requestedRefreshHint < minRefreshInterval {
					log.WarnContext(
						ctx,
						"Hinted refresh interval is too short, using minimum value instead",
						slog.Duration("requested_refresh_hint", requestedRefreshHint),
						slog.Duration("min_refresh_interval", minRefreshInterval),
					)
					refreshHint = minRefreshInterval
				} else if requestedRefreshHint > maxRefreshInterval {
					log.WarnContext(
						ctx,
						"Hinted refresh interval is too long, using maximum value instead",
						slog.Duration("requested_refresh_hint", requestedRefreshHint),
						slog.Duration("max_refresh_interval", maxRefreshInterval),
					)
					refreshHint = maxRefreshInterval
				} else {
					refreshHint = requestedRefreshHint
				}
			}
			if s.cfg.Clock.Now().After(out.Status.CurrentBundleSyncedAt.AsTime().Add(refreshHint)) {
				// Ok we need to refresh!

			}
		} else {

		}
	case out.Spec.BundleSource.Static != nil:
		if out.Status.CurrentBundle != out.Spec.BundleSource.Static.Bundle {
			log.DebugContext(ctx, "Updating status.current_bundle using spec.bundle_source.static.bundle because it was empty")
			out.Status.CurrentBundle = out.Spec.BundleSource.Static.Bundle
			out.Status.CurrentBundleSyncedAt = timestamppb.New(s.cfg.Clock.Now())
		}
	}

	if !proto.Equal(persisted, out) {
		// Persist updated SPIFFEFederation
		_, err := s.cfg.Store.UpsertSPIFFEFederation(ctx, out)
		if err != nil {
			panic("oh no")
		}
	}
}
