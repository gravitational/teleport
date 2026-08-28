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

package legacyspiffe

import (
	"context"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"os"
	"path"
	"sync"
	"testing"
	"time"

	tlsv3pb "github.com/envoyproxy/go-control-plane/envoy/extensions/transport_sockets/tls/v3"
	discoveryv3pb "github.com/envoyproxy/go-control-plane/envoy/service/discovery/v3"
	secretv3pb "github.com/envoyproxy/go-control-plane/envoy/service/secret/v3"
	gocmp "github.com/google/go-cmp/cmp"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/svid/jwtsvid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/anypb"

	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/attrs"
	"github.com/gravitational/teleport/lib/tbot/workloadidentity/workloadattest"
	"github.com/gravitational/teleport/lib/utils/log/logtest"
	libtestutils "github.com/gravitational/teleport/lib/utils/testutils"
	"github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestFilterSVIDRequests(t *testing.T) {
	// This test is more for overall behavior. Use the _field test for
	// each individual field.
	ctx := context.Background()
	log := logtest.NewLogger()
	tests := []struct {
		name string
		att  *workloadidentityv1pb.WorkloadAttrs
		in   []SVIDRequestWithRules
		want []SVIDRequest
	}{
		{
			name: "no rules",
			in: []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
				},
				{
					SVIDRequest: SVIDRequest{
						Path: "/bar",
					},
				},
			},
			want: []SVIDRequest{
				{
					Path: "/foo",
				},
				{
					Path: "/bar",
				},
			},
		},
		{
			name: "no rules with attestation",
			att: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Uid:      1000,
					Gid:      1001,
					Pid:      1002,
				},
			},
			in: []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
				},
				{
					SVIDRequest: SVIDRequest{
						Path: "/bar",
					},
				},
			},
			want: []SVIDRequest{
				{
					Path: "/foo",
				},
				{
					Path: "/bar",
				},
			},
		},
		{
			name: "no rules with attestation",
			att: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					// We don't expect that workloadattest will ever return
					// Attested: false and include UID/PID/GID but we want to
					// ensure we handle this by failing regardless.
					Attested: false,
					Uid:      1000,
					Gid:      1001,
					Pid:      1002,
				},
			},
			in: []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
								UID: ptr(1000),
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "no matching rules with attestation",
			att: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Uid:      1000,
					Gid:      1001,
					Pid:      1002,
				},
			},
			in: []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
								UID: ptr(1000),
								PID: ptr(1),
							},
						},
						{
							Unix: SVIDRequestRuleUnix{
								GID: ptr(1),
							},
						},
					},
				},
				{
					SVIDRequest: SVIDRequest{
						Path: "/bar",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
								UID: ptr(1),
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "no matching rules without attestation",
			in: []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
								PID: ptr(1),
							},
						},
						{
							Unix: SVIDRequestRuleUnix{
								GID: ptr(1),
							},
						},
					},
				},
				{
					SVIDRequest: SVIDRequest{
						Path: "/bar",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
								UID: ptr(1),
							},
						},
					},
				},
			},
			want: nil,
		},
		{
			name: "some matching rules with uds",
			att: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Uid:      1000,
					Gid:      1001,
					Pid:      1002,
				},
			},
			in: []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/fizz",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
								UID: ptr(1),
							},
						},
					},
				},
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
					Rules: []SVIDRequestRule{},
				},
				{
					SVIDRequest: SVIDRequest{
						Path: "/bar",
					},
					Rules: []SVIDRequestRule{
						{
							Unix: SVIDRequestRuleUnix{
								UID: ptr(1000),
								GID: ptr(1500),
							},
						},
						{
							Unix: SVIDRequestRuleUnix{
								UID: ptr(1000),
								PID: ptr(1002),
							},
						},
					},
				},
			},
			want: []SVIDRequest{
				{
					Path: "/foo",
				},
				{
					Path: "/bar",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterSVIDRequests(ctx, log, tt.in, attrs.FromWorkloadAttrs(tt.att))
			assert.Empty(t, gocmp.Diff(tt.want, got))
		})
	}
}

