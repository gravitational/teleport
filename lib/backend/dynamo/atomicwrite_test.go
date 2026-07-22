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

package dynamo

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

func newAtomicWriteBackend(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
	dynamoCfg := map[string]any{
		"table_name":         dynamoDBTestTable(),
		"poll_stream_period": 300 * time.Millisecond,
	}

	testCfg, err := test.ApplyOptions(options)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if testCfg.MirrorMode {
		return nil, nil, test.ErrMirrorNotSupported
	}

	// This would seem to be a bad thing for dynamo to omit
	if testCfg.ConcurrentBackend != nil {
		return nil, nil, test.ErrConcurrentAccessNotSupported
	}

	uut, err := New(context.Background(), dynamoCfg)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	clock := clockwork.NewFakeClockAt(time.Now())
	uut.clock = clock
	return uut, clock, nil
}

func TestAtomicWriteSuite(t *testing.T) {
	ensureTestsEnabled(t)

	test.RunAtomicWriteComplianceSuite(t, newAtomicWriteBackend)
}

func TestAtomicWriteShim(t *testing.T) {
	ensureTestsEnabled(t)

	test.RunBackendComplianceSuiteWithAtomicWriteShim(t, newAtomicWriteBackend)
}
