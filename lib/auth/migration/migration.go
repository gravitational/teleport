/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package migration

import (
	"cmp"
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"time"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	oteltrace "go.opentelemetry.io/otel/trace"

	"github.com/gravitational/teleport/api/observability/tracing"
	"github.com/gravitational/teleport/lib/backend"
)

// applyConfig is a group of options used to
// customize Apply when running tests.
type applyConfig struct {
	// migrations to execute during Apply instead of the
	// default registered set.
	migrations []migration
}

// withMigrations overrides the default set of migrations used by Apply.
func withMigrations(m []migration) func(c *applyConfig) {
	return func(c *applyConfig) {
		c.migrations = m
	}
}

var tracer = tracing.NewTracer("migrations")

// migration is an interface responsible for applying data migrations to the backend.
//
// Only a single migration should be run at once. Migrations are applied in sequential
// order based on their Version. No two migrations should have the same Version, and there
// should be no holes in the Version sequence.
type migration interface {
	// Up runs the migration, performing any task needed to convert, alter, or move
	// resources to new key ranges. A successful migration must return nil, any failed
	// migrations must return an error.
	Up(ctx context.Context, b backend.Backend) error
	// Down rolls back the migration, restoring the backend to the state it was in
	// prior to performing Up.
	Down(ctx context.Context, b backend.Backend) error
	// Version is the sequence identifier of the migration.
	Version() int64
	// Name is a friendly identifier used to provide context on what the migration is doing.
	Name() string
}

// Apply executes any outstanding registered migrations.
func Apply(ctx context.Context, log *slog.Logger, b backend.Backend, opts ...func(c *applyConfig)) (err error) {
	cfg := applyConfig{
		migrations: []migration{
			createDBAuthority{},
		},
	}

	for _, o := range opts {
		o(&cfg)
	}

	ctx, span := tracer.Start(ctx, "migrations/Apply")
	defer func() { tracing.EndSpan(span, err) }()

	slices.SortFunc(cfg.migrations, func(a, b migration) int {
		return cmp.Compare(a.Version(), b.Version())
	})

	current, err := getCurrentMigration(ctx, b)
	if err != nil {
		return trace.Wrap(err, "unable to determine current migration version")
	}

	migrationCount := len(cfg.migrations)

	if migrationCount < current.Version {
		return trace.BadParameter("unable to apply migrations: the number of registered migrations is less than the current version, this can be caused by downgraded without rolling back migrations")
	}

	if current.Version > 0 && current.Phase != migrationPhaseComplete {
		return trace.BadParameter("previous attempt to apply migration %d never completed - the failure must be remedied before further migrations can be applied", current.Version)
	}

	for i, m := range cfg.migrations {
		version := i + 1
		if m.Version() != int64(version) {
			return trace.BadParameter("missing migration %d", version)
		}

		if m.Version() <= int64(current.Version) {
			continue
		}

		log := log.With("version", version, "name", m.Name())
		log.InfoContext(ctx, "Starting migration.")
		span.AddEvent("Starting migration", oteltrace.WithAttributes(attribute.Int("migration", version)))

		started := time.Now().UTC()
		if err := setCurrentMigration(ctx, b, migrationStatus{Version: version, Phase: migrationPhaseInProgress, Started: started}); err != nil {
			return trace.Wrap(err)
		}

		if err := m.Up(ctx, b); err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
			return trace.NewAggregate(err, setCurrentMigration(ctx, b, migrationStatus{Version: version, Phase: migrationPhaseError, Started: started, Completed: time.Now().UTC()}))
		}

		if err := setCurrentMigration(ctx, b, migrationStatus{Version: version, Phase: migrationPhaseComplete, Started: started, Completed: time.Now().UTC()}); err != nil {
			span.SetStatus(codes.Error, err.Error())
			span.RecordError(err)
			return trace.Wrap(err)
		}

		log.InfoContext(ctx, "Completed migration.")
		span.AddEvent("Completed migration", oteltrace.WithAttributes(attribute.Int("migration", version)))
	}

	return nil
}

type migrationPhase int

const (
	migrationPhasePending migrationPhase = iota
	migrationPhaseInProgress
	migrationPhaseComplete
	migrationPhaseError
)

type migrationStatus struct {
	Version   int            `json:"version"`
	Phase     migrationPhase `json:"phase"`
	Started   time.Time      `json:"started"`
	Completed time.Time      `json:"completed"`
}

var key = backend.NewKey("migrations", "current")

func setCurrentMigration(ctx context.Context, b backend.Backend, status migrationStatus) error {
	value, err := json.Marshal(status)
	if err != nil {
		return trace.Wrap(err)
	}

	_, err = b.Put(ctx, backend.Item{Key: key, Value: value})
	return trace.Wrap(err)
}

func getCurrentMigration(ctx context.Context, b backend.Backend) (*migrationStatus, error) {
	item, err := b.Get(ctx, key)
	if err != nil {
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err, "unable to determine current migration")
		}

		return &migrationStatus{Version: 0}, nil
	}

	var status migrationStatus
	if err := json.Unmarshal(item.Value, &status); err != nil {
		return nil, trace.Wrap(err)
	}

	return &status, nil
}
