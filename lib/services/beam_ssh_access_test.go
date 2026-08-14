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

	"github.com/gravitational/teleport"
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

	dynamicOverrideNode := newBeamNode(t, "beam-dyn", "alice").(*types.ServerV2)
	dynamicOverrideNode.Spec.CmdLabels = map[string]types.CommandLabelV2{
		types.BeamOwnerLabel: {Result: "mallory"},
	}

	partialNode, err := types.NewServer("partial", types.KindNode, types.ServerSpecV2{Hostname: "partial"})
	require.NoError(t, err)
	partialNode.SetStaticLabels(map[string]string{
		types.BeamOwnerLabel: "alice", // owner present but no beam ID
	})

	tests := []struct {
		name         string
		username     string
		impersonator string
		target       types.Server
		wantErr      string
	}{
		{
			name:     "owner can access own beam",
			username: "alice",
			target:   beamNode,
		},
		{
			name:         "owner via self role-impersonation is allowed",
			username:     "alice",
			impersonator: "alice",
			target:       beamNode,
		},
		{
			name:         "impersonated owner identity is denied",
			username:     "alice",
			impersonator: "mallory",
			target:       beamNode,
			wantErr:      "impersonated identities cannot access beams",
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
			name:     "beam without owner label is denied",
			username: "alice",
			target:   newBeamNode(t, "beam-456", ""),
			wantErr:  "missing owner label",
		},
		{
			name:     "dynamic label cannot override the owner",
			username: "mallory",
			target:   types.Server(dynamicOverrideNode),
			wantErr:  `owned by "alice"`,
		},
		{
			name:     "dynamic override does not lock out the real owner",
			username: "alice",
			target:   types.Server(dynamicOverrideNode),
		},
		{
			name:     "partially marked node fails closed",
			username: "alice",
			target:   partialNode,
			wantErr:  "inconsistently marked",
		},
		{
			name:     "regular node is not affected",
			username: "anyone",
			target:   regularNode,
		},
		{
			name:         "regular node is not affected for impersonated identities",
			username:     "anyone",
			impersonator: "mallory",
			target:       regularNode,
		},
		{
			name:     "nil target is allowed (re-checked with concrete target)",
			username: "anyone",
			target:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckBeamSSHOwnership(tt.username, tt.impersonator, tt.target)
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

func TestCheckBeamSSHLogin(t *testing.T) {
	beamNode := newBeamNode(t, "beam-123", "alice")

	regularNode, err := types.NewServer("regular", types.KindNode, types.ServerSpecV2{Hostname: "server1"})
	require.NoError(t, err)

	tests := []struct {
		name    string
		osUser  string
		target  types.Server
		wantErr bool
	}{
		{
			name:   "beams login allowed on beam",
			osUser: types.BeamsLogin,
			target: beamNode,
		},
		{
			name:   "session join principal allowed on beam",
			osUser: teleport.SSHSessionJoinPrincipal,
			target: beamNode,
		},
		{
			name:    "root denied on beam even for the owner's connection",
			osUser:  "root",
			target:  beamNode,
			wantErr: true,
		},
		{
			name:    "arbitrary login denied on beam",
			osUser:  "ubuntu",
			target:  beamNode,
			wantErr: true,
		},
		{
			name:   "any login allowed on regular node",
			osUser: "root",
			target: regularNode,
		},
		{
			name:   "nil target allowed",
			osUser: "root",
			target: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckBeamSSHLogin(tt.osUser, tt.target)
			if !tt.wantErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.True(t, trace.IsAccessDenied(err), "expected access denied, got: %v", err)
		})
	}
}

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

	newChecker := func(info *AccessInfo) AccessChecker {
		info.Roles = []string{broadRole.GetName()}
		return NewAccessCheckerWithRoleSet(info, "localhost", NewRoleSet(broadRole))
	}

	t.Run("owner allowed", func(t *testing.T) {
		checker := newChecker(&AccessInfo{Username: "alice"})
		require.NoError(t, checker.CheckAccess(beamNode, AccessState{MFAVerified: true}, NewLoginMatcher(types.BeamsLogin)))
	})

	t.Run("non-owner denied despite wildcard node access", func(t *testing.T) {
		checker := newChecker(&AccessInfo{Username: "bob"})
		err := checker.CheckAccess(beamNode, AccessState{MFAVerified: true}, NewLoginMatcher(types.BeamsLogin))
		require.True(t, trace.IsAccessDenied(err), err)
		require.ErrorContains(t, err, `owned by "alice"`)
	})

	t.Run("impersonated owner denied", func(t *testing.T) {
		checker := newChecker(&AccessInfo{Username: "alice", Impersonator: "mallory"})
		err := checker.CheckAccess(beamNode, AccessState{MFAVerified: true}, NewLoginMatcher(types.BeamsLogin))
		require.True(t, trace.IsAccessDenied(err), err)
		require.ErrorContains(t, err, "impersonated identities")
	})

	t.Run("listing filters other users' beams but not the owner's", func(t *testing.T) {
		require.NoError(t, newChecker(&AccessInfo{Username: "alice"}).CheckAccess(beamNode, AccessState{MFAVerified: true}))
		err := newChecker(&AccessInfo{Username: "bob"}).CheckAccess(beamNode, AccessState{MFAVerified: true})
		require.True(t, trace.IsAccessDenied(err), err)
	})
}

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
