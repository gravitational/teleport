//go:build piv

package hardwarekey

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"os"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

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
	// This test expects a yubiKey to be connected with default PIV
	// settings and will overwrite any PIV data on the yubiKey.
	if os.Getenv("TELEPORT_TEST_YUBIKEY_PIV") == "" {
		t.Skip("Skipping TestAttestHardwareKey because TELEPORT_TEST_YUBIKEY_PIV is not set")
	}

	ctx := context.Background()
	resetYubikey(t)
	t.Cleanup(func() { resetYubikey(t) })

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	basicKey, err := keys.NewPrivateKey(priv, nil)
	require.NoError(t, err)

	hardwareKey, err := keys.GetYubiKeyPrivateKey(ctx, keys.PrivateKeyPolicyHardwareKey, "")
	require.NoError(t, err)
	hardwareKeyPubDER, err := x509.MarshalPKIXPublicKey(hardwareKey.Public())
	require.NoError(t, err)

	hardwareTouchKey, err := keys.GetYubiKeyPrivateKey(ctx, keys.PrivateKeyPolicyHardwareKeyTouch, "")
	require.NoError(t, err)

	for _, tt := range []struct {
		name                 string
		requiredPolicy       keys.PrivateKeyPolicy
		pub                  crypto.PublicKey
		att                  *keys.AttestationStatement
		s                    AttestationServer
		expectAttestedPolicy keys.PrivateKeyPolicy
		assertError          require.ErrorAssertionFunc
	}{
		{
			name:                 "policy met by provided attestation",
			requiredPolicy:       keys.PrivateKeyPolicyHardwareKey,
			pub:                  hardwareKey.Public(),
			att:                  hardwareKey.GetAttestationStatement(),
			s:                    &mockAttestationServer{make(map[string]*keys.AttestationData)},
			expectAttestedPolicy: keys.PrivateKeyPolicyHardwareKey,
			assertError:          require.NoError,
		}, {
			name:                 "policy exceeded by provided attestation",
			requiredPolicy:       keys.PrivateKeyPolicyHardwareKey,
			pub:                  hardwareTouchKey.Public(),
			att:                  hardwareTouchKey.GetAttestationStatement(),
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
			requiredPolicy: keys.PrivateKeyPolicyHardwareKey,
			pub:            basicKey.Public(),
			att:            basicKey.GetAttestationStatement(),
			s:              &mockAttestationServer{make(map[string]*keys.AttestationData)},
			assertError:    require.Error,
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
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			attestedPolicy, err := AttestHardwareKey(ctx, tt.s, tt.requiredPolicy, tt.att, tt.pub, 0)
			tt.assertError(t, err)
			require.Equal(t, tt.expectAttestedPolicy, attestedPolicy)
		})
	}
}

// resetYubikey connects to the first yubiKey and resets it to defaults.
func resetYubikey(t *testing.T) {
	t.Helper()
	y, err := keys.FindYubiKey(0)
	require.NoError(t, err)
	require.NoError(t, y.Reset())
}
