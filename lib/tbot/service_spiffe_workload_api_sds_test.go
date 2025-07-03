/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package tbot

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path"
	"path/filepath"
	"testing"
	"time"

	tlsv3pb "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	discoveryv3pb "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	secretv3pb "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/services/legacyspiffe"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/testutils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

type mockTrustBundleCache struct {
	currentBundle *workloadidentity.BundleSet
}

func (m *mockTrustBundleCache) GetBundleSet(ctx context.Context) (*workloadidentity.BundleSet, error) {
	return m.currentBundle, nil
}

// Test_E2E_SPIFFE_SDS is an end-to-end test of Workload ID's ability
// to issue a SPIFFE SVID to a workload connecting via the SDS API
func Test_E2E_SPIFFE_SDS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	t.Parallel()
	ctx := context.Background()
	log := utils.NewSlogLoggerForTests()

	// Make a new auth server.
	process := testenv.MakeTestServer(t, defaultTestServerOpts(t, log))
	rootClient := testenv.MakeDefaultAuthClient(t, process)

	// Create a role that allows the bot to issue a SPIFFE SVID.
	role, err := types.NewRole("spiffe-issuer", types.RoleSpecV6{
		Allow: types.RoleConditions{
			SPIFFE: []*types.SPIFFERoleCondition{
				{
					Path: "/*",
					DNSSANs: []string{
						"*",
					},
					IPSANs: []string{
						"0.0.0.0/0",
					},
				},
			},
		},
	})
	require.NoError(t, err)
	role, err = rootClient.UpsertRole(ctx, role)
	require.NoError(t, err)

	pid := os.Getpid()

	tempDir := t.TempDir()
	socketPath := "unix://" + path.Join(tempDir, "sock")
	onboarding, _ := makeBot(t, rootClient, "test", role.GetName())
	botConfig := defaultBotConfig(
		t, process, onboarding, config.ServiceConfigs{
			&legacyspiffe.WorkloadAPIConfig{
				Listen: socketPath,
				SVIDs: []legacyspiffe.SVIDRequestWithRules{
					// Intentionally unmatching PID to ensure this SVID
					// is not issued.
					{
						SVIDRequest: legacyspiffe.SVIDRequest{
							Path: "/bar",
						},
						Rules: []legacyspiffe.SVIDRequestRule{
							{
								Unix: legacyspiffe.SVIDRequestRuleUnix{
									PID: ptr(0),
								},
							},
						},
					},
					// SVID with rule that matches on PID.
					{
						SVIDRequest: legacyspiffe.SVIDRequest{
							Path: "/foo",
							Hint: "hint",
							SANS: legacyspiffe.SVIDRequestSANs{
								DNS: []string{"example.com"},
								IP:  []string{"10.0.0.1"},
							},
						},
						Rules: []legacyspiffe.SVIDRequestRule{
							{
								Unix: legacyspiffe.SVIDRequestRuleUnix{
									PID: &pid,
								},
							},
						},
					},
				},
			},
		},
		defaultBotConfigOpts{
			useAuthServer: true,
			insecure:      true,
		},
	)
	botConfig.Oneshot = false
	b := New(botConfig, log)

	// Run bot in the background for the remainder of the test.
	testutils.RunTestBackgroundTask(ctx, t, &testutils.TestBackgroundTask{
		Name: "bot",
		Task: b.Run,
	})

	// Wait for the socket to come up.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := os.Stat(filepath.Join(tempDir, "sock"))
		assert.NoError(t, err)
	}, 10*time.Second, 100*time.Millisecond)

	conn, err := grpc.NewClient(
		socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		conn.Close()
	})

	client := secretv3pb.NewSecretDiscoveryServiceClient(conn)
	stream, err := client.StreamSecrets(ctx)
	require.NoError(t, err)

	// Request all secrets.
	typeUrl := "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.Secret"
	err = stream.Send(&discoveryv3pb.DiscoveryRequest{
		TypeUrl:       typeUrl,
		ResourceNames: []string{},
	})
	require.NoError(t, err)

	resp, err := stream.Recv()
	require.NoError(t, err)
	assert.NotEmpty(t, resp.VersionInfo)
	assert.NotEmpty(t, resp.Nonce)
	assert.Equal(t, typeUrl, resp.TypeUrl)
	// We should expect to find two resources within the response
	assert.Len(t, resp.Resources, 2)
	// There's no specific order we should expect, so we'll need to assert that
	// each actually exists

	// First check we got our certificate...
	checkSVID := func(secret *tlsv3pb.Secret) {
		tlsCert := secret.GetTlsCertificate()
		require.NotNil(t, tlsCert)
		require.NotNil(t, tlsCert.CertificateChain)
		tlsCertBytes := tlsCert.CertificateChain.GetInlineBytes()
		require.NotEmpty(t, tlsCertBytes)
		require.NotNil(t, tlsCert.PrivateKey)
		privateKeyBytes := tlsCert.PrivateKey.GetInlineBytes()
		require.NotEmpty(t, privateKeyBytes)
		goTLSCert, err := tls.X509KeyPair(tlsCertBytes, privateKeyBytes)
		require.NoError(t, err)
		// Sanity check we generated an ECDSA key (testenv cluster uses
		// balanced-v1 algorithm suite)
		require.IsType(t, &ecdsa.PrivateKey{}, goTLSCert.PrivateKey)
	}
	checkSVID(findSecret(t, resp.Resources, "spiffe://root/foo"))

	// Now check we got the CA
	caSecret := findSecret(t, resp.Resources, "spiffe://root")
	validationContext := caSecret.GetValidationContext()
	require.NotNil(t, validationContext.CustomValidatorConfig)
	require.Equal(t, "envoy.tls.cert_validator.spiffe", validationContext.CustomValidatorConfig.Name)
	spiffeValidatorConfig := &tlsv3pb.SPIFFECertValidatorConfig{}
	require.NoError(t, validationContext.CustomValidatorConfig.TypedConfig.UnmarshalTo(spiffeValidatorConfig))
	require.Len(t, spiffeValidatorConfig.TrustDomains, 1)
	require.Equal(t, "root", spiffeValidatorConfig.TrustDomains[0].Name)
	block, _ := pem.Decode(spiffeValidatorConfig.TrustDomains[0].TrustBundle.GetInlineBytes())
	require.Equal(t, "CERTIFICATE", block.Type)
	x509Cert, err := x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)
	// Sanity check we generated an ECDSA key (testenv cluster uses balanced-v1
	// algorithm suite)
	require.IsType(t, &ecdsa.PublicKey{}, x509Cert.PublicKey)

	// We should send the response ACK we expect envoy to send.
	err = stream.Send(&discoveryv3pb.DiscoveryRequest{
		TypeUrl:       typeUrl,
		VersionInfo:   resp.VersionInfo,
		ResponseNonce: resp.Nonce,
		ResourceNames: []string{},
	})
	require.NoError(t, err)

	// Try specifying a specific resource
	err = stream.Send(&discoveryv3pb.DiscoveryRequest{
		TypeUrl: typeUrl,
		ResourceNames: []string{
			"spiffe://root/foo",
		},
		VersionInfo:   resp.VersionInfo,
		ResponseNonce: resp.Nonce,
	})
	require.NoError(t, err)

	resp, err = stream.Recv()
	require.NoError(t, err)
	assert.NotEmpty(t, resp.VersionInfo)
	assert.NotEmpty(t, resp.Nonce)
	assert.Len(t, resp.Resources, 1)
	checkSVID(findSecret(t, resp.Resources, "spiffe://root/foo"))
}

func findSecret(t *testing.T, resources []*anypb.Any, name string) *tlsv3pb.Secret {
	for _, a := range resources {
		secret := &tlsv3pb.Secret{}
		require.NoError(t, a.UnmarshalTo(secret))
		if secret.Name == name {
			return secret
		}
	}
	return nil
}
