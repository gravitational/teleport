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

package subca_test

import (
	"crypto/x509"
	"slices"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/subca"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
)

func TestValidateAndParseCAOverride(t *testing.T) {
	t.Parallel()

	const caType = types.WindowsCA

	// A testenv is the simplest way to get a valid-looking CAOverride.
	env := subcaenv.New(t, subcaenv.EnvParams{
		CATypesToCreate:  []types.CertAuthType{caType},
		SkipExternalRoot: true,
	})

	// External CA chain.
	const chainLength = 3
	caChain := env.MakeCAChain(t, chainLength)
	leafToRootChain := caChain.LeafToRootPEMs()
	// Create overrides from the tip of the external chain.
	env.ExternalRoot = caChain[len(caChain)-1]

	// Cloned before every test.
	sharedCAOverride := env.NewOverrideForCAType(t, caType)

	// Used to test various public key mismatch scenarios. "Random".
	const unrelatedPublicKey = `9852b3bbc867cc047e6d894333488da322df27fa96aa20ebb29c0bf44ff6327f`

	// Forge a CA that has the correct Subject to match the override certificate,
	// but has a different set of keys.
	forgedExternalRoot, err := subcaenv.NewSelfSignedCA(&subcaenv.CAParams{
		Clock: env.Clock,
		Template: &x509.Certificate{
			Subject: env.ExternalRoot.Cert.Subject,
		},
	})
	require.NoError(t, err)

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		_, err := subca.ValidateAndParseCAOverride(nil)
		assert.ErrorContains(t, err, "ca override required")
	})

	tests := []struct {
		name    string
		modify  func(ca *subcav1.CertAuthorityOverride)
		wantErr string
	}{
		{
			name: "OK: Valid CA override",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				// Don't modify anything, take the default testenv override.
			},
		},
		{
			name: "OK: Minimal CA override",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec = &subcav1.CertAuthorityOverrideSpec{}
			},
		},

		{
			name: "empty kind",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Kind = ""
			},
			wantErr: "kind",
		},
		{
			name: "invalid kind",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Kind = types.KindCertAuthority // wrong type
			},
			wantErr: "kind",
		},
		{
			name: "empty sub_kind",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.SubKind = ""
			},
			wantErr: "sub_kind",
		},
		{
			name: "invalid sub_kind",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.SubKind = string(types.DatabaseCA) // not allowed
			},
			wantErr: "sub_kind",
		},
		{
			name: "empty version",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Version = ""
			},
			wantErr: "version",
		},
		{
			name: "invalid version",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Version = types.V2
			},
			wantErr: "version",
		},
		{
			name: "nil metadata",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Metadata = nil
			},
			wantErr: "metadata required",
		},
		{
			name: "empty name",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Metadata.Name = ""
			},
			wantErr: "name/clusterName required",
		},
		{
			name: "nil spec",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec = nil
			},
			wantErr: "spec required",
		},
		{
			name: "nil certificate_override",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides = append(ca.Spec.CertificateOverrides, nil)
			},
			wantErr: "nil certificate override",
		},
		{
			name: "certificate_override: empty certificate and public key (enabled)",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides[0].Certificate = ""
				ca.Spec.CertificateOverrides[0].PublicKey = ""
				ca.Spec.CertificateOverrides[0].Disabled = false
			},
			wantErr: "certificate required",
		},
		{
			name: "certificate_override: empty certificate and public key (disabled)",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides[0].Certificate = ""
				ca.Spec.CertificateOverrides[0].PublicKey = ""
				ca.Spec.CertificateOverrides[0].Disabled = true
			},
			wantErr: "certificate or public key required",
		},
		{
			name: "certificate_override: invalid certificate",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides[0].Certificate = "ceci n'est pas a certificate"
			},
			wantErr: "expected PEM",
		},
		{
			name: "certificate_override: invalid public key",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides[0].PublicKey = "not a valid key"
			},
			wantErr: "invalid public key",
		},
		{
			name: "certificate_override: certificate and public key mismatch",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				// Doesn't match the Certificate field.
				ca.Spec.CertificateOverrides[0].PublicKey = unrelatedPublicKey
			},
			wantErr: "public key mismatch",
		},
		{
			name: "certificate_override: chain without certificate",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Chain = leafToRootChain
				co.Certificate = ""
				co.Disabled = true
			},
			wantErr: "chain not allowed with an empty certificate",
		},
		{
			name: "certificate_override: chain certificate invalid",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
				co.Chain = []string{
					leafToRootChain[0],
					"ceci n'est pas a certificate",
					leafToRootChain[1],
					leafToRootChain[2],
				}
			},
			wantErr: "chain[1]: expected PEM",
		},
		{
			name: "certificate_override: certificate included in chain",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
				co.Chain = append([]string{co.Certificate}, leafToRootChain...)
			},
			wantErr: "override certificate should not be included",
		},
		{
			name: "certificate_override: chain out of order",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
				co.Chain = caChain.RootToLeafPEMs() // reverse order
			},
			wantErr: "chain out of order",
		},
		{
			name: "certificate_override: chain signature invalid (forged CA)",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Chain = append(
					[]string{string(forgedExternalRoot.CertPEM)},
					leafToRootChain[1:]...,
				)
			},
			wantErr: "chain signature check failed",
		},
		{
			name: "certificate_override: chain has too many entries",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
				co.Chain = slices.Repeat([]string{leafToRootChain[0]}, 20)
			},
			wantErr: "chain has too many entries",
		},
		{
			name: "certificate_override: duplicate public key",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				ca.Spec.CertificateOverrides = append(
					ca.Spec.CertificateOverrides,
					&subcav1.CertificateOverride{PublicKey: co.PublicKey, Disabled: true},
				)
			},
			wantErr: "duplicate override",
		},

		{
			name: "OK: Enabled override",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Disabled = false
			},
		},
		{
			name: "OK: Override without public key",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
			},
		},
		{
			name: "OK: Disabled override with only public key",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Certificate = ""
			},
		},
		{
			name: "OK: Override with chain",
			modify: func(ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Chain = leafToRootChain
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			caOverride := proto.Clone(sharedCAOverride).(*subcav1.CertAuthorityOverride)
			test.modify(caOverride)

			_, err := subca.ValidateAndParseCAOverride(caOverride)
			if test.wantErr == "" {
				assert.NoError(t, err, "ValidateAndParseCAOverride errored")
				return
			}

			require.ErrorContains(t, err, test.wantErr, "ValidateAndParseCAOverride error mismatch")
			assert.ErrorAs(t, err, new(*trace.BadParameterError), "ValidateAndParseCAOverride error type mismatch")
		})
	}
}

