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

package auth_test

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authtest"
	"github.com/gravitational/teleport/lib/auth/moderation"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
)

// fakeCommandEvaluator is a test moderation.CommandEvaluator that records the
// request it received and returns a canned result.
type fakeCommandEvaluator struct {
	gotReq moderation.CommandEvaluationRequest
	result moderation.CommandEvaluationResult
	err    error
}

func (f *fakeCommandEvaluator) EvaluateCommand(ctx context.Context, req moderation.CommandEvaluationRequest) (moderation.CommandEvaluationResult, error) {
	f.gotReq = req
	return f.result, f.err
}

// serverWithRolesForServerID builds a ServerWithRoles whose identity is the
// builtin role for the given serverID (host id).
func serverWithRolesForServerID(t *testing.T, as *authtest.AuthServer, role types.SystemRole, serverID string) *auth.ServerWithRoles {
	t.Helper()
	authzContext := authz.ContextWithUser(t.Context(), authtest.TestServerID(role, serverID).I)
	ctxIdentity, err := as.Authorizer.Authorize(authzContext)
	require.NoError(t, err)

	authWithRole := auth.NewServerWithRoles(as.AuthServer, as.AuditLog, *ctxIdentity)
	t.Cleanup(func() { authWithRole.Close() })
	return authWithRole
}

// createAICommandApprovalTracker creates a session tracker hosted by hostID with
// an AI command-approval policy.
func createAICommandApprovalTracker(t *testing.T, ctx context.Context, as *authtest.AuthServer, sessionID, hostID string) {
	t.Helper()
	tracker, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
		SessionID:   sessionID,
		Kind:        string(types.SSHSessionKind),
		State:       types.SessionState_SessionStateRunning,
		Hostname:    "test-host",
		HostID:      hostID,
		ClusterName: as.ClusterName,
		Login:       "root",
		HostUser:    "root",
		HostPolicies: []*types.SessionTrackerPolicySet{
			{
				Name:    "ai-policy",
				Version: types.V7,
				RequireSessionJoin: []*types.SessionRequirePolicy{
					{
						Name:  "ai-approval",
						Kinds: []string{string(types.SSHSessionKind)},
						CommandApproval: &types.CommandApproval{
							Enabled:  true,
							Approver: types.CommandApproverAI,
						},
					},
				},
			},
		},
	})
	require.NoError(t, err)

	_, err = as.AuthServer.CreateSessionTracker(ctx, tracker)
	require.NoError(t, err)
}

