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
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	joiningv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/scopes/joining/v1"
	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/migrate/hoststep"
	"github.com/gravitational/teleport/lib/scopes/joining"
	"github.com/gravitational/teleport/lib/utils"
)

// applyHostTimeout bounds one host's apply: minting, the install, which
// downloads teleport, and the wait for the scoped agent to join.
const applyHostTimeout = 10 * time.Minute

// tokenTTL is how long a token apply mints stays usable.
const tokenTTL = 30 * time.Minute

// ApplyOptions select the hosts an apply migrates.
type ApplyOptions struct {
	// Optional: empty migrates every eligible host. Otherwise, node/<uuid> or
	// node/<hostname> migrates only that host, which must be unstarted or
	// failed.
	Resource string
}

// Apply installs the scoped agent beside the source agent on every unstarted
// or failed host, or only the one opts.Resource names, and records each host
// migrated or failed. A host that fails is listed in the report, not returned
// as the error; a canceled ctx discards the hosts still running unless they
// finish migrated. Once hosts have started, the report comes back with any
// error and counts the hosts recorded.
func (m *Migrator) Apply(ctx context.Context, opts ApplyOptions) (*Report, error) {
	if err := m.requireState(); err != nil {
		return nil, trace.Wrap(err)
	}
	hosts, err := m.hostsInStates(opts.Resource, "an unstarted or failed host", stateUnstarted, stateFailed)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(hosts) == 0 {
		return &Report{Verb: "migrated"}, nil
	}
	pins, err := m.target.GetCAPins(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "reading the target cluster's CA pins")
	}
	m.progressf("migrating hosts: %d", len(hosts))
	return m.act(ctx, "migrated", hosts, applyHostTimeout, applyRun{m: m, pins: pins}.applyHost)
}

// applyRun binds the CA pins, read once per apply, to applyHost, which act
// runs per host.
type applyRun struct {
	m *Migrator
	// pins are the target cluster's CA pins.
	pins []string
}

// applyHost migrates h and returns its outcome: migrated, or failed with the
// error as its reason and the scoped agent's host UUID, from this install or
// an earlier failed one, so a rollback can remove its node. A host cut short
// by a cancel has no outcome, so it keeps its state.
func (r applyRun) applyHost(ctx context.Context, h *host) (*outcome, error) {
	targetID, err := r.migrateHost(ctx, h)
	switch {
	case err == nil:
		return &outcome{targetID: targetID, matchedBy: matchedByLabel, state: stateMigrated}, nil
	case errors.Is(ctx.Err(), context.Canceled):
		return nil, trace.Wrap(err)
	}
	return &outcome{targetID: cmp.Or(targetID, h.TargetID), state: stateFailed, reason: joinLines(err.Error())}, trace.Wrap(err)
}

// migrateHost issues h's join credential, installs the scoped agent and
// returns its host UUID once the target lists it in h's scope.
func (r applyRun) migrateHost(ctx context.Context, h *host) (string, error) {
	// Plan blocks a host that failed preflight, so this only catches a host
	// plan never preflighted or a state file edited by hand.
	if h.Preflight == nil || h.Preflight.Error != "" {
		return "", trace.BadParameter("host has not passed preflight; run plan --check --resource node/%s", h.ID)
	}
	m := r.m
	credential, err := m.issueJoinCredential(ctx, h, mintedTokenName(h.TargetScope, h.ID))
	if err != nil {
		return "", trace.Wrap(err, "issuing the join credential")
	}
	var response hoststep.InstallResponse
	if err := m.runHostStep(ctx, m.sourceHosts, h.ID, hoststep.Install, hoststep.InstallRequest{
		Proxy:              m.config.Target.ProxyServer,
		JoinMethod:         h.Preflight.JoinMethod,
		TokenName:          credential.tokenName,
		RegistrationSecret: credential.registrationSecret,
		CAPins:             r.pins,
		SourceCluster:      m.source.ClusterName(),
		SourceHostUUID:     h.ID,
		Services:           h.Preflight.Services,
		Wait:               targetWait,
	}, &response); err != nil {
		return "", trace.Wrap(err)
	}
	return response.HostUUID, trace.Wrap(m.awaitJoin(ctx, h, response.HostUUID), "waiting for the scoped agent to join")
}

// joinCredential is what the scoped agent's join_params carry. Neither field
// is logged or stored; both reach the host only on the install step's stdin.
type joinCredential struct {
	// tokenName is the token's scope and name, with the secret encoded into
	// the name for the token method.
	tokenName scopes.QualifiedName
	// registrationSecret is the bound_keypair registration secret.
	registrationSecret string
}

// issueJoinCredential returns the credential an agent for h joins the target
// with. For the token and bound_keypair methods it mints the token called name
// for h and returns its secret. Any other method proves the host's cloud or
// hardware identity, so its token can't be minted per host: the operator
// creates it in h's target scope under the source token's name, plan checks it
// is there, and this returns that name with no secret.
func (m *Migrator) issueJoinCredential(ctx context.Context, h *host, name string) (joinCredential, error) {
	if !hoststep.IsMintedJoinMethod(h.Preflight.JoinMethod) {
		tokenName, err := parseTokenName(h.Preflight.TokenName)
		if err != nil {
			return joinCredential{}, trace.Wrap(err)
		}
		return joinCredential{tokenName: scopes.QualifiedName{Scope: h.TargetScope, Name: tokenName.Name}}, nil
	}
	token, err := m.mintToken(ctx, h, name)
	if err != nil {
		return joinCredential{}, trace.Wrap(err)
	}
	if h.Preflight.JoinMethod == string(types.JoinMethodBoundKeypair) {
		registrationSecret := token.GetStatus().GetUsage().GetBoundKeypair().GetRegistrationSecret()
		if registrationSecret == "" {
			return joinCredential{}, trace.BadParameter("the target returned scoped token %q without a registration secret", name)
		}
		return joinCredential{
			tokenName:          scopes.QualifiedName{Scope: h.TargetScope, Name: name},
			registrationSecret: registrationSecret,
		}, nil
	}
	secret := token.GetStatus().GetSecret()
	if secret == "" {
		return joinCredential{}, trace.BadParameter("the target returned scoped token %q without a secret", name)
	}
	return joinCredential{tokenName: scopes.QualifiedName{Scope: h.TargetScope, Name: joining.EncodeScopedToken(name, secret)}}, nil
}