func TestFilterSVIDRequests_field(t *testing.T) {
	ctx := context.Background()
	log := logtest.NewLogger()
	tests := []struct {
		field       string
		matching    *workloadidentityv1pb.WorkloadAttrs
		nonMatching *workloadidentityv1pb.WorkloadAttrs
		rule        SVIDRequestRule
	}{
		{
			field: "unix.pid",
			rule: SVIDRequestRule{
				Unix: SVIDRequestRuleUnix{
					PID: ptr(1000),
				},
			},
			matching: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Pid:      1000,
				},
			},
			nonMatching: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Pid:      200,
				},
			},
		},
		{
			field: "unix.uid",
			rule: SVIDRequestRule{
				Unix: SVIDRequestRuleUnix{
					UID: ptr(1000),
				},
			},
			matching: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Uid:      1000,
				},
			},
			nonMatching: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Uid:      200,
				},
			},
		},
		{
			field: "unix.gid",
			rule: SVIDRequestRule{
				Unix: SVIDRequestRuleUnix{
					GID: ptr(1000),
				},
			},
			matching: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Gid:      1000,
				},
			},
			nonMatching: &workloadidentityv1pb.WorkloadAttrs{
				Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
					Attested: true,
					Gid:      200,
				},
			},
		},
		{
			field: "unix.namespace",
			rule: SVIDRequestRule{
				Kubernetes: SVIDRequestRuleKubernetes{
					Namespace: "foo",
				},
			},
			matching: &workloadidentityv1pb.WorkloadAttrs{
				Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
					Attested:  true,
					Namespace: "foo",
				},
			},
			nonMatching: &workloadidentityv1pb.WorkloadAttrs{
				Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
					Attested:  true,
					Namespace: "bar",
				},
			},
		},
		{
			field: "kubernetes.service_account",
			rule: SVIDRequestRule{
				Kubernetes: SVIDRequestRuleKubernetes{
					ServiceAccount: "foo",
				},
			},
			matching: &workloadidentityv1pb.WorkloadAttrs{
				Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
					Attested:       true,
					ServiceAccount: "foo",
				},
			},
			nonMatching: &workloadidentityv1pb.WorkloadAttrs{
				Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
					Attested:       true,
					ServiceAccount: "bar",
				},
			},
		},
		{
			field: "kubernetes.pod_name",
			rule: SVIDRequestRule{
				Kubernetes: SVIDRequestRuleKubernetes{
					PodName: "foo",
				},
			},
			matching: &workloadidentityv1pb.WorkloadAttrs{
				Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
					Attested: true,
					PodName:  "foo",
				},
			},
			nonMatching: &workloadidentityv1pb.WorkloadAttrs{
				Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
					Attested: true,
					PodName:  "bar",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			rules := []SVIDRequestWithRules{
				{
					SVIDRequest: SVIDRequest{
						Path: "/foo",
					},
					Rules: []SVIDRequestRule{tt.rule},
				},
			}
			t.Run("matching", func(t *testing.T) {
				assert.Len(t, filterSVIDRequests(ctx, log, rules, attrs.FromWorkloadAttrs(tt.matching)), 1)
			})
			t.Run("non-matching", func(t *testing.T) {
				assert.Empty(t, filterSVIDRequests(ctx, log, rules, attrs.FromWorkloadAttrs(tt.nonMatching)))
			})
		})
	}
}

