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
	"crypto/x509/pkix"
	"slices"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	subcav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/subca/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/subca"
	subcaenv "github.com/gravitational/teleport/lib/subca/testenv"
	"github.com/gravitational/teleport/lib/tlsca"
)

func TestParseCAOverride_nil(t *testing.T) {
	t.Parallel()

	// See TestValidateAndParseCAOverride for additional parsing scenarios.

	got, err := subca.ParseCAOverride(nil)
	assert.NoError(t, err)
	assert.Nil(t, got)
}

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

	// Used for scenarios that test invalid certificate override certificates.
	makeInvalidCertificateFn := func(modify func(template *x509.Certificate)) func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
		return func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
			co := ca.Spec.CertificateOverrides[0]
			cert, err := tlsca.ParseCertificatePEM([]byte(co.Certificate))
			require.NoError(t, err, "ParseCertificatePEM: override certificate")

			intCA, err := env.ExternalRoot.NewIntermediateCA(&subcaenv.CAParams{
				Clock: env.Clock,
				Pub:   cert.PublicKey,
				Template: &x509.Certificate{
					Subject: pkix.Name{
						Organization: []string{env.ClusterName},
					},
				},
				ModifyCertificate: modify,
			})
			require.NoError(t, err, "NewIntermediateCA")

			co.Certificate = string(intCA.CertPEM)
			ca.Spec.CertificateOverrides = []*subcav1.CertificateOverride{co}
		}
	}

	t.Run("nil", func(t *testing.T) {
		t.Parallel()
		_, err := subca.ValidateAndParseCAOverride(nil)
		assert.ErrorContains(t, err, "ca override required")
	})

	tests := []struct {
		name         string
		modify       func(t *testing.T, ca *subcav1.CertAuthorityOverride)
		wantErr      string
		wantParseErr bool // if true ParseCAOverride should fail too.
	}{
		{
			name: "OK: Valid CA override",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				// Don't modify anything, take the default testenv override.
			},
		},
		{
			name: "OK: Minimal CA override",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.Spec = &subcav1.CertAuthorityOverrideSpec{}
			},
		},

		{
			name: "empty kind",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.Kind = ""
			},
			wantErr: "kind",
		},
		{
			name: "invalid kind",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.Kind = types.KindCertAuthority // wrong type
			},
			wantErr: "kind",
		},
		{
			name: "empty sub_kind",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.SubKind = ""
			},
			wantErr: "sub_kind",
		},
		{
			name: "invalid sub_kind",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.SubKind = string(types.DatabaseCA) // not allowed
			},
			wantErr: "sub_kind",
		},
		{
			name: "empty version",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.Version = ""
			},
			wantErr: "version",
		},
		{
			name: "invalid version",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.Version = types.V2
			},
			wantErr: "version",
		},
		{
			name: "nil metadata",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.Metadata = nil
			},
			wantErr: "metadata required",
		},
		{
			name: "empty name",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.Metadata.Name = ""
			},
			wantErr: "name/clusterName required",
		},
		{
			name: "nil spec",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.Spec = nil
			},
			wantErr: "spec required",
		},
		{
			name: "nil certificate_override",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides = append(ca.Spec.CertificateOverrides, nil)
			},
			wantErr: "nil certificate override",
		},
		{
			name: "certificate_override: empty certificate and public key (enabled)",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides[0].Certificate = ""
				ca.Spec.CertificateOverrides[0].PublicKey = ""
				ca.Spec.CertificateOverrides[0].Disabled = false
			},
			wantErr: "certificate required",
		},
		{
			name: "certificate_override: empty certificate and public key (disabled)",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides[0].Certificate = ""
				ca.Spec.CertificateOverrides[0].PublicKey = ""
				ca.Spec.CertificateOverrides[0].Disabled = true
			},
			wantErr: "certificate or public key required",
		},
		{
			name: "certificate_override: invalid certificate",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides[0].Certificate = "ceci n'est pas a certificate"
			},
			wantErr:      "expected PEM",
			wantParseErr: true,
		},
		{
			name: "certificate_override: invalid public key",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				ca.Spec.CertificateOverrides[0].PublicKey = "not a valid key"
			},
			wantErr: "invalid public key",
		},
		{
			name: "certificate_override: certificate and public key mismatch",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				// Doesn't match the Certificate field.
				ca.Spec.CertificateOverrides[0].PublicKey = unrelatedPublicKey
			},
			wantErr: "public key mismatch",
		},
		{
			name: "certificate_override: chain without certificate",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Chain = leafToRootChain
				co.Certificate = ""
				co.Disabled = true
			},
			wantErr: "chain not allowed with an empty certificate",
		},
		{
			name: "certificate_override: chain certificate invalid",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
				co.Chain = []string{
					leafToRootChain[0],
					"ceci n'est pas a certificate",
					leafToRootChain[1],
					leafToRootChain[2],
				}
			},
			wantErr:      "chain[1]: expected PEM",
			wantParseErr: true,
		},
		{
			name: "certificate_override: certificate included in chain",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
				co.Chain = append([]string{co.Certificate}, leafToRootChain...)
			},
			wantErr: "override certificate should not be included",
		},
		{
			name: "certificate_override: chain out of order",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
				co.Chain = caChain.RootToLeafPEMs() // reverse order
			},
			wantErr: "chain out of order",
		},
		{
			name: "certificate_override: chain signature invalid (forged CA)",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
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
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
				co.Chain = slices.Repeat([]string{leafToRootChain[0]}, 20)
			},
			wantErr: "chain has too many entries",
		},
		{
			name: "certificate_override: duplicate public key",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
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
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Disabled = false
			},
		},
		{
			name: "OK: Override without public key",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.PublicKey = ""
			},
		},
		{
			name: "OK: Disabled override with only public key",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Certificate = ""
			},
		},
		{
			name: "OK: Override with chain",
			modify: func(t *testing.T, ca *subcav1.CertAuthorityOverride) {
				co := ca.Spec.CertificateOverrides[0]
				co.Chain = leafToRootChain
			},
		},

		// Override certificate validation.
		{
			name: "certificate_override: missing cluster name",
			modify: makeInvalidCertificateFn(func(template *x509.Certificate) {
				template.Subject = pkix.Name{
					// Lacks "O=ClusterName" or "tlsca.CAClusterNameExtensionOID=ClusterName"
					CommonName: "Llama CA",
				}
			}),
			wantErr: "missing cluster name",
		},
		{
			name: "certificate_override: invalid cluster name (1)",
			modify: makeInvalidCertificateFn(func(template *x509.Certificate) {
				template.Subject = pkix.Name{
					Organization: []string{"not the cluster name"},
				}
			}),
			wantErr: "incorrect cluster name",
		},
		{
			name: "certificate_override: invalid cluster name (2)",
			modify: makeInvalidCertificateFn(func(template *x509.Certificate) {
				template.Subject = pkix.Name{
					// Correct, but it doesn't matter. OID takes precedence.
					Organization: []string{env.ClusterName},
					ExtraNames: []pkix.AttributeTypeAndValue{
						{
							Type:  tlsca.CAClusterNameExtensionOID,
							Value: "not the cluster name",
						},
					},
				}
			}),
			wantErr: "incorrect cluster name",
		},
		{
			name: "certificate_override: IsCA=false",
			modify: makeInvalidCertificateFn(func(template *x509.Certificate) {
				template.IsCA = false
			}),
			wantErr: "IsCA",
		},
		{
			name: "certificate_override: KeyUsage missing keyCertSign",
			modify: makeInvalidCertificateFn(func(template *x509.Certificate) {
				template.KeyUsage = x509.KeyUsageCRLSign
			}),
			wantErr: "keyCertSign",
		},
		{
			name: "certificate_override: KeyUsage missing cRLSign",
			modify: makeInvalidCertificateFn(func(template *x509.Certificate) {
				template.KeyUsage = x509.KeyUsageCertSign
			}),
			wantErr: "cRLSign",
		},
		{
			name: "certificate_override: NotBefore>NotAfter",
			modify: makeInvalidCertificateFn(func(template *x509.Certificate) {
				now := env.Clock.Now()
				template.NotBefore = now.Add(1 * time.Second)
				template.NotAfter = now
			}),
			wantErr: "NotBefore",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			caOverride := proto.Clone(sharedCAOverride).(*subcav1.CertAuthorityOverride)
			test.modify(t, caOverride)

			parsed, err := subca.ValidateAndParseCAOverride(caOverride)
			if test.wantErr != "" {
				assert.ErrorContains(t, err, test.wantErr, "ValidateAndParseCAOverride error mismatch")
				assert.ErrorAs(t, err, new(*trace.BadParameterError), "ValidateAndParseCAOverride error type mismatch")

				// Make sure parsing handles errors gracefully.
				t.Run("ParseCAOverride", func(t *testing.T) {
					_, err := subca.ParseCAOverride(caOverride)
					if test.wantParseErr {
						assert.Error(t, err, "ParseCAOverride error mismatch")
					} else {
						assert.NoError(t, err, "ParseCAOverride errored")
					}
				})
				return
			}
			require.NoError(t, err, "ValidateAndParseCAOverride errored")

			t.Run("ParseCAOverride", func(t *testing.T) {
				got, err := subca.ParseCAOverride(caOverride)
				require.NoError(t, err, "ParseCAOverride errored")
				if diff := cmp.Diff(parsed, got, protocmp.Transform()); diff != "" {
					t.Errorf("ParsedCAOverride mismatch (-want +got)\n%s", diff)
				}
			})
		})
	}
}

