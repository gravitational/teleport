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
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/sync/errgroup"

	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/migrate/hoststep"
	"github.com/gravitational/teleport/lib/scopes/joining"
)

// progressEvery is how many nodes a reader reads between progress lines.
const progressEvery = 1000

// sourceInventory is the source cluster's nodes.
type sourceInventory struct {
	// nodesByID is keyed by node.ID, the host UUID.
	nodesByID map[string]node
}

// targetInventory is the target cluster's nodes and scoped tokens.
type targetInventory struct {
	// nodesByID is keyed by node.ID, the host UUID.
	nodesByID map[string]node
	// nodesBySourceLabel holds the target nodes labeled as migrated from a
	// source host, keyed by that host's UUID. A node counts in one of two
	// cases. It carries both tool labels, and the source cluster label names
	// the source cluster. Or it carries types.ReconfigureSourceUUIDLabel,
	// which teleport reconfigure stamps on its own.
	nodesBySourceLabel map[string][]node
	// tokensByName is keyed by the token's scope-qualified name.
	tokensByName map[scopes.QualifiedName]scopedToken
}

// readInventories reads both clusters at once. Plan judges every host against
// this one snapshot of the two clusters.
func (m *Migrator) readInventories(ctx context.Context) (sourceInventory, targetInventory, error) {
	var source sourceInventory
	var target targetInventory
	group, ctx := errgroup.WithContext(ctx)
	group.Go(func() error {
		var err error
		source, err = m.readSource(ctx)
		return trace.Wrap(err, "reading source cluster %q", m.source.ClusterName())
	})
	group.Go(func() error {
		var err error
		target, err = m.readTarget(ctx)
		return trace.Wrap(err, "reading target cluster %q", m.target.ClusterName())
	})
	if err := group.Wait(); err != nil {
		return sourceInventory{}, targetInventory{}, trace.Wrap(err)
	}
	m.progressf("%d source nodes, %d target nodes", len(source.nodesByID), len(target.nodesByID))
	return source, target, nil
}

// readSource reads every node in the source cluster: the hosts a mapping can
// select for migration.
func (m *Migrator) readSource(ctx context.Context) (sourceInventory, error) {
	inventory := sourceInventory{nodesByID: make(map[string]node)}
	for n, err := range m.source.Nodes(ctx) {
		if err != nil {
			return sourceInventory{}, trace.Wrap(err)
		}
		inventory.nodesByID[n.ID] = n
		if len(inventory.nodesByID)%progressEvery == 0 {
			m.progressf("read %d source nodes", len(inventory.nodesByID))
		}
	}
	return inventory, nil
}

// labeledSourceHost returns the source host UUID n is labeled as migrated
// from, or "": both tool labels with the source cluster label naming
// sourceCluster, or types.ReconfigureSourceUUIDLabel, which teleport
// reconfigure stamps on its own.
func labeledSourceHost(n node, sourceCluster string) string {
	if id := n.Labels[hoststep.SourceHostLabel]; id != "" && n.Labels[hoststep.SourceClusterLabel] == sourceCluster {
		return id
	}
	return n.Labels[types.ReconfigureSourceUUIDLabel]
}

// readTarget reads every node in the target cluster and the scoped tokens in
// each migration's target scope, and indexes the nodes labeled as migrated
// from a source host, so pairing can find a host's scoped agent by the host's
// UUID.
func (m *Migrator) readTarget(ctx context.Context) (targetInventory, error) {
	inventory := targetInventory{
		nodesByID:          make(map[string]node),
		nodesBySourceLabel: make(map[string][]node),
		tokensByName:       make(map[scopes.QualifiedName]scopedToken),
	}
	sourceCluster := m.source.ClusterName()
	for n, err := range m.target.Nodes(ctx) {
		if err != nil {
			return targetInventory{}, trace.Wrap(err)
		}
		inventory.nodesByID[n.ID] = n
		if id := labeledSourceHost(n, sourceCluster); id != "" {
			inventory.nodesBySourceLabel[id] = append(inventory.nodesBySourceLabel[id], n)
		}
		if len(inventory.nodesByID)%progressEvery == 0 {
			m.progressf("read %d target nodes", len(inventory.nodesByID))
		}
	}
	// A host joins with a token in its own target scope, so no other scope's
	// tokens are read.
	targetScopes := make(map[string]bool)
	for _, mapping := range m.config.Migrations {
		if targetScopes[mapping.TargetScope] {
			continue
		}
		targetScopes[mapping.TargetScope] = true
		for token, err := range m.target.ScopedTokens(ctx, mapping.TargetScope) {
			if err != nil {
				return targetInventory{}, trace.Wrap(err)
			}
			inventory.tokensByName[scopes.QualifiedName{Scope: token.Scope, Name: token.Name}] = token
		}
	}
	return inventory, nil
}

