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

package spreadwork

import (
	"context"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// ApplyOverTimeConfig is the configuration values for the ApplyOverTime method.
type ApplyOverTimeConfig struct {
	// MaxDuration is the max duration for the operation to complete.
	// It does not account for time spent in API calls.
	MaxDuration time.Duration

	// BatchInterval is the time spent between API calls to prevent server pressure.
	// Defaults to 1 second.
	BatchInterval time.Duration

	// MinBatchSize is the min batch size per iteration.
	// Defaults to 5 items.
	MinBatchSize int

	// clock is used for tests only.
	clock clockwork.Clock
}

// CheckAndSetDefaults checks that the MaxDuration was set.
func (c *ApplyOverTimeConfig) CheckAndSetDefaults() error {
	if c.MaxDuration == 0 {
		return trace.BadParameter("max duration is required")
	}

	if c.BatchInterval == 0 {
		c.BatchInterval = time.Second
	}

	if c.MaxDuration < c.BatchInterval {
		return trace.BadParameter("max interval must be greater than batch interval")
	}

	if c.MinBatchSize == 0 {
		c.MinBatchSize = 5
	}

	if c.clock == nil {
		c.clock = clockwork.NewRealClock()
	}

	return nil
}

// ApplyOverTime applies the applyFn to all elements of items during a period of time.
func ApplyOverTime[T any](ctx context.Context, conf ApplyOverTimeConfig, items []T, applyFn func(T)) error {
	if err := conf.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}
	ticker := conf.clock.NewTicker(conf.BatchInterval)
	defer ticker.Stop()

	maxBatches := int(conf.MaxDuration / conf.BatchInterval)
	dynamicBatchSize := max((len(items)/maxBatches)+1, conf.MinBatchSize)

	for {
		if dynamicBatchSize > len(items) {
			dynamicBatchSize = len(items)
		}
		chunk := items[0:dynamicBatchSize:dynamicBatchSize]
		items = items[dynamicBatchSize:]

		for _, c := range chunk {
			// If ctx is Done, return without processing the other elements of the current chunk.
			select {
			case <-ctx.Done():
				return trace.Wrap(ctx.Err())
			default:
				applyFn(c)
			}
		}
		if len(items) == 0 {
			return nil
		}

		select {
		// If ctx is Done, return without processing the remaining chunks.
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-ticker.Chan():
		}
	}
}
