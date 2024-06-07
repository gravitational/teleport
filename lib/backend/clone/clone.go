package clone

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/trace"
)

const (
	// bufferSize is the number of backend items that are queried at a time.
	bufferSize = 10000
)

// Cloner manages cloning data between two [backend.Backend] interfaces.
type Cloner struct {
	src      backend.Backend
	dst      backend.Backend
	parallel int
	force    bool
	migrated atomic.Int64
	log      *slog.Logger
}

// Config contains the configuration for cloning a [backend.Backend].
// All items from the source are copied to the destination. All Teleport Auth
// Service instances should be stopped when running clone to avoid data
// inconsistencies.
type Config struct {
	// Source is the backend [backend.Config] items are cloned from.
	Source backend.Config `yaml:"src"`
	// Destination is the [backend.Config] items are cloned to.
	Destination backend.Config `yaml:"dst"`
	// Parallel is the number of items that will be cloned in parallel.
	Parallel int `yaml:"parallel"`
	// Force indicates whether to clone data regardless of whether data already
	// exists in the destination [backend.Backend].
	Force bool `yaml:"force"`
	// Log logs the progress of cloning.
	Log *slog.Logger
}

// New returns a [Cloner] based on the provided [Config].
func New(ctx context.Context, config Config) (*Cloner, error) {
	src, err := backend.New(ctx, config.Source.Type, config.Source.Params)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create source backend")
	}
	dst, err := backend.New(ctx, config.Destination.Type, config.Destination.Params)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create destination backend")
	}
	cloner := &Cloner{
		src:      src,
		dst:      dst,
		parallel: config.Parallel,
		force:    config.Force,
		log:      config.Log,
	}
	if cloner.parallel <= 0 {
		cloner.parallel = 1
	}
	if cloner.log == nil {
		cloner.log = slog.With(teleport.ComponentKey, "backend.clone")
	}
	return cloner, nil
}

// Close ensures the source and destination backends are closed.
func (c *Cloner) Close() error {
	var errs []error
	if c.src != nil {
		err := c.src.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if c.dst != nil {
		err := c.dst.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}

// Run runs backend cloning until complete.
func (c *Cloner) Clone(ctx context.Context) error {
	itemC := make(chan backend.Item, bufferSize)
	start := backend.Key("")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	if !c.force {
		result, err := c.dst.GetRange(ctx, start, backend.RangeEnd(start), 1)
		if err != nil {
			return trace.Wrap(err, "failed to check destination for existing data")
		}
		if len(result.Items) > 0 {
			return trace.Errorf("unable to clone data to destination with existing data; this may be overriden by configuring 'force: true'")
		}
	} else {
		c.log.Warn("Skipping check for existing data in destination.")
	}

	group, ctx := errgroup.WithContext(ctx)
	// Add 1 to ensure a goroutine exists for getting items.
	group.SetLimit(c.parallel + 1)

	group.Go(func() error {
		var result *backend.GetResult
		pageKey := start
		defer close(itemC)
		for {
			err := retry(ctx, 3, func() error {
				var err error
				result, err = c.src.GetRange(ctx, pageKey, backend.RangeEnd(start), bufferSize)
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
			pageKey = backend.RangeEnd(result.Items[len(result.Items)-1].Key)
		}
	})

	logProgress := func() {
		c.log.InfoContext(ctx, "Backend clone still in progress", "items_copied" c.migrated.Load()))
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
		item := item
		group.Go(func() error {
			if err := retry(ctx, 3, func() error {
				if _, err := c.dst.Put(ctx, item); err != nil {
					return trace.Wrap(err)
				}
				return nil
			}); err != nil {
				return trace.Wrap(err)
			}
			c.migrated.Add(1)
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

	for i := 0; i < attempts; i++ {
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
