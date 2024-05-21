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

// Migration manages a migration between two [backend.Backend] interfaces.
type Migration struct {
	src      backend.Backend
	dst      backend.Backend
	parallel int
	total    int
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
	if migration.parallel == 0 {
		migration.parallel = 1
	}
	if migration.log == nil {
		migration.log = logrus.WithField(teleport.ComponentKey, "migration")
	}
	return nil, nil
}

// Run runs a [Migration] until complete.
func (m *Migration) Run(ctx context.Context) error {
	var all []backend.Item
	start := backend.Key("")
	err := retry(ctx, 3, func() error {
		result, err := m.src.GetRange(ctx, start, backend.RangeEnd(start), 0)
		if err != nil {
			return trace.Wrap(err)
		}
		all = result.Items
		return nil
	})
	if err != nil {
		return trace.Wrap(err)
	}
	m.total = len(all)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	group, ctx := errgroup.WithContext(ctx)
	group.SetLimit(m.parallel)

	logProgress := func() {
		m.log.Info("Migrated %d/%d", m.migrated.Load(), m.total)
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

	for _, item := range all {
		item := item
		group.Go(func() error {
			if err := retry(ctx, 3, func() error {
				if _, err := m.dst.Put(ctx, item); err != nil {
					return trace.Wrap(err)
				}
				return nil
			}); err != nil {
				return trace.Wrap(err)
			}
			m.migrated.Add(1)
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