// findMapping decides which target scope a host with labels goes to: it returns
// the first mapping that selects labels, or nil when none does. When mappings that select labels name different target scopes,
// conflict says so; mappings that overlap on the same scope do not conflict.
func (m *Migrator) findMapping(labels map[string]string) (mapping *HostMapping, conflict string) {
	var hits []string
	disagree := false
	for i := range m.config.Migrations {
		candidate := &m.config.Migrations[i]
		if !candidate.matches(labels) {
			continue
		}
		if mapping == nil {
			mapping = candidate
		} else if !isSameScope(candidate.TargetScope, mapping.TargetScope) {
			disagree = true
		}
		hits = append(hits, candidate.String())
	}
	if disagree {
		// Sorted so the reason, and so UpdatedAt, does not change between plans.
		slices.Sort(hits)
		conflict = "matches mappings with different target scopes (" + strings.Join(hits, "; ") + "); a host belongs in a single scope"
	}
	return mapping, conflict
}

// isSameScope reports whether a and b are the same scope.
func isSameScope(a, b string) bool { return scopes.Compare(a, b) == scopes.Equivalent }

// selectHosts decides which source hosts the migration tracks, the first step
// of every plan. It carries over what the state file already knows about each
// host, refreshes its hostname, labels and target scope from the source, and
// returns the records keyed by source host UUID; previous is not modified.
//   - A source node a mapping selects gets a record: a copy of its previous
//     one, or a new unstarted one, with the node's current hostname and labels
//     and the mapping's target scope.
//   - A previous record whose node has left the source is kept unchanged if
//     it is migrated, retired or promoted; any other is dropped.
//   - A previous record whose node is still in the source but no mapping
//     selects it any more is dropped.
func (m *Migrator) selectHosts(previous map[string]*host, source sourceInventory, now time.Time) map[string]*host {
	hosts := make(map[string]*host)
	for id, n := range source.nodesByID {
		mapping, _ := m.findMapping(n.Labels)
		if mapping == nil {
			continue
		}
		selected := &host{
			ID:        id,
			State:     stateUnstarted,
			UpdatedAt: now,
		}
		if old := previous[id]; old != nil {
			copied := *old
			selected = &copied
		}
		selected.Hostname = n.Hostname
		selected.Labels = maps.Clone(n.Labels)
		selected.TargetScope = mapping.TargetScope
		hosts[id] = selected
	}
	for id, old := range previous {
		if _, selected := hosts[id]; selected {
			continue
		}
		if _, present := source.nodesByID[id]; present {
			continue
		}
		// A down host with nothing done to it is not worth tracking. Retire
		// deletes the source node, or leaves it to expire, so a retired or
		// promoted host usually lands here.
		if old.State == stateMigrated || old.State == stateRetired || old.State == statePromoted {
			kept := *old
			kept.Labels = maps.Clone(kept.Labels)
			hosts[id] = &kept
		}
	}
	return hosts
}

// targetGoneReason is why a migrated host whose target agent vanished is
// unstarted again.
const targetGoneReason = "target agent no longer present"

// outcome is a host's new state, target agent and reason, as pairing or a
// command concludes it.
type outcome struct {
	targetID  string
	scopedID  string
	matchedBy string
	state     string
	reason    string
}

