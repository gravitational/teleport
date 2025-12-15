/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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
	"os"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

// Testing requires a local psql backend to be set up, and for params to be passed via env. Ex:
// $ docker run -it --rm --env POSTGRES_PASSWORD=insecure -p '127.0.0.1:5432:5432' debezium/postgres:14
// $ export TELEPORT_PGBK_TEST_PARAMS_JSON='{"conn_string":"postgresql://postgres:insecure@localhost/teleport_backend","expiry_interval":"500ms","change_feed_poll_interval":"500ms"}'

// newAtomicWriteTestBackend builds a backend suitable for the atomic write test suite. Once all backends implement AtomicWrite,
// it will be integrated into the main backend interface and we can get rid of this separate helper.
func newAtomicWriteTestBackend(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
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
	if err := json.Unmarshal([]byte(os.Getenv("TELEPORT_PGBK_TEST_PARAMS_JSON")), &params); err != nil {
		return nil, nil, trace.Wrap(err)
	}

	uut, err := NewFromParams(context.Background(), params)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return uut, test.BlockingFakeClock{Clock: clockwork.NewRealClock()}, nil
}

// TestAtomicWriteSuite runs the main atomic write test suite.
func TestAtomicWriteSuite(t *testing.T) {
	if os.Getenv("TELEPORT_PGBK_TEST_PARAMS_JSON") == "" {
		t.Skip("Postgres backend tests are disabled. Enable them by setting the TELEPORT_PGBK_TEST_PARAMS_JSON variable.")
	}

	test.RunAtomicWriteComplianceSuite(t, newAtomicWriteTestBackend)
}

// TestAtomicWriteShim runs the classic test suite using a shim that reimplements all single-item writes as calls
// to AtomicWrite.
func TestAtomicWriteShim(t *testing.T) {
	if os.Getenv("TELEPORT_PGBK_TEST_PARAMS_JSON") == "" {
		t.Skip("Postgres backend tests are disabled. Enable them by setting the TELEPORT_PGBK_TEST_PARAMS_JSON variable.")
	}

	test.RunBackendComplianceSuiteWithAtomicWriteShim(t, newAtomicWriteTestBackend)
}
