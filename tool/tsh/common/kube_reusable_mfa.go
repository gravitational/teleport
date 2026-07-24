/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package common

import (
	"context"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/semaphore"

	"github.com/gravitational/teleport/api/client/proto"
)

// reusableMFA holds the reusable MFA response shared across cert issuances
// and the single-flight lock for issuances that may prompt the user.
type reusableMFA struct {
	// requester starts as TSH_KUBE_LOCAL_PROXY_MULTI and permanently drops to
	// the legacy TSH_KUBE_LOCAL_PROXY, with one non-reusable ceremony per cluster,
	// once an auth server rejects reuse.
	requester proto.UserCertsRequest_Requester
	// response is the reusable MFA response captured from a fresh ceremony.
	response *proto.MFAAuthenticateResponse
	// ceremonyLock single-flights issuances that may prompt the user.
	ceremonyLock *semaphore.Weighted
	mu           sync.Mutex
}

func newReusableMFA() *reusableMFA {
	return &reusableMFA{
		requester:    proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI,
		ceremonyLock: semaphore.NewWeighted(1),
	}
}

// State returns the current requester and reusable MFA response.
func (m *reusableMFA) State() (proto.UserCertsRequest_Requester, *proto.MFAAuthenticateResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requester, m.response
}

// Capture stores a response produced by a fresh ceremony for other issuances to replay.
// It is dropped after the fallback to the legacy requester, which does not accept reuse.
func (m *reusableMFA) Capture(response *proto.MFAAuthenticateResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.requester == proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY_MULTI {
		m.response = response
	}
}

// Clear drops the shared reusable response if it still is the given stale one,
// keeping a fresher response a peer may have captured.
func (m *reusableMFA) Clear(stale *proto.MFAAuthenticateResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.response == stale {
		m.response = nil
	}
}

// FallbackToLegacy permanently drops to the legacy per-cluster requester,
// with one non-reusable ceremony per cluster.
func (m *reusableMFA) FallbackToLegacy(ctx context.Context, err error) {
	logger.DebugContext(ctx, "Auth server does not allow reusable MFA for the kube local proxy, falling back to per-cluster MFA ceremonies", "error", err)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requester = proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY
	m.response = nil
}

// FallbackActive reports whether the fallback to the legacy requester happened.
func (m *reusableMFA) FallbackActive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requester == proto.UserCertsRequest_TSH_KUBE_LOCAL_PROXY
}

// AcquireCeremonyLock takes the single-flight ceremony lock, waiting for the current holder to release it first.
func (m *reusableMFA) AcquireCeremonyLock(ctx context.Context) (release func(), err error) {
	if err := m.ceremonyLock.Acquire(ctx, 1); err != nil {
		return nil, trace.Wrap(err)
	}
	var once sync.Once // release is idempotent
	return func() { once.Do(func() { m.ceremonyLock.Release(1) }) }, nil
}