// setOutcome records what pairing concluded about h. UpdatedAt changes only
// when the result does, so it shows when the host's state last changed rather
// than when plan last ran.
func (h *host) setOutcome(o outcome, now time.Time) {
	if h.TargetID == o.targetID && h.ScopedID == o.scopedID && h.MatchedBy == o.matchedBy && h.State == o.state && h.Reason == o.reason {
		return
	}
	h.TargetID = o.targetID
	h.ScopedID = o.scopedID
	h.MatchedBy = o.matchedBy
	h.State = o.state
	h.Reason = o.reason
	h.UpdatedAt = now
}

// pairHost decides h's state from the target without changing h: migrated when
// its scoped agent is already there, blocked when something stops the tool from
// migrating it, and otherwise ready to migrate. The evidence for the scoped
// agent is, strongest first: the migration label on a target agent, then the
// host_uuid of a scoped install that preflight read on the host, unless that
// agent carries another host's label.
func pairHost(h *host, target targetInventory) outcome {
	labeled := target.nodesBySourceLabel[h.ID]
	// More than one agent claims the host: a person has to pick.
	if len(labeled) > 1 {
		ids := make([]string, 0, len(labeled))
		for _, n := range labeled {
			ids = append(ids, n.ID)
		}
		// Sorted so the reason, and so UpdatedAt, does not change between plans.
		slices.Sort(ids)
		return outcome{state: stateBlocked, reason: fmt.Sprintf("%d target agents carry this host's migration label (%s); remove it from all but the right one", len(ids), strings.Join(ids, ", "))}
	}
	// Exactly one agent claims the host: migrated, if it is in the right scope.
	if len(labeled) == 1 {
		return classifyLabeledNode(h, labeled[0], matchedByLabel)
	}
	// No labeled agent, but preflight found a scoped install on the host.
	if h.Preflight != nil && h.Preflight.SuffixHostUUID != "" {
		// An agent labeled for another host belongs to that host, whichever
		// host ID sorts first.
		if n, ok := target.nodesByID[h.Preflight.SuffixHostUUID]; ok && !isLabeledForAnotherHost(n, h.ID) {
			return classifyLabeledNode(h, n, matchedByCheck)
		}
	}
	// No scoped agent yet: blocked if the tool cannot migrate the host.
	if reason := findBlockReason(h, target); reason != "" {
		return outcome{state: stateBlocked, reason: reason}
	}
	// No scoped agent and nothing in the way: ready to migrate.
	switch h.State {
	case stateFailed, stateUnstarted:
		return outcome{state: h.State, reason: h.Reason}
	case stateMigrated:
		return outcome{state: stateUnstarted, reason: targetGoneReason}
	}
	// A blocked host whose block is gone is ready again.
	return outcome{state: stateUnstarted}
}

// isLabeledForAnotherHost reports whether n carries a migration label that
// names a host other than id.
func isLabeledForAnotherHost(n node, id string) bool {
	for _, label := range []string{hoststep.SourceHostLabel, types.ReconfigureSourceUUIDLabel} {
		if other := n.Labels[label]; other != "" && other != id {
			return true
		}
	}
	return false
}

// classifyLabeledNode returns h's outcome when n carries h's migration label:
// migrated when n is in h's target scope, blocked when it is in another.
func classifyLabeledNode(h *host, n node, matchedBy string) outcome {
	if !isSameScope(n.Scope, h.TargetScope) {
		return outcome{state: stateBlocked, reason: fmt.Sprintf("already migrated to scope %q; a host belongs in a single scope", n.Scope)}
	}
	return outcome{
		targetID:  n.ID,
		matchedBy: matchedBy,
		state:     stateMigrated,
	}
}

// notPreflightedReason blocks a host no plan has preflighted yet.
const notPreflightedReason = "not preflighted yet; run plan"

