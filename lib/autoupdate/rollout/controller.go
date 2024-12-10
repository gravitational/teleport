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

package rollout

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const (
	defaultReconcilerPeriod = time.Minute
)

// Controller wakes up every minute to reconcile the autoupdate_agent_rollout resource.
// See the reconciler godoc for more details about the reconciliation process.
// We currently wake up every minute, in the future we might decide to also watch for events
// (from autoupdate_config and autoupdate_version changefeed) to react faster.
type Controller struct {
	// TODO(hugoShaka) add prometheus metrics describing the reconciliation status
	reconciler reconciler
	clock      clockwork.Clock
	log        *slog.Logger
	period     time.Duration
}

// NewController creates a new Controller for the autoupdate_agent_rollout kind.
// The period can be specified to control the sync frequency. This is mainly
// used to speed up tests or for demo purposes. When empty, the controller picks
// a sane default value.
func NewController(client Client, log *slog.Logger, clock clockwork.Clock, period time.Duration) (*Controller, error) {
	if client == nil {
		return nil, trace.BadParameter("missing client")
	}
	if log == nil {
		return nil, trace.BadParameter("missing log")
	}
	if clock == nil {
		return nil, trace.BadParameter("missing clock")
	}

	if period <= 0 {
		period = defaultReconcilerPeriod
	}

	return &Controller{
		clock: clock,
		log:   log,
		reconciler: reconciler{
			clt:               client,
			log:               log,
			rolloutStrategies: []rolloutStrategy{
				// TODO(hugoShaka): add the strategies here as we implement them
			},
		},
		period: period,
	}, nil
}

// Run the autoupdate_agent_rollout controller. This function returns only when its context is canceled.
func (c *Controller) Run(ctx context.Context) error {
	config := interval.Config{
		Duration:      c.period,
		FirstDuration: c.period,
		Jitter:        retryutils.SeventhJitter,
		Clock:         c.clock,
	}
	ticker := interval.New(config)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			c.log.InfoContext(ctx, "Stopping autoupdate_agent_rollout controller", "reason", ctx.Err())
			return ctx.Err()
		case <-ticker.Next():
			c.log.DebugContext(ctx, "Reconciling autoupdate_agent_rollout")
			if err := c.tryAndCatch(ctx); err != nil {
				c.log.ErrorContext(ctx, "Failed to reconcile autoudpate_agent_controller", "error", err)
			}
		}
	}
}

// tryAndCatch tries to run the controller reconciliation logic and recovers from potential panic by converting them
// into errors. This ensures that a critical bug in the reconciler cannot bring down the whole Teleport cluster.
func (c *Controller) tryAndCatch(ctx context.Context) (err error) {
	// If something terribly bad happens during the reconciliation, we recover and return an error
	defer func() {
		if r := recover(); r != nil {
			c.log.ErrorContext(ctx, "Recovered from panic in the autoupdate_agent_rollout controller", "panic", r)
			err = trace.NewAggregate(err, trace.Errorf("Panic recovered during reconciliation: %v", r))
		}
	}()
	err = trace.Wrap(c.reconciler.reconcile(ctx))
	return
}
