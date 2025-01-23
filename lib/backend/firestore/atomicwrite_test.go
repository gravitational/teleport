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

package firestore

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

func newAtomicWriteTestBackend(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
	cfg := firestoreParams()

	testCfg, err := test.ApplyOptions(options)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if testCfg.MirrorMode {
		return nil, nil, test.ErrMirrorNotSupported
	}

	// This would seem to be a bad thing for firestore to omit
	if testCfg.ConcurrentBackend != nil {
		return nil, nil, test.ErrConcurrentAccessNotSupported
	}

	clock := clockwork.NewRealClock()

	// we can't fiddle with clocks inside the firestore client, so instead of creating
	// and returning a fake clock, we wrap the real clock used by the client
	// in a FakeClock interface that sleeps instead of instantly advancing.
	sleepingClock := test.BlockingFakeClock{Clock: clock}

	uut, err := New(context.Background(), cfg, Options{Clock: sleepingClock})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	return uut, sleepingClock, nil
}

// TestAtomicWriteSuite runs the main atomic write test suite.
func TestAtomicWriteSuite(t *testing.T) {
	ensureTestsEnabled(t)
	ensureEmulatorRunning(t, firestoreParams())

	test.RunAtomicWriteComplianceSuite(t, newAtomicWriteTestBackend)
}

// TestAtomicWriteShim runs the classic test suite using a shim that reimplements all single-item writes as calls
// to AtomicWrite.
func TestAtomicWriteShim(t *testing.T) {
	ensureTestsEnabled(t)
	ensureEmulatorRunning(t, firestoreParams())

	test.RunBackendComplianceSuiteWithAtomicWriteShim(t, newAtomicWriteTestBackend)
}
