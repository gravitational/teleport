// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pgevents

import (
	"context"
	"net/url"
	"os"
	"testing"

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
