// Teleport
// Copyright (C) 2024 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package backend

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
)

// bufferSize is the number of backend items that are queried at a time.
const bufferSize = 10000

// CloneConfig contains the configuration for cloning a [Backend].
// All items from the source are copied to the destination. All Teleport Auth
// Service instances should be stopped when running clone to avoid data
// inconsistencies.
type CloneConfig struct {
	// Source is the backend [Config] items are cloned from.
	Source Config `yaml:"src"`
	// Destination is the [Config] items are cloned to.
	Destination Config `yaml:"dst"`
	// Parallel is the number of items that will be cloned in parallel.
	Parallel int `yaml:"parallel"`
	// Force indicates whether to clone data regardless of whether data already
	// exists in the destination [Backend].
	Force bool `yaml:"force"`
}

// Clone copies all items from a source to a destination [Backend].
func Clone(ctx context.Context, src, dst Backend, parallel int, force bool) error {
	log := slog.With(teleport.ComponentKey, "clone")
	itemC := make(chan Item, bufferSize)
	start := NewKey("")
	migrated := &atomic.Int32{}

	if parallel <= 0 {
		parallel = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if !force {
		result, err := dst.GetRange(ctx, start, RangeEnd(start), 1)
		if err != nil {
			return trace.Wrap(err, "failed to check destination for existing data")
		}
		if len(result.Items) > 0 {
			return trace.Errorf("unable to clone data to destination with existing data; this may be overridden by configuring 'force: true'")
		}
	} else {
		log.WarnContext(ctx, "Skipping check for existing data in destination.")
	}

	group, ctx := errgroup.WithContext(ctx)
	// Add 1 to ensure a goroutine exists for getting items.
	group.SetLimit(parallel + 1)

	group.Go(func() error {
		var result *GetResult
		pageKey := start
		defer close(itemC)
		for {
			err := retry(ctx, 3, func() error {
				var err error
				result, err = src.GetRange(ctx, pageKey, RangeEnd(start), bufferSize)
				if err != nil {
					return trace.Wrap(err)
				}
				return nil
			})
			if err != nil {
				return trace.Wrap(err)
			}
			for _, item := range result.Items {
				select {
				case itemC <- item:
				case <-ctx.Done():
					return trace.Wrap(ctx.Err())
				}
			}
			if len(result.Items) < bufferSize {
				return nil
			}
			pageKey = RangeEnd(result.Items[len(result.Items)-1].Key)
		}
	})

	logProgress := func() {
		log.InfoContext(ctx, "Backend clone still in progress", "items_copied", migrated.Load())
	}
	defer logProgress()
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		select {
		case <-ticker.C:
			logProgress()
		case <-ctx.Done():
			return
		}
	}()

	for item := range itemC {
		group.Go(func() error {
			if err := retry(ctx, 3, func() error {
				if _, err := dst.Put(ctx, item); err != nil {
					return trace.Wrap(err)
				}
				return nil
			}); err != nil {
				return trace.Wrap(err)
			}
			migrated.Add(1)
			return nil
		})
		if err := ctx.Err(); err != nil {
			break
		}
	}
	if err := group.Wait(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func retry(ctx context.Context, attempts int, fn func() error) error {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(time.Millisecond * 100),
		Max:    time.Second * 2,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	if attempts <= 0 {
		return trace.Errorf("retry attempts must be > 0")
	}

	for range attempts {
		err = fn()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-retry.After():
			retry.Inc()
		}
	}
	return trace.Wrap(err)
}