func TestValidateAndParseCAOverride_ParsedResource(t *testing.T) {
	t.Parallel()

	// Use a pre-made caOverride so we have predictable certificates/public keys.
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
					//PublicKey: "e96aa5dc0b1c238bb2201191f381f91d7f2d27eca890591de14998e5e15ce417",
					Certificate: "-----BEGIN CERTIFICATE-----\nMIIBxzCCAWygAwIBAgIBBzAKBggqhkjOPQQDAjAxMSMwIQYDVQQDExpFWFRFUk5B\nTCBJTlRFUk1FRElBVEUgQ0EgMzEKMAgGA1UEBRMBMzAeFw0yNjA0MDcyMDM0MzRa\nFw0yNjA0MDcyMTM1MzRaMEMxEDAOBgNVBAoTB3phcnF1b24xIzAhBgNVBAMTGkVY\nVEVSTkFMIElOVEVSTUVESUFURSBDQSA3MQowCAYDVQQFEwE3MFkwEwYHKoZIzj0C\nAQYIKoZIzj0DAQcDQgAE5xcb9chZRNgg/K/AXhMQ9GMUu1wBvPo97L107e4iQ/Dp\nc+RG8ywYGHbqSXIcJP+ONOgbDaLYeSg0Y+89k1Bgg6NjMGEwDgYDVR0PAQH/BAQD\nAgGGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFHC7cgHrQtquasktOLXLZRt3\nQizJMB8GA1UdIwQYMBaAFKauhVOH7awfXqv4WkNmyzsP00ucMAoGCCqGSM49BAMC\nA0kAMEYCIQCLbjjjwvTxATYa5FQCtvE/sg0z5n6zLEPuDUapmZ2bswIhAM6mljTl\nldbZY/y+1kC52QRpOWCTNGLlEQwTY0JN966I\n-----END CERTIFICATE-----\n",
					Chain: []string{
						"-----BEGIN CERTIFICATE-----\nMIIBtDCCAVqgAwIBAgIBAzAKBggqhkjOPQQDAjAxMSMwIQYDVQQDExpFWFRFUk5B\nTCBJTlRFUk1FRElBVEUgQ0EgMjEKMAgGA1UEBRMBMjAeFw0yNjA0MDcyMDM0MzRa\nFw0yNjA0MDcyMTM1MzRaMDExIzAhBgNVBAMTGkVYVEVSTkFMIElOVEVSTUVESUFU\nRSBDQSAzMQowCAYDVQQFEwEzMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEYq0e\nBGPQYXZzo41zQOAOosID5Y/bDR0k8aQsjuWp+GVPzXFwd3i8xRDQVTv3B7b7G8CY\nZqrrR4knVcEMV9N+vaNjMGEwDgYDVR0PAQH/BAQDAgGGMA8GA1UdEwEB/wQFMAMB\nAf8wHQYDVR0OBBYEFKauhVOH7awfXqv4WkNmyzsP00ucMB8GA1UdIwQYMBaAFJn1\nQfodXtsY3vOFfZDazgamLir8MAoGCCqGSM49BAMCA0gAMEUCIQDsLHRY/go3anuQ\ncy/r/dG2X928Aer2r33Cx+Vi9kIoYAIgDEFkGM1xoznXoHM2P6+mhMXsUBuAcekt\no5Hzu+f8MH4=\n-----END CERTIFICATE-----\n",
						"-----BEGIN CERTIFICATE-----\nMIIBrjCCAVWgAwIBAgIBAjAKBggqhkjOPQQDAjApMRswGQYDVQQDExJFWFRFUk5B\nTCBST09UIENBIDExCjAIBgNVBAUTATEwHhcNMjYwNDA3MjAzNDM0WhcNMjYwNDA3\nMjEzNTM0WjAxMSMwIQYDVQQDExpFWFRFUk5BTCBJTlRFUk1FRElBVEUgQ0EgMjEK\nMAgGA1UEBRMBMjBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABFsxbOWrOS86n597\nwSJ0Fn1kVza5rFsVtGbyrUG8Vw9gMkDAWya4jJGg6rus3ZK2Mpq0tfp23heKARAo\naJgWOIWjZjBkMA4GA1UdDwEB/wQEAwIBhjASBgNVHRMBAf8ECDAGAQH/AgEBMB0G\nA1UdDgQWBBSZ9UH6HV7bGN7zhX2Q2s4Gpi4q/DAfBgNVHSMEGDAWgBSsMuNnj6ZK\n3Cnx1+/GP/EHtVbF5zAKBggqhkjOPQQDAgNHADBEAiAKUfe9STknEx7BDIC2kPAM\nS1Buktg0sx1oFTXT5T1+kQIgDnpLGh1vpvj6uZXRbABG2Pp3XxeW5xxgZvL8vKE4\nCcU=\n-----END CERTIFICATE-----\n",
						"-----BEGIN CERTIFICATE-----\nMIIBhjCCASygAwIBAgIBATAKBggqhkjOPQQDAjApMRswGQYDVQQDExJFWFRFUk5B\nTCBST09UIENBIDExCjAIBgNVBAUTATEwHhcNMjYwNDA3MjAzNDM0WhcNMjYwNDA3\nMjEzNTM0WjApMRswGQYDVQQDExJFWFRFUk5BTCBST09UIENBIDExCjAIBgNVBAUT\nATEwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAARfTXGAS719qco6RuW/i9PIymfI\nRCGU6qKVEGDjHaFK6nlPod0vHxBL0GbcKNoiEVO+8VGyHYyIvqpGOtnBZCMWo0Uw\nQzAOBgNVHQ8BAf8EBAMCAYYwEgYDVR0TAQH/BAgwBgEB/wIBAjAdBgNVHQ4EFgQU\nrDLjZ4+mStwp8dfvxj/xB7VWxecwCgYIKoZIzj0EAwIDSAAwRQIhALlT/cKaQV8c\nPCfbWdb2RP3TZseQDLEyUkPeqVgvwKnDAiAWjntSrPBwRpaf7K1rB/PZs+4gXIJw\n3bRU1xKbDLGzkw==\n-----END CERTIFICATE-----\n",
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
	const publicKey0 = "e96aa5dc0b1c238bb2201191f381f91d7f2d27eca890591de14998e5e15ce417"
	const chain0Key0 = "36b3cc3e289c286de54fa166ddc867058fe52f7bfc33ea04572c23b867fe2011"
	const chain0Key1 = "8510a1fee7c494b6883a3d80ca8bee2d47535fb9b6b2650946c36845845adf40"
	const chain0Key2 = "6dbe15f7e4774851894b7e7732ae5d5155cbb0a25cddb61ab7cb310e5c3afae7"
	const publicKey1 = "89b32ef334d6fa94e2a7d9b3cc122115a3a9ed19907a7f25d3fe71bbc734619d" // lowercase!

	// Verify ParsedCAOverride.
	parsed, err := subca.ValidateAndParseCAOverride(caOverride)
	require.NoError(t, err)
	assert.Same(t, caOverride, parsed.CAOverride, "CAOverride")
	require.Len(t, parsed.CertificateOverrides, 2, "CertificateOverrides mismatch")

	// Verify CertificateOverride #0.
	co0 := parsed.CertificateOverrides[0]
	assert.Same(t, caOverride.Spec.CertificateOverrides[0], co0.CertificateOverride, "CertificateOverride changed")
	assert.Equal(t, publicKey0, co0.PublicKey, "PublicKey mismatch")
	assert.NotNil(t, co0.Certificate, "Certificate expected")
	assert.Equal(t, publicKey0, subca.HashCertificatePublicKey(co0.Certificate), "Certificate public key mismatch")
	assert.Len(t, co0.Chain, 3, "Chain mismatch")

	chainPublicKeys := []string{
		chain0Key0,
		chain0Key1,
		chain0Key2,
	}
	for i, chain := range co0.Chain {
		assert.NotNil(t, chain, "Chain[%d] certificate expected", i)
		assert.Equal(t, chainPublicKeys[i], subca.HashCertificatePublicKey(chain),
			"Chain[%d] certificate public key mismatch", i)
	}

	// Verify CertificateOverride #1.
	co1 := parsed.CertificateOverrides[1]
	assert.Same(t, caOverride.Spec.CertificateOverrides[1], co1.CertificateOverride, "CertificateOverride changed")
	assert.Equal(t, publicKey1, co1.PublicKey, "PublicKey mismatch")
	assert.Nil(t, co1.Certificate)
	assert.Empty(t, co1.Chain)

	t.Run("ParseCAOverride", func(t *testing.T) {
		got, err := subca.ParseCAOverride(caOverride)
		require.NoError(t, err, "ParseCAOverride errored")
		if diff := cmp.Diff(parsed, got, protocmp.Transform()); diff != "" {
			t.Errorf("ParsedCAOverride mismatch (-want +got)\n%s", diff)
		}
	})
}
