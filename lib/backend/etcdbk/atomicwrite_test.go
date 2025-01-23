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

package etcdbk

import (
	"context"
	"testing"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/test"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

// newAtomicWriteTestBackend builds a backend suitable for the atomic write test suite. Once all backends implement AtomicWrite,
// it will be integrated into the main backend interface and we can get rid of this separate helper.
func newAtomicWriteTestBackend(options ...test.ConstructionOption) (backend.Backend, clocki.FakeClock, error) {
	opts, err := test.ApplyOptions(options)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if opts.MirrorMode {
		return nil, nil, test.ErrMirrorNotSupported
	}

	// No need to check target backend - all Etcd backends create by this test
	// point to the same datastore.

	bk, err := New(context.Background(), commonEtcdParams, commonEtcdOptions...)
	if err != nil {
		return nil, nil, err
	}

	// we can't fiddle with clocks inside the etcd client, so instead of creating
	// and returning a fake clock, we wrap the real clock used by the etcd client
	// in a FakeClock interface that sleeps instead of instantly advancing.
	sleepingClock := test.BlockingFakeClock{Clock: bk.clock}

	return bk, sleepingClock, nil
}

// TestAtomicWriteSuite runs the main atomic write test suite.
func TestAtomicWriteSuite(t *testing.T) {
	if !etcdTestEnabled() {
		t.Skip("This test requires etcd, run `make run-etcd` and set TELEPORT_ETCD_TEST=yes in your environment")
	}

	test.RunAtomicWriteComplianceSuite(t, newAtomicWriteTestBackend)
}

// TestAtomicWriteShim runs the classic test suite using a shim that reimplements all single-item writes as calls
// to AtomicWrite.
func TestAtomicWriteShim(t *testing.T) {
	if !etcdTestEnabled() {
		t.Skip("This test requires etcd, run `make run-etcd` and set TELEPORT_ETCD_TEST=yes in your environment")
	}

	test.RunBackendComplianceSuiteWithAtomicWriteShim(t, newAtomicWriteTestBackend)
}
