/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package auth

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/events"
)

func TestAuth_RegisterUsingToken(t *testing.T) {
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)

	// create a static token
	staticToken := types.ProvisionTokenV1{
		Roles: []types.SystemRole{types.RoleNode},
		Token: "static_token",
	}
	staticTokens, err := types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{staticToken},
	})
	require.NoError(t, err)
	err = p.a.SetStaticTokens(staticTokens)
	require.NoError(t, err)

	// create a valid dynamic token
	dynamicToken := generateTestToken(
		ctx,
		t,
		types.SystemRoles{types.RoleNode},
		p.a.GetClock().Now().Add(time.Minute*30),
		p.a,
	)

	// create an expired dynamic token
	expiredDynamicToken := generateTestToken(
		ctx,
		t,
		types.SystemRoles{types.RoleNode},
		p.a.GetClock().Now().Add(-time.Minute*30),
		p.a,
	)

	sshPrivateKey, sshPublicKey, err := testauthority.New().GenerateKeyPair()
	require.NoError(t, err)

	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	testcases := []struct {
		desc             string
		req              *types.RegisterUsingTokenRequest
		certsAssertion   func(*proto.Certs)
		errorAssertion   func(error) bool
		waitTokenDeleted bool // Expired tokens are deleted in background, might need slight delay in relevant test
	}{
		{
			desc:           "reject empty",
			req:            &types.RegisterUsingTokenRequest{},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "reject no token",
			req: &types.RegisterUsingTokenRequest{
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "reject no HostID",
			req: &types.RegisterUsingTokenRequest{
				Token:        staticToken.Token,
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "allow no NodeName",
			req: &types.RegisterUsingTokenRequest{
				Token:        staticToken.Token,
				HostID:       "localhost",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
		},
		{
			desc: "reject no SSH pub",
			req: &types.RegisterUsingTokenRequest{
				Token:        staticToken.Token,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "reject no TLS pub",
			req: &types.RegisterUsingTokenRequest{
				Token:        staticToken.Token,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
			},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "reject bad token",
			req: &types.RegisterUsingTokenRequest{
				Token:        "not a token",
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsAccessDenied,
		},
		{
			desc: "allow static token",
			req: &types.RegisterUsingTokenRequest{
				Token:        staticToken.Token,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
		},
		{
			desc: "reject wrong role static",
			req: &types.RegisterUsingTokenRequest{
				Token:        staticToken.Token,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleProxy,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "allow dynamic token",
			req: &types.RegisterUsingTokenRequest{
				Token:        dynamicToken,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
		},
		{
			desc: "reject wrong role dynamic",
			req: &types.RegisterUsingTokenRequest{
				Token:        dynamicToken,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleProxy,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsBadParameter,
		},
		{
			desc: "check additional principals",
			req: &types.RegisterUsingTokenRequest{
				Token:                dynamicToken,
				HostID:               "localhost",
				NodeName:             "node-name",
				Role:                 types.RoleNode,
				PublicSSHKey:         sshPublicKey,
				PublicTLSKey:         tlsPublicKey,
				AdditionalPrincipals: []string{"example.com"},
			},
			certsAssertion: func(certs *proto.Certs) {
				hostCert, err := sshutils.ParseCertificate(certs.SSH)
				require.NoError(t, err)
				require.Contains(t, hostCert.ValidPrincipals, "example.com")
			},
		},
		{
			desc: "reject expired dynamic token",
			req: &types.RegisterUsingTokenRequest{
				Token:        expiredDynamicToken,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			waitTokenDeleted: true,
			errorAssertion:   trace.IsAccessDenied,
		},
		{
			// relies on token being deleted during previous testcase
			desc: "expired token should be gone",
			req: &types.RegisterUsingTokenRequest{
				Token:        expiredDynamicToken,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			errorAssertion: trace.IsAccessDenied,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			certs, err := p.a.RegisterUsingToken(ctx, tc.req)
			if tc.errorAssertion != nil {
				require.True(t, tc.errorAssertion(err))
				if tc.waitTokenDeleted {
					require.Eventually(t, func() bool {
						_, err := p.a.ValidateToken(ctx, tc.req.Token)
						return err != nil && strings.Contains(err.Error(), TokenExpiredOrNotFound)
					}, time.Millisecond*100, time.Millisecond*10)
				}
				return
			}
			require.NoError(t, err)
			if tc.certsAssertion != nil {
				tc.certsAssertion(certs)
			}

			// Check audit log event is emitted
			evt := p.mockEmitter.LastEvent()
			require.NotNil(t, evt)
			joinEvent, ok := evt.(*apievents.InstanceJoin)
			require.True(t, ok)
			require.Equal(t, events.InstanceJoinEvent, joinEvent.Type)
			require.Equal(t, events.InstanceJoinCode, joinEvent.Code)
			require.Equal(t, tc.req.NodeName, joinEvent.NodeName)
			require.Equal(t, tc.req.HostID, joinEvent.HostID)
			require.EqualValues(t, tc.req.Role, joinEvent.Role)
			require.EqualValues(t, types.JoinMethodToken, joinEvent.Method)
		})
	}
}