// findBlockReason returns why the tool cannot migrate an unpaired host, or "".
// The first match wins: discovery labels, then a missing preflight (plan
// --resource preflights one host), the last preflight's error, a service a
// scope cannot run, and the join method.
func findBlockReason(h *host, target targetInventory) string {
	for _, group := range [][]string{
		{types.AWSAccountIDLabel, types.AWSInstanceIDLabel},
		{types.VMIDLabelInternal},
		{types.ProjectIDLabelDiscovery, types.ZoneLabelDiscovery, types.NameLabelDiscovery},
	} {
		// A complete group marks a host the source cluster's discovery installed.
		if hasAllLabels(h.Labels, group) {
			return "managed by the source cluster's discovery service; migrate by hand or wait for scoped discovery"
		}
	}
	preflight := h.Preflight
	if preflight == nil {
		return notPreflightedReason
	}
	if preflight.Error != "" {
		return preflight.Error
	}
	for _, service := range preflight.Services {
		if !isScopable(service) {
			return fmt.Sprintf("%s cannot run in a scope yet", service)
		}
	}
	return joinBlockReason(preflight, h.TargetScope, target.tokensByName)
}

// hasAllLabels reports whether labels has a non-empty value for every key.
func hasAllLabels(labels map[string]string, keys []string) bool {
	for _, key := range keys {
		if labels[key] == "" {
			return false
		}
	}
	return true
}

// isScopable reports whether a scoped agent can run service today.
func isScopable(service string) bool {
	switch service {
	case "ssh_service", "app_service", "kubernetes_service":
		return true
	}
	return false
}

// joinBlockReason returns why the host's join method blocks it, or "". The
// minted methods pass; a delegated method needs a scoped token in scope, under
// the source token's name, with the same method, assigning scope and granting
// every role the host's services need.
func joinBlockReason(preflight *hoststep.PreflightResult, scope string, tokens map[scopes.QualifiedName]scopedToken) string {
	method := types.JoinMethod(preflight.JoinMethod)
	if method == types.JoinMethodBoundKeypair && preflight.StaticKey {
		return fmt.Sprintf("join method bound_keypair uses a static key (static_key_path); pre-register a key in a scoped token in %s, and join the host by hand", scope)
	}
	if hoststep.IsMintedJoinMethod(preflight.JoinMethod) {
		return ""
	}
	switch method {
	case types.JoinMethodEC2, types.JoinMethodIAM, types.JoinMethodGCP, types.JoinMethodAzure,
		types.JoinMethodOracle, types.JoinMethodGenericOIDC, types.JoinMethodTPM:
		tokenName, err := parseTokenName(preflight.TokenName)
		if err != nil {
			return fmt.Sprintf("join_params token_name %q is not a valid scope-qualified name", preflight.TokenName)
		}
		if tokenName.Name == "" {
			return fmt.Sprintf("join method %s has no token name in join_params", method)
		}
		qualified := scopes.QualifiedName{Scope: scope, Name: tokenName.Name}
		token, ok := tokens[qualified]
		if !ok {
			return fmt.Sprintf("create scoped token %s (method %s) in the target, then run plan again", qualified, method)
		}
		if token.Method != string(method) {
			return fmt.Sprintf("scoped token %s uses method %s, not %s; recreate it with method %s, then run plan again", qualified, token.Method, method, method)
		}
		// Hosts share the token, and a rolled-back host joins with it again.
		if mode := joining.TokenUsageMode(token.UsageMode); mode == joining.TokenUsageModeSingle || mode == joining.TokenUsageModeBot {
			return fmt.Sprintf("scoped token %s has usage_mode %s; hosts share it, so set it to %s, then run plan again", qualified, mode, joining.TokenUsageModeUnlimited)
		}
		// The agent lands where the token assigns it, which can be a scope
		// below the one the token lives in.
		if !isSameScope(token.AssignedScope, scope) {
			return fmt.Sprintf("scoped token %s assigns scope %s, not %s; change its assigned scope, then run plan again", qualified, token.AssignedScope, scope)
		}
		if missing := missingTokenRoles(token, preflight.Services); len(missing) > 0 {
			return fmt.Sprintf("scoped token %s does not grant %s; add the role(s), then run plan again", qualified, strings.Join(missing, ", "))
		}
		return ""
	}
	return fmt.Sprintf("join method %s is not supported by the tool", method)
}