func TestValidateAndParseCAOverride_ParsedResource(t *testing.T) {
	t.Parallel()

	// Use a pre-made caOverride so we have predicatable certificates/public keys.
	caOverride := &subcav1.CertAuthorityOverride{
		Kind:    "cert_authority_override",
		SubKind: "windows",
		Version: "v1",
		Metadata: &headerv1.Metadata{
			Name: "zarquon",
		},
		Spec: &subcav1.CertAuthorityOverrideSpec{
			CertificateOverrides: []*subcav1.CertificateOverride{
				{
					//PublicKey:   "b05b24e7c4b141d8a081f49242ad859218d95ab826f23cf823c4e07138c95a9f",
					Certificate: "-----BEGIN CERTIFICATE-----\nMIIBtDCCAVqgAwIBAgIBBDAKBggqhkjOPQQDAjAxMSMwIQYDVQQDExpFWFRFUk5B\nTCBJTlRFUk1FRElBVEUgQ0EgMzEKMAgGA1UEBRMBMzAeFw0yNjAzMTIxMzA4MTZa\nFw0yNjAzMTIxNDA5MTZaMDExIzAhBgNVBAMTGkVYVEVSTkFMIElOVEVSTUVESUFU\nRSBDQSA0MQowCAYDVQQFEwE0MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE+quy\nog1ZSEDzXc3rGhRqQG98tbn2rnrf8ynmRCejQRCmfWA8fHQB1ObedzRGbnkyUvYa\nfKCJxhhHS6vhgM0xM6NjMGEwDgYDVR0PAQH/BAQDAgEGMA8GA1UdEwEB/wQFMAMB\nAf8wHQYDVR0OBBYEFJFkotRmGwBC0gKTDIGqDcAKtdeJMB8GA1UdIwQYMBaAFNuI\nI2v0B+R00TyVP+uJ5wzUhrQoMAoGCCqGSM49BAMCA0gAMEUCIQCnugqmqxDg1JnJ\nwjT8MpDMPrwPO5+IVth5E67QAq9ZIAIgcvkIy0V+k3Q180AziL7ZrLtLY2h0FRqv\nyWt+1Lnx+EY=\n-----END CERTIFICATE-----\n",
					Chain: []string{
						"-----BEGIN CERTIFICATE-----\nMIIBtTCCAVqgAwIBAgIBAzAKBggqhkjOPQQDAjAxMSMwIQYDVQQDExpFWFRFUk5B\nTCBJTlRFUk1FRElBVEUgQ0EgMjEKMAgGA1UEBRMBMjAeFw0yNjAzMTIxMzA4MTZa\nFw0yNjAzMTIxNDA5MTZaMDExIzAhBgNVBAMTGkVYVEVSTkFMIElOVEVSTUVESUFU\nRSBDQSAzMQowCAYDVQQFEwEzMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEPBu0\nRewncuGV9LP2CUEW5U5JHRMbgcL4RUNkl+gYo3WYAxFYRdjqFGRD0d7wzo2t4I+s\niT/CwF1EfNO1AAtePKNjMGEwDgYDVR0PAQH/BAQDAgEGMA8GA1UdEwEB/wQFMAMB\nAf8wHQYDVR0OBBYEFNuII2v0B+R00TyVP+uJ5wzUhrQoMB8GA1UdIwQYMBaAFKWp\nG0/k4iUwHJf1hjDBbOGgJIicMAoGCCqGSM49BAMCA0kAMEYCIQC/yLDWxElbkYJJ\nD7B7HnMNWabsJu0EDA408s+M0JcS9gIhAI1Cs+smAkb5f+UXQdBv7lqOlHFfR/3c\ni/ONL+HCRA1P\n-----END CERTIFICATE-----\n",
						"-----BEGIN CERTIFICATE-----\nMIIBrzCCAVWgAwIBAgIBAjAKBggqhkjOPQQDAjApMRswGQYDVQQDExJFWFRFUk5B\nTCBST09UIENBIDExCjAIBgNVBAUTATEwHhcNMjYwMzEyMTMwODE2WhcNMjYwMzEy\nMTQwOTE2WjAxMSMwIQYDVQQDExpFWFRFUk5BTCBJTlRFUk1FRElBVEUgQ0EgMjEK\nMAgGA1UEBRMBMjBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABHX5Lb4OjZG21k/7\nqAdDRPCNXn1G4XndwMoBL53Ni3xd+ovx4zIFajLq+8i6jg/rihxI74QBnR38j/D7\nlaPp48WjZjBkMA4GA1UdDwEB/wQEAwIBBjASBgNVHRMBAf8ECDAGAQH/AgEBMB0G\nA1UdDgQWBBSlqRtP5OIlMByX9YYwwWzhoCSInDAfBgNVHSMEGDAWgBSgEnGXl1ci\nDYWoRhm8uNXB5T3t8zAKBggqhkjOPQQDAgNIADBFAiBc1KqvASkx1PYuVjZEQr7b\nWCizRoOn3J/yLbi5DVo/TgIhAPHPm8ErDxl/AoZybbdT8QVwQYPeB1jsK+h9Kcm1\nAkDR\n-----END CERTIFICATE-----\n",
						"-----BEGIN CERTIFICATE-----\nMIIBhjCCASygAwIBAgIBATAKBggqhkjOPQQDAjApMRswGQYDVQQDExJFWFRFUk5B\nTCBST09UIENBIDExCjAIBgNVBAUTATEwHhcNMjYwMzEyMTMwODE2WhcNMjYwMzEy\nMTQwOTE2WjApMRswGQYDVQQDExJFWFRFUk5BTCBST09UIENBIDExCjAIBgNVBAUT\nATEwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAASP68oeRXs8P10r2KMP0yx1zkTk\nyUDdy9PCMMQgdDl85jMwOLcEKeOR+rwqp4FvHJVkDebQdtU4mqIweyOW1u8to0Uw\nQzAOBgNVHQ8BAf8EBAMCAQYwEgYDVR0TAQH/BAgwBgEB/wIBAjAdBgNVHQ4EFgQU\noBJxl5dXIg2FqEYZvLjVweU97fMwCgYIKoZIzj0EAwIDSAAwRQIgWXfQuNOSTrY2\nEHG41FDbIY3kB5730M7NNLm09ZuecRICIQCXwm9EH13zx462z+6eahLAoYeUdtP6\nhBW0accYgMXnmQ==\n-----END CERTIFICATE-----\n",
					},
					Disabled: true,
				},
				{
					PublicKey: "89B32EF334D6FA94E2A7D9B3CC122115A3A9ED19907A7F25D3FE71BBC734619D",
					Disabled:  true,
				},
			},
		},
	}

	// Verify ParsedCAOverride.
	parsed, err := subca.ValidateAndParseCAOverride(caOverride)
	require.NoError(t, err)
	assert.Same(t, caOverride, parsed.CAOverride, "CAOverride")
	require.Len(t, parsed.CertificateOverrides, 2, "CertificateOverrides mismatch")

	// Verify CertificateOverride #0.
	const publicKey0 = "b05b24e7c4b141d8a081f49242ad859218d95ab826f23cf823c4e07138c95a9f"
	co0 := parsed.CertificateOverrides[0]
	assert.Same(t, caOverride.Spec.CertificateOverrides[0], co0.CertificateOverride, "CertificateOverride changed")
	assert.Equal(t, publicKey0, co0.PublicKey, "PublicKey mismatch")
	assert.NotNil(t, co0.Certificate, "Certificate expected")
	assert.Equal(t, publicKey0, subca.HashCertificatePublicKey(co0.Certificate), "Certificate public key mismatch")
	assert.Len(t, co0.Chain, 3, "Chain mismatch")

	chainPublicKeys := []string{
		"6fe333ee9900316999fecc7b026b833fa97f301ed08ec545728a21c2ef04a7ba",
		"76e4355c0493658b869450c097f233ed4017ccb8eb1f7f6d251e8745b33f4336",
		"b14d63ec97168d52dee4de8f80be040441964e2b06443ce50786b60f0028ec42",
	}
	for i, chain := range co0.Chain {
		assert.NotNil(t, chain, "Chain[%d] certificate expected", i)
		assert.Equal(t, chainPublicKeys[i], subca.HashCertificatePublicKey(chain),
			"Chain[%d] certificate public key mismatch", i)
	}

	// Verify CertificateOverride #1.
	const publicKey1 = "89b32ef334d6fa94e2a7d9b3cc122115a3a9ed19907a7f25d3fe71bbc734619d" // lowercase!
	co1 := parsed.CertificateOverrides[1]
	assert.Same(t, caOverride.Spec.CertificateOverrides[1], co1.CertificateOverride, "CertificateOverride changed")
	assert.Equal(t, publicKey1, co1.PublicKey, "PublicKey mismatch")
	assert.Nil(t, co1.Certificate)
	assert.Empty(t, co1.Chain)
}
