/*
Copyright 2023 Gravitational, Inc.

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

package awsoidc

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/ec2instanceconnect"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
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

	sshSigner, err := SendSSHPublicKeyToEC2(ctx, m, SendSSHPublicKeyToEC2Request{
		InstanceID:      "id-123",
		EC2SSHLoginUser: "root",
	})
	require.NoError(t, err)
	require.NotNil(t, sshSigner)

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