// tokenRoles returns the system roles a token must grant for a host running
// services: Node, which every scoped agent joins as and which covers
// ssh_service, and each other service's role.
func tokenRoles(services []string) []string {
	need := types.SystemRoles{types.RoleNode}
	for _, service := range services {
		switch service {
		case "app_service":
			need = append(need, types.RoleApp)
		case "kubernetes_service":
			need = append(need, types.RoleKube)
		}
	}
	return need.StringSlice()
}

// missingTokenRoles returns the system roles a host running services needs
// that token does not grant. Role names match ignoring case, as in Teleport.
func missingTokenRoles(token scopedToken, services []string) []string {
	// The target checked the roles when it stored the token, so the error is
	// ignored: a role it does not know grants nothing.
	granted, _ := types.NewTeleportRoles(token.Roles)
	var missing []string
	for _, role := range tokenRoles(services) {
		if !granted.Include(types.SystemRole(role)) {
			missing = append(missing, role)
		}
	}
	return missing
}

// parseTokenName parses a join token_name, which has an empty scope when it
// is not scope-qualified. token_name also takes the path of a file holding the
// token, so only the separator, not a leading "/" as in
// scopes.ParseOptionallyQualifiedName, makes a name qualified.
func parseTokenName(name string) (scopes.QualifiedName, error) {
	if !strings.Contains(name, scopes.QualifiedNameSeparator) {
		return scopes.QualifiedName{Name: name}, nil
	}
	qualified, err := scopes.ParseQualifiedName(name)
	if err != nil {
		return scopes.QualifiedName{}, trace.Wrap(err, "parsing token_name %q", name)
	}
	return qualified, nil
}

// pairHosts pairs every tracked host with its scoped agent and finds the target
// agents in a migration scope that no host claims, which plan reports as agents
// the migration did not put there. It returns each host's outcome keyed by host
// UUID and the unclaimed agents sorted by ID. Hosts are visited in ID order, so
// a target two hosts point at goes to the lower ID and blocks the other. A
// retired or promoted host gets no outcome: its agent is its only one, so it
// keeps its pairing, and no other host can claim that agent. It does not change
// hosts.
func (m *Migrator) pairHosts(hosts map[string]*host, target targetInventory) (outcomes map[string]outcome, unmatched []targetAgent) {
	outcomes = make(map[string]outcome, len(hosts))
	// claimedBy maps a target agent ID to the host paired with it.
	claimedBy := make(map[string]string)
	ids := slices.Sorted(maps.Keys(hosts))
	for _, id := range ids {
		if h := hosts[id]; (h.State == stateRetired || h.State == statePromoted) && h.TargetID != "" {
			claimedBy[h.TargetID] = id
		}
	}
	for _, id := range ids {
		h := hosts[id]
		if h.State == stateRetired || h.State == statePromoted {
			continue
		}
		result := pairHost(h, target)
		// A pairing does not settle which scope the host belongs in. It is
		// kept, so the pairing stays visible, but the host is blocked until
		// its mappings agree.
		if _, conflict := m.findMapping(h.Labels); conflict != "" {
			result.state = stateBlocked
			result.reason = conflict
		}
		if result.targetID != "" {
			if other, taken := claimedBy[result.targetID]; taken {
				result = outcome{
					state:  stateBlocked,
					reason: fmt.Sprintf("conflict: target agent %q is also claimed by %q", result.targetID, other),
				}
			} else {
				claimedBy[result.targetID] = id
			}
		}
		outcomes[id] = result
	}
	for _, id := range slices.Sorted(maps.Keys(target.nodesByID)) {
		n := target.nodesByID[id]
		if _, claimed := claimedBy[id]; !claimed && m.isMigrationScope(n.Scope) {
			unmatched = append(unmatched, targetAgent{
				ID:       id,
				Hostname: n.Hostname,
				Scope:    n.Scope,
			})
		}
	}
	return outcomes, unmatched
}

// isMigrationScope reports whether scope is a mapping's target scope.
func (m *Migrator) isMigrationScope(scope string) bool {
	for _, mapping := range m.config.Migrations {
		if isSameScope(mapping.TargetScope, scope) {
			return true
		}
	}
	return false
}

