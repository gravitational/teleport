/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package workloadidentity

import (
	"bytes"
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/gravitational/trace"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
)

type CRLSet struct {
	LocalCRL []byte
	stale    chan struct{}
}

// Clone returns a deep copy of the CRLSet.
func (b *CRLSet) Clone() *CRLSet {
	clone := &CRLSet{}
	if b.LocalCRL != nil {
		clone.LocalCRL = make([]byte, len(b.LocalCRL))
		copy(clone.LocalCRL, b.LocalCRL)
	}
	return clone
}

// Stale returns a channel that will be closed when the CRLSet is stale
// and a new CRLSet is available.
func (b *CRLSet) Stale() <-chan struct{} {
	return b.stale
}

type CRLCache struct {
	revocationsClient workloadidentityv1pb.WorkloadIdentityRevocationServiceClient
	logger            *slog.Logger

	mu     sync.Mutex
	crlSet *CRLSet
	// initialized will close when the cache is fully initialized.
	initialized chan struct{}
}

// String returns a string representation of the CRLCache. Implements the
// tbot Service interface and fmt.Stringer interface.
func (m *CRLCache) String() string {
	return "crl-cache"
}

func (m *CRLCache) Run(ctx context.Context) error {
	for {
		m.logger.InfoContext(
			ctx,
			"Initializing cache",
		)
		if err := m.watch(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			// TODO(noah): DELETE IN V19 once CRL streaming functionality is
			// available on all supported versions.
			if trace.IsNotImplemented(err) {
				m.logger.WarnContext(
					ctx, "Server does not support X509 CRL functionality",
				)
				// Set empty CRL set so consumers are unblocked.
				m.setCRLSet(ctx, &CRLSet{})
				return nil
			}
			m.logger.ErrorContext(
				ctx,
				"Cache failed, will attempt to re-initialize after back off",
				"error", err,
				"backoff", trustBundleInitFailureBackoff,
			)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(trustBundleInitFailureBackoff):
			continue
		}
	}
}

func (m *CRLCache) watch(ctx context.Context) error {
	stream, err := m.revocationsClient.StreamSignedCRL(
		ctx, &workloadidentityv1pb.StreamSignedCRLRequest{},
	)
	if err != nil {
		return trace.Wrap(err, "streaming CRL")
	}

	for {
		res, err := stream.Recv()
		if err != nil {
			return trace.Wrap(err, "receiving CRL")
		}
		m.setCRLSet(ctx, &CRLSet{
			LocalCRL: res.Crl,
		})
	}
}

func (m *CRLCache) setCRLSet(ctx context.Context, crlSet *CRLSet) {
	m.mu.Lock()
	old := m.crlSet

	// Exit early if the CRL set is the same as the current one.
	if old != nil {
		if bytes.Equal(old.LocalCRL, crlSet.LocalCRL) {
			m.logger.DebugContext(ctx, "Ignoring unchanged CRL set")
		}
	}

	// Clone the CRL set to avoid the caller mutating the state after it has
	// been set.
	m.crlSet = crlSet.Clone()
	m.crlSet.stale = make(chan struct{})

	if old == nil {
		// Indicate that the first CRL set is now available.
		close(m.initialized)
	} else {
		// Indicate that a new CRL set is available.
		close(old.stale)
	}
	m.mu.Unlock()
}

func (m *CRLCache) getCRLSet() *CRLSet {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.crlSet == nil {
		return nil
	}
	// Clone so a receiver cannot mutate the current state without calling
	// setCRLSet.
	return m.crlSet.Clone()
}

// GetCRLSet returns the current CRLSet. If the cache is not yet
// initialized, it will block until it is.
func (m *CRLCache) GetCRLSet(
	ctx context.Context,
) (*CRLSet, error) {
	select {
	case <-m.initialized:
		return m.getCRLSet(), nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
