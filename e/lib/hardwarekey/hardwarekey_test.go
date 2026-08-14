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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	attestation "github.com/gravitational/teleport/api/gen/proto/go/attestation/v1"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
)

type mockAttestationServer struct {
	attestations map[string]*keys.AttestationData
}

func (s *mockAttestationServer) UpsertKeyAttestationData(ctx context.Context, attestationData *keys.AttestationData, ttl time.Duration) error {
	s.attestations[string(attestationData.PublicKeyDER)] = attestationData
	return nil
}

func (s *mockAttestationServer) GetKeyAttestationData(ctx context.Context, pubDER []byte) (*keys.AttestationData, error) {
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
	basicKey, err := keys.NewPrivateKey(priv)
	require.NoError(t, err)

	priv, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	hardwareKey, err := keys.NewPrivateKey(priv)
	require.NoError(t, err)
	hardwareKeyPubDER, err := x509.MarshalPKIXPublicKey(hardwareKey.Public())
	require.NoError(t, err)

	testAttestationData := &keys.AttestationData{
		PublicKeyDER:     hardwareKeyPubDER,
		PrivateKeyPolicy: keys.PrivateKeyPolicyHardwareKey,
	}

	for _, tt := range []struct {
		name                  string
		s                     AttestationServer
		pub                   crypto.PublicKey
		attestationData       *keys.AttestationData
		assertAttestError     require.ErrorAssertionFunc
		expectAttestationData *keys.AttestationData
	}{
		{
			name: "no attestation",
			s:    &mockAttestationServer{make(map[string]*keys.AttestationData)},
			pub:  hardwareKey.Public(),
			assertAttestError: func(tt require.TestingT, err error, i ...interface{}) {
				assert.True(t, trace.IsNotFound(err), "expected not found err but got %v", err)
			},
			expectAttestationData: nil,
		}, {
			name:                  "attestation from statement",
			pub:                   hardwareKey.Public(),
			attestationData:       testAttestationData,
			s:                     &mockAttestationServer{make(map[string]*keys.AttestationData)},
			assertAttestError:     require.NoError,
			expectAttestationData: testAttestationData,
		}, {
			name: "attestation from backend",
			pub:  hardwareKey.Public(),
			s: &mockAttestationServer{
				attestations: map[string]*keys.AttestationData{
					string(hardwareKeyPubDER): testAttestationData,
				},
			},
			assertAttestError:     require.NoError,
			expectAttestationData: testAttestationData,
		}, {
			name:            "attestation doesn't match public key",
			pub:             basicKey.Public(),
			s:               &mockAttestationServer{make(map[string]*keys.AttestationData)},
			attestationData: testAttestationData,
			assertAttestError: func(tt require.TestingT, err error, i ...interface{}) {
				assert.True(t, trace.IsBadParameter(err), "expected bad parameter err but got %v", err)
				assert.ErrorContains(t, err, "does not match the given public key")
			},
			expectAttestationData: nil,
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
			var attestationStatement *hardwarekey.AttestationStatement
			if tt.attestationData != nil {
				attestationStatement = &hardwarekey.AttestationStatement{
					AttestationStatement: &attestation.AttestationStatement_YubikeyAttestationStatement{},
				}
			}

			attestationData, err := AttestHardwareKey(ctx, tt.s, attestationStatement, tt.pub, 0)
			tt.assertAttestError(t, err)
			require.Equal(t, tt.expectAttestationData, attestationData)
		})
	}
}
