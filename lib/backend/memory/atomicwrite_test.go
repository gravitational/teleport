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

package memory

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

// newAtomicWriteTestBackend builds a backend suitable for the atomic write test suite. Once all backends implement AtomicWrite,
// it will be integrated into the main backend interface and we can get rid of this separate helper.
func newAtomicWriteTestBackend(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
	cfg, err := test.ApplyOptions(options)

	if err != nil {
		return nil, nil, err
	}

	if cfg.MirrorMode {
		// atomic write does not support mirror mode
		return nil, nil, test.ErrMirrorNotSupported
	}

	if cfg.ConcurrentBackend != nil {
		switch bk := cfg.ConcurrentBackend.(type) {
		case *Memory:
			return bk, nil, nil
		case test.AtomicWriteShim:
			if _, ok := bk.Backend.(*Memory); !ok {
				return nil, nil, trace.BadParameter("target is not a Memory backend (%T)", cfg.ConcurrentBackend)
			}
			return bk, nil, nil
		default:
			return nil, nil, trace.BadParameter("target is not a Memory backend (%T)", cfg.ConcurrentBackend)
		}
	}

	clock := clockwork.NewFakeClock()
	mem, err := New(Config{
		Clock: clock,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}
	return mem, clock, nil
}

// TestAtomicWriteSuite runs the main atomic write test suite.
func TestAtomicWriteSuite(t *testing.T) {
	test.RunAtomicWriteComplianceSuite(t, newAtomicWriteTestBackend)
}

// TestAtomicWriteShim runs the classic test suite using a shim that reimplements all single-item writes as calls
// to AtomicWrite.
func TestAtomicWriteShim(t *testing.T) {
	test.RunBackendComplianceSuiteWithAtomicWriteShim(t, newAtomicWriteTestBackend)
}
