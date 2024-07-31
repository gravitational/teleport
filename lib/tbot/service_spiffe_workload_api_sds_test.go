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
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sync"
	"testing"
	"time"

	tlsv3pb "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	discoveryv3pb "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	secretv3pb "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	workloadpb "github.com/spiffe/go-spiffe/v2/proto/spiffe/workload"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/uds"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/golden"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestSDS_FetchSecrets performs a unit-test over the FetchSecrets method.
// It tests the generation of the DiscoveryResponses and that authentication
// is enforced.
func TestSDS_FetchSecrets(t *testing.T) {
	log := utils.NewSlogLoggerForTests()
	ctx := context.Background()

	td, err := spiffeid.TrustDomainFromString("example.com")
	require.NoError(t, err)

	b, _ := pem.Decode([]byte(fixtures.TLSCACertPEM))
	require.NotNil(t, b, "Decode failed")
	ca, err := x509.ParseCertificate(b.Bytes)
	require.NoError(t, err)

	uid := 100
	notUID := 200
	clientAuthenticator := func(ctx context.Context) (*slog.Logger, *uds.Creds, error) {
		return log, &uds.Creds{
			UID: uid,
		}, nil
	}

	bundle := x509bundle.New(td)
	bundle.AddX509Authority(ca)
	trustBundleGetter := func() *x509bundle.Bundle {
		return bundle
	}
	trustBundleUpdateSubscriber := func() (ch chan struct{}, unsubscribe func()) {
		return nil, func() {}
	}
	svidFetcher := func(
		ctx context.Context,
		log *slog.Logger,
		svidRequests []config.SVIDRequest,
	) ([]*workloadpb.X509SVID, error) {
		if len(svidRequests) != 2 {
			return nil, trace.BadParameter("expected 2 svids requested")
		}
		return []*workloadpb.X509SVID{
			{
				SpiffeId:    "spiffe://example.com/default",
				X509Svid:    []byte("CERT-spiffe://example.com/default"),
				X509SvidKey: []byte("KEY-spiffe://example.com/default"),
			},
			{
				SpiffeId:    "spiffe://example.com/second",
				X509Svid:    []byte("CERT-spiffe://example.com/second"),
				X509SvidKey: []byte("KEY-spiffe://example.com/second"),
			},
		}, nil
	}
	botConfig := &config.BotConfig{
		RenewalInterval: time.Minute,
	}
	cfg := &config.SPIFFEWorkloadAPIService{
		SVIDs: []config.SVIDRequestWithRules{
			{
				SVIDRequest: config.SVIDRequest{
					Path: "/default",
				},
				Rules: []config.SVIDRequestRule{
					{
						Unix: config.SVIDRequestRuleUnix{
							UID: &uid,
						},
					},
				},
			},
			{
				SVIDRequest: config.SVIDRequest{
					Path: "/second",
				},
				Rules: []config.SVIDRequestRule{
					{
						Unix: config.SVIDRequestRuleUnix{
							UID: &uid,
						},
					},
				},
			},
			{
				SVIDRequest: config.SVIDRequest{
					Path: "/not-matching",
				},
				Rules: []config.SVIDRequestRule{
					{
						Unix: config.SVIDRequestRuleUnix{
							UID: &notUID,
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name string

		svids         []config.SVIDRequestWithRules
		resourceNames []string

		wantErr string
	}{
		{
			name:          "all",
			resourceNames: []string{},
		},
		{
			name: "specific svid",
			resourceNames: []string{
				"spiffe://example.com/second",
			},
		},
		{
			name: "specific ca",
			resourceNames: []string{
				"spiffe://example.com",
			},
		},
		{
			name: "special: default",
			resourceNames: []string{
				envoyDefaultSVIDName,
			},
		},
		{
			name: "special: ROOTCA",
			resourceNames: []string{
				envoyDefaultBundleName,
			},
		},
		{
			name: "special: ALL",
			resourceNames: []string{
				envoyAllBundlesName,
			},
		},
		{
			name:          "no results",
			resourceNames: []string{"spiffe://example.com/not-matching"},
			wantErr:       "unknown resource names: [spiffe://example.com/not-matching]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sds := &spiffeSDSHandler{
				log:    log,
				cfg:    cfg,
				botCfg: botConfig,

				clientAuthenticator:         clientAuthenticator,
				trustBundleGetter:           trustBundleGetter,
				trustBundleUpdateSubscriber: trustBundleUpdateSubscriber,
				svidFetcher:                 svidFetcher,
			}

			req := &discoveryv3pb.DiscoveryRequest{
				TypeUrl:       "type.googleapis.com/envoy.extensions.transport_sockets.tls.v3.Secret",
				ResourceNames: tt.resourceNames,
			}

			res, err := sds.FetchSecrets(ctx, req)
			if tt.wantErr != "" {
				require.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			if golden.ShouldSet() {
				resBytes, err := protojson.MarshalOptions{
					Multiline: true,
				}.Marshal(res)
				require.NoError(t, err)
				golden.Set(t, resBytes)
			}

			want := &discoveryv3pb.DiscoveryResponse{}
			require.NoError(t, protojson.Unmarshal(golden.Get(t), want))
			require.Empty(t, cmp.Diff(res, want, protocmp.Transform()))
		})
	}
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
			&config.SPIFFEWorkloadAPIService{
				Listen: socketPath,
				SVIDs: []config.SVIDRequestWithRules{
					// Intentionally unmatching PID to ensure this SVID
					// is not issued.
					{
						SVIDRequest: config.SVIDRequest{
							Path: "/bar",
						},
						Rules: []config.SVIDRequestRule{
							{
								Unix: config.SVIDRequestRuleUnix{
									PID: ptr(0),
								},
							},
						},
					},
					// SVID with rule that matches on PID.
					{
						SVIDRequest: config.SVIDRequest{
							Path: "/foo",
							Hint: "hint",
							SANS: config.SVIDRequestSANs{
								DNS: []string{"example.com"},
								IP:  []string{"10.0.0.1"},
							},
						},
						Rules: []config.SVIDRequestRule{
							{
								Unix: config.SVIDRequestRuleUnix{
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

	// Spin up goroutine for bot to run in
	botCtx, cancelBot := context.WithCancel(ctx)
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := b.Run(botCtx)
		assert.NoError(t, err, "bot should not exit with error")
		cancelBot()
	}()
	t.Cleanup(func() {
		// Shut down bot and make sure it exits.
		cancelBot()
		wg.Wait()
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
		_, err = tls.X509KeyPair(tlsCertBytes, privateKeyBytes)
		require.NoError(t, err)
	}
	checkSVID(findSecret(t, resp.Resources, "spiffe://root/foo"))

	// Now check we got the CA
	caSecret := findSecret(t, resp.Resources, "spiffe://root")
	validationContext := caSecret.GetValidationContext()
	require.NotNil(t, validationContext.CustomValidatorConfig)
	require.Equal(t, envoySPIFFECertValidator, validationContext.CustomValidatorConfig.Name)
	spiffeValidatorConfig := &tlsv3pb.SPIFFECertValidatorConfig{}
	require.NoError(t, validationContext.CustomValidatorConfig.TypedConfig.UnmarshalTo(spiffeValidatorConfig))
	require.Len(t, spiffeValidatorConfig.TrustDomains, 1)
	require.Equal(t, "root", spiffeValidatorConfig.TrustDomains[0].Name)
	block, _ := pem.Decode(spiffeValidatorConfig.TrustDomains[0].TrustBundle.GetInlineBytes())
	require.Equal(t, "CERTIFICATE", block.Type)
	_, err = x509.ParseCertificate(block.Bytes)
	require.NoError(t, err)

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
