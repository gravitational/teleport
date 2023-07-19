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

package pgbk

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
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

	newBackend := func(options ...test.ConstructionOption) (backend.Backend, clockwork.FakeClock, error) {
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
