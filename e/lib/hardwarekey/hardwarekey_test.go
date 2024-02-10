//go:build pivtest

package hardwarekey

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	attestation "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	"github.com/gravitational/teleport/api/utils/keys"
)

type mockAttestationServer struct {
	attestations map[string]*keys.AttestationData
}

func (s *mockAttestationServer) UpsertKeyAttestationData(ctx context.Context, attestationData *keys.AttestationData, ttl time.Duration) error {
	s.attestations[string(attestationData.PublicKeyDER)] = attestationData
	return nil
}

func (s *mockAttestationServer) GetKeyAttestationData(ctx context.Context, publicKey crypto.PublicKey) (*keys.AttestationData, error) {
	pubDER, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	att, ok := s.attestations[string(pubDER)]
	if !ok {
		return nil, trace.NotFound("attestation data not found")
	}
	return att, nil
}

// TestAttestHardwareKey tests AttestHardwareKey.
func TestAttestHardwareKey(t *testing.T) {
	ctx := context.Background()

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	basicKey, err := keys.NewPrivateKey(priv, nil)
	require.NoError(t, err)

	priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	hardwareKey, err := keys.NewPrivateKey(priv, nil)
	require.NoError(t, err)
	hardwareKeyPubDER, err := x509.MarshalPKIXPublicKey(hardwareKey.Public())
	require.NoError(t, err)

	for _, tt := range []struct {
		name                 string
		requiredPolicy       keys.PrivateKeyPolicy
		pub                  crypto.PublicKey
		attestationstatement *keys.AttestationStatement
		attestationData      *keys.AttestationData
		s                    AttestationServer
		expectAttestedPolicy keys.PrivateKeyPolicy
		assertError          require.ErrorAssertionFunc
	}{
		{
			name:           "policy met by provided attestation",
			requiredPolicy: keys.PrivateKeyPolicyHardwareKey,
			pub:            hardwareKey.Public(),
			attestationData: &keys.AttestationData{
				PublicKeyDER:     hardwareKeyPubDER,
				PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKey,
			},
			s:                    &mockAttestationServer{make(map[string]*keys.AttestationData)},
			expectAttestedPolicy: keys.PrivateKeyPolicyHardwareKey,
			assertError:          require.NoError,
		}, {
			name:           "policy exceeded by provided attestation",
			requiredPolicy: keys.PrivateKeyPolicyHardwareKey,
			pub:            hardwareKey.Public(),
			attestationData: &keys.AttestationData{
				PublicKeyDER:     hardwareKeyPubDER,
				PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
			},
			s:                    &mockAttestationServer{make(map[string]*keys.AttestationData)},
			assertError:          require.NoError,
			expectAttestedPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
		}, {
			name:           "policy met by stored attestation",
			requiredPolicy: keys.PrivateKeyPolicyHardwareKey,
			pub:            hardwareKey.Public(),
			s: &mockAttestationServer{
				attestations: map[string]*keys.AttestationData{
					string(hardwareKeyPubDER): {
						PublicKeyDER:     hardwareKeyPubDER,
						PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKey,
					},
				},
			},
			expectAttestedPolicy: keys.PrivateKeyPolicyHardwareKey,
			assertError:          require.NoError,
		}, {
			name:           "policy not met by provided attestation",
			requiredPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
			pub:            hardwareKey.Public(),
			attestationData: &keys.AttestationData{
				PublicKeyDER:     hardwareKeyPubDER,
				PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKey,
			},
			s:           &mockAttestationServer{make(map[string]*keys.AttestationData)},
			assertError: require.Error,
		}, {
			name:           "policy not met by stored attestation",
			requiredPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
			pub:            hardwareKey.Public(),
			s: &mockAttestationServer{
				attestations: map[string]*keys.AttestationData{
					string(hardwareKeyPubDER): {
						PublicKeyDER:     hardwareKeyPubDER,
						PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKey,
					},
				},
			},
			assertError: require.Error,
		}, {
			name:           "attestation data doesn't match public key",
			requiredPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
			pub:            basicKey.Public(),
			s:              &mockAttestationServer{make(map[string]*keys.AttestationData)},
			attestationData: &keys.AttestationData{
				PublicKeyDER:     hardwareKeyPubDER,
				PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKeyTouch,
			},
			assertError: require.Error,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// If fake attestation data is provided set it in the fake attestation data provider.
			fakeAttestationData = tt.attestationData
			t.Cleanup(func() {
				fakeAttestationData = nil
			})

			// If fake attestation data is provided, provide an empty yubikey attestation statement
			// to force "attestYubikey" to be called.
			var attestationStatement *keys.AttestationStatement
			if tt.attestationData != nil {
				attestationStatement = &keys.AttestationStatement{
					AttestationStatement: &attestation.AttestationStatement_YubikeyAttestationStatement{},
				}
			}

			attestedPolicy, err := AttestHardwareKey(ctx, tt.s, tt.requiredPolicy, attestationStatement, tt.pub, 0)
			tt.assertError(t, err)
			require.Equal(t, tt.expectAttestedPolicy, attestedPolicy)
		})
	}
}
