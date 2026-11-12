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

package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/migrate/hoststep"
)

// promoteHostTimeout bounds one host's promote: the install, which downloads
// the default install's Teleport, the wait for the promoted agent to join,
// and the scoped install's removal.
const promoteHostTimeout = 15 * time.Minute

// PromoteOptions select the hosts a promote covers.
type PromoteOptions struct {
	// Optional: empty promotes every retired host. Otherwise, node/<uuid> or
	// node/<hostname> promotes only that host, which must be retired.
	Resource string
}

// Promote replaces the scoped agent of every retired host, or only the one
// opts.Resource names, with an agent on teleport.service, /etc/teleport.yaml
// and the default data directory, the way apply replaced the source agent:
// the new agent joins the target in the same scope with the same labels, then
// the scoped install is removed through it. It reaches each host through the
// target cluster. A host that fails is listed in the report, not returned as
// the error. Once hosts have started, the report comes back with any error.
func (m *Migrator) Promote(ctx context.Context, opts PromoteOptions) (*Report, error) {
	if err := m.requireState(); err != nil {
		return nil, trace.Wrap(err)
	}
	hosts, err := m.hostsInStates(opts.Resource, "a retired host", stateRetired)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(hosts) == 0 {
		return &Report{Verb: "promoted"}, nil
	}
	pins, err := m.target.GetCAPins(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "reading the target cluster's CA pins")
	}
	m.progressf("promoting hosts: %d", len(hosts))
	return m.act(ctx, "promoted", hosts, promoteHostTimeout, promoteRun{m: m, pins: pins}.promoteHost)
}

// promoteRun binds the CA pins, read once per promote, to promoteHost.
type promoteRun struct {
	m *Migrator
	// pins are the target cluster's CA pins.
	pins []string
}

// promoteHost promotes h in steps:
//   - finds the agent to reach h through (promoteVia)
//   - mints a token for the promoted agent, unless h was promoted by hand
//   - runs the promote step, which installs and starts the promoted agent
//   - waits for the target to list it (awaitJoin)
//   - removes the scoped install through it (removeScopedInstall)
//   - deletes the scoped agent's records from the target (deleteScopedRecords)
//
// Until the promoted agent joins, a failure leaves h retired with the scoped
// agent serving, and stops any promoted agent the step started (abortPromote);
// a rerun replaces it. Once it has joined, h is promoted under the promoted
// agent's host UUID, even when a later step fails. A host promoted by hand
// gets no token and no abort; its scoped install, if any, is still removed.
func (r promoteRun) promoteHost(ctx context.Context, h *host) (*outcome, error) {
	m := r.m
	if h.Preflight == nil {
		return nil, trace.BadParameter("no preflight facts; run plan --check --resource node/%s", h.ID)
	}
	via, byHand, err := m.promoteVia(ctx, h)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	request := hoststep.PromoteRequest{
		Proxy:          m.config.Target.ProxyServer,
		SourceHostUUID: h.ID,
		Wait:           targetWait,
		RecordOnly:     byHand,
	}
	if !byHand {
		credential, err := m.issueJoinCredential(ctx, h, promotedTokenName(h.TargetScope, h.ID))
		if err != nil {
			return nil, trace.Wrap(err, "issuing the join credential")
		}
		request.JoinMethod = h.Preflight.JoinMethod
		request.TokenName = credential.tokenName
		request.RegistrationSecret = credential.registrationSecret
		request.CAPins = r.pins
	}
	var response hoststep.InstallResponse
	if err := m.runHostStep(ctx, m.targetHosts, via, hoststep.Promote, request, &response); err != nil {
		// A step that exited stopped any promoted agent it had started. A
		// step that did not finish may have left one running.
		var exit *stepExitError
		if byHand || errors.As(err, &exit) {
			return nil, trace.Wrap(err)
		}
		if abortErr := m.abortPromote(ctx, via, request); abortErr != nil {
			return nil, trace.NewAggregate(trace.Wrap(err, "the promote step did not finish"), abortErr)
		}
		return nil, trace.Wrap(err, "the promote step did not finish; the promoted agent was stopped and disabled again and the scoped agent still serves")
	}
	if err := m.awaitJoin(ctx, h, response.HostUUID); err != nil {
		if byHand {
			return nil, trace.Wrap(err, "waiting for the target to list the agent promoted by hand")
		}
		if abortErr := m.abortPromote(ctx, via, request); abortErr != nil {
			return nil, trace.NewAggregate(trace.Wrap(err, "waiting for the promoted agent to join"), abortErr)
		}
		return nil, trace.Wrap(err, "waiting for the promoted agent to join; the promoted agent was stopped and disabled again and the scoped agent still serves")
	}
	scopedID := h.TargetID
	promoted := &outcome{targetID: response.HostUUID, scopedID: scopedID, matchedBy: h.MatchedBy, state: statePromoted}
	if err := m.removeScopedInstall(ctx, h, response.HostUUID); err != nil {
		return promoted, trace.Wrap(err, "the promoted agent joined, but removing the scoped install failed; remove it by hand")
	}
	return promoted, trace.Wrap(m.deleteScopedRecords(ctx, h, scopedID))
}

