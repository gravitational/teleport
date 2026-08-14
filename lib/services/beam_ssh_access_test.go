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

package services

import (
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func newBeamNode(t *testing.T, beamID, owner string) types.Server {
	t.Helper()
	node, err := types.NewServer(beamID, types.KindNode, types.ServerSpecV2{Hostname: "beam-" + beamID})
	require.NoError(t, err)
	labels := map[string]string{types.BeamIDLabel: beamID}
	if owner != "" {
		labels[types.BeamOwnerLabel] = owner
	}
	node.SetStaticLabels(labels)
	return node
}

func TestCheckBeamSSHOwnership(t *testing.T) {
	beamNode := newBeamNode(t, "beam-123", "alice")

	regularNode, err := types.NewServer("regular", types.KindNode, types.ServerSpecV2{Hostname: "server1"})
	require.NoError(t, err)
	regularNode.SetStaticLabels(map[string]string{"env": "prod"})

	tests := []struct {
		name              string
		username          string
		fromRemoteCluster bool
		target            types.Server
		wantErr           string
	}{
		{
			name:     "owner can access own beam",
			username: "alice",
			target:   beamNode,
		},
		{
			name:     "non-owner cannot access beam",
			username: "bob",
			target:   beamNode,
			wantErr:  `owned by "alice"`,
		},
		{
			name:     "empty username cannot access beam",
			username: "",
			target:   beamNode,
			wantErr:  `owned by "alice"`,
		},
		{
			name:              "remote cluster identity with owner's username is denied",
			username:          "alice",
			fromRemoteCluster: true,
			target:            beamNode,
			wantErr:           "cross-cluster",
		},
		{
			name:     "beam without owner label is denied",
			username: "alice",
			target:   newBeamNode(t, "beam-456", ""),
			wantErr:  "missing owner label",
		},
		{
			name:     "regular node is not affected",
			username: "anyone",
			target:   regularNode,
		},
		{
			name:              "regular node is not affected for remote identities",
			username:          "anyone",
			fromRemoteCluster: true,
			target:            regularNode,
		},
		{
			name:     "nil target is allowed (re-checked with concrete target)",
			username: "anyone",
			target:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckBeamSSHOwnership(tt.username, tt.fromRemoteCluster, tt.target)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

// TestBeamOwnershipViaCheckAccess verifies that the beam ownership check is
// enforced by the accessChecker's CheckAccess funnel, which every SSH access
// evaluation (node, proxying, scoped, PDP, listing) routes through, even when
// the user's roles would otherwise grant access to every node.
func TestBeamOwnershipViaCheckAccess(t *testing.T) {
	beamNode := newBeamNode(t, "beam-123", "alice")

	broadRole, err := types.NewRole("broad-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{types.BeamsLogin},
			NodeLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
		},
	})
	require.NoError(t, err)

	newChecker := func(username string, fromRemoteCluster bool) AccessChecker {
		return NewAccessCheckerWithRoleSet(
			&AccessInfo{
				Username:          username,
				Roles:             []string{broadRole.GetName()},
				FromRemoteCluster: fromRemoteCluster,
			},
			"localhost",
			NewRoleSet(broadRole),
		)
	}

	t.Run("owner allowed", func(t *testing.T) {
		err := newChecker("alice", false).CheckAccess(beamNode, AccessState{MFAVerified: true}, NewLoginMatcher(types.BeamsLogin))
		require.NoError(t, err)
	})

	t.Run("non-owner denied despite wildcard node access", func(t *testing.T) {
		err := newChecker("bob", false).CheckAccess(beamNode, AccessState{MFAVerified: true}, NewLoginMatcher(types.BeamsLogin))
		require.True(t, trace.IsAccessDenied(err), err)
		require.ErrorContains(t, err, `owned by "alice"`)
	})

	t.Run("remote identity with owner's username denied", func(t *testing.T) {
		err := newChecker("alice", true).CheckAccess(beamNode, AccessState{MFAVerified: true}, NewLoginMatcher(types.BeamsLogin))
		require.True(t, trace.IsAccessDenied(err), err)
		require.ErrorContains(t, err, "cross-cluster")
	})

	t.Run("listing filters other users' beams but not the owner's", func(t *testing.T) {
		// CanAccessSSHServer-style listing check: MFA verified, no login matcher.
		require.NoError(t, newChecker("alice", false).CheckAccess(beamNode, AccessState{MFAVerified: true}))
		err := newChecker("bob", false).CheckAccess(beamNode, AccessState{MFAVerified: true})
		require.True(t, trace.IsAccessDenied(err), err)
	})
}

// TestBeamOwnershipOrderingWithSessionMFA verifies that role-derived signals
// such as ErrSessionMFARequired surface before the beam ownership check. The
// IsMFARequired RPC probes node access and only reacts to
// ErrSessionMFARequired: if the beam check ran first, a non-owner probe would
// mask the MFA requirement. For the owner the ordering is irrelevant (the
// beam check passes), but keeping role signals first keeps probes accurate
// for every caller. Ordering does not weaken enforcement: access requires
// both the role evaluation and the beam ownership check to pass.
func TestBeamOwnershipOrderingWithSessionMFA(t *testing.T) {
	beamNode := newBeamNode(t, "beam-123", "alice")

	mfaRole, err := types.NewRole("beam-mfa", types.RoleSpecV6{
		Options: types.RoleOptions{
			RequireMFAType: types.RequireMFAType_SESSION,
		},
		Allow: types.RoleConditions{
			Logins: []string{types.BeamsLogin},
			NodeLabels: types.Labels{
				types.Wildcard: []string{types.Wildcard},
			},
		},
	})
	require.NoError(t, err)

	newChecker := func(username string) AccessChecker {
		return NewAccessCheckerWithRoleSet(
			&AccessInfo{Username: username, Roles: []string{mfaRole.GetName()}},
			"localhost",
			NewRoleSet(mfaRole),
		)
	}

	t.Run("MFA requirement surfaces before beam ownership", func(t *testing.T) {
		err := newChecker("bob").CheckAccess(beamNode, AccessState{}, NewLoginMatcher(types.BeamsLogin))
		require.ErrorIs(t, err, ErrSessionMFARequired)
	})

	t.Run("non-owner still denied once MFA is satisfied", func(t *testing.T) {
		err := newChecker("bob").CheckAccess(beamNode, AccessState{MFAVerified: true}, NewLoginMatcher(types.BeamsLogin))
		require.True(t, trace.IsAccessDenied(err), err)
		require.ErrorContains(t, err, `owned by "alice"`)
	})

	t.Run("owner passes once MFA is satisfied", func(t *testing.T) {
		err := newChecker("alice").CheckAccess(beamNode, AccessState{MFAVerified: true}, NewLoginMatcher(types.BeamsLogin))
		require.NoError(t, err)
	})
}
