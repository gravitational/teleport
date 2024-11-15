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

	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/utils/interval"
)

const (
	reconcilerPeriod = time.Minute
)

// Controller wakes up every minute to reconcile the autoupdate_agent_rollout resource.
// See the reconciler godoc for more details about the reconciliation process.
// We currently wake up every minute, in the future we might decide to also watch for events
// (from autoupdate_config and autoupdate_version changefeed) to react faster.
type Controller struct {
	reconciler reconciler
	clock      clockwork.Clock
	log        *slog.Logger
}

// NewController creates a new Controller for the autoupdate_agent_rollout kind.
func NewController(client Client, log *slog.Logger, clock clockwork.Clock) *Controller {
	return &Controller{
		clock: clock,
		log:   log,
		reconciler: reconciler{
			clt: client,
			log: log,
		},
	}
}

// Run the autoupdate_agent_rollout controller. This function returns only when its context is canceled.
func (c *Controller) Run(ctx context.Context) error {
	config := interval.Config{
		Duration:      reconcilerPeriod,
		FirstDuration: reconcilerPeriod,
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
			err := c.reconciler.reconcile(ctx)
			if err != nil {
				c.log.ErrorContext(ctx, "Failed to reconcile autoudpate_agent_controller", "error", err)
			}
		}
	}
}
