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

package subcav1

import (
	"context"
	"crypto"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/tlsutils"
	"github.com/gravitational/teleport/lib/auth/keystore"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
	"github.com/gravitational/teleport/lib/tlscatest"
)

type HAEnvParams struct {
	Storage     subcaenv.EnvParams
	Service     EnvParams
	NumServices int
}

// NewHAEnv creates a new HA-like (High Availability) env, with multiple Auths
// plugged into distinct "HSM" modules (using a fake KeystoreManager
// implementation).
func NewHAEnv(t *testing.T, params HAEnvParams) []*Env {
	storageEnv := subcaenv.New(t, params.Storage)
	params.Service.StorageEnv = storageEnv

	// Each service gets its own fakeKeystoreManager, which can sign one key for
	// every created caType.
	type keyPEM = []byte
	keymanagerKeyPEMs := make([][]keyPEM, params.NumServices)

	for _, caType := range params.Storage.CATypesToCreate {
		const loadKeys = true
		ca, err := storageEnv.Trust.GetCertAuthority(t.Context(), types.CertAuthID{
			Type:       caType,
			DomainName: storageEnv.ClusterName,
		}, loadKeys)
		require.NoError(t, err)

		aks := ca.GetActiveKeys()

		// 1st key (created by storageEnv).
		{
			kp0 := aks.TLS[0]
			kp0.KeyType = types.PrivateKeyType_PKCS11 // faked by "fakeKeystoreManager".
			keymanagerKeyPEMs[0] = append(keymanagerKeyPEMs[0], kp0.Key)
			kp0.Key = nil
		}

		// 2nd to Nth keys (newly created).
		numExtraKeys := params.NumServices - 1
		for j := range numExtraKeys {
			keyPEM, certPEM, err := tlscatest.GenerateSelfSignedCA(tlscatest.GenerateCAConfig{
				ClusterName: storageEnv.ClusterName,
			})
			require.NoError(t, err)
			kp := &types.TLSKeyPair{
				Cert:    certPEM,
				KeyType: types.PrivateKeyType_PKCS11,
			}
			aks.TLS = append(aks.TLS, kp)
			keymanagerKeyPEMs[j+1] = append(keymanagerKeyPEMs[j+1], keyPEM)
		}

		require.NoError(t, ca.SetActiveKeys(aks))
		_, err = storageEnv.Trust.UpdateCertAuthority(t.Context(), ca)
		require.NoError(t, err)
	}

	envs := make([]*Env, params.NumServices)
	for i := range params.NumServices {
		params.Service.KeystoreManager = newFakeKeystoreManager(t, keymanagerKeyPEMs[i])
		envs[i] = NewEnv(t, params.Service)
	}
	return envs
}

type comparablePublicKey interface {
	Equal(crypto.PublicKey) bool
}

type fakeKeystoreManager struct {
	knownKeys []crypto.Signer
}

func newFakeKeystoreManager(t *testing.T, keyPEMs [][]byte) *fakeKeystoreManager {
	knownKeys := make([]crypto.Signer, len(keyPEMs))
	for i, pem := range keyPEMs {
		priv, err := keys.ParsePrivateKey(pem)
		require.NoError(t, err)
		signer := priv.Signer

		_, isComparable := signer.Public().(comparablePublicKey)
		require.True(t, isComparable, "signer.Public() is not comparable (signer=%T)", signer)

		knownKeys[i] = signer
	}
	return &fakeKeystoreManager{
		knownKeys: knownKeys,
	}
}

func (f *fakeKeystoreManager) TLSSigner(ctx context.Context, kp *types.TLSKeyPair) (crypto.Signer, error) {
	cert, err := tlsutils.ParseCertificatePEM(kp.Cert)
	if err != nil {
		return nil, err
	}
	pub := cert.PublicKey

	for _, knownKey := range f.knownKeys {
		if knownKey.Public().(comparablePublicKey).Equal(pub) {
			return knownKey, nil
		}
	}
	return nil, keystore.ErrUnusableKey
}
