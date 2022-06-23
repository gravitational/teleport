/*
Copyright 2018-2022 Gravitational, Inc.

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

package sqlbk

import (
	"context"
	"errors"
	"time"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/interval"
	"github.com/gravitational/trace"
)

// start background goroutine to track expired leases, emit events, and purge records.
func (b *Backend) start(ctx context.Context) error {
	lastEventID, err := b.initLastEventID(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	b.buf.SetInit()
	go b.run(lastEventID)
	return nil
}

// initLastEventID returns the ID of the most recent event stored in the
// database. It will continue to retry on error until the context is canceled.
//
// No background processing can continue until this routine succeeds, so there
// is no internal timeout. Typically, errors will occur when the database is
// down, so this routine will keep trying until the context is canceled or the
// database is up and responds to the query. On startup, the context is the one
// passed to New; after startup it is the backend's close context.
func (b *Backend) initLastEventID(ctx context.Context) (lastEventID int64, err error) {
	var periodic *interval.Interval
	var logged bool
	for {
		tx := b.db.ReadOnly(ctx)
		lastEventID = tx.GetLastEventID()
		if tx.Commit() == nil {
			break
		}
		if !logged {
			b.Log.Errorf("Failed to query for last event ID: %v. Background routine is paused.", tx.Err())
			logged = true
		}

		// Retry after a short delay.
		if periodic == nil {
			periodic = interval.New(interval.Config{
				Duration:      b.PollStreamPeriod,
				FirstDuration: utils.HalfJitter(b.PollStreamPeriod),
				Jitter:        utils.NewSeventhJitter(),
			})
			defer periodic.Stop()
		}
		select {
		case <-periodic.Next():
		case <-ctx.Done():
			return 0, trace.Wrap(ctx.Err())
		}
	}

	if logged {
		b.Log.Info("Successfully queried for last event ID. Background routine has started.")
	}

	return lastEventID, nil
}

// run background process.
// - Poll the database to delete expired leases and emit events every PollStreamPeriod (1s).
// - Purge expired backend items and emitted events every PurgePeriod (20s).
func (b *Backend) run(eventID int64) {
	defer close(b.bgDone)

	pollPeriodic := interval.New(interval.Config{
		Duration:      b.PollStreamPeriod,
		FirstDuration: utils.HalfJitter(b.PollStreamPeriod),
		Jitter:        utils.NewSeventhJitter(),
	})
	defer pollPeriodic.Stop()

	purgePeriodic := interval.New(interval.Config{
		Duration:      b.PurgePeriod,
		FirstDuration: utils.HalfJitter(b.PurgePeriod),
		Jitter:        utils.NewSeventhJitter(),
	})
	defer purgePeriodic.Stop()

	var err error
	var loggedError bool // don't spam logs
	for {
		select {
		case <-b.closeCtx.Done():
			return

		case <-pollPeriodic.Next():
			eventID, err = b.poll(eventID)

		case <-purgePeriodic.Next():
			err = b.purge()
		}

		if err == nil {
			loggedError = false
			continue
		}

		if !loggedError {
			// Downgrade log level on timeout. Operation will try again.
			if errors.Is(err, context.Canceled) {
				b.Log.Warn(err)
			} else {
				b.Log.Error(err)
			}
			loggedError = true
		}
	}
}

// purge events and expired items.
func (b *Backend) purge() error {
	ctx, cancel := context.WithTimeout(b.closeCtx, b.PollStreamPeriod)
	defer cancel()
	tx := b.db.Begin(ctx)
	tx.DeleteExpiredLeases()
	tx.DeleteEvents(b.now().Add(-backend.DefaultEventsTTL))
	tx.DeleteItems()
	return tx.Commit()
}

// poll for expired leases and create delete events. Then emit events whose ID
// is greater than fromEventID. Events are emitted in the order they were
// created. Return the event ID of the last event emitted.
//
// This function also resets the buffer when it detects latency emitting events.
// The buffer is reset when the number of events remaining to emit combined with
// the maximum number of events emitted each poll period exceeds EventsTTL. Or
// simply, there are too many events to emit before they will be deleted, so we
// need to start over to prevent missing events and corrupting downstream caches.
func (b *Backend) poll(fromEventID int64) (lastEventID int64, err error) {
	ctx, cancel := context.WithTimeout(b.closeCtx, b.PollStreamPeriod)
	defer cancel()

	tx := b.db.Begin(ctx)

	var item backend.Item
	for _, lease := range tx.GetExpiredLeases() {
		item.ID = lease.ID
		item.Key = lease.Key
		tx.InsertEvent(types.OpDelete, item)
		if tx.Err() != nil {
			return fromEventID, tx.Err()
		}
	}

	limit := b.Config.BufferSize / 2
	events := tx.GetEvents(fromEventID, limit)
	if tx.Commit() != nil {
		return fromEventID, tx.Err()
	}

	// Latency check.
	timeNeeded := time.Duration(events.Remaining/limit) * b.PollStreamPeriod
	if timeNeeded > b.EventsTTL {
		b.buf.Reset()
		lastEventID, err := b.initLastEventID(b.closeCtx)
		if err != nil { // err = closeCtx.Err()
			return 0, trace.Wrap(err)
		}
		b.buf.SetInit()
		return lastEventID, nil
	}

	b.buf.Emit(events.BackendEvents...)

	return events.LastID, nil
}
