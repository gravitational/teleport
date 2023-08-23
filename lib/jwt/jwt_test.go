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

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"
	josejwt "gopkg.in/square/go-jose.v2/jwt"

	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/utils"
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

// TestPublicOnlyVerifyAzure checks that a non-signing key used to validate a JWT
// can be created. Azure version.
func TestPublicOnlyVerifyAzure(t *testing.T) {
	publicBytes, privateBytes, err := GenerateKeyPair()
	require.NoError(t, err)
	privateKey, err := utils.ParsePrivateKey(privateBytes)
	require.NoError(t, err)
	publicKey, err := utils.ParsePublicKey(publicBytes)
	require.NoError(t, err)

	// Create a new key that can sign and verify tokens.
	key, err := New(&Config{
		PrivateKey:  privateKey,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: "example.com",
	})
	require.NoError(t, err)

	// Sign a token with the new key.
	token, err := key.SignAzureToken(AzureTokenClaims{
		TenantID: "dummy-tenant-id",
		Resource: "my-resource",
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
	claims, err := key.VerifyAzureToken(token)
	require.NoError(t, err)
	require.Equal(t, AzureTokenClaims{
		TenantID: "dummy-tenant-id",
		Resource: "my-resource",
	}, *claims)

	// Make sure this key returns an error when trying to sign.
	_, err = key.SignAzureToken(*claims)
	require.Error(t, err)
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
		Traits: wrappers.Traits{
			"trait1": []string{"value-1", "value-2"},
		},
		Expires: clock.Now().Add(1 * time.Minute),
		URI:     "http://127.0.0.1:8080",
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

func TestKey_SignAndVerifyPROXY(t *testing.T) {
	_, privateBytes, err := GenerateKeyPair()
	require.NoError(t, err)
	privateKey, err := utils.ParsePrivateKey(privateBytes)
	require.NoError(t, err)

	clock := clockwork.NewFakeClockAt(time.Now())
	const clusterName = "teleport-test"

	// Create a new key that can sign and verify tokens.
	key, err := New(&Config{
		PrivateKey:  privateKey,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: clusterName,
		Clock:       clock,
	})
	require.NoError(t, err)
	source := "1.2.3.4:555"
	destination := "4.3.2.1:666:"

	// Sign a token with the new key.
	token, err := key.SignPROXYJWT(PROXYSignParams{
		ClusterName:        clusterName,
		SourceAddress:      source,
		DestinationAddress: destination,
	})
	require.NoError(t, err)

	// Successfully verify
	_, err = key.VerifyPROXY(PROXYVerifyParams{
		ClusterName:        clusterName,
		SourceAddress:      source,
		DestinationAddress: destination,
		RawToken:           token,
	})
	require.NoError(t, err)

	// Check that if params don't match verification fails
	_, err = key.VerifyPROXY(PROXYVerifyParams{
		ClusterName:        clusterName + "1",
		SourceAddress:      source,
		DestinationAddress: destination,
		RawToken:           token,
	})
	require.ErrorContains(t, err, "invalid issuer")

	_, err = key.VerifyPROXY(PROXYVerifyParams{
		ClusterName:        clusterName,
		SourceAddress:      destination,
		DestinationAddress: source,
		RawToken:           token,
	})
	require.ErrorContains(t, err, "invalid subject")

	// Rewind clock backward and verify that token is not valid yet
	clock.Advance(time.Minute * -2)
	_, err = key.VerifyPROXY(PROXYVerifyParams{
		ClusterName:        clusterName,
		SourceAddress:      source,
		DestinationAddress: destination,
		RawToken:           token,
	})
	require.ErrorContains(t, err, "token not valid yet")

	// Advance clock and verify that token is expired now
	clock.Advance(time.Minute*2 + expirationPROXY*2)
	_, err = key.VerifyPROXY(PROXYVerifyParams{
		ClusterName:        clusterName,
		SourceAddress:      source,
		DestinationAddress: destination,
		RawToken:           token,
	})
	require.ErrorContains(t, err, "token is expired")
}

func TestKey_SignAndVerifyAWSOIDC(t *testing.T) {
	_, privateBytes, err := GenerateKeyPair()
	require.NoError(t, err)
	privateKey, err := utils.ParsePrivateKey(privateBytes)
	require.NoError(t, err)

	clock := clockwork.NewFakeClockAt(time.Now())
	const clusterName = "teleport-test"

	// Create a new key that can sign and verify tokens.
	key, err := New(&Config{
		PrivateKey:  privateKey,
		Algorithm:   defaults.ApplicationTokenAlgorithm,
		ClusterName: clusterName,
		Clock:       clock,
	})
	require.NoError(t, err)

	// Sign a token with the new key.
	expiresIn := time.Minute * 5
	token, err := key.SignAWSOIDC(SignParams{
		Username: "user",
		Issuer:   "https://localhost/",
		URI:      "https://localhost/",
		Subject:  "system:proxy",
		Audience: "discover.teleport",
		Expires:  clock.Now().Add(expiresIn),
	})
	require.NoError(t, err)

	// Successfully verify
	_, err = key.VerifyAWSOIDC(AWSOIDCVerifyParams{
		RawToken: token,
		Issuer:   "https://localhost/",
	})
	require.NoError(t, err, token)

	// Check that if params don't match verification fails
	_, err = key.VerifyAWSOIDC(AWSOIDCVerifyParams{
		RawToken: token,
		Issuer:   "https://localhost/" + "1",
	})
	require.ErrorContains(t, err, "invalid issuer")

	// Rewind clock backward and verify that token is not valid yet
	clock.Advance(time.Minute * -2)
	_, err = key.VerifyAWSOIDC(AWSOIDCVerifyParams{
		RawToken: token,
		Issuer:   "https://localhost/",
	})
	require.ErrorContains(t, err, "token not valid yet")
	// Revert time to before this sub-test.
	clock.Advance(time.Minute * 2)

	// Advance clock and verify that token is expired now
	clock.Advance(expiresIn + time.Minute)
	_, err = key.VerifyAWSOIDC(AWSOIDCVerifyParams{
		RawToken: token,
		Issuer:   "https://localhost/",
	})
	require.ErrorContains(t, err, "token is expired")
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
		Traits: wrappers.Traits{
			"trait1": []string{"value-1", "value-2"},
		},
		Expires: clock.Now().Add(1 * time.Minute),
		URI:     "http://127.0.0.1:8080",
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