// TestBotSPIFFEWorkloadAPI is an end-to-end test of Workload ID's ability to
// issue a SPIFFE SVID to a workload connecting via the SPIFFE Workload API.
func TestBotSPIFFEWorkloadAPI(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	log := logtest.NewLogger()

	// Make a new auth server.
	process, err := testenv.NewTeleportProcess(t.TempDir(), defaultTestServerOpts(log))
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, process.Close())
		require.NoError(t, process.Wait())
	})
	rootClient, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rootClient.Close() })
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
	socketPath := "unix://" + path.Join(tempDir, "spiffe.sock")

	proxyAddr, err := process.ProxyWebAddr()
	require.NoError(t, err)

	connCfg := connection.Config{
		Address:     proxyAddr.Addr,
		AddressKind: connection.AddressKindProxy,
		Insecure:    true,
	}

	onboarding, _ := makeBot(t, rootClient, "test", role.GetName())

	trustBundleCache := workloadidentity.NewTrustBundleCacheFacade()

	b, err := bot.New(bot.Config{
		Connection:      connCfg,
		Logger:          log,
		Onboarding:      *onboarding,
		InternalStorage: destination.NewMemory(),
		Services: []bot.ServiceBuilder{
			trustBundleCache.Builder(),
			WorkloadAPIServiceBuilder(
				&WorkloadAPIConfig{
					Listen: socketPath,
					SVIDs: []SVIDRequestWithRules{
						// Intentionally unmatching PID to ensure this SVID
						// is not issued.
						{
							SVIDRequest: SVIDRequest{
								Path: "/bar",
							},
							Rules: []SVIDRequestRule{
								{
									Unix: SVIDRequestRuleUnix{
										PID: ptr(0),
									},
								},
							},
						},
						// SVID with rule that matches on PID.
						{
							SVIDRequest: SVIDRequest{
								Path: "/foo",
								Hint: "hint",
								SANS: SVIDRequestSANs{
									DNS: []string{"example.com"},
									IP:  []string{"10.0.0.1"},
								},
							},
							Rules: []SVIDRequestRule{
								{
									Unix: SVIDRequestRuleUnix{
										PID: &pid,
									},
								},
							},
						},
					},
					Attestors: workloadattest.Config{
						Unix: workloadattest.UnixAttestorConfig{
							BinaryHashMaxSizeBytes: workloadattest.TestBinaryHashMaxBytes,
						},
					},
				},
				trustBundleCache,
				bot.DefaultCredentialLifetime,
			),
		},
	})
	require.NoError(t, err)

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

	t.Run("X509", func(t *testing.T) {
		t.Parallel()

		// This has a little flexibility internally in terms of waiting for the
		// socket to come up, so we don't need a manual sleep/retry here.
		source, err := workloadapi.NewX509Source(
			ctx,
			workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)),
		)
		require.NoError(t, err)
		defer source.Close()

		svid, err := source.GetX509SVID()
		require.NoError(t, err)

		// SVID has successfully been issued. We can now assert that it's correct.
		require.Equal(t, "spiffe://root/foo", svid.ID.String())
		cert := svid.Certificates[0]
		require.Equal(t, "spiffe://root/foo", cert.URIs[0].String())
		require.True(t, net.IPv4(10, 0, 0, 1).Equal(cert.IPAddresses[0]))
		require.Equal(t, []string{"example.com"}, cert.DNSNames)
		require.WithinRange(
			t,
			cert.NotAfter,
			cert.NotBefore.Add(time.Hour-time.Minute),
			cert.NotBefore.Add(time.Hour+time.Minute),
		)
	})

	t.Run("JWT", func(t *testing.T) {
		t.Parallel()

		source, err := workloadapi.NewJWTSource(
			ctx,
			workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)),
		)
		require.NoError(t, err)
		defer source.Close()

		validateSVID := func(
			t *testing.T,
			svid *jwtsvid.SVID,
			wantAudience string,
		) {
			t.Helper()
			// First, check the response fields
			require.Equal(t, "spiffe://root/foo", svid.ID.String())
			require.Equal(t, "hint", svid.Hint)

			// Validate "locally" that the SVID is correct.
			validatedSVID, err := jwtsvid.ParseAndValidate(
				svid.Marshal(),
				source,
				[]string{wantAudience},
			)
			require.NoError(t, err)
			require.Equal(t, svid.Claims, validatedSVID.Claims)
			require.Equal(t, svid.ID, validatedSVID.ID)

			// Validate "remotely" that the SVID is correct using the Workload
			// API.
			validatedSVID, err = workloadapi.ValidateJWTSVID(
				ctx,
				svid.Marshal(),
				wantAudience,
				workloadapi.WithAddr(socketPath),
			)
			require.NoError(t, err)
			require.Equal(t, svid.Claims, validatedSVID.Claims)
			require.Equal(t, svid.ID, validatedSVID.ID)
		}

		svids, err := source.FetchJWTSVIDs(ctx, jwtsvid.Params{
			Audience:       "example.com",
			ExtraAudiences: []string{"2.example.com"},
			Subject:        spiffeid.RequireFromString("spiffe://root/foo"),
		})
		require.NoError(t, err)
		require.Len(t, svids, 1)
		validateSVID(t, svids[0], "2.example.com")

		// Try again with no specified subject (e.g receive all)
		svids, err = source.FetchJWTSVIDs(ctx, jwtsvid.Params{
			Audience: "example.com",
		})
		require.NoError(t, err)
		require.Len(t, svids, 1)
		validateSVID(t, svids[0], "example.com")
	})
}

