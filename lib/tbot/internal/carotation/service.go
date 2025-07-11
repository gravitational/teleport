/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package carotation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/readyz"
)

const caRotationRetryBackoff = time.Second * 2

// Config contains configuration options for the CA Rotation service.
type Config struct {
	// BroadcastFn is a function that will be called to broadcast that a
	// rotation has taken place.
	BroadcastFn func()

	// Client that will be used to watch the rotations stream.
	Client *client.Client

	// GetBotIdentityFn will be called to get the bot's internal identity.
	GetBotIdentityFn func() *identity.Identity

	// BotIdentityReadyCh is a channel that will be received from to block until
	// the bot's internal identity has been renewed.
	BotIdentityReadyCh <-chan struct{}

	// StatusReporter is used to report the service's health.
	StatusReporter readyz.Reporter

	// Logger to which errors and messages will be written.
	Logger *slog.Logger
}

func (cfg *Config) CheckAndSetDefaults() error {
	if cfg.BroadcastFn == nil {
		return trace.BadParameter("BroadcastFn is required")
	}
	if cfg.Client == nil {
		return trace.BadParameter("Client is required")
	}
	if cfg.GetBotIdentityFn == nil {
		return trace.BadParameter("GetBotIdentityFn is required")
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.StatusReporter == nil {
		cfg.StatusReporter = readyz.NoopReporter()
	}
	return nil
}

// NewService creates a new CA Rotation service.
func NewService(cfg Config) (*Service, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}
	return &Service{cfg: cfg}, nil
}

// Service watches for CA rotations in the cluster, and triggers a renewal when
// it detects a relevant CA rotation.
//
// See https://github.com/gravitational/teleport/blob/1aa38f4bc56997ba13b26a1ef1b4da7a3a078930/lib/auth/rotate.go#L135
// for server side details of how a CA rotation takes place.
//
// We can leverage the existing renewal system to fetch new certificates and
// CAs.
//
// We need to force a renewal for the following transitions:
//   - Init -> Update Clients: So we can receive a set of certificates issued by
//     the new CA, and trust both the old and new CA.
//   - Update Clients, Update Servers -> Rollback: So we can receive a set of
//     certificates issued by the old CA, and stop trusting the new CA.
//   - Update Servers -> Standby: So we can stop trusting the old CA.
type Service struct {
	cfg Config
}

func (s *Service) String() string { return "ca-rotation" }

// Run continually triggers `watchCARotations` until the context is canceled.
// This allows the watcher to be re-established if an error occurs.
//
// Run also manages debouncing the renewals across multiple watch attempts.
func (s *Service) Run(ctx context.Context) error {
	rd := debouncer{
		f:              s.cfg.BroadcastFn,
		debouncePeriod: time.Second * 10,
	}
	jitter := retryutils.DefaultJitter

	if s.cfg.BotIdentityReadyCh != nil {
		select {
		case <-s.cfg.BotIdentityReadyCh:
		default:
			s.cfg.Logger.InfoContext(ctx, "Waiting for internal bot identity to be renewed before running")
			select {
			case <-s.cfg.BotIdentityReadyCh:
			case <-ctx.Done():
				return nil
			}
		}
	}

	for {
		err := s.watchCARotations(ctx, rd.attempt)
		if ctx.Err() != nil {
			return nil
		}

		backoffPeriod := jitter(caRotationRetryBackoff)

		// If the error is due to the client being replaced with a new client
		// as part of the credentials renewal. Ignore it, and immediately begin
		// watching again with the new client. We can safely check for Canceled
		// here, because if the context was actually canceled, it would've
		// been caught in the error check immediately following watchCARotations
		var statusErr interface {
			GRPCStatus() *status.Status
		}
		isCancelledErr := errors.As(err, &statusErr) && statusErr.GRPCStatus().Code() == codes.Canceled
		if isCancelledErr {
			s.cfg.Logger.DebugContext(ctx, "CA watcher detected client closing. Waiting to rewatch", "wait", backoffPeriod)
		} else if err != nil {
			s.cfg.StatusReporter.Report(readyz.Unhealthy)
			s.cfg.Logger.ErrorContext(ctx, "Error occurred whilst watching CA rotations. Waiting to retry", "wait", backoffPeriod, "error", err)
		}

		select {
		case <-ctx.Done():
			s.cfg.Logger.WarnContext(ctx, "Context canceled during backoff for CA rotation watcher. Aborting")
			return nil
		case <-time.After(backoffPeriod):
		}
	}
}

