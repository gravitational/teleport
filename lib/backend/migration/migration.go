package migration

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
)

const (
	// bufferSize is the number of backend items that are queried at a time.
	bufferSize = 10000
)

// Migration manages a migration between two [backend.Backend] interfaces.
type Migration struct {
	src      backend.Backend
	dst      backend.Backend
	parallel int
	migrated atomic.Int64
	log      logrus.FieldLogger
}

// MigrationConfig configures a [Migration] with a source and destination backend.
// All items from the source are copied to the destination. All Teleport Auth
// Service instances should be stopped when running a migration to avoid data
// inconsistencies.
type MigrationConfig struct {
	// Source is the backend [backend.Config] items are migrated from.
	Source backend.Config `yaml:"src"`
	// Destination is the [backend.Config] items are migrated to.
	Destination backend.Config `yaml:"dst"`
	// Parallel is the number of items that will be migraated in parallel.
	Parallel int `yaml:"parallel"`
	// Log logs the progress of a [Migration]
	Log logrus.FieldLogger
}

// New returns a [Migration] based on the provided [MigrationConfig].
func New(ctx context.Context, config MigrationConfig) (*Migration, error) {
	src, err := backend.New(ctx, config.Source.Type, config.Source.Params)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create source backend")
	}
	dst, err := backend.New(ctx, config.Destination.Type, config.Destination.Params)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create destination backend")
	}
	migration := &Migration{
		src:      src,
		dst:      dst,
		parallel: config.Parallel,
		log:      config.Log,
	}
	if migration.parallel <= 0 {
		migration.parallel = 1
	}
	if migration.log == nil {
		migration.log = logrus.WithField(teleport.ComponentKey, "migration")
	}
	return migration, nil
}

func (m *Migration) Close() error {
	var errs []error
	if m.src != nil {
		err := m.src.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if m.dst != nil {
		err := m.dst.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}

// Run runs a [Migration] until complete.
func (m *Migration) Run(ctx context.Context) error {
	itemC := make(chan backend.Item, bufferSize)
	start := backend.Key("")

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	putGroup, putCtx := errgroup.WithContext(ctx)
	putGroup.SetLimit(m.parallel)

	getGroup, getCtx := errgroup.WithContext(ctx)
	getGroup.Go(func() error {
		for {
			var result *backend.GetResult
			defer close(itemC)
			err := retry(getCtx, 3, func() error {
				var err error
				result, err = m.src.GetRange(getCtx, start, backend.RangeEnd(start), bufferSize)
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
				case <-getCtx.Done():
					return trace.Wrap(getCtx.Err())
				// This case indicates no consumers are pulling items
				// from the channel. Return to avoid deadlock.
				case <-putCtx.Done():
					return trace.Wrap(putCtx.Err())
				}
			}
			if len(result.Items) < bufferSize {
				return nil
			}
		}
	})

	logProgress := func() {
		m.log.Infof("Migrated %d", m.migrated.Load())
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
		putGroup.Go(func() error {
			if err := retry(putCtx, 3, func() error {
				if _, err := m.dst.Put(putCtx, item); err != nil {
					return trace.Wrap(err)
				}
				return nil
			}); err != nil {
				return trace.Wrap(err)
			}
			m.migrated.Add(1)
			return nil
		})
		if err := putCtx.Err(); err != nil {
			break
		}
	}

	getErr := getGroup.Wait()
	putErr := putGroup.Wait()
	return trace.NewAggregate(getErr, putErr)
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