// deleteScopedRecords deletes the scoped agent's node and the token apply
// minted for h from the target. It runs detached from ctx, so a run canceled
// or out of time after the scoped install's removal still deletes them. A
// record already gone is not an error, and neither is a node the target
// identity may not delete, which is left to expire.
func (m *Migrator) deleteScopedRecords(ctx context.Context, h *host, scopedID string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	var errs []error
	if err := m.deleteNode(cleanupCtx, m.target, h.TargetScope, scopedID); err != nil {
		errs = append(errs, trace.Wrap(err, "remove the scoped agent %s with tctl rm node %s::%s", scopedID, h.TargetScope, scopedID))
	}
	if hoststep.IsMintedJoinMethod(h.Preflight.JoinMethod) {
		name := mintedTokenName(h.TargetScope, h.ID)
		if err := m.target.DeleteScopedToken(cleanupCtx, h.TargetScope, name); err != nil && !trace.IsNotFound(err) {
			errs = append(errs, trace.Wrap(err, "remove scoped token %s::%s with tctl rm scoped_token %s::%s", h.TargetScope, name, h.TargetScope, name))
		}
	}
	if len(errs) > 0 {
		return trace.Wrap(trace.NewAggregate(errs...), "host promoted, target cleanup incomplete")
	}
	return nil
}

// abortPromote stops and disables a promoted agent the promote step may have
// started, so it does not keep retrying beside the scoped agent. It runs the
// step with Abort through via, the scoped agent, and returns an error only
// when that stop fails.
//
// The stop runs detached from ctx, so a run canceled or out of time still
// stops the agent. Its timeout adds request.Wait, how long the abort step
// waits for the install lock that an unfinished step may still hold.
func (m *Migrator) abortPromote(ctx context.Context, via string, request hoststep.PromoteRequest) error {
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), request.Wait+cleanupTimeout)
	defer cancel()
	request.Abort = true
	err := m.runHostStep(stopCtx, m.targetHosts, via, hoststep.Promote, request, &struct{}{})
	return trace.Wrap(err, "stopping the promoted agent; stop and disable teleport.service on the host by hand")
}

// promoteVia returns the agent the promote step reaches h through. That is
// h's scoped agent while the target lists it. Once a by-hand promote has
// removed it, it is the one agent in h's scope labeled as migrated from h,
// and byHand is set: that agent is the host's only one, so the step must only
// record it, because stopping or restarting it would end the step's session.
// Its two refusals carry hoststep.PromoteRefusal; an error looking the agents
// up is returned as it is, since a rerun may get past it.
func (m *Migrator) promoteVia(ctx context.Context, h *host) (via string, byHand bool, err error) {
	_, err = m.target.GetNode(ctx, h.TargetScope, h.TargetID)
	switch {
	case err == nil:
		// The target lists the scoped agent, so the step runs through it.
		return h.TargetID, false, nil
	case !trace.IsNotFound(err):
		return "", false, trace.Wrap(err, "looking up the scoped agent %s", h.TargetID)
	}
	// The scoped agent is gone from the target. Look for the agent a by-hand
	// promote put in its place: an agent in h's scope labeled as migrated from
	// h. Exactly one must exist, or the host is refused.
	sourceCluster := m.source.ClusterName()
	var ids []string
	for n, err := range m.target.Nodes(ctx) {
		if err != nil {
			return "", false, trace.Wrap(err)
		}
		if n.ID != h.TargetID && isSameScope(n.Scope, h.TargetScope) && labeledSourceHost(n, sourceCluster) == h.ID {
			ids = append(ids, n.ID)
		}
	}
	switch len(ids) {
	case 1:
		return ids[0], true, nil
	case 0:
		return "", false, trace.NotFound("%s: the scoped agent %s is not in the target, and no other agent in %s carries this host's migration label", hoststep.PromoteRefusal, h.TargetID, h.TargetScope)
	}
	slices.Sort(ids)
	return "", false, trace.BadParameter("%s: the scoped agent %s is not in the target, and %d agents in %s carry this host's migration label (%s)", hoststep.PromoteRefusal, h.TargetID, len(ids), h.TargetScope, strings.Join(ids, ", "))
}

// removeScopedInstall runs the uninstall step on h through its promoted agent
// promotedID. The target lists a joined agent before its proxy can route to
// it, so an unreachable host is retried for up to targetWait; uninstall is
// idempotent, so a retry after a dropped session is safe.
func (m *Migrator) removeScopedInstall(ctx context.Context, h *host, promotedID string) error {
	retry, err := retryutils.NewConstant(pollInterval)
	if err != nil {
		return trace.Wrap(err)
	}
	waitCtx, cancel := context.WithTimeout(ctx, targetWait)
	defer cancel()
	var last error
	err = retry.For(waitCtx, func() error {
		last = m.runHostStep(ctx, m.targetHosts, promotedID, hoststep.Uninstall, hoststep.UninstallRequest{
			Proxy:          m.config.Target.ProxyServer,
			SourceHostUUID: h.ID,
			Promote:        true,
		}, &struct{}{})
		if last != nil && !trace.IsConnectionProblem(last) {
			return retryutils.PermanentRetryError(last)
		}
		return last
	})
	if err != nil && !retryutils.IsPermanent(err) && last != nil {
		return trace.Wrap(last)
	}
	return trace.Wrap(err)
}

// promotedTokenName is the name of the token promote mints for hostID in
// scope. It differs from apply's, so minting it never replaces the token the
// running scoped agent joined with.
func promotedTokenName(scope, hostID string) string {
	sum := sha256.Sum256([]byte(scope + "\x00" + hostID + "\x00promote"))
	return "migrate-" + hex.EncodeToString(sum[:8])
}
