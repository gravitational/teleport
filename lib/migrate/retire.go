// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package migrate

import (
	"context"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/migrate/hoststep"
)

// retireHostTimeout bounds one host's retire: the host step and the deletion
// of the source node.
const retireHostTimeout = 5 * time.Minute

// RetireOptions select the hosts a retire covers.
type RetireOptions struct {
	// Optional: empty retires every migrated host. Otherwise, node/<uuid> or
	// node/<hostname> retires only that host, which must be migrated.
	Resource string
}

// Retire removes the source agent from every migrated host, or only the one
// opts.Resource names, and its node from the source cluster, leaving the
// scoped agent as the host's only agent. It reaches each host through the
// target cluster, because a session through the source would ride the agent
// it removes. A host that fails is listed in the report, not returned as the
// error. Once hosts have started, the report comes back with any error.
func (m *Migrator) Retire(ctx context.Context, opts RetireOptions) (*Report, error) {
	if err := m.requireState(); err != nil {
		return nil, trace.Wrap(err)
	}
	hosts, err := m.hostsInStates(opts.Resource, "a migrated host", stateMigrated)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(hosts) == 0 {
		return &Report{Verb: "retired"}, nil
	}
	m.progressf("retiring hosts: %d", len(hosts))
	return m.act(ctx, "retired", hosts, retireHostTimeout, m.retireHost)
}

// retireHost checks that the scoped agent is registered in the target, runs
// the retire step on it, then deletes h's node from the source, or leaves it
// to expire when the source identity may not delete nodes. The step
// reads the host itself, so a preflight error from a later plan is ignored.
func (m *Migrator) retireHost(ctx context.Context, h *host) (*outcome, error) {
	if h.Preflight == nil {
		return nil, trace.BadParameter("no preflight facts; run plan --check --resource node/%s", h.ID)
	}
	if err := hoststep.RefuseOtherServices(h.Preflight.Services); err != nil {
		return nil, trace.Wrap(err)
	}
	if _, err := m.target.GetNode(ctx, h.TargetScope, h.TargetID); err != nil {
		if trace.IsNotFound(err) {
			return nil, trace.BadParameter("target agent %s is not in scope %s; run plan", h.TargetID, h.TargetScope)
		}
		return nil, trace.Wrap(err, "reading target agent %s", h.TargetID)
	}
	if err := m.runHostStep(ctx, m.targetHosts, h.TargetID, hoststep.Retire, hoststep.RetireRequest{
		TargetHostUUID: h.TargetID,
		SourceHostUUID: h.ID,
		ConfigPath:     h.Preflight.ConfigPath,
	}, &struct{}{}); err != nil {
		return nil, trace.Wrap(err)
	}
	// The deletion runs detached from ctx, so a run canceled or out of time
	// after the source agent is gone still removes its node.
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	if err := m.deleteNode(cleanupCtx, m.source, "" /* the source is unscoped */, h.ID); err != nil {
		return nil, trace.Wrap(err, "the source agent is already removed from the host; run retire again to delete source node %s", h.ID)
	}
	return &outcome{targetID: h.TargetID, matchedBy: h.MatchedBy, state: stateRetired}, nil
}
