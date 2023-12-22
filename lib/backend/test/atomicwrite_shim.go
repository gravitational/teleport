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
package test

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
)

// RunBackendComplianceSuiteWithAtomicWriteShim runs the old backend compliance suite against the provided backend
// with a shim that converts all calls to single-write methods (all write methods but DeleteRange) into calls to
// AtomicWrite. This is done to ensure that the relationship between the conditional actions of AtomicWrite and the
// single-write methods is well defined, and to improve overall coverage of AtomicWrite implementations via reuse.
func RunBackendComplianceSuiteWithAtomicWriteShim(t *testing.T, newBackend AtomicWriteConstructor) {
	RunBackendComplianceSuite(t, func(options ...ConstructionOption) (backend.Backend, clockwork.FakeClock, error) {
		bk, clock, err := newBackend(options...)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		return atomicWriteShim{bk}, clock, nil
	})
}

// atomciWriteShim reimplements all single-write backend methods as calls to AtomicWrite.
type atomicWriteShim struct {
	backend.AtomicWriteBackend
}

// Create creates item if it does not exist
func (a atomicWriteShim) Create(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	rev, err := a.AtomicWrite(ctx, backend.ConditionalAction{
		Key:       i.Key,
		Condition: backend.NotExists(),
		Action:    backend.Put(i),
	})
	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.AlreadyExists("already exists")
		}
		return nil, trace.Wrap(err)
	}
	return &backend.Lease{
		Key:      i.Key,
		Revision: rev,
	}, nil
}

// Put puts value into backend (creates if it does not
// exists, updates it otherwise)
func (a atomicWriteShim) Put(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	rev, err := a.AtomicWrite(ctx, backend.ConditionalAction{
		Key:       i.Key,
		Condition: backend.Whatever(),
		Action:    backend.Put(i),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &backend.Lease{
		Key:      i.Key,
		Revision: rev,
	}, nil
}

// CompareAndSwap compares item with existing item
// and replaces is with replaceWith item
func (a atomicWriteShim) CompareAndSwap(ctx context.Context, expected backend.Item, replaceWith backend.Item) (*backend.Lease, error) {
	existing, err := a.Get(ctx, replaceWith.Key)
	if err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.CompareFailed("compare failed")
		}
		return nil, trace.Wrap(err)
	}

	if !bytes.Equal(expected.Value, existing.Value) {
		return nil, trace.CompareFailed("not equal")
	}

	rev, err := a.AtomicWrite(ctx, backend.ConditionalAction{
		Key:       replaceWith.Key,
		Condition: backend.Revision(existing.Revision),
		Action:    backend.Put(replaceWith),
	})

	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.CompareFailed("compare failed")
		}
		return nil, trace.Wrap(err)
	}

	return &backend.Lease{
		Key:      replaceWith.Key,
		Revision: rev,
	}, nil
}

// Update updates value in the backend
func (a atomicWriteShim) Update(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	rev, err := a.AtomicWrite(ctx, backend.ConditionalAction{
		Key:       i.Key,
		Condition: backend.Exists(),
		Action:    backend.Put(i),
	})

	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.NotFound("not found")
		}

		return nil, trace.Wrap(err)
	}

	return &backend.Lease{
		Key:      i.Key,
		Revision: rev,
	}, nil
}

// Delete deletes item by key, returns NotFound error
// if item does not exist
func (a atomicWriteShim) Delete(ctx context.Context, key []byte) error {
	_, err := a.AtomicWrite(ctx, backend.ConditionalAction{
		Key:       key,
		Condition: backend.Exists(),
		Action:    backend.Delete(),
	})

	if errors.Is(err, backend.ErrConditionFailed) {
		return trace.NotFound("not found")
	}

	return trace.Wrap(err)
}

// ConditionalUpdate updates the value in the backend if the revision of the [backend.Item] matches
// the stored revision.
func (a atomicWriteShim) ConditionalUpdate(ctx context.Context, i backend.Item) (*backend.Lease, error) {
	rev, err := a.AtomicWrite(ctx, backend.ConditionalAction{
		Key:       i.Key,
		Condition: backend.Revision(i.Revision),
		Action:    backend.Put(i),
	})

	if err != nil {
		if errors.Is(err, backend.ErrConditionFailed) {
			return nil, trace.Wrap(backend.ErrIncorrectRevision)
		}

		return nil, trace.Wrap(err)
	}

	return &backend.Lease{
		Key:      i.Key,
		Revision: rev,
	}, nil
}

// ConditionalDelete deletes the item by key if the revision matches the stored revision.
func (a atomicWriteShim) ConditionalDelete(ctx context.Context, key []byte, revision string) error {
	_, err := a.AtomicWrite(ctx, backend.ConditionalAction{
		Key:       key,
		Condition: backend.Revision(revision),
		Action:    backend.Delete(),
	})

	if errors.Is(err, backend.ErrConditionFailed) {
		return trace.Wrap(backend.ErrIncorrectRevision)
	}

	return trace.Wrap(err)
}
