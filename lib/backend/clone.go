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
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

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
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	log := slog.With(teleport.ComponentKey, "clone")

	start := NewKey("")
	cloned, failed := &atomic.Int64{}, &atomic.Int64{}

	if parallel <= 0 {
		parallel = 1
	}

	if !force {
		result, err := dst.GetRange(ctx, start, RangeEnd(start), 1)
		if err != nil {
			return trace.Wrap(err, "failed to check destination for existing data")
		}
		if len(result.Items) > 0 {
			return trace.Errorf("unable to clone data to destination with existing data; this may be overridden by configuring 'force: true'")
		}
	}

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(parallel)

	go func() {
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				log.InfoContext(ctx, "Backend clone in progress", "cloned_items", cloned.Load(), "failed_items", failed.Load())
			case <-ctx.Done():
				return
			}
		}
	}()

	const maxAttempts = 3
	end := RangeEnd(start)
	for i := range maxAttempts {
		lastRead, err := cloneItems(ctx, src, dst, start, end, log, cloned, failed, group, maxAttempts)
		if err == nil {
			break
		}

		if i == maxAttempts-1 {
			log.ErrorContext(ctx, "Cloning backend failed. Encountered too many errors reading from the source backend.",
				"error", trace.Unwrap(err),
				"cloned_items", cloned.Load(),
				"failed_items", failed.Load(),
				"last_cloned_item", logutils.StringerAttr(lastRead),
			)
			groupError := group.Wait()
			return trace.NewAggregate(groupError, trace.Wrap(err, "retrieving backend items for cloning"))
		}

		if lastRead.Compare(end) < 0 {
			start = lastRead
		}
	}

	err := group.Wait()
	cloneCount := cloned.Load()
	failCount := failed.Load()

	logger := log.With("cloned_items", cloneCount, "failed_items", failed.Load())
	switch {
	case err == nil && cloneCount == 0 && failCount == 0:
		logger.WarnContext(ctx, "No data was found in the source backend, ensure the backend configuration is correct")
	case err != nil && cloneCount > 0 && failCount > 0:
		logger.InfoContext(ctx, "Cloning backend failed: some backend items were unable to be copied", "err", err)
		return trace.Wrap(err)
	case err != nil && cloneCount == 0 && failCount > 0:
		logger.InfoContext(ctx, "Cloning backend failed: no backend items were able to be copied", "err", err)
		return trace.Wrap(err)
	default:
		logger.InfoContext(ctx, "Cloning backend completed successfully")
	}

	return nil
}

func cloneItems(ctx context.Context, src, dst Backend, start, end Key, log *slog.Logger, cloned, failed *atomic.Int64, group *errgroup.Group, maxAttempts int) (Key, error) {
	lastRead := start
	for item, err := range src.Items(ctx, ItemsParams{StartKey: start, EndKey: end}) {
		if err != nil {
			log.WarnContext(ctx, "Failed reading backend item",
				"error", trace.Unwrap(err),
				"cloned_items", cloned.Load(),
				"failed_items", failed.Load(),
			)

			return lastRead, trace.Wrap(err)
		}

		// Prevent processing the same key twice. If reading from
		// the destination has previously failed, then start will be
		// the last item that was previously processed. Since there
		// is no way to exclude the start key from the Items range, it
		// has to be skipped manually instead.
		if item.Key.Compare(start) == 0 {
			continue
		}

		lastRead = item.Key
		group.Go(func() error {
			if err := retryPut(ctx, maxAttempts, dst, item); err != nil {
				failed.Add(1)
				return trace.Wrap(err)
			}
			cloned.Add(1)
			return nil
		})
	}

	return lastRead, nil
}

func retryPut(ctx context.Context, attempts int, b Backend, item Item) error {
	if attempts <= 0 {
		return trace.BadParameter("retry attempts must be greater than 0")
	}

	var retry retryutils.Retry
	for i := range attempts {
		if _, err := b.Put(ctx, item); err == nil {
			return nil
		} else if i >= attempts-1 {
			return trace.LimitExceeded("Failed to clone item %d times: %s", attempts, err)
		}

		if retry == nil {
			r, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
				Driver: retryutils.NewExponentialDriver(time.Millisecond * 100),
				Max:    time.Second * 2,
			})
			if err != nil {
				return trace.Wrap(err, "creating retry driver")
			}

			retry = r
		}

		select {
		case <-ctx.Done():
			return trace.Wrap(ctx.Err())
		case <-retry.After():
			retry.Inc()
		}
	}

	return nil
}
