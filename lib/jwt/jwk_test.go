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

package jwt

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
)

func TestMarshalJWK(t *testing.T) {
	t.Parallel()

	for _, alg := range supportedAlgorithms {
		t.Run(alg.String(), func(t *testing.T) {
			key, err := cryptosuites.GenerateKeyWithAlgorithm(alg)
			require.NoError(t, err)

			pubBytes, err := keys.MarshalPublicKey(key.Public())
			require.NoError(t, err)

			jwk, err := MarshalJWK(pubBytes)
			require.NoError(t, err)

			// Required for integrating with AWS OpenID Connect Identity Provider.
			require.Equal(t, "sig", jwk.Use)
		})
	}
}

func TestUnmarshalECDSAJWK(t *testing.T) {
	t.Parallel()

	key, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)

	pubBytes, err := keys.MarshalPublicKey(key.Public())
	require.NoError(t, err)

	jwk, err := MarshalJWK(pubBytes)
	require.NoError(t, err)

	strippedLeadingZeroPublicKey, err := ecdsa.ParseUncompressedPublicKey(elliptic.P256(), []byte{
		0x04,
		0x98, 0x6a, 0xe2, 0x50, 0x6f, 0x1f, 0xf1, 0x04,
		0xd0, 0x42, 0x30, 0x86, 0x1d, 0x8f, 0x4b, 0x49,
		0x8f, 0x4b, 0xc4, 0xc6, 0xd0, 0x09, 0xb3, 0x0f,
		0x75, 0x44, 0xdc, 0x12, 0x9b, 0x82, 0xd2, 0x8d,
		0x00,
		0x3c, 0xcc, 0xc0, 0xa6, 0x46, 0x0e, 0x0a, 0xe3,
		0x28, 0xa4, 0xd9, 0x7d, 0x3c, 0x7b, 0x61, 0xd8,
		0x6f, 0xc6, 0x28, 0x9c, 0x18, 0x9f, 0x25, 0x25,
		0x11, 0x0c, 0x44, 0x1b, 0xb0, 0x7e, 0x97,
	})
	require.NoError(t, err)

	tests := []struct {
		name          string
		jwk           JWK
		wantErr       bool
		wantPublicKey *ecdsa.PublicKey
	}{
		{
			name:          "round trip",
			jwk:           jwk,
			wantPublicKey: key.Public().(*ecdsa.PublicKey),
		},
		{
			name: "stripped leading zero coordinate",
			jwk: JWK{
				KeyType:   keyTypeEC,
				Algorithm: "ES256",
				Curve:     "P-256",
				X:         "mGriUG8f8QTQQjCGHY9LSY9LxMbQCbMPdUTcEpuC0o0",
				Y:         "PMzApkYOCuMopNl9PHth2G_GKJwYnyUlEQxEG7B-lw",
			},
			wantPublicKey: strippedLeadingZeroPublicKey,
		},
		{
			name: "malformed coordinates",
			jwk: JWK{
				KeyType:   keyTypeEC,
				Algorithm: "ES256",
				Curve:     "P-256",
				X:         "AA",
				Y:         "AA",
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pub, err := UnmarshalJWK(test.jwk)
			if test.wantErr {
				require.Error(t, err)
				require.Nil(t, pub)
				return
			}

			require.NoError(t, err)
			require.True(t, test.wantPublicKey.Equal(pub))
		})
	}
}

func TestKeyIDHasConsistentOutputForAnInput(t *testing.T) {
	t.Parallel()

	for _, alg := range supportedAlgorithms {
		t.Run(alg.String(), func(t *testing.T) {
			key, err := cryptosuites.GenerateKeyWithAlgorithm(alg)
			require.NoError(t, err)
			id1, err := KeyID(key.Public())
			require.NoError(t, err)
			id2, err := KeyID(key.Public())
			require.NoError(t, err)
			require.NotEmpty(t, id1)
			require.Equal(t, id1, id2)

			expectedLength := base64.RawURLEncoding.EncodedLen(sha256.Size)
			require.Len(t, id1, expectedLength, "expected key id to always be %d characters long", expectedLength)
		})
	}
}

func TestKeyIDHasDistinctOutputForDifferingInputs(t *testing.T) {
	t.Parallel()

	for _, alg := range supportedAlgorithms {
		t.Run(alg.String(), func(t *testing.T) {
			privateKey1, err := cryptosuites.GenerateKeyWithAlgorithm(alg)
			require.NoError(t, err)
			privateKey2, err := cryptosuites.GenerateKeyWithAlgorithm(alg)
			require.NoError(t, err)
			id1, err := KeyID(privateKey1.Public())
			require.NoError(t, err)
			id2, err := KeyID(privateKey2.Public())
			require.NoError(t, err)
			require.NotEmpty(t, id1)
			require.NotEmpty(t, id2)
			require.NotEqual(t, id1, id2)
		})
	}
}

// TestKeyIDCompatibility ensures we do not introduce a change in the KeyID algorithm for existing keys.
// It does so by ensuring that a pre-generated public key results in the expected value.
func TestKeyIDCompatibility(t *testing.T) {
	for _, tc := range []struct {
		desc       string
		pubKeyPEM  string
		expectedID string
	}{
		{
			desc: "RSA",
			pubKeyPEM: `-----BEGIN RSA PUBLIC KEY-----
MEgCQQDOS7WRzZm+ADCp8dL/fNtJvKegWx0ShJ8jzenoIyK4i7KW8Y23/mr5EEul
+B3xNVX2pMu3WOsgH4kZ088x9vb3AgMBAAE=
-----END RSA PUBLIC KEY-----`,
			expectedID: "GDLHLDvPUYmNLVU3WgshDX7bAw8xEmML8ypeE9KRAEQ",
		},
		{
			desc: "ECDSA",
			pubKeyPEM: `-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEoqR1cHAPOWIhqbvXhBfQZq9jndZH
PsPcvHNBFaa5GxTtwWFgzLEM17ERKDdBCbCf8oME2GRMKXlWOADlC3MYxg==
-----END PUBLIC KEY-----`,
			expectedID: "fLYYX_JCuFA6XuN6BVCeas1bbEWRd7clCTkr8QG6Djk",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			publicKey, err := keys.ParsePublicKey([]byte(tc.pubKeyPEM))
			require.NoError(t, err)

			kid, err := KeyID(publicKey)
			require.NoError(t, err)

			require.Equal(t, tc.expectedID, kid)
		})
	}
}
