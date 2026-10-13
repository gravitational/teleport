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

package mfa

import (
	"context"
	"errors"
	"strings"
	"sync"

	"github.com/gravitational/trace"
	"golang.org/x/sync/semaphore"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/mfa"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

var reusableMFALog = logutils.NewPackageLogger(teleport.ComponentKey, teleport.ComponentTSH)

// ReusableMFA coordinates certificate issuance using a shared MFA response.
// Issuances that may prompt the user hold the ceremony lock; replays of a shared
// response can run concurrently.
type ReusableMFA struct {
	// requester starts as the reusable requester (eg TSH_KUBE_LOCAL_PROXY_MULTI) and
	// permanently drops to the legacy requester (eg TSH_KUBE_LOCAL_PROXY) when
	// an auth server rejects reuse.
	requester proto.UserCertsRequest_Requester
	// reusableRequester is the requester used when attempting the shared MFA flow
	reusableRequester proto.UserCertsRequest_Requester
	// legacyRequester is the requester to fall back to if shared MFA is not able to be used
	legacyRequester proto.UserCertsRequest_Requester
	// response is the reusable MFA response captured from a fresh ceremony.
	response *proto.MFAAuthenticateResponse
	// ceremonyLock single-flights issuances that may prompt the user.
	ceremonyLock *semaphore.Weighted
	mu           sync.Mutex
}

// NewReusableMFA creates state for one local proxy invocation. reusableRequester
// selects the flow that supports MFA reuse; legacyRequester selects the fallback
// flow when the Auth server rejects reuse.
func NewReusableMFA(reusableRequester, legacyRequester proto.UserCertsRequest_Requester) *ReusableMFA {
	return &ReusableMFA{
		// initialize the requester to the reusable one
		requester:         reusableRequester,
		reusableRequester: reusableRequester,
		legacyRequester:   legacyRequester,
		ceremonyLock:      semaphore.NewWeighted(1),
	}
}

// State returns the current requester and reusable MFA response.
func (m *ReusableMFA) State() (proto.UserCertsRequest_Requester, *proto.MFAAuthenticateResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requester, m.response
}

// Capture stores a response produced by a fresh ceremony for other issuances to replay.
// Calls after fallback to the legacy requester are ignored.
func (m *ReusableMFA) Capture(response *proto.MFAAuthenticateResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.requester == m.reusableRequester {
		m.response = response
	}
}

// Clear drops the shared reusable response if it still is the given stale one,
// keeping a fresher response a peer may have captured.
func (m *ReusableMFA) Clear(stale *proto.MFAAuthenticateResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.response == stale {
		m.response = nil
	}
}

// FallbackToLegacy permanently switches to the configured legacy requester and
// clears the shared reusable response.
func (m *ReusableMFA) FallbackToLegacy(ctx context.Context, err error) {
	reusableMFALog.DebugContext(ctx, "Auth server does not allow reusable MFA, falling back to per-cluster MFA ceremonies", "error", err)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.requester = m.legacyRequester
	m.response = nil
}

// AcquireCeremonyLock waits for the ceremony lock or for ctx to be canceled.
// On success, the caller must call the returned release function when finished.
// Calling release more than once is safe.
func (m *ReusableMFA) AcquireCeremonyLock(ctx context.Context) (release func(), err error) {
	if err := m.ceremonyLock.Acquire(ctx, 1); err != nil {
		return nil, trace.Wrap(err)
	}
	return sync.OnceFunc(func() { m.ceremonyLock.Release(1) }), nil
}

// IssueParams contains the MFA settings selected for an issuance attempt.
// Callers apply these settings to their certificate request on every callback.
type IssueParams struct {
	// RequesterName selects the reusable or legacy certificate issuance flow.
	RequesterName proto.UserCertsRequest_Requester
	// ReusableMFAResponse is nil for a fresh ceremony, or the response to replay.
	ReusableMFAResponse *proto.MFAAuthenticateResponse
	// FailOnExpiredReusableMFAResponse prevents a replay from prompting outside the ceremony lock.
	FailOnExpiredReusableMFAResponse bool
}

// Issue coordinates certificate issuance through issue. It replays a shared MFA
// response when available and otherwise serializes attempts that may prompt the
// user. An expired response is refreshed; a rejection of reuse triggers fallback
// to the legacy requester.
//
// The callback performs issuance and any result processing, retains its result
// for the caller, and must call Capture when a fresh ceremony produces a reusable
// MFA response. It may be called multiple times as Issue retries. The callback
// runs synchronously, but replay callbacks from separate Issue calls may run
// concurrently. Processing in the callback remains inside the ceremony lock for
// attempts that may prompt the user.
func (m *ReusableMFA) Issue(ctx context.Context, issue func(ctx context.Context, params IssueParams) error) error {
	var params IssueParams
	// Tolerate up to 3 rejected replays before giving up. Only two rejections are recoverable:
	// - The replayed response outlived the server's reuse window and expired.
	// - This auth server does not allow reuse at all (a mixed-version auth pool).
	// The worst honest sequence is one of each, so the bound leaves margin while
	// making termination unconditional. Any other error fails the issuance immediately.
	const maxRejections = 3
	var rejections int
	var lastErr error
	for rejections < maxRejections {
		requester, reusable := m.State()

		if reusable == nil {
			done, err := m.issueWithCeremony(ctx, params, issue)
			if err != nil {
				return trace.Wrap(err)
			}
			if !done {
				// A peer's ceremony captured a fresh response. Replay it.
				continue
			}
			return nil
		}

		params.RequesterName = requester
		params.ReusableMFAResponse = reusable
		params.FailOnExpiredReusableMFAResponse = true
		err := issue(ctx, params)
		switch {
		case errors.Is(err, &mfa.ErrExpiredReusableMFAResponse):
			// The response expired mid-flight. Drop it and loop into the ceremony path.
			m.Clear(reusable)
		case isMFAReuseRejected(err):
			// This auth server does not allow reuse (a mixed-version auth pool).
			m.FallbackToLegacy(ctx, err)
		default:
			return trace.Wrap(err)
		}
		rejections++
		lastErr = err
		params.ReusableMFAResponse = nil
		params.FailOnExpiredReusableMFAResponse = false
	}
	return trace.Wrap(lastErr, "giving up after %d rejected MFA responses", maxRejections)
}

// issueWithCeremony runs an issuance that may prompt the user, under the single-flight lock.
// It reports done=false without issuing when the ceremony it waited on captured a reusable response.
// The caller replays it instead, with no prompt.
func (m *ReusableMFA) issueWithCeremony(ctx context.Context, params IssueParams, issue func(ctx context.Context, params IssueParams) error) (done bool, err error) {
	release, err := m.AcquireCeremonyLock(ctx)
	if err != nil {
		return false, trace.Wrap(err)
	}
	defer release()

	requester, reusable := m.State()
	if reusable != nil {
		return false, nil
	}

	params.RequesterName = requester
	params.ReusableMFAResponse = nil
	params.FailOnExpiredReusableMFAResponse = false
	err = issue(ctx, params)
	if requester == m.reusableRequester {
		switch {
		case isMFAReuseRejected(err):
			// An auth server that predates the multi-requester rejected the ceremony.
			m.FallbackToLegacy(ctx, err)
			params.RequesterName = m.legacyRequester
			err = issue(ctx, params)
		case isMFAReuseRejectionSuspected(err):
			// A masked rejection from an old auth server and
			// a transient challenge-creation failure are indistinguishable, so fall back for this issuance only.
			// Every fresh ceremony probes the reusable requester again.
			// A transient failure cannot degrade the proxy permanently,
			// and an old auth server costs one failed request per ceremony.
			reusableMFALog.DebugContext(ctx, "MFA challenge creation failed, retrying this issuance with a per-cluster ceremony", "error", err)
			params.RequesterName = m.legacyRequester
			err = issue(ctx, params)
		}
	}
	return true, trace.Wrap(err)
}

// isMFAReuseRejected reports whether an auth server unambiguously rejected the reusable MFA flow.
func isMFAReuseRejected(err error) bool {
	// Auth servers that validate challenge scopes reject the unknown scope with a typed error at challenge creation.
	if errors.Is(err, &mfa.ErrUnknownChallengeScope) {
		return true
	}
	// Servers that predate scope validation reject the response at validation without a typed error, recognized by message below.
	if trace.IsAccessDenied(err) || trace.IsBadParameter(err) {
		msg := err.Error()
		return strings.Contains(msg, "is not satisfied by the given") || // response scope unknown to the server, rejected at validation
			strings.Contains(msg, "reuse is not permitted") // server knows the scope but does not allow reuse for the requester
	}
	return false
}

// isMFAReuseRejectionSuspected reports whether the error may be a masked reuse rejection.
// Auth servers that predate challenge scope validation reject the unknown scope
// at challenge creation, masked behind their generic challenge-creation failure message.
func isMFAReuseRejectionSuspected(err error) bool {
	return trace.IsAccessDenied(err) && strings.Contains(err.Error(), "unable to create MFA challenges")
}
