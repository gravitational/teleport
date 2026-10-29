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

// rollbackHostTimeout bounds one host's rollback: the uninstall step and the
// target cleanup.
const rollbackHostTimeout = 5 * time.Minute

// RollbackOptions select the hosts a rollback covers.
type RollbackOptions struct {
	// Optional: empty rolls back every eligible host. Otherwise, node/<uuid>
	// or node/<hostname> rolls back only that host, which must be migrated or
	// failed.
	Resource string
}

// Rollback removes the scoped agent from every migrated or failed host, or
// only the one opts.Resource names, and from the target cluster, and returns
// each host to unstarted. The source agent is not touched. A host that fails
// is listed in the report, not returned as the error. Once hosts have
// started, the report comes back with any error.
func (m *Migrator) Rollback(ctx context.Context, opts RollbackOptions) (*Report, error) {
	if err := m.requireState(); err != nil {
		return nil, trace.Wrap(err)
	}
	hosts, err := m.hostsInStates(opts.Resource, "a migrated or failed host", stateMigrated, stateFailed)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(hosts) == 0 {
		return &Report{Verb: "rolled back"}, nil
	}
	m.progressf("rolling back hosts: %d", len(hosts))
	report, err := m.act(ctx, "rolled back", hosts, rollbackHostTimeout, m.rollbackHost)
	// The scoped agent is gone, but when the target cleanup failed its node
	// can still be listed, and the next plan would pair the host with it again
	// through this host_uuid.
	cleared := false
	for _, h := range hosts {
		if h.State == stateUnstarted && h.Preflight != nil && h.Preflight.SuffixHostUUID != "" {
			h.Preflight.SuffixHostUUID = ""
			cleared = true
		}
	}
	if cleared {
		err = trace.NewAggregate(err, m.stateFile.save())
	}
	return report, err
}

// rollbackHost uninstalls the scoped agent from h, then deletes its node and
// minted token from the target. A host that refuses the uninstall keeps its
// state. Once the uninstall succeeded, h is unstarted even when the target
// cleanup fails, and the failure is reported.
func (m *Migrator) rollbackHost(ctx context.Context, h *host) (*outcome, error) {
	if err := m.runHostStep(ctx, m.sourceHosts, h.ID, hoststep.Uninstall, hoststep.UninstallRequest{
		Proxy:          m.config.Target.ProxyServer,
		SourceHostUUID: h.ID,
	}, &struct{}{}); err != nil {
		return nil, trace.Wrap(err)
	}
	// The cleanup runs detached from ctx, so a run canceled or out of time
	// after the uninstall still removes the target's records.
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
	defer cancel()
	var errs []error
	if h.TargetID != "" {
		if err := m.target.DeleteNode(cleanupCtx, h.TargetScope, h.TargetID); err != nil && !trace.IsNotFound(err) {
			errs = append(errs, trace.Wrap(err, "remove target agent %s with tctl rm node %s::%s", h.TargetID, h.TargetScope, h.TargetID))
		}
	}
	if h.Preflight != nil && hoststep.IsMintedJoinMethod(h.Preflight.JoinMethod) {
		name := mintedTokenName(h.TargetScope, h.ID)
		if err := m.target.DeleteScopedToken(cleanupCtx, h.TargetScope, name); err != nil && !trace.IsNotFound(err) {
			errs = append(errs, trace.Wrap(err, "remove scoped token %s::%s with tctl rm scoped_token %s::%s", h.TargetScope, name, h.TargetScope, name))
		}
	}
	done := &outcome{state: stateUnstarted}
	if len(errs) > 0 {
		return done, trace.Wrap(trace.NewAggregate(errs...), "host rolled back, target cleanup incomplete")
	}
	return done, nil
}
