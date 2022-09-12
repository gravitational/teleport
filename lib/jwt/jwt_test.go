/*
Copyright 2020 Gravitational, Inc.

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

package jwt

import (
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
	josejwt "gopkg.in/square/go-jose.v2/jwt"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
)

func TestSignAndVerify(t *testing.T) {
	_, privateBytes, err := GenerateKeyPair()
	require.NoError(t, err)
	privateKey, err := utils.ParsePrivateKey(privateBytes)
	require.NoError(t, err)

	clock := clockwork.NewFakeClockAt(time.Now())

	// Create a new key that can sign and verify tokens.
	key, err := New(&Config{
		Clock:       clock,
		PrivateKey:  privateKey,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: "example.com",
	})
	require.NoError(t, err)

	// Sign a token with the new key.
	token, err := key.Sign(SignParams{
		Username: "foo@example.com",
		Roles:    []string{"foo", "bar"},
		Expires:  clock.Now().Add(1 * time.Minute),
		URI:      "http://127.0.0.1:8080",
	})
	require.NoError(t, err)

	// Verify that the token can be validated and values match expected values.
	claims, err := key.Verify(VerifyParams{
		Username: "foo@example.com",
		RawToken: token,
		URI:      "http://127.0.0.1:8080",
	})
	require.NoError(t, err)
	require.Equal(t, claims.Username, "foo@example.com")
	require.Equal(t, claims.Roles, []string{"foo", "bar"})
}

// TestPublicOnlyVerify checks that a non-signing key used to validate a JWT
// can be created.
func TestPublicOnlyVerify(t *testing.T) {
	publicBytes, privateBytes, err := GenerateKeyPair()
	require.NoError(t, err)
	privateKey, err := utils.ParsePrivateKey(privateBytes)
	require.NoError(t, err)
	publicKey, err := utils.ParsePublicKey(publicBytes)
	require.NoError(t, err)

	clock := clockwork.NewFakeClockAt(time.Now())

	// Create a new key that can sign and verify tokens.
	key, err := New(&Config{
		PrivateKey:  privateKey,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: "example.com",
	})
	require.NoError(t, err)

	// Sign a token with the new key.
	token, err := key.Sign(SignParams{
		Username: "foo@example.com",
		Roles:    []string{"foo", "bar"},
		Expires:  clock.Now().Add(1 * time.Minute),
		URI:      "http://127.0.0.1:8080",
	})
	require.NoError(t, err)

	// Create a new key that can only verify tokens and make sure the token
	// values match the expected values.
	key, err = New(&Config{
		PublicKey:   publicKey,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: "example.com",
	})
	require.NoError(t, err)
	claims, err := key.Verify(VerifyParams{
		Username: "foo@example.com",
		URI:      "http://127.0.0.1:8080",
		RawToken: token,
	})
	require.NoError(t, err)
	require.Equal(t, claims.Username, "foo@example.com")
	require.Equal(t, claims.Roles, []string{"foo", "bar"})

	// Make sure this key returns an error when trying to sign.
	_, err = key.Sign(SignParams{
		Username: "foo@example.com",
		Roles:    []string{"foo", "bar"},
		Expires:  clock.Now().Add(1 * time.Minute),
		URI:      "http://127.0.0.1:8080",
	})
	require.Error(t, err)
}

// TestExpiry checks that token expiration works.
func TestExpiry(t *testing.T) {
	_, privateBytes, err := GenerateKeyPair()
	require.NoError(t, err)
	privateKey, err := utils.ParsePrivateKey(privateBytes)
	require.NoError(t, err)

	clock := clockwork.NewFakeClockAt(time.Now())

	// Create a new key that can be used to sign and verify tokens.
	key, err := New(&Config{
		Clock:       clock,
		PrivateKey:  privateKey,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: "example.com",
	})
	require.NoError(t, err)

	// Sign a token with a 1 minute expiration.
	token, err := key.Sign(SignParams{
		Username: "foo@example.com",
		Roles:    []string{"foo", "bar"},
		Expires:  clock.Now().Add(1 * time.Minute),
		URI:      "http://127.0.0.1:8080",
	})
	require.NoError(t, err)

	// Verify that the token is still valid.
	claims, err := key.Verify(VerifyParams{
		Username: "foo@example.com",
		URI:      "http://127.0.0.1:8080",
		RawToken: token,
	})
	require.NoError(t, err)
	require.Equal(t, claims.Username, "foo@example.com")
	require.Equal(t, claims.Roles, []string{"foo", "bar"})
	require.Equal(t, claims.IssuedAt, josejwt.NewNumericDate(clock.Now()))

	// Advance time by two minutes and verify the token is no longer valid.
	clock.Advance(2 * time.Minute)
	_, err = key.Verify(VerifyParams{
		Username: "foo@example.com",
		URI:      "http://127.0.0.1:8080",
		RawToken: token,
	})
	require.Error(t, err)
}
