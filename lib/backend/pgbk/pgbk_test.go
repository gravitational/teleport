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

package pgbk

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/lib/backend"
	pgcommon "github.com/gravitational/teleport/lib/backend/pgbk/common"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils/clocki"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
)

func TestMain(m *testing.M) {
	logtest.InitLogger(testing.Verbose)
	os.Exit(m.Run())
}

func TestPostgresBackend(t *testing.T) {
	// expiry_interval needs to be really short to pass some of the tests, and a
	// faster poll interval helps a bit with runtime:
	// {"conn_string":"...","expiry_interval":"500ms","change_feed_poll_interval":"500ms"}
	paramString := os.Getenv("TELEPORT_PGBK_TEST_PARAMS_JSON")
	if paramString == "" {
		t.Skip("Postgres backend tests are disabled. Enable them by setting the TELEPORT_PGBK_TEST_PARAMS_JSON variable.")
	}

	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
		testCfg, err := test.ApplyOptions(options)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		if testCfg.MirrorMode {
			return nil, nil, test.ErrMirrorNotSupported
		}

		if testCfg.ConcurrentBackend != nil {
			return nil, nil, test.ErrConcurrentAccessNotSupported
		}

		var params backend.Params
		require.NoError(t, json.Unmarshal([]byte(paramString), &params))

		uut, err := NewFromParams(context.Background(), params)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return uut, test.BlockingFakeClock{Clock: clockwork.NewRealClock()}, nil
	}

	test.RunBackendComplianceSuite(t, newBackend)
}

func TestSetupAndMigrateDynamicConcurrent(t *testing.T) {
	ctx := t.Context()
	pool := newPostgresPoolForTest(t, ctx)

	id := time.Now().UnixNano()

	versionTableName := fmt.Sprintf("pgbk_version_%d", id)
	migratedTableName := fmt.Sprintf("pgbk_migration_%d", id)
	migratedTable := pgx.Identifier{migratedTableName}.Sanitize()
	versionTable := pgx.Identifier{versionTableName}.Sanitize()

	cleanup := func(ctx context.Context) {
		_, _ = pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", migratedTable), pgx.QueryExecModeExec)
		_, _ = pool.Exec(ctx, fmt.Sprintf("DROP TABLE IF EXISTS %s", versionTable), pgx.QueryExecModeExec)
	}
	cleanup(ctx) // This should be impossible, but ensure the tables are gone either way.
	t.Cleanup(func() {
		cleanup(ctx)
	})

	staleMigrationReady := make(chan struct{})
	releaseStaleMigration := make(chan struct{})
	var builderCalls atomic.Int64
	schemas := []string{
		fmt.Sprintf("CREATE TABLE %s (key bytea PRIMARY KEY)", migratedTable),
	}

	schemasBuilder := func(*pgx.Conn) ([]string, error) {
		if builderCalls.Add(1) == 1 {
			close(staleMigrationReady)
			select {
			case <-releaseStaleMigration:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		return schemas, nil
	}

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return pgcommon.SetupAndMigrateDynamic(groupCtx, logtest.NewLogger(), pool, versionTableName, schemasBuilder)
	})

	select {
	case <-staleMigrationReady:
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	committedMigration := make(chan struct{})
	group.Go(func() error {
		err := pgcommon.SetupAndMigrateDynamic(groupCtx, logtest.NewLogger(), pool, versionTableName, schemasBuilder)
		close(committedMigration)
		return err
	})

	<-committedMigration
	close(releaseStaleMigration)

	require.NoError(t, group.Wait())
	require.GreaterOrEqual(t, builderCalls.Load(), int64(3))
}

func newPostgresPoolForTest(t *testing.T, ctx context.Context) *pgxpool.Pool {
	t.Helper()

	paramString := os.Getenv("TELEPORT_PGBK_TEST_PARAMS_JSON")
	if paramString == "" {
		t.Skip("Postgres backend tests are disabled. Enable them by setting the TELEPORT_PGBK_TEST_PARAMS_JSON variable.")
	}

	var cfg Config
	require.NoError(t, json.Unmarshal([]byte(paramString), &cfg))
	require.NotEmpty(t, cfg.ConnString)

	poolConfig, err := pgxpool.ParseConfig(cfg.ConnString)
	require.NoError(t, err)

	// We need at least 2 connections to test concurrent migrations.
	poolConfig.MaxConns = max(poolConfig.MaxConns, 2)

	require.NoError(t, cfg.AuthConfig.ApplyToPoolConfigs(ctx, logtest.NewLogger(), poolConfig))

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	return pool
}
