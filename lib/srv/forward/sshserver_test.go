/*
 *
 * Copyright 2022 Gravitational, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package forward

import (
	"context"
	"crypto/rand"
	"errors"
	"os/user"
	"sync/atomic"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport"
	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils"
	"github.com/gravitational/teleport/lib/utils"
)

func TestSignersWithSHA1Fallback(t *testing.T) {
	assertSHA2Signer := func(t *testing.T, signer ssh.Signer) {
		require.Equal(t, ssh.CertAlgoRSAv01, signer.PublicKey().Type())

		sha2AlgSigner, ok := signer.(ssh.AlgorithmSigner)
		require.True(t, ok)

		data := make([]byte, 32)
		// This is how x/crypto signs SSH certificates.
		sig, err := sha2AlgSigner.SignWithAlgorithm(rand.Reader, data, ssh.KeyAlgoRSASHA512)
		require.NoError(t, err)
		require.Equal(t, ssh.KeyAlgoRSASHA512, sig.Format)
	}

	assertSHA1Signer := func(t *testing.T, signer ssh.Signer) {
		require.Equal(t, ssh.CertAlgoRSAv01, signer.PublicKey().Type())

		// We should not be able to case the signer to ssh.AlgorithmSigner.
		// Otherwise, x/crypto will use SHA-2-512 for signing.
		_, ok := signer.(ssh.AlgorithmSigner)
		require.False(t, ok)

		data := make([]byte, 32)
		sig, err := signer.Sign(rand.Reader, data)
		require.NoError(t, err)
		require.Equal(t, ssh.KeyAlgoRSA, sig.Format)
	}

	tests := []struct {
		name      string
		signersCb func(t *testing.T) []ssh.Signer
		want      func(t *testing.T, got []ssh.Signer)
	}{
		{
			name: "simple",
			signersCb: func(t *testing.T) []ssh.Signer {
				signer, err := apisshutils.MakeTestSSHCA()
				require.NoError(t, err)
				cert, err := apisshutils.MakeRealHostCert(signer)
				require.NoError(t, err)
				return []ssh.Signer{cert}
			},
			want: func(t *testing.T, signers []ssh.Signer) {
				// We expect 2 certificates, order matters.
				require.Len(t, signers, 2)
				assertSHA2Signer(t, signers[0])
				assertSHA1Signer(t, signers[1])
			},
		},
		{
			name: "public key only",
			signersCb: func(t *testing.T) []ssh.Signer {
				signer, err := apisshutils.MakeTestSSHCA()
				require.NoError(t, err)
				return []ssh.Signer{signer}
			},
			want: func(t *testing.T, signers []ssh.Signer) {
				// public key should not be copied
				require.Len(t, signers, 1)
				require.Equal(t, ssh.KeyAlgoRSA, signers[0].PublicKey().Type())
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			getSignersFn := signersWithSHA1Fallback(tt.signersCb(t))
			signers, err := getSignersFn()
			require.NoError(t, err)
			tt.want(t, signers)
		})
	}
}

type newChannelMock struct {
	channelType string
	accepted    atomic.Bool
	rejected    atomic.Bool
}

func (n *newChannelMock) Accept() (ssh.Channel, <-chan *ssh.Request, error) {
	n.accepted.Store(true)
	return nil, nil, errors.New("mock channel")
}

func (n *newChannelMock) Reject(reason ssh.RejectionReason, message string) error {
	n.rejected.Store(true)
	return nil
}

func (n *newChannelMock) ChannelType() string {
	return n.channelType
}

func (n *newChannelMock) ExtraData() []byte {
	return ssh.Marshal(sshutils.DirectTCPIPReq{
		Host:     "localhost",
		Port:     0,
		Orig:     "localhost",
		OrigPort: 0,
	})
}

// TestDirectTCPIP ensures that ssh client using SessionJoinPrincipal as Login
// cannot connect using "direct-tcpip" on forward mode.
//
// Forward requires a lot of dependencies and we don't have top level tests
// yet here. If we add it in future, test should be rework to use public methods
// instead of internals.
func TestDirectTCPIP(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	cases := []struct {
		name           string
		login          string
		expectAccepted bool
		expectRejected bool
	}{
		{
			name:           "join principal rejected",
			login:          teleport.SSHSessionJoinPrincipal,
			expectAccepted: false,
			expectRejected: true,
		},
		{
			name: "user allowed",
			login: func() string {
				u, err := user.Current()
				require.NoError(t, err)
				return u.Username
			}(),
			expectAccepted: true,
			// expectRejected is set to true because we are using mock channel
			// which return errors on accept.
			expectRejected: true,
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s := Server{
				log:             utils.NewLoggerForTests().WithField(trace.Component, "test"),
				identityContext: srv.IdentityContext{Login: tt.login},
			}

			nch := &newChannelMock{channelType: teleport.ChanDirectTCPIP}
			s.handleChannel(ctx, nch)
			require.Equal(t, tt.expectRejected, nch.rejected.Load())
			require.Equal(t, tt.expectAccepted, nch.accepted.Load())
		})
	}
}
