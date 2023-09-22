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
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func Test_findDuplicatedCertificates(t *testing.T) {
	allCerts := CertAuthorityMap{}

	addCA := func(t *testing.T, certType types.CertAuthType) {
		// Use weak private key for better performance.
		caKey, err := rsa.GenerateKey(rand.Reader, 512)
		require.NoError(t, err)
		name := "teleport.localhost.me"
		cert, err := tlsca.GenerateSelfSignedCAWithSigner(caKey, pkix.Name{CommonName: name}, nil, time.Minute)
		require.NoError(t, err)
		_, key, err := utils.MarshalPrivateKey(caKey)
		require.NoError(t, err)
		ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
			Type:        certType,
			ClusterName: name,
			ActiveKeys: types.CAKeySet{
				TLS: []*types.TLSKeyPair{{
					Cert: cert,
					Key:  key,
				}},
			},
		})
		require.NoError(t, err)
		allCerts[certType] = ca
	}

	addCA(t, types.UserCA)
	addCA(t, types.HostCA)
	addCA(t, types.OpenSSHCA)
	// create duplicated cert
	allCerts[types.DatabaseCA] = allCerts[types.HostCA].Clone()

	tests := []struct {
		name    string
		caTypes []types.CertAuthType
		want    []types.CertAuthType
	}{
		{
			name:    "no duplicates",
			caTypes: []types.CertAuthType{types.UserCA},
			want:    []types.CertAuthType{types.UserCA},
		},
		{
			name:    "duplicates",
			caTypes: []types.CertAuthType{types.HostCA},
			want:    []types.CertAuthType{types.HostCA, types.DatabaseCA},
		},
		{
			name:    "duplicates - other provided",
			caTypes: []types.CertAuthType{types.DatabaseCA},
			want:    []types.CertAuthType{types.HostCA, types.DatabaseCA},
		},
		{
			name:    "duplicates and no duplicates",
			caTypes: []types.CertAuthType{types.UserCA, types.HostCA, types.OpenSSHCA},
			want:    []types.CertAuthType{types.UserCA, types.HostCA, types.DatabaseCA, types.OpenSSHCA},
		},
		{
			name:    "rotate all",
			caTypes: []types.CertAuthType{types.UserCA, types.HostCA, types.DatabaseCA, types.OpenSSHCA},
			want:    []types.CertAuthType{types.UserCA, types.HostCA, types.DatabaseCA, types.OpenSSHCA},
		},
	}

	for _, tt := range tests {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := findDuplicatedCertificates(tt.caTypes, allCerts)
			// matches elements, ignores order
			require.ElementsMatchf(t, tt.want, got, "findDuplicatedCertificates() = %v, want %v", got, tt.want)
		})
	}
}
