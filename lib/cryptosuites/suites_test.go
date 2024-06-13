// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package cryptosuites

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

// TestSuites tests that each algorithm suite defines a valid algorithm for each key purpose.
func TestSuites(t *testing.T) {
	ctx := context.Background()

	for s := range types.SignatureAlgorithmSuite_name {
		suite := types.SignatureAlgorithmSuite(s)
		t.Run(suite.String(), func(t *testing.T) {
			authPrefGetter := &fakeAuthPrefGetter{
				suite: suite,
			}
			for purpose := keyPurposeUnspecified + 1; purpose < keyPurposeMax; purpose++ {
				alg, err := AlgorithmForKey(ctx, authPrefGetter, purpose)
				require.NoError(t, err)
				assert.Greater(t, alg, algorithmUnspecified)
				assert.Less(t, alg, algorithmMax)
			}
		})
	}
}

type fakeAuthPrefGetter struct {
	suite types.SignatureAlgorithmSuite
}

func (f *fakeAuthPrefGetter) GetAuthPreference(context.Context) (types.AuthPreference, error) {
	return &types.AuthPreferenceV2{
		Spec: types.AuthPreferenceSpecV2{
			SignatureAlgorithmSuite: f.suite,
		},
	}, nil
}

// TestGenerateKeyWithAlgorithm sanity tests that each key algorithm yields keys of the expected type.
func TestGenerateKeyWithAlgorithm(t *testing.T) {
	key, err := GenerateKeyWithAlgorithm(RSA2048)
	require.NoError(t, err)
	_, ok := key.(*rsa.PrivateKey)
	require.True(t, ok)

	key, err = GenerateKeyWithAlgorithm(ECDSAP256)
	require.NoError(t, err)
	_, ok = key.(*ecdsa.PrivateKey)
	require.True(t, ok)

	key, err = GenerateKeyWithAlgorithm(Ed25519)
	require.NoError(t, err)
	_, ok = key.(ed25519.PrivateKey)
	require.True(t, ok)
}
