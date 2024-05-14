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

package tbot

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

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth"
)

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

type channelBroadcaster struct {
	mu      sync.Mutex
	chanSet map[chan struct{}]struct{}
}

func (cb *channelBroadcaster) subscribe() (ch chan struct{}, unsubscribe func()) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	ch = make(chan struct{}, 1)
	cb.chanSet[ch] = struct{}{}
	// Returns a function that should be called to unsubscribe the channel
	return ch, func() {
		cb.mu.Lock()
		defer cb.mu.Unlock()
		_, ok := cb.chanSet[ch]
		if ok {
			delete(cb.chanSet, ch)
			close(ch)
		}
	}
}

func (cb *channelBroadcaster) broadcast() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	for ch := range cb.chanSet {
		select {
		case ch <- struct{}{}:
			// Successfully sent notification
		default:
			// Channel already has valued queued
		}
	}
}

const caRotationRetryBackoff = time.Second * 2

// caRotationService watches for CA rotations in the cluster, and
// triggers a renewal when it detects a relevant CA rotation.
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
type caRotationService struct {
	log               *slog.Logger
	reloadBroadcaster *channelBroadcaster
	botClient         *auth.Client
	getBotIdentity    getBotIdentityFn
}

func (s *caRotationService) String() string {
	return "ca-rotation"
}

// Run continually triggers `watchCARotations` until the context is
// canceled. This allows the watcher to be re-established if an error occurs.
//
// Run also manages debouncing the renewals across multiple watch
// attempts.
func (s *caRotationService) Run(ctx context.Context) error {
	rd := debouncer{
		f:              s.reloadBroadcaster.broadcast,
		debouncePeriod: time.Second * 10,
	}
	jitter := retryutils.NewJitter()

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
			s.log.DebugContext(ctx, "CA watcher detected client closing. Waiting to rewatch", "wait", backoffPeriod)
		} else if err != nil {
			s.log.ErrorContext(ctx, "Error occurred whilst watching CA rotations. Waiting to retry", "wait", backoffPeriod, "error", err)
		}

		select {
		case <-ctx.Done():
			s.log.WarnContext(ctx, "Context canceled during backoff for CA rotation watcher. Aborting")
			return nil
		case <-time.After(backoffPeriod):
		}
	}
}

// watchCARotations establishes a watcher for CA rotations in the cluster, and
// attempts to trigger a renewal via the debounced reload channel when it
// detects the entry into an important rotation phase.
func (s *caRotationService) watchCARotations(ctx context.Context, queueReload func()) error {
	s.log.DebugContext(ctx, "Attempting to establish watch for CA events")

	ident := s.getBotIdentity()
	clusterName := ident.ClusterName
	watcher, err := s.botClient.NewWatcher(ctx, types.Watch{
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
				s.log.InfoContext(ctx, "Started watching for CA rotations")
				continue
			}

			ignoreReason := filterCAEvent(ctx, s.log, event, clusterName)
			if ignoreReason != "" {
				s.log.DebugContext(ctx, "Ignoring CA event", "reason", ignoreReason)
				continue
			}

			// We need to debounce here, as multiple events will be received if
			// the user is rotating multiple CAs at once.
			s.log.InfoContext(ctx, "CA Rotation step detected; queueing renewa.")
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
