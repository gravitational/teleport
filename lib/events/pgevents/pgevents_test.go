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

package pgevents

import (
	"context"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/events/test"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

const urlEnvVar = "TELEPORT_TEST_PGEVENTS_URL"

func TestPostgresEvents(t *testing.T) {
	s, ok := os.LookupEnv(urlEnvVar)
	if !ok {
		t.Skipf("Missing %v environment variable.", urlEnvVar)
	}

	u, err := url.Parse(s)
	require.NoError(t, err)

	var cfg Config
	require.NoError(t, cfg.SetFromURL(u))

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	log, err := New(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })

	suite := test.EventsSuite{
		Log:   log,
		Clock: clockwork.NewRealClock(),
	}

	// the tests in the suite expect a blank slate each time
	setup := func(t *testing.T) {
		_, err := log.pool.Exec(ctx, "TRUNCATE events")
		require.NoError(t, err)
	}

	t.Run("SessionEventsCRUD", func(t *testing.T) {
		setup(t)
		suite.SessionEventsCRUD(t)
	})
	t.Run("EventPagination", func(t *testing.T) {
		setup(t)
		suite.EventPagination(t)
	})
	t.Run("SearchSessionEventsBySessionID", func(t *testing.T) {
		setup(t)
		suite.SearchSessionEventsBySessionID(t)
	})
}

func TestConfig(t *testing.T) {
	configs := map[string]*Config{
		"postgres://foo#auth_mode=azure": {
			AuthMode:        AzureADAuth,
			RetentionPeriod: defaultRetentionPeriod,
			CleanupInterval: defaultCleanupInterval,
		},
		"postgres://foo": {
			RetentionPeriod: defaultRetentionPeriod,
			CleanupInterval: defaultCleanupInterval,
		},
		"postgres://foo#retention_period=2160h": {
			RetentionPeriod: 2160 * time.Hour,
			CleanupInterval: defaultCleanupInterval,
		},
		"postgres://foo#disable_cleanup=true": {
			DisableCleanup:  true,
			RetentionPeriod: defaultRetentionPeriod,
			CleanupInterval: defaultCleanupInterval,
		},

		"postgres://foo#auth_mode=invalid-auth-mode": nil,
	}

	for u, expectedConfig := range configs {
		u, err := url.Parse(u)
		require.NoError(t, err)
		var actualConfig Config
		require.NoError(t, actualConfig.SetFromURL(u))

		if expectedConfig == nil {
			require.Error(t, actualConfig.CheckAndSetDefaults())
			continue
		}

		require.NoError(t, actualConfig.CheckAndSetDefaults())
		actualConfig.Log = nil
		actualConfig.PoolConfig = nil

		require.Equal(t, expectedConfig, &actualConfig)
	}
}