// mintedTokenName is the name of the token apply mints for hostID in scope,
// derived from both so a retry replaces it.
func mintedTokenName(scope, hostID string) string {
	sum := sha256.Sum256([]byte(scope + "\x00" + hostID))
	return "migrate-" + hex.EncodeToString(sum[:8])
}

// mintToken creates h's scoped token called name, replacing one an earlier
// attempt left behind, which may be spent or expired.
func (m *Migrator) mintToken(ctx context.Context, h *host, name string) (*joiningv1.ScopedToken, error) {
	token, err := m.newToken(h, name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	created, err := m.target.CreateScopedToken(ctx, token)
	if trace.IsAlreadyExists(err) {
		// A bound_keypair token loses the key its agent bound, so an agent
		// that already joined with it must register again if it loses its
		// data dir.
		m.progressf("%s: replacing scoped token %q left by an earlier attempt", utils.EscapeControl(h.Hostname), name)
		if err := m.target.DeleteScopedToken(ctx, h.TargetScope, name); err != nil {
			return nil, trace.Wrap(err, "replacing scoped token %q", name)
		}
		created, err = m.target.CreateScopedToken(ctx, token)
	}
	if err != nil {
		return nil, trace.Wrap(err, "creating scoped token %q", name)
	}
	return created, nil
}

// newToken builds h's scoped token: single-use with a TTL for the token
// method, and unlimited with a registration deadline for bound_keypair, whose
// agent must be able to register again after losing its data dir. The token
// grants the roles h's services need and carries the correlation labels as
// immutable SSH labels. Any other join method is refused.
func (m *Migrator) newToken(h *host, name string) (*joiningv1.ScopedToken, error) {
	now := time.Now().UTC()
	metadata := headerv1.Metadata_builder{Name: name, Labels: map[string]string{migrationTokenLabel: "true"}}
	spec := joiningv1.ScopedTokenSpec_builder{
		AssignedScope: h.TargetScope,
		Roles:         tokenRoles(h.Preflight.Services),
		ImmutableLabels: joiningv1.ImmutableLabels_builder{
			Ssh: map[string]string{hoststep.SourceClusterLabel: m.source.ClusterName(), hoststep.SourceHostLabel: h.ID},
		}.Build(),
	}
	switch method := types.JoinMethod(h.Preflight.JoinMethod); method {
	case types.JoinMethodToken:
		spec.JoinMethod = string(types.JoinMethodToken)
		spec.UsageMode = string(joining.TokenUsageModeSingle)
		metadata.Expires = timestamppb.New(now.Add(tokenTTL))
	case types.JoinMethodBoundKeypair:
		spec.JoinMethod = string(types.JoinMethodBoundKeypair)
		spec.UsageMode = string(joining.TokenUsageModeUnlimited)
		spec.BoundKeypair = joiningv1.BoundKeypairSpec_builder{
			Onboarding: joiningv1.BoundKeypairSpec_OnboardingSpec_builder{
				MustRegisterBefore: timestamppb.New(now.Add(tokenTTL)),
			}.Build(),
		}.Build()
	default:
		return nil, trace.BadParameter("apply mints tokens only for the token and bound_keypair join methods, not %q", method)
	}
	return joiningv1.ScopedToken_builder{
		Kind:     types.KindScopedToken,
		Version:  types.V1,
		Metadata: metadata.Build(),
		Scope:    h.TargetScope,
		Spec:     spec.Build(),
	}.Build(), nil
}

// awaitJoin waits up to targetWait for the target to list agent id in h's
// scope. A point lookup per host keeps a large run from reading the target's
// whole fleet. Only NotFound is retried; any other lookup error ends the
// wait, and a wait that runs out ends in a LimitExceeded.
func (m *Migrator) awaitJoin(ctx context.Context, h *host, id string) error {
	retry, err := retryutils.NewConstant(pollInterval)
	if err != nil {
		return trace.Wrap(err)
	}
	// Only the wait runs out; a lookup keeps ctx, so the timeout never cuts one
	// short into an error that would end the wait.
	waitCtx, cancel := context.WithTimeout(ctx, targetWait)
	defer cancel()
	err = retry.For(waitCtx, func() error {
		_, err := m.target.GetNode(ctx, h.TargetScope, id)
		if err != nil && !trace.IsNotFound(err) {
			return retryutils.PermanentRetryError(trace.Wrap(err, "looking up agent %s", id))
		}
		return err
	})
	// For ends with a non-permanent error only when waitCtx does.
	if err != nil && !retryutils.IsPermanent(err) {
		return trace.LimitExceeded("agent %s did not appear in scope %s in time", id, h.TargetScope)
	}
	return trace.Wrap(err)
}