// Summary counts what a plan found, for the command to print.
type Summary struct {
	// StateFile is the path of the migration's state file.
	StateFile string `json:"state_file"`
	// Hosts is how many hosts the migration tracks.
	Hosts int `json:"hosts"`
	// ByState counts hosts by state; a state with no hosts is absent.
	ByState map[string]int `json:"by_state"`
	// UnmatchedTargets counts target agents in a migration scope that no host
	// claims.
	UnmatchedTargets int `json:"unmatched_targets"`
	// Unreachable counts the hosts blocked because preflight could not connect
	// to them.
	Unreachable int `json:"unreachable"`
	// TbotHosts counts the hosts not yet retired or promoted where preflight
	// found tbot.service active beside an updater install.
	TbotHosts int `json:"tbot_hosts"`
}

// PlanOptions select the hosts a plan preflights.
type PlanOptions struct {
	// Optional: when set, preflights every host again, not only the hosts with
	// no preflight yet.
	Check bool
	// Optional: empty preflights every eligible host. Otherwise, node/<uuid> or
	// node/<hostname> preflights only that host, even when it has a preflight
	// or is migrated; a retired or promoted host is refused.
	Resource string
}

// check refuses a malformed Resource.
func (o *PlanOptions) check() error {
	if o.Resource == "" {
		return nil
	}
	return trace.Wrap(CheckResource(o.Resource))
}