// watchCARotations establishes a watcher for CA rotations in the cluster, and
// attempts to trigger a renewal via the debounced reload channel when it
// detects the entry into an important rotation phase.
func (s *Service) watchCARotations(ctx context.Context, queueReload func()) error {
	s.cfg.Logger.DebugContext(ctx, "Attempting to establish watch for CA events")

	ident := s.cfg.GetBotIdentityFn()
	clusterName := ident.ClusterName
	watcher, err := s.cfg.Client.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{
			Kind: types.KindCertAuthority,
			Filter: types.CertAuthorityFilter{
				types.HostCA:     clusterName,
				types.UserCA:     clusterName,
				types.DatabaseCA: clusterName,
			}.IntoMap(),
		}},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	for {
		select {
		case event := <-watcher.Events():
			// OpInit is a special case omitted by the Watcher when the
			// connection succeeds.
			if event.Type == types.OpInit {
				s.cfg.StatusReporter.Report(readyz.Healthy)
				s.cfg.Logger.InfoContext(ctx, "Started watching for CA rotations")
				continue
			}

			ignoreReason := filterCAEvent(ctx, s.cfg.Logger, event, clusterName)
			if ignoreReason != "" {
				s.cfg.Logger.DebugContext(ctx, "Ignoring CA event", "reason", ignoreReason)
				continue
			}

			// We need to debounce here, as multiple events will be received if
			// the user is rotating multiple CAs at once.
			s.cfg.Logger.InfoContext(ctx, "CA Rotation step detected; queueing renewal")
			queueReload()
		case <-watcher.Done():
			if err := watcher.Error(); err != nil {
				return trace.Wrap(err)
			}
			return nil
		case <-ctx.Done():
			return nil
		}
	}
}

// filterCAEvent returns a reason why an event should be ignored or an empty
// string is a renewal is needed.
func filterCAEvent(ctx context.Context, log *slog.Logger, event types.Event, clusterName string) string {
	if event.Type != types.OpPut {
		return "type not PUT"
	}
	ca, ok := event.Resource.(types.CertAuthority)
	if !ok {
		return fmt.Sprintf("event resource was not CertAuthority (%T)", event.Resource)
	}
	log.DebugContext(ctx, "Filtering CA", "ca", ca, "ca_kind", ca.GetKind(), "ca_sub_kind", ca.GetSubKind())

	// We want to update for all phases but init and update_servers
	phase := ca.GetRotation().Phase
	if slices.Contains([]string{
		"", types.RotationPhaseInit, types.RotationPhaseUpdateServers,
	}, phase) {
		return fmt.Sprintf("skipping due to phase '%s'", phase)
	}

	// Skip anything not from our cluster
	if ca.GetClusterName() != clusterName {
		return fmt.Sprintf(
			"skipping due to cluster name of CA: was '%s', wanted '%s'",
			ca.GetClusterName(),
			clusterName,
		)
	}

	// We want to skip anything that is not host, user, or db
	if !slices.Contains([]string{
		string(types.HostCA),
		string(types.UserCA),
		string(types.DatabaseCA),
	}, ca.GetSubKind()) {
		return fmt.Sprintf("skipping due to CA kind '%s'", ca.GetSubKind())
	}

	return ""
}

// debouncer accepts a duration, and a function. When `attempt` is called on
// debouncer, it waits the duration, ignoring further attempts during this
// period, before calling the provided function.
//
// This allows us to handle multiple events arriving within a short period, and
// attempts to reduce the risk of the server going away during a renewal by
// deferring the renewal until after the server-side elements of the rotation
// have occurred.
type debouncer struct {
	mu    sync.Mutex
	timer *time.Timer

	// debouncePeriod is the amount of time that debouncer should wait from an
	// initial trigger before triggering `f`, and in that time ignore further
	// attempts.
	debouncePeriod time.Duration

	// f is the function that should be called by the debouncer.
	f func()
}

func (rd *debouncer) attempt() {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	if rd.timer != nil {
		return
	}

	rd.timer = time.AfterFunc(rd.debouncePeriod, func() {
		rd.mu.Lock()
		defer rd.mu.Unlock()
		rd.timer = nil

		rd.f()
	})
}