func TestEvaluateCommand(t *testing.T) {
	ctx := context.Background()

	// Moderated session trackers (which carry the AI command-approval policy)
	// can only be created with the enterprise build. The OSS fail-closed
	// behaviour of EvaluateCommand itself is driven by whether a command
	// evaluator is registered, not by the build type.
	modulestest.SetTestModules(t, modulestest.Modules{TestBuildType: modules.BuildEnterprise})

	as, err := authtest.NewAuthServer(authtest.AuthServerConfig{Dir: t.TempDir()})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, as.Close()) })

	const hostID = "node-host-id"
	const otherHostID = "other-host-id"

	t.Run("non-node caller is denied", func(t *testing.T) {
		// A proxy is a builtin server role but not a node: it must be denied.
		s := serverWithRolesForServerID(t, as, types.RoleProxy, "proxy-host-id")
		_, err := s.EvaluateCommand(ctx, &proto.EvaluateCommandRequest{SessionID: "sess-1"})
		require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
	})

	t.Run("node not hosting the session is denied", func(t *testing.T) {
		sessionID := "sess-host-mismatch"
		createAICommandApprovalTracker(t, ctx, as, sessionID, otherHostID)

		// Caller is a node, but with a different host id.
		s := serverWithRolesForServerID(t, as, types.RoleNode, hostID)
		_, err := s.EvaluateCommand(ctx, &proto.EvaluateCommandRequest{SessionID: sessionID})
		require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
	})

	t.Run("node hosting a non-AI session is denied", func(t *testing.T) {
		sessionID := "sess-no-ai-policy"
		tracker, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
			SessionID:   sessionID,
			Kind:        string(types.SSHSessionKind),
			State:       types.SessionState_SessionStateRunning,
			HostID:      hostID,
			ClusterName: as.ClusterName,
			Login:       "root",
			HostUser:    "root",
		})
		require.NoError(t, err)
		_, err = as.AuthServer.CreateSessionTracker(ctx, tracker)
		require.NoError(t, err)

		s := serverWithRolesForServerID(t, as, types.RoleNode, hostID)
		_, err = s.EvaluateCommand(ctx, &proto.EvaluateCommandRequest{SessionID: sessionID})
		require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
	})

	t.Run("node hosting AI session with no evaluator is enterprise-only", func(t *testing.T) {
		sessionID := "sess-no-evaluator"
		createAICommandApprovalTracker(t, ctx, as, sessionID, hostID)

		// Ensure no evaluator is registered (OSS default).
		as.AuthServer.SetCommandEvaluator(nil)

		s := serverWithRolesForServerID(t, as, types.RoleNode, hostID)
		_, err := s.EvaluateCommand(ctx, &proto.EvaluateCommandRequest{SessionID: sessionID})
		require.True(t, trace.IsNotImplemented(err), "expected NotImplemented, got %v", err)
	})

	t.Run("node hosting AI session with evaluator returns mapped response", func(t *testing.T) {
		sessionID := "sess-with-evaluator"
		createAICommandApprovalTracker(t, ctx, as, sessionID, hostID)

		fake := &fakeCommandEvaluator{
			result: moderation.CommandEvaluationResult{Approved: true, Reasoning: "looks safe"},
		}
		as.AuthServer.SetCommandEvaluator(fake)
		t.Cleanup(func() { as.AuthServer.SetCommandEvaluator(nil) })

		s := serverWithRolesForServerID(t, as, types.RoleNode, hostID)
		resp, err := s.EvaluateCommand(ctx, &proto.EvaluateCommandRequest{
			SessionID:   sessionID,
			Command:     "rm -rf /",
			Policy:      "deny destructive commands",
			Model:       "my-model",
			Participant: "bob",
			Login:       "root",
			ServerID:    hostID,
			SessionKind: "ssh",
		})
		require.NoError(t, err)
		require.True(t, resp.Approved)
		require.Equal(t, "looks safe", resp.Reasoning)

		// The request fields are forwarded to the evaluator.
		require.Equal(t, "rm -rf /", fake.gotReq.Command)
		require.Equal(t, "deny destructive commands", fake.gotReq.Policy)
		require.Equal(t, "my-model", fake.gotReq.Model)
		require.Equal(t, "bob", fake.gotReq.Participant)
		require.Equal(t, sessionID, fake.gotReq.SessionID)
	})

	t.Run("regular user caller is denied", func(t *testing.T) {
		// A normal authenticated (non-builtin) user must never be able to use
		// auth as an LLM proxy: the very first guard rejects any caller that is
		// not a builtin node.
		user, role, err := authtest.CreateUserAndRole(as.AuthServer, "regular-user", nil, nil)
		require.NoError(t, err)

		identity := authtest.TestUserWithRoles(user.GetName(), []string{role.GetName()})
		authzContext := authz.ContextWithUser(t.Context(), identity.I)
		ctxIdentity, err := as.Authorizer.Authorize(authzContext)
		require.NoError(t, err)
		s := auth.NewServerWithRoles(as.AuthServer, as.AuditLog, *ctxIdentity)
		t.Cleanup(func() { s.Close() })

		_, err = s.EvaluateCommand(ctx, &proto.EvaluateCommandRequest{SessionID: "sess-regular-user"})
		require.True(t, trace.IsAccessDenied(err), "expected AccessDenied, got %v", err)
	})

	t.Run("evaluator error propagates (fail closed)", func(t *testing.T) {
		sessionID := "sess-evaluator-error"
		createAICommandApprovalTracker(t, ctx, as, sessionID, hostID)

		sentinel := trace.Errorf("evaluator exploded")
		fake := &fakeCommandEvaluator{
			// Even if a stale "approved" result is set, an error must take
			// precedence and the command must not be approved.
			result: moderation.CommandEvaluationResult{Approved: true},
			err:    sentinel,
		}
		as.AuthServer.SetCommandEvaluator(fake)
		t.Cleanup(func() { as.AuthServer.SetCommandEvaluator(nil) })

		s := serverWithRolesForServerID(t, as, types.RoleNode, hostID)
		resp, err := s.EvaluateCommand(ctx, &proto.EvaluateCommandRequest{SessionID: sessionID})
		require.Error(t, err)
		require.ErrorIs(t, err, sentinel)
		require.Nil(t, resp)
	})

	t.Run("session not found", func(t *testing.T) {
		// The caller is a legitimate node, but the referenced session does not
		// exist. The tracker lookup must fail and no approval is returned.
		s := serverWithRolesForServerID(t, as, types.RoleNode, hostID)
		resp, err := s.EvaluateCommand(ctx, &proto.EvaluateCommandRequest{SessionID: "does-not-exist"})
		require.Error(t, err)
		require.True(t, trace.IsNotFound(err), "expected NotFound, got %v", err)
		require.Nil(t, resp)
	})
}
