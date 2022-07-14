/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tbot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils"
	libUtils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// See https://github.com/gravitational/teleport/blob/1aa38f4bc56997ba13b26a1ef1b4da7a3a078930/lib/auth/rotate.go#L135
// for server side details of how a CA rotation takes place.
//
// We can leverage the existing renewal system to fetch new certificates and
// CAs.
//
// We need to force a renewal for the following transitions:
// - Init -> Update Clients: So we can receive a set of certificates issued by
//   the new CA, and trust both the old and new CA.
// - Update Clients, Update Servers -> Rollback: So we can receive a set of
//   certificates issued by the old CA, and stop trusting the new CA.
// - Update Servers -> Standby: So we can stop trusting the old CA.

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

const caRotationRetryBackoff = time.Second * 2

// caRotationLoop continually triggers `watchCARotations` until the context is
// canceled. This allows the watcher to be re-established if an error occurs.
//
// caRotationLoop also manages debouncing the renewals across multiple watch
// attempts.
func (b *Bot) caRotationLoop(ctx context.Context) error {
	rd := debouncer{
		f: func() {
			select {
			case <-ctx.Done():
				return
			default:
			}

			select {
			case b.reloadChan <- struct{}{}:
				b.log.Infof("Renewal triggered due to CA rotation.")
			default:
				b.log.Debugf("Renewal already queued, ignoring reload request.")
			}
		},
		debouncePeriod: time.Second * 10,
	}
	jitter := libUtils.NewJitter()

	for {
		err := b.watchCARotations(ctx, rd.attempt)
		if ctx.Err() != nil {
			return nil
		}

		backoffPeriod := jitter(caRotationRetryBackoff)
		b.log.WithError(err).Errorf("Error occurred whilst watching CA rotations, retrying in %s.", backoffPeriod)
		select {
		case <-ctx.Done():
			b.log.Warn("Context canceled during backoff for CA rotation watcher. Aborting.")
			return nil
		case <-time.After(backoffPeriod):
		}
	}
}

// watchCARotations establishes a watcher for CA rotations in the cluster, and
// attempts to trigger a renewal via the debounced reload channel when it
// detects the entry into an important rotation phase.
func (b *Bot) watchCARotations(ctx context.Context, queueReload func()) error {
	clusterName := b.ident().ClusterName
	b.log.Debugf("Attempting to establish watch for CA events")
	watcher, err := b.client().NewWatcher(ctx, types.Watch{
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
				b.log.Infof("Started watching for CA rotations")
				continue
			}

			ignoreReason := filterCAEvent(b.log, event, clusterName)
			if ignoreReason != "" {
				b.log.Debugf("Ignoring CA event: %s", ignoreReason)
				continue
			}

			// We need to debounce here, as multiple events will be received if
			// the user is rotating multiple CAs at once.
			b.log.Infof("CA Rotation step detected; queueing renewal.")
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
func filterCAEvent(log logrus.FieldLogger, event types.Event, clusterName string) string {
	if event.Type != types.OpPut {
		return "type not PUT"
	}
	ca, ok := event.Resource.(types.CertAuthority)
	if !ok {
		return fmt.Sprintf("event resource was not CertAuthority (%T)", event.Resource)
	}
	log.Debugf("Filtering CA: %+v %s %s", ca, ca.GetKind(), ca.GetSubKind())

	// We want to update for all phases but init and update_servers
	phase := ca.GetRotation().Phase
	if utils.SliceContainsStr([]string{
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

	// We want to skip anything that is not host, user, db
	if !utils.SliceContainsStr([]string{
		string(types.HostCA),
		string(types.UserCA),
		string(types.DatabaseCA),
	}, ca.GetSubKind()) {
		return fmt.Sprintf("skipping due to CA kind '%s'", ca.GetSubKind())
	}

	return ""
}