// Test_E2E_SPIFFE_SDS is an end-to-end test of Workload ID's ability
// to issue a SPIFFE SVID to a workload connecting via the SDS API
func Test_E2E_SPIFFE_SDS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode")
	}
	t.Parallel()
	ctx := context.Background()
	log := logtest.NewLogger()

	// Make a new auth server.
	process, err := testenv.NewTeleportProcess(t.TempDir(), defaultTestServerOpts(log))
	require.NoError(t, err)

	rootClient, err := testenv.NewDefaultAuthClient(process)
	require.NoError(t, err)

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

	proxyAddr, err := process.ProxyWebAddr()
	require.NoError(t, err)

	connCfg := connection.Config{
		Address:     proxyAddr.Addr,
		AddressKind: connection.AddressKindProxy,
		Insecure:    true,
	}

	onboarding, _ := makeBot(t, rootClient, "test", role.GetName())

	// Create a trust bundle cache
	trustBundleCache := workloadidentity.NewTrustBundleCacheFacade()

	b, err := bot.New(bot.Config{
		Connection:      connCfg,
		Logger:          log,
		Onboarding:      *onboarding,
		InternalStorage: destination.NewMemory(),
		Services: []bot.ServiceBuilder{
			trustBundleCache.Builder(),
			WorkloadAPIServiceBuilder(
				&WorkloadAPIConfig{
					Listen: socketPath,
					SVIDs: []SVIDRequestWithRules{
						// Intentionally unmatching PID to ensure this SVID
						// is not issued.
						{
							SVIDRequest: SVIDRequest{
								Path: "/bar",
							},
							Rules: []SVIDRequestRule{
								{
									Unix: SVIDRequestRuleUnix{
										PID: ptr(0),
									},
								},
							},
						},
						// SVID with rule that matches on PID.
						{
							SVIDRequest: SVIDRequest{
								Path: "/foo",
								Hint: "hint",
								SANS: SVIDRequestSANs{
									DNS: []string{"example.com"},
									IP:  []string{"10.0.0.1"},
								},
							},
							Rules: []SVIDRequestRule{
								{
									Unix: SVIDRequestRuleUnix{
										PID: &pid,
									},
								},
							},
						},
					},
					Attestors: workloadattest.Config{
						Unix: workloadattest.UnixAttestorConfig{
							BinaryHashMaxSizeBytes: workloadattest.TestBinaryHashMaxBytes,
						},
					},
				},
				trustBundleCache,
				bot.DefaultCredentialLifetime,
			),
		},
	})
	require.NoError(t, err)

	// Run bot in the background for the remainder of the test.
	libtestutils.RunTestBackgroundTask(ctx, t, &libtestutils.TestBackgroundTask{
		Name: "bot",
		Task: func(ctx context.Context) error {
			return b.Run(ctx)
		},
	})

	// Wait for the socket to come up.
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		_, err := os.Stat(path.Join(tempDir, "sock"))
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
