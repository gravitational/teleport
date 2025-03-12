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

package awsoidc

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/lib/cryptosuites"
)

func TestSendSSHPublicKeyRequest(t *testing.T) {
	isBadParamErrFn := func(tt require.TestingT, err error, i ...any) {
		require.True(tt, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
	}

	baseReqFn := func() SendSSHPublicKeyToEC2Request {
		return SendSSHPublicKeyToEC2Request{
			InstanceID:      "i-123",
			EC2SSHLoginUser: "root",
		}
	}

	for _, tt := range []struct {
		name            string
		req             func() SendSSHPublicKeyToEC2Request
		errCheck        require.ErrorAssertionFunc
		reqWithDefaults SendSSHPublicKeyToEC2Request
	}{
		{
			name: "no fields",
			req: func() SendSSHPublicKeyToEC2Request {
				return SendSSHPublicKeyToEC2Request{}
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing instance id",
			req: func() SendSSHPublicKeyToEC2Request {
				r := baseReqFn()
				r.InstanceID = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
		{
			name: "missing login user",
			req: func() SendSSHPublicKeyToEC2Request {
				r := baseReqFn()
				r.EC2SSHLoginUser = ""
				return r
			},
			errCheck: isBadParamErrFn,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.req()
			err := r.CheckAndSetDefaults()
			tt.errCheck(t, err)

			if err != nil {
				return
			}
			require.Empty(t, cmp.Diff(tt.reqWithDefaults, r))
		})
	}
}

func TestSendSSHPublicKeyToEC2(t *testing.T) {
	ctx := context.Background()

	m := &mockSendSSHPublicKeyClient{}

	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.Ed25519)
	require.NoError(t, err)
	sshSigner, err := ssh.NewSignerFromSigner(key)
	require.NoError(t, err)

	err = SendSSHPublicKeyToEC2(ctx, m, SendSSHPublicKeyToEC2Request{
		InstanceID:      "id-123",
		EC2SSHLoginUser: "root",
		PublicKey:       sshSigner.PublicKey(),
	})
	require.NoError(t, err)

	sshPublicKeyFromSigner := string(ssh.MarshalAuthorizedKey(sshSigner.PublicKey()))
	require.Equal(t, sshPublicKeyFromSigner, m.sshKeySent)
	require.Equal(t, "root", m.sshForUserSent)
}

type mockSendSSHPublicKeyClient struct {
	sshKeySent     string
	sshForUserSent string
}

func (m *mockSendSSHPublicKeyClient) SendSSHPublicKey(ctx context.Context, params *ec2instanceconnect.SendSSHPublicKeyInput, optFns ...func(*ec2instanceconnect.Options)) (*ec2instanceconnect.SendSSHPublicKeyOutput, error) {
	m.sshKeySent = *params.SSHPublicKey
	m.sshForUserSent = *params.InstanceOSUser
	return nil, nil
}
