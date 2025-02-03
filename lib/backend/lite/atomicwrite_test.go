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

package lite

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

// newAtomicWriteTestBackendBuilder builds a backend suitable for the atomic write test suite. Once all backends implement AtomicWrite,
// it will be integrated into the main backend interface and we can get rid of this separate helper.
func newAtomicWriteTestBackendBuilder(t *testing.T) test.Constructor {
	return func(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
		clock := clockwork.NewFakeClock()

		cfg, err := test.ApplyOptions(options)
		if err != nil {
			return nil, nil, err
		}

		if cfg.ConcurrentBackend != nil {
			return nil, nil, test.ErrConcurrentAccessNotSupported
		}

		if cfg.MirrorMode {
			return nil, nil, test.ErrMirrorNotSupported
		}

		backend, err := NewWithConfig(context.Background(), Config{
			Path:             t.TempDir(),
			PollStreamPeriod: 300 * time.Millisecond,
			Clock:            clock,
		})

		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return backend, clock, nil
	}
}

// TestAtomicWriteSuite runs the main atomic write test suite.
func TestAtomicWriteSuite(t *testing.T) {
	test.RunAtomicWriteComplianceSuite(t, newAtomicWriteTestBackendBuilder(t))
}

// TestAtomicWriteShim runs the classic test suite using a shim that reimplements all single-item writes as calls
// to AtomicWrite.
func TestAtomicWriteShim(t *testing.T) {
	test.RunBackendComplianceSuiteWithAtomicWriteShim(t, newAtomicWriteTestBackendBuilder(t))
}