// Plan works out where every host the migration covers stands. It reads both
// clusters, selects the source hosts the mappings choose, preflights the ones
// that need it, pairs each with the scoped agent already in the target, if any,
// and saves each host's state, and the reason when it is blocked, to the state
// file. A host that fails preflight is recorded as blocked, not returned as an
// error; a canceled ctx discards the preflights still running.
func (m *Migrator) Plan(ctx context.Context, opts PlanOptions) (*Summary, error) {
	if err := opts.check(); err != nil {
		return nil, trace.Wrap(err)
	}
	source, target, err := m.readInventories(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	current := m.stateFile.state
	current.Hosts = m.selectHosts(current.Hosts, source, time.Now().UTC())
	// Save before preflight so the selection survives a failure below.
	if err := m.stateFile.save(); err != nil {
		return nil, trace.Wrap(err)
	}
	if err := m.preflightHosts(ctx, opts); err != nil {
		return nil, trace.Wrap(err)
	}
	now := time.Now().UTC()
	outcomes, unmatched := m.pairHosts(current.Hosts, target)
	for id, o := range outcomes {
		current.Hosts[id].setOutcome(o, now)
	}
	current.UnmatchedTargets = unmatched
	if err := m.stateFile.save(); err != nil {
		return nil, trace.Wrap(err)
	}
	return m.summarize(), nil
}

// summarize counts the state file's hosts.
func (m *Migrator) summarize() *Summary {
	current := m.stateFile.state
	summary := &Summary{
		StateFile:        m.stateFile.path,
		Hosts:            len(current.Hosts),
		ByState:          make(map[string]int),
		UnmatchedTargets: len(current.UnmatchedTargets),
	}
	for _, h := range current.Hosts {
		summary.ByState[h.State]++
		if h.State == stateBlocked && h.Preflight != nil && h.Preflight.Unreachable && h.Reason == h.Preflight.Error {
			summary.Unreachable++
		}
		if h.State != stateRetired && h.State != statePromoted && h.Preflight != nil && h.Preflight.TbotActive {
			summary.TbotHosts++
		}
	}
	return summary
}

// needsPreflight reports whether plan preflights h: never when it is retired
// or promoted, since its source agent is gone; otherwise when it has no
// preflight yet, or when check is set.
func needsPreflight(h *host, check bool) bool {
	return h.State != stateRetired && h.State != statePromoted && (h.Preflight == nil || check)
}

// preflightProgressEvery is how many preflights finish between progress lines.
const preflightProgressEvery = 100

// preflightHosts preflights the hosts that need it in ID order, or only the
// one opts.Resource names, and records each result on its host. A failed
// preflight is recorded, not returned. Results that arrive after ctx is
// canceled are discarded, and ctx's error is returned after the save.
func (m *Migrator) preflightHosts(ctx context.Context, opts PlanOptions) error {
	current := m.stateFile.state
	var hosts []*host
	if opts.Resource != "" {
		name, err := parseNodeRef(opts.Resource)
		if err != nil {
			return trace.Wrap(err)
		}
		h, err := current.findHost(name)
		if err != nil {
			return trace.Wrap(err)
		}
		if h.State == stateRetired || h.State == statePromoted {
			return trace.NotFound("%q is %s; plan does not preflight retired or promoted hosts", opts.Resource, h.State)
		}
		hosts = []*host{h}
	} else {
		for _, id := range slices.Sorted(maps.Keys(current.Hosts)) {
			if h := current.Hosts[id]; needsPreflight(h, opts.Check) {
				hosts = append(hosts, h)
			}
		}
	}
	if len(hosts) == 0 {
		return nil
	}
	m.progressf("preflighting %d hosts", len(hosts))
	finished := 0
	for result := range forEachHost(ctx, hosts, planHostTimeout, m.runPreflight) {
		// A preflight the cancel cut short can only fail, and recording that
		// would leave the host with a preflight, which the next plan skips.
		if ctx.Err() != nil {
			continue
		}
		result.host.Preflight = newPreflightResult(result.host.Preflight, result.value, result.err, time.Now().UTC())
		if finished++; finished%preflightProgressEvery == 0 {
			m.progressf("preflighted %d of %d hosts", finished, len(hosts))
		}
	}
	if err := m.stateFile.save(); err != nil {
		return trace.Wrap(err)
	}
	return trace.Wrap(ctx.Err())
}

// retryHint ends a preflight failure's reason, because plan preflights a host
// that already has a preflight only under --check.
const retryHint = "; run plan --check to preflight it again"

// newPreflightResult returns the preflight to record from one run's facts and
// error. A run that read no facts keeps previous's facts, or starts from none.
// The error is the host's reason when the host refused, and otherwise says
// whether the login was refused, the run timed out or the host could not be
// reached. It is kept on one line.
func newPreflightResult(previous, facts *hoststep.PreflightResult, err error, now time.Time) *hoststep.PreflightResult {
	var result hoststep.PreflightResult
	switch {
	case facts != nil:
		result = *facts
	case previous != nil:
		result = *previous
	}
	result.At = now
	result.Error = ""
	result.Unreachable = false
	switch {
	case err == nil:
	case facts != nil:
		result.Error = joinLines(err.Error()) + retryHint
	case trace.IsAccessDenied(err):
		result.Error = joinLines("ssh access denied: "+err.Error()) + retryHint
	// Before the deadline check: a dial that timed out on its own is a
	// connection problem that wraps context.DeadlineExceeded.
	case trace.IsConnectionProblem(err):
		result.Error = joinLines("unreachable: "+err.Error()) + retryHint
		result.Unreachable = true
	case errors.Is(err, context.DeadlineExceeded):
		result.Error = fmt.Sprintf("preflight did not finish within %v (the first run downloads teleport); run plan --check again", planHostTimeout)
	default:
		result.Error = joinLines(err.Error()) + retryHint
	}
	return &result
}

// joinLines joins message's trimmed, non-empty lines with a space. A wrapped
// trace error and a host's stderr span several lines, and a reason is printed
// as one table cell.
func joinLines(message string) string {
	var lines []string
	for line := range strings.Lines(message) {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, " ")
}

// runPreflight runs the preflight step on h. A host that refuses returns its
// reason as the error, together with the facts read before it; any other
// error comes with no facts.
func (m *Migrator) runPreflight(ctx context.Context, h *host) (*hoststep.PreflightResult, error) {
	var response hoststep.PreflightResponse
	if err := m.runHostStep(ctx, m.sourceHosts, h.ID, hoststep.Preflight, hoststep.PreflightRequest{SourceHostUUID: h.ID}, &response); err != nil {
		return nil, trace.Wrap(err)
	}
	if response.Reason != "" {
		return &response.Facts, errors.New(response.Reason)
	}
	return &response.Facts, nil
}
