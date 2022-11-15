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
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"

	apisshutils "github.com/gravitational/teleport/api/utils/sshutils"
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
