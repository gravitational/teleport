/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package auth

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestAuth_RegisterUsingToken(t *testing.T) {
	ctx := context.Background()
	p, err := newTestPack(ctx, t.TempDir())
	require.NoError(t, err)
	a := p.a

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

	// create a dynamic token
	dynamicToken, err := a.GenerateToken(ctx, GenerateTokenRequest{
		Roles: types.SystemRoles{types.RoleNode},
		TTL:   time.Hour,
	})
	require.NoError(t, err)
	require.NotNil(t, dynamicToken)

	sshPrivateKey, sshPublicKey, err := a.GenerateKeyPair("")
	require.NoError(t, err)

	tlsPublicKey, err := PrivateKeyToPublicKeyTLS(sshPrivateKey)
	require.NoError(t, err)

	testcases := []struct {
		desc           string
		req            *types.RegisterUsingTokenRequest
		certsAssertion func(*proto.Certs)
		errorAssertion func(error) bool
		clock          clockwork.Clock
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
			desc: "check additional pricipals",
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
				Token:        dynamicToken,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			clock:          clockwork.NewFakeClockAt(time.Now().Add(time.Hour + 1)),
			errorAssertion: trace.IsAccessDenied,
		},
		{
			// relies on token being deleted during previous testcase
			desc: "expired token should be gone",
			req: &types.RegisterUsingTokenRequest{
				Token:        dynamicToken,
				HostID:       "localhost",
				NodeName:     "node-name",
				Role:         types.RoleNode,
				PublicSSHKey: sshPublicKey,
				PublicTLSKey: tlsPublicKey,
			},
			clock:          clockwork.NewRealClock(),
			errorAssertion: trace.IsAccessDenied,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			if tc.clock == nil {
				tc.clock = clockwork.NewRealClock()
			}
			a.SetClock(tc.clock)
			certs, err := a.RegisterUsingToken(ctx, tc.req)
			if tc.errorAssertion != nil {
				require.True(t, tc.errorAssertion(err))
				return
			}
			require.NoError(t, err)
			if tc.certsAssertion != nil {
				tc.certsAssertion(certs)
			}
		})
	}
}
