// Teleport
// Copyright (C) 2024 Gravitational, Inc.
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

package workloadidentityv1_test

import (
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	apiproto "github.com/gravitational/teleport/api/client/proto"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/integrations/lib/testing/fakejoin"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/join"
	"github.com/gravitational/teleport/lib/auth/machineid/workloadidentityv1"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/cryptosuites"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	libjwt "github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

func newTestTLSServer(t testing.TB, opts ...auth.TestTLSServerOption) (*auth.TestTLSServer, *eventstest.MockRecorderEmitter) {
	as, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
	})
	require.NoError(t, err)

	emitter := &eventstest.MockRecorderEmitter{}
	opts = append(opts, func(config *auth.TestTLSServerConfig) {
		config.APIConfig.Emitter = emitter
	})
	srv, err := as.NewTestTLSServer(opts...)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := srv.Close()
		if errors.Is(err, net.ErrClosed) {
			return
		}
		require.NoError(t, err)
	})

	return srv, emitter
}

type issuanceTestPack struct {
	srv                     *auth.TestTLSServer
	eventRecorder           *eventstest.MockRecorderEmitter
	clock                   clockwork.Clock
	sigstorePolicyEvaluator *mockSigstorePolicyEvaluator

	issuer             string
	spiffeX509CAPool   *x509.CertPool
	spiffeJWTSigner    crypto.Signer
	spiffeJWTSignerKID string
}

func newIssuanceTestPack(t *testing.T, ctx context.Context) *issuanceTestPack {
	srv, eventRecorder := newTestTLSServer(t)
	clock := srv.Auth().GetClock()

	// Upsert a fake proxy to ensure we have a public address to use for the
	// issuer.
	proxy, err := types.NewServer("proxy", types.KindProxy, types.ServerSpecV2{
		PublicAddrs: []string{"teleport.example.com"},
	})
	require.NoError(t, err)
	err = srv.Auth().UpsertProxy(ctx, proxy)
	require.NoError(t, err)
	wantIssuer := "https://teleport.example.com/workload-identity"

	// Fetch X509 SPIFFE CA for validation of signature later
	spiffeX509CA, err := srv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: srv.ClusterName(),
	}, false)
	require.NoError(t, err)
	spiffeX509CAPool, err := services.CertPool(spiffeX509CA)
	require.NoError(t, err)
	// Fetch JWT CA to validate JWTs
	jwtCA, err := srv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: "localhost",
	}, true)
	require.NoError(t, err)
	jwtSigner, err := srv.Auth().GetKeyStore().GetJWTSigner(ctx, jwtCA)
	require.NoError(t, err)
	kid, err := libjwt.KeyID(jwtSigner.Public())
	require.NoError(t, err)

	sigstorePolicyEvaluator := newMockSigstorePolicyEvaluator(t)
	srv.Auth().SetSigstorePolicyEvaluator(sigstorePolicyEvaluator)

	return &issuanceTestPack{
		srv:                     srv,
		eventRecorder:           eventRecorder,
		clock:                   clock,
		sigstorePolicyEvaluator: sigstorePolicyEvaluator,
		issuer:                  wantIssuer,
		spiffeX509CAPool:        spiffeX509CAPool,
		spiffeJWTSigner:         jwtSigner,
		spiffeJWTSignerKID:      kid,
	}
}

func newMockSigstorePolicyEvaluator(t *testing.T) *mockSigstorePolicyEvaluator {
	t.Helper()

	eval := new(mockSigstorePolicyEvaluator)
	t.Cleanup(func() { _ = eval.AssertExpectations(t) })

	return eval
}

type mockSigstorePolicyEvaluator struct {
	mock.Mock
}

func (m *mockSigstorePolicyEvaluator) Evaluate(ctx context.Context, policyNames []string, attrs *workloadidentityv1pb.Attrs) (map[string]error, error) {
	result := m.Called(ctx, policyNames, attrs)
	return result.Get(0).(map[string]error), result.Error(1)
}

// TestIssueWorkloadIdentityE2E performs a more E2E test than the RPC specific
// tests in this package. The idea is to validate that the various Auth Server
// APIs necessary for a bot to join and then issue a workload identity are
// functioning correctly.
func TestIssueWorkloadIdentityE2E(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tp := newIssuanceTestPack(t, ctx)

	role, err := types.NewRole("my-role", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				types.NewRule(types.KindWorkloadIdentity, []string{types.VerbRead, types.VerbList}),
			},
			WorkloadIdentityLabels: map[string]apiutils.Strings{
				"my-label": []string{"my-value"},
			},
		},
	})
	require.NoError(t, err)

	wid, err := tp.srv.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "my-wid",
			Labels: map[string]string{
				"my-label": "my-value",
			},
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Rules: &workloadidentityv1pb.WorkloadIdentityRules{
				Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
					{
						Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
							{
								Attribute: "join.kubernetes.service_account.namespace",
								Operator: &workloadidentityv1pb.WorkloadIdentityCondition_Eq{
									Eq: &workloadidentityv1pb.WorkloadIdentityConditionEq{
										Value: "my-namespace",
									},
								},
							},
						},
					},
				},
			},
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: `/example/{{ user.name }}/{{ user.traits["organizational-unit"] }}/{{ join.kubernetes.service_account.namespace }}/{{ join.kubernetes.pod.name }}/{{ workload.unix.pid }}`,
			},
		},
	})
	require.NoError(t, err)

	bot := &machineidv1.Bot{
		Kind:    types.KindBot,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "my-bot",
		},
		Spec: &machineidv1.BotSpec{
			Roles: []string{
				role.GetName(),
			},
			Traits: []*machineidv1.Trait{
				{
					Name:   "organizational-unit",
					Values: []string{"finance-department"},
				},
			},
		},
	}

	k8s, err := fakejoin.NewKubernetesSigner(tp.clock)
	require.NoError(t, err)
	jwks, err := k8s.GetMarshaledJWKS()
	require.NoError(t, err)
	fakePSAT, err := k8s.SignServiceAccountJWT(
		"my-pod",
		"my-namespace",
		"my-service-account",
		tp.srv.ClusterName(),
	)
	require.NoError(t, err)

	token, err := types.NewProvisionTokenFromSpec(
		"my-k8s-token",
		time.Time{},
		types.ProvisionTokenSpecV2{
			Roles:      types.SystemRoles{types.RoleBot},
			JoinMethod: types.JoinMethodKubernetes,
			BotName:    bot.Metadata.Name,
			Kubernetes: &types.ProvisionTokenSpecV2Kubernetes{
				Type: types.KubernetesJoinTypeStaticJWKS,
				StaticJWKS: &types.ProvisionTokenSpecV2Kubernetes_StaticJWKSConfig{
					JWKS: jwks,
				},
				Allow: []*types.ProvisionTokenSpecV2Kubernetes_Rule{
					{
						ServiceAccount: "my-namespace:my-service-account",
					},
				},
			},
		},
	)
	require.NoError(t, err)

	adminClient, err := tp.srv.NewClient(auth.TestAdmin())
	require.NoError(t, err)
	_, err = adminClient.CreateRole(ctx, role)
	require.NoError(t, err)
	_, err = adminClient.BotServiceClient().CreateBot(ctx, &machineidv1.CreateBotRequest{
		Bot: bot,
	})
	require.NoError(t, err)
	err = adminClient.CreateToken(ctx, token)
	require.NoError(t, err)

	// With the basic setup complete, we can now "fake" a join.
	botCerts, err := join.Register(ctx, join.RegisterParams{
		Token:      token.GetName(),
		JoinMethod: types.JoinMethodKubernetes,
		ID: state.IdentityID{
			Role: types.RoleBot,
		},
		AuthServers: []utils.NetAddr{*utils.MustParseAddr(tp.srv.Addr().String())},
		KubernetesReadFileFunc: func(name string) ([]byte, error) {
			return []byte(fakePSAT), nil
		},
	})
	require.NoError(t, err)

	// We now have to actually impersonate the role cert to be able to issue
	// a workload identity.
	privateKeyPEM, err := keys.MarshalPrivateKey(botCerts.PrivateKey)
	require.NoError(t, err)
	tlsCert, err := tls.X509KeyPair(botCerts.Certs.TLS, privateKeyPEM)
	require.NoError(t, err)
	sshPub, err := ssh.NewPublicKey(botCerts.PrivateKey.Public())
	require.NoError(t, err)
	tlsPub, err := keys.MarshalPublicKey(botCerts.PrivateKey.Public())
	require.NoError(t, err)
	botClient := tp.srv.NewClientWithCert(tlsCert)
	certs, err := botClient.GenerateUserCerts(ctx, apiproto.UserCertsRequest{
		SSHPublicKey: ssh.MarshalAuthorizedKey(sshPub),
		TLSPublicKey: tlsPub,
		Username:     "bot-my-bot",
		RoleRequests: []string{
			role.GetName(),
		},
		UseRoleRequests: true,
		Expires:         tp.clock.Now().Add(time.Hour),
	})
	require.NoError(t, err)
	roleTLSCert, err := tls.X509KeyPair(certs.TLS, privateKeyPEM)
	require.NoError(t, err)
	roleClient := tp.srv.NewClientWithCert(roleTLSCert)

	// Generate a keypair to generate x509 SVIDs for.
	workloadKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	workloadKeyPubBytes, err := x509.MarshalPKIXPublicKey(workloadKey.Public())
	require.NoError(t, err)
	// Finally, we can request the issuance of a SVID
	c := workloadidentityv1pb.NewWorkloadIdentityIssuanceServiceClient(
		roleClient.GetConnection(),
	)
	res, err := c.IssueWorkloadIdentity(ctx, &workloadidentityv1pb.IssueWorkloadIdentityRequest{
		Name: wid.Metadata.Name,
		WorkloadAttrs: &workloadidentityv1pb.WorkloadAttrs{
			Unix: &workloadidentityv1pb.WorkloadAttrsUnix{
				Pid: 123,
			},
		},
		Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_X509SvidParams{
			X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
				PublicKey: workloadKeyPubBytes,
			},
		},
	})
	require.NoError(t, err)

	// Perform a minimal validation of the returned credential - enough to prove
	// that the returned value is a valid SVID with the SPIFFE ID we expect.
	// Other tests in this package validate this more fully.
	x509SVID := res.GetCredential().GetX509Svid()
	require.NotNil(t, x509SVID)
	cert, err := x509.ParseCertificate(x509SVID.GetCert())
	require.NoError(t, err)
	// Check included public key matches
	require.Equal(t, workloadKey.Public(), cert.PublicKey)
	require.Equal(t, "spiffe://localhost/example/bot-my-bot/finance-department/my-namespace/my-pod/123", cert.URIs[0].String())
}

func TestIssueWorkloadIdentity(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tp := newIssuanceTestPack(t, ctx)

	wildcardAccess, _, err := auth.CreateUserAndRole(
		tp.srv.Auth(),
		"dog",
		[]string{},
		[]types.Rule{
			types.NewRule(
				types.KindWorkloadIdentity,
				[]string{types.VerbRead, types.VerbList},
			),
		},
		auth.WithRoleMutator(func(role types.Role) {
			role.SetWorkloadIdentityLabels(types.Allow, types.Labels{
				types.Wildcard: []string{types.Wildcard},
			})
		}),
	)
	require.NoError(t, err)
	wilcardAccessClient, err := tp.srv.NewClient(auth.TestUser(wildcardAccess.GetName()))
	require.NoError(t, err)

	specificAccess, _, err := auth.CreateUserAndRole(
		tp.srv.Auth(),
		"cat",
		[]string{},
		[]types.Rule{
			types.NewRule(
				types.KindWorkloadIdentity,
				[]string{types.VerbRead, types.VerbList},
			),
		},
		auth.WithRoleMutator(func(role types.Role) {
			role.SetWorkloadIdentityLabels(types.Allow, types.Labels{
				"foo": []string{"bar"},
			})
		}),
	)
	require.NoError(t, err)
	specificAccessClient, err := tp.srv.NewClient(auth.TestUser(specificAccess.GetName()))
	require.NoError(t, err)

	// Generate a keypair to generate x509 SVIDs for.
	workloadKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	workloadKeyPubBytes, err := x509.MarshalPKIXPublicKey(workloadKey.Public())
	require.NoError(t, err)

	// Create some WorkloadIdentity resources
	full, err := tp.srv.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "full",
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Rules: &workloadidentityv1pb.WorkloadIdentityRules{
				Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
					{
						Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
							{
								Attribute: "user.name",
								Operator: &workloadidentityv1pb.WorkloadIdentityCondition_Eq{
									Eq: &workloadidentityv1pb.WorkloadIdentityConditionEq{
										Value: "dog",
									},
								},
							},
							{
								Attribute: "workload.kubernetes.namespace",
								Operator: &workloadidentityv1pb.WorkloadIdentityCondition_Eq{
									Eq: &workloadidentityv1pb.WorkloadIdentityConditionEq{
										Value: "default",
									},
								},
							},
						},
					},
				},
			},
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id:   "/example/{{user.name}}/{{ workload.kubernetes.namespace }}/{{ workload.kubernetes.service_account }}",
				Hint: "Wow - what a lovely hint, {{user.name}}!",
				X509: &workloadidentityv1pb.WorkloadIdentitySPIFFEX509{
					DnsSans: []string{
						"example.com",
						"{{user.name}}.example.com",
					},
				},
			},
		},
	})
	require.NoError(t, err)

	subjectTemplate, err := tp.srv.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "subject-template",
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: "/foo",
				X509: &workloadidentityv1pb.WorkloadIdentitySPIFFEX509{
					SubjectTemplate: &workloadidentityv1pb.X509DistinguishedNameTemplate{
						CommonName:         "{{user.name}}",
						Organization:       "{{user.name}} Inc",
						OrganizationalUnit: "Team {{user.name}}",
					},
				},
			},
		},
	})
	require.NoError(t, err)

	extraClaimTemplates, err := structpb.NewStruct(map[string]any{
		"user_name": "{{user.name}}",
		"k8s": map[string]any{
			"names": []any{"{{workload.kubernetes.pod_name}}"},
		},
	})
	require.NoError(t, err)

	extraClaims, err := tp.srv.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "extra-claims",
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: "/foo",
				Jwt: &workloadidentityv1pb.WorkloadIdentitySPIFFEJWT{
					ExtraClaims: extraClaimTemplates,
				},
			},
		},
	})
	require.NoError(t, err)

	modifiedMaxTTL, err := tp.srv.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "max-ttl-modified",
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: "/foo",
				X509: &workloadidentityv1pb.WorkloadIdentitySPIFFEX509{
					MaximumTtl: durationpb.New(time.Hour * 30),
				},
				Jwt: &workloadidentityv1pb.WorkloadIdentitySPIFFEJWT{
					MaximumTtl: durationpb.New(time.Minute * 15),
				},
			},
		},
	})
	require.NoError(t, err)

	sigstorePolicyRequired, err := tp.srv.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "sigstore-policy-required",
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: "/foo",
			},
			Rules: &workloadidentityv1pb.WorkloadIdentityRules{
				Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
					{Expression: `sigstore.policy_satisfied("foo") || sigstore.policy_satisfied("bar")`},
				},
			},
		},
	})
	require.NoError(t, err)

	for policy, result := range map[string]error{
		"foo": errors.New("missing artifact signature"),
		"bar": nil,
	} {
		tp.sigstorePolicyEvaluator.On("Evaluate", mock.Anything, []string{policy}, mock.Anything).
			Return(map[string]error{policy: result}, nil)
	}

	workloadAttrs := func(f func(attrs *workloadidentityv1pb.WorkloadAttrs)) *workloadidentityv1pb.WorkloadAttrs {
		attrs := &workloadidentityv1pb.WorkloadAttrs{
			Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
				Attested:       true,
				Namespace:      "default",
				PodName:        "test",
				ServiceAccount: "bar",
			},
		}
		if f != nil {
			f(attrs)
		}
		return attrs
	}
	tests := []struct {
		name       string
		client     *authclient.Client
		req        *workloadidentityv1pb.IssueWorkloadIdentityRequest
		requireErr require.ErrorAssertionFunc
		assert     func(*testing.T, *workloadidentityv1pb.IssueWorkloadIdentityResponse)
	}{
		{
			name:   "jwt svid",
			client: wilcardAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: full.GetMetadata().GetName(),
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_JwtSvidParams{
					JwtSvidParams: &workloadidentityv1pb.JWTSVIDParams{
						Audiences: []string{"example.com", "test.example.com"},
					},
				},
				RequestedTtl:  durationpb.New(time.Minute * 14),
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentityResponse) {
				cred := res.Credential
				require.NotNil(t, res.Credential)

				wantTTL := time.Minute * 14
				wantSPIFFEID := "spiffe://localhost/example/dog/default/bar"
				require.Empty(t, cmp.Diff(
					cred,
					&workloadidentityv1pb.Credential{
						Ttl:                      durationpb.New(wantTTL),
						SpiffeId:                 wantSPIFFEID,
						Hint:                     "Wow - what a lovely hint, dog!",
						WorkloadIdentityName:     full.GetMetadata().GetName(),
						WorkloadIdentityRevision: full.GetMetadata().GetRevision(),
					},
					protocmp.Transform(),
					protocmp.IgnoreFields(
						&workloadidentityv1pb.Credential{},
						"expires_at",
					),
					protocmp.IgnoreOneofs(
						&workloadidentityv1pb.Credential{},
						"credential",
					),
				))
				// Check expiry makes sense
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cred.GetExpiresAt().AsTime(), time.Second)

				// Check the JWT
				parsed, err := jwt.ParseSigned(cred.GetJwtSvid().GetJwt())
				require.NoError(t, err)

				claims := jwt.Claims{}
				err = parsed.Claims(tp.spiffeJWTSigner.Public(), &claims)
				require.NoError(t, err)
				// Check headers
				require.Len(t, parsed.Headers, 1)
				require.Equal(t, tp.spiffeJWTSignerKID, parsed.Headers[0].KeyID)
				// Check claims
				require.Equal(t, wantSPIFFEID, claims.Subject)
				require.NotEmpty(t, claims.ID)
				require.Equal(t, jwt.Audience{"example.com", "test.example.com"}, claims.Audience)
				require.Equal(t, tp.issuer, claims.Issuer)
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), claims.Expiry.Time(), 5*time.Second)
				require.WithinDuration(t, tp.clock.Now(), claims.IssuedAt.Time(), 5*time.Second)

				// Check audit log event
				evt, ok := tp.eventRecorder.LastEvent().(*events.SPIFFESVIDIssued)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Equal(t, claims.ID, evt.JTI)
				require.Equal(t, claims.ID, cred.GetJwtSvid().GetJti())
				require.Empty(t, cmp.Diff(
					evt,
					&events.SPIFFESVIDIssued{
						Metadata: events.Metadata{
							Type: libevents.SPIFFESVIDIssuedEvent,
							Code: libevents.SPIFFESVIDIssuedSuccessCode,
						},
						UserMetadata: events.UserMetadata{
							User:     wildcardAccess.GetName(),
							UserKind: events.UserKind_USER_KIND_HUMAN,
						},
						SPIFFEID:                 "spiffe://localhost/example/dog/default/bar",
						SVIDType:                 "jwt",
						Hint:                     "Wow - what a lovely hint, dog!",
						WorkloadIdentity:         full.GetMetadata().GetName(),
						WorkloadIdentityRevision: full.GetMetadata().GetRevision(),
					},
					cmpopts.IgnoreFields(
						events.SPIFFESVIDIssued{},
						"ConnectionMetadata",
						"JTI",
						"Attributes",
					),
				))
			},
		},
		{
			name:   "jwt svid - extra claims",
			client: wilcardAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: extraClaims.GetMetadata().GetName(),
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_JwtSvidParams{
					JwtSvidParams: &workloadidentityv1pb.JWTSVIDParams{
						Audiences: []string{"example.com"},
					},
				},
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentityResponse) {
				parsed, err := jwt.ParseSigned(res.GetCredential().GetJwtSvid().GetJwt())
				require.NoError(t, err)

				var claims struct {
					UserName string `json:"user_name"`
					K8s      struct {
						Names []string `json:"names"`
					} `json:"k8s"`
				}
				err = parsed.Claims(tp.spiffeJWTSigner.Public(), &claims)
				require.NoError(t, err)
				require.Equal(t, "dog", claims.UserName)
				require.Equal(t, []string{"test"}, claims.K8s.Names)
			},
		},
		{
			name:   "x509 svid",
			client: wilcardAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: full.GetMetadata().GetName(),
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_X509SvidParams{
					X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
						PublicKey: workloadKeyPubBytes,
					},
				},
				WorkloadAttrs: workloadAttrs(nil),
				RequestedTtl:  durationpb.New(time.Hour * 2),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentityResponse) {
				cred := res.Credential
				require.NotNil(t, res.Credential)

				wantSPIFFEID := "spiffe://localhost/example/dog/default/bar"
				wantTTL := time.Hour * 2
				require.Empty(t, cmp.Diff(
					cred,
					&workloadidentityv1pb.Credential{
						Ttl:                      durationpb.New(wantTTL),
						SpiffeId:                 wantSPIFFEID,
						Hint:                     "Wow - what a lovely hint, dog!",
						WorkloadIdentityName:     full.GetMetadata().GetName(),
						WorkloadIdentityRevision: full.GetMetadata().GetRevision(),
					},
					protocmp.Transform(),
					protocmp.IgnoreFields(
						&workloadidentityv1pb.Credential{},
						"expires_at",
					),
					protocmp.IgnoreOneofs(
						&workloadidentityv1pb.Credential{},
						"credential",
					),
				))
				// Check expiry makes sense
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cred.GetExpiresAt().AsTime(), time.Second)

				// Check the X509
				cert, err := x509.ParseCertificate(cred.GetX509Svid().GetCert())
				require.NoError(t, err)
				// Check included public key matches
				require.Equal(t, workloadKey.Public(), cert.PublicKey)
				// Check cert expiry
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cert.NotAfter, time.Second)
				// Check cert nbf
				require.WithinDuration(t, tp.clock.Now().Add(-1*time.Minute), cert.NotBefore, time.Second)
				// Check cert TTL
				require.Equal(t, cert.NotAfter.Sub(cert.NotBefore), wantTTL+time.Minute)
				require.Equal(t, []string{"example.com", "dog.example.com"}, cert.DNSNames)

				// Check against SPIFFE SPEC
				// References are to https://github.com/spiffe/spiffe/blob/main/standards/X509-SVID.md
				// 2: An X.509 SVID MUST contain exactly one URI SAN, and by extension, exactly one SPIFFE ID
				require.Len(t, cert.URIs, 1)
				require.Equal(t, wantSPIFFEID, cert.URIs[0].String())
				// 4.1: leaf certificates MUST set the cA field to false.
				require.False(t, cert.IsCA)
				require.Greater(t, cert.KeyUsage&x509.KeyUsageDigitalSignature, 0)
				// 4.3: They MAY set keyEncipherment and/or keyAgreement
				require.Greater(t, cert.KeyUsage&x509.KeyUsageKeyEncipherment, 0)
				require.Greater(t, cert.KeyUsage&x509.KeyUsageKeyAgreement, 0)
				// 4.3: Leaf SVIDs MUST NOT set keyCertSign or cRLSign
				require.EqualValues(t, 0, cert.KeyUsage&x509.KeyUsageCertSign)
				require.EqualValues(t, 0, cert.KeyUsage&x509.KeyUsageCRLSign)
				// 4.4: When included, fields id-kp-serverAuth and id-kp-clientAuth MUST be set.
				require.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
				require.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)
				// Expect blank subject field.
				require.Equal(t, pkix.Name{}, cert.Subject)

				// Check cert signature is valid
				_, err = cert.Verify(x509.VerifyOptions{
					Roots:       tp.spiffeX509CAPool,
					CurrentTime: tp.srv.Auth().GetClock().Now(),
				})
				require.NoError(t, err)

				// Check audit log event
				evt, ok := tp.eventRecorder.LastEvent().(*events.SPIFFESVIDIssued)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Equal(t, cred.GetX509Svid().GetSerialNumber(), evt.SerialNumber)
				require.Empty(t, cmp.Diff(
					evt,
					&events.SPIFFESVIDIssued{
						Metadata: events.Metadata{
							Type: libevents.SPIFFESVIDIssuedEvent,
							Code: libevents.SPIFFESVIDIssuedSuccessCode,
						},
						UserMetadata: events.UserMetadata{
							User:     wildcardAccess.GetName(),
							UserKind: events.UserKind_USER_KIND_HUMAN,
						},
						SPIFFEID:                 "spiffe://localhost/example/dog/default/bar",
						SVIDType:                 "x509",
						Hint:                     "Wow - what a lovely hint, dog!",
						WorkloadIdentity:         full.GetMetadata().GetName(),
						WorkloadIdentityRevision: full.GetMetadata().GetRevision(),
						DNSSANs: []string{
							"example.com",
							"dog.example.com",
						},
					},
					cmpopts.IgnoreFields(
						events.SPIFFESVIDIssued{},
						"ConnectionMetadata",
						"SerialNumber",
						"Attributes",
					),
				))
			},
		},
		{
			name:   "x509 svid - subject templating",
			client: wilcardAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: subjectTemplate.GetMetadata().GetName(),
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_X509SvidParams{
					X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
						PublicKey: workloadKeyPubBytes,
					},
				},
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentityResponse) {
				cred := res.Credential
				require.NotNil(t, res.Credential)

				wantSPIFFEID := "spiffe://localhost/foo"
				wantTTL := time.Hour
				require.Empty(t, cmp.Diff(
					cred,
					&workloadidentityv1pb.Credential{
						Ttl:                      durationpb.New(wantTTL),
						SpiffeId:                 wantSPIFFEID,
						WorkloadIdentityName:     subjectTemplate.GetMetadata().GetName(),
						WorkloadIdentityRevision: subjectTemplate.GetMetadata().GetRevision(),
					},
					protocmp.Transform(),
					protocmp.IgnoreFields(
						&workloadidentityv1pb.Credential{},
						"expires_at",
					),
					protocmp.IgnoreOneofs(
						&workloadidentityv1pb.Credential{},
						"credential",
					),
				))
				// Check expiry makes sense
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cred.GetExpiresAt().AsTime(), time.Second)

				// Check the X509
				cert, err := x509.ParseCertificate(cred.GetX509Svid().GetCert())
				require.NoError(t, err)
				// Check included public key matches
				require.Equal(t, workloadKey.Public(), cert.PublicKey)
				// Check cert expiry
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cert.NotAfter, time.Second)
				// Check cert nbf
				require.WithinDuration(t, tp.clock.Now().Add(-1*time.Minute), cert.NotBefore, time.Second)
				// Check cert TTL
				require.Equal(t, cert.NotAfter.Sub(cert.NotBefore), wantTTL+time.Minute)

				// Check against SPIFFE SPEC
				// References are to https://github.com/spiffe/spiffe/blob/main/standards/X509-SVID.md
				// 2: An X.509 SVID MUST contain exactly one URI SAN, and by extension, exactly one SPIFFE ID
				require.Len(t, cert.URIs, 1)
				require.Equal(t, wantSPIFFEID, cert.URIs[0].String())
				// 4.1: leaf certificates MUST set the cA field to false.
				require.False(t, cert.IsCA)
				require.Greater(t, cert.KeyUsage&x509.KeyUsageDigitalSignature, 0)
				// 4.3: They MAY set keyEncipherment and/or keyAgreement
				require.Greater(t, cert.KeyUsage&x509.KeyUsageKeyEncipherment, 0)
				require.Greater(t, cert.KeyUsage&x509.KeyUsageKeyAgreement, 0)
				// 4.3: Leaf SVIDs MUST NOT set keyCertSign or cRLSign
				require.EqualValues(t, 0, cert.KeyUsage&x509.KeyUsageCertSign)
				require.EqualValues(t, 0, cert.KeyUsage&x509.KeyUsageCRLSign)
				// 4.4: When included, fields id-kp-serverAuth and id-kp-clientAuth MUST be set.
				require.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageServerAuth)
				require.Contains(t, cert.ExtKeyUsage, x509.ExtKeyUsageClientAuth)

				// Check subject has been templated
				require.Empty(t, cmp.Diff(pkix.Name{
					CommonName:         "dog",
					Organization:       []string{"dog Inc"},
					OrganizationalUnit: []string{"Team dog"},
				}, cert.Subject, cmpopts.IgnoreFields(pkix.Name{}, "Names")))

				// Check cert signature is valid
				_, err = cert.Verify(x509.VerifyOptions{
					Roots:       tp.spiffeX509CAPool,
					CurrentTime: tp.srv.Auth().GetClock().Now(),
				})
				require.NoError(t, err)

				// Check audit log event
				evt, ok := tp.eventRecorder.LastEvent().(*events.SPIFFESVIDIssued)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Equal(t, cred.GetX509Svid().GetSerialNumber(), evt.SerialNumber)
				require.Empty(t, cmp.Diff(
					evt,
					&events.SPIFFESVIDIssued{
						Metadata: events.Metadata{
							Type: libevents.SPIFFESVIDIssuedEvent,
							Code: libevents.SPIFFESVIDIssuedSuccessCode,
						},
						UserMetadata: events.UserMetadata{
							User:     wildcardAccess.GetName(),
							UserKind: events.UserKind_USER_KIND_HUMAN,
						},
						SPIFFEID:                 "spiffe://localhost/foo",
						SVIDType:                 "x509",
						WorkloadIdentity:         subjectTemplate.GetMetadata().GetName(),
						WorkloadIdentityRevision: subjectTemplate.GetMetadata().GetRevision(),
					},
					cmpopts.IgnoreFields(
						events.SPIFFESVIDIssued{},
						"ConnectionMetadata",
						"SerialNumber",
						"Attributes",
					),
				))
			},
		},
		{
			name:   "x509 svid - ttl limited by default max",
			client: wilcardAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: subjectTemplate.GetMetadata().GetName(),
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_X509SvidParams{
					X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
						PublicKey: workloadKeyPubBytes,
					},
				},
				RequestedTtl:  durationpb.New(time.Hour * 32),
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentityResponse) {
				cred := res.Credential
				require.NotNil(t, res.Credential)
				wantTTL := time.Hour * 24
				// Check expiry makes sense
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cred.GetExpiresAt().AsTime(), time.Second)
				// Check the X509
				cert, err := x509.ParseCertificate(cred.GetX509Svid().GetCert())
				require.NoError(t, err)
				// Check cert expiry
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cert.NotAfter, time.Second)
				// Check cert nbf
				require.WithinDuration(t, tp.clock.Now().Add(-1*time.Minute), cert.NotBefore, time.Second)
				// Check cert TTL
				require.Equal(t, cert.NotAfter.Sub(cert.NotBefore), wantTTL+time.Minute)
			},
		},
		{
			name:   "x509 svid - unspecified ttl",
			client: wilcardAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: subjectTemplate.GetMetadata().GetName(),
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_X509SvidParams{
					X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
						PublicKey: workloadKeyPubBytes,
					},
				},
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentityResponse) {
				cred := res.Credential
				require.NotNil(t, res.Credential)
				wantTTL := time.Hour
				// Check expiry makes sense
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cred.GetExpiresAt().AsTime(), time.Second)
				// Check the X509
				cert, err := x509.ParseCertificate(cred.GetX509Svid().GetCert())
				require.NoError(t, err)
				// Check cert expiry
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cert.NotAfter, time.Second)
				// Check cert nbf
				require.WithinDuration(t, tp.clock.Now().Add(-1*time.Minute), cert.NotBefore, time.Second)
				// Check cert TTL
				require.Equal(t, cert.NotAfter.Sub(cert.NotBefore), wantTTL+time.Minute)
			},
		},
		{
			name:   "x509 svid - ttl limited by configured limit",
			client: wilcardAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: modifiedMaxTTL.GetMetadata().GetName(),
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_X509SvidParams{
					X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
						PublicKey: workloadKeyPubBytes,
					},
				},
				RequestedTtl:  durationpb.New(time.Hour * 32),
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentityResponse) {
				cred := res.Credential
				require.NotNil(t, res.Credential)
				wantTTL := time.Hour * 30
				// Check expiry makes sense
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cred.GetExpiresAt().AsTime(), time.Second)
				// Check the X509
				cert, err := x509.ParseCertificate(cred.GetX509Svid().GetCert())
				require.NoError(t, err)
				// Check cert expiry
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cert.NotAfter, time.Second)
				// Check cert nbf
				require.WithinDuration(t, tp.clock.Now().Add(-1*time.Minute), cert.NotBefore, time.Second)
				// Check cert TTL
				require.Equal(t, cert.NotAfter.Sub(cert.NotBefore), wantTTL+time.Minute)
			},
		},
		{
			name:   "x509 svid - ok ttl between default and configured limit",
			client: wilcardAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: modifiedMaxTTL.GetMetadata().GetName(),
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_X509SvidParams{
					X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
						PublicKey: workloadKeyPubBytes,
					},
				},
				RequestedTtl:  durationpb.New(time.Hour * 28),
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentityResponse) {
				cred := res.Credential
				require.NotNil(t, res.Credential)
				wantTTL := time.Hour * 28
				// Check expiry makes sense
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cred.GetExpiresAt().AsTime(), time.Second)
				// Check the X509
				cert, err := x509.ParseCertificate(cred.GetX509Svid().GetCert())
				require.NoError(t, err)
				// Check cert expiry
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cert.NotAfter, time.Second)
				// Check cert nbf
				require.WithinDuration(t, tp.clock.Now().Add(-1*time.Minute), cert.NotBefore, time.Second)
				// Check cert TTL
				require.Equal(t, cert.NotAfter.Sub(cert.NotBefore), wantTTL+time.Minute)
			},
		},
		{
			name:   "jwt svid ttl exceeds max default",
			client: wilcardAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: full.GetMetadata().GetName(),
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_JwtSvidParams{
					JwtSvidParams: &workloadidentityv1pb.JWTSVIDParams{
						Audiences: []string{"example.com", "test.example.com"},
					},
				},
				RequestedTtl:  durationpb.New(time.Hour * 30),
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentityResponse) {
				cred := res.Credential
				require.NotNil(t, res.Credential)

				wantTTL := time.Hour * 24
				// Check expiry makes sense
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cred.GetExpiresAt().AsTime(), time.Second)
				parsed, err := jwt.ParseSigned(cred.GetJwtSvid().GetJwt())
				require.NoError(t, err)
				claims := jwt.Claims{}
				err = parsed.Claims(tp.spiffeJWTSigner.Public(), &claims)
				require.NoError(t, err)
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), claims.Expiry.Time(), 5*time.Second)
				require.WithinDuration(t, tp.clock.Now(), claims.IssuedAt.Time(), 5*time.Second)
			},
		},
		{
			name:   "jwt svid ttl exceeds configured default",
			client: wilcardAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: modifiedMaxTTL.GetMetadata().GetName(),
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_JwtSvidParams{
					JwtSvidParams: &workloadidentityv1pb.JWTSVIDParams{
						Audiences: []string{"example.com", "test.example.com"},
					},
				},
				RequestedTtl:  durationpb.New(time.Hour * 14),
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentityResponse) {
				cred := res.Credential
				require.NotNil(t, res.Credential)

				wantTTL := time.Minute * 15
				// Check expiry makes sense
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), cred.GetExpiresAt().AsTime(), time.Second)
				parsed, err := jwt.ParseSigned(cred.GetJwtSvid().GetJwt())
				require.NoError(t, err)
				claims := jwt.Claims{}
				err = parsed.Claims(tp.spiffeJWTSigner.Public(), &claims)
				require.NoError(t, err)
				require.WithinDuration(t, tp.clock.Now().Add(wantTTL), claims.Expiry.Time(), 5*time.Second)
				require.WithinDuration(t, tp.clock.Now(), claims.IssuedAt.Time(), 5*time.Second)
			},
		},
		{
			name:   "sigstore policy required",
			client: wilcardAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: sigstorePolicyRequired.GetMetadata().GetName(),
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_JwtSvidParams{
					JwtSvidParams: &workloadidentityv1pb.JWTSVIDParams{
						Audiences: []string{"example.com", "test.example.com"},
					},
				},
				WorkloadAttrs: func() *workloadidentityv1pb.WorkloadAttrs {
					attrs := workloadAttrs(nil)
					attrs.Sigstore = &workloadidentityv1pb.WorkloadAttrsSigstore{
						Payloads: []*workloadidentityv1pb.SigstoreVerificationPayload{
							{Bundle: []byte(`bundle`)},
						},
					}
					return attrs
				}(),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentityResponse) {
				require.NotNil(t, res.Credential)

				evt, ok := tp.eventRecorder.LastEvent().(*events.SPIFFESVIDIssued)
				require.True(t, ok)

				attrsJSON, err := evt.Attributes.MarshalJSON()
				require.NoError(t, err)

				attrs := make(map[string]any)
				require.NoError(t, json.Unmarshal(attrsJSON, &attrs))

				sigstoreAttrs := attrs["workload"].(map[string]any)["sigstore"]
				require.Empty(t, cmp.Diff(map[string]any{
					"payload_count": float64(1),
					"evaluated_policies": map[string]any{
						"bar": map[string]any{"satisfied": true},
						"foo": map[string]any{"reason": "missing artifact signature", "satisfied": false},
					},
				}, sigstoreAttrs))
			},
		},
		{
			name:   "unauthorized by rules",
			client: wilcardAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: full.GetMetadata().GetName(),
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_JwtSvidParams{
					JwtSvidParams: &workloadidentityv1pb.JWTSVIDParams{
						Audiences: []string{"example.com", "test.example.com"},
					},
				},
				WorkloadAttrs: workloadAttrs(func(attrs *workloadidentityv1pb.WorkloadAttrs) {
					attrs.Kubernetes.Namespace = "not-default"
				}),
			},
			requireErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:   "unauthorized by labels",
			client: specificAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: full.GetMetadata().GetName(),
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_JwtSvidParams{
					JwtSvidParams: &workloadidentityv1pb.JWTSVIDParams{
						Audiences: []string{"example.com", "test.example.com"},
					},
				},
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:   "does not exist",
			client: specificAccessClient,
			req: &workloadidentityv1pb.IssueWorkloadIdentityRequest{
				Name: "does-not-exist",
				Credential: &workloadidentityv1pb.IssueWorkloadIdentityRequest_JwtSvidParams{
					JwtSvidParams: &workloadidentityv1pb.JWTSVIDParams{
						Audiences: []string{"example.com", "test.example.com"},
					},
				},
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp.eventRecorder.Reset()
			c := workloadidentityv1pb.NewWorkloadIdentityIssuanceServiceClient(
				tt.client.GetConnection(),
			)
			res, err := c.IssueWorkloadIdentity(ctx, tt.req)
			tt.requireErr(t, err)
			if tt.assert != nil {
				tt.assert(t, res)
			}
		})
	}
}

func TestIssueWorkloadIdentities(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tp := newIssuanceTestPack(t, ctx)

	user, _, err := auth.CreateUserAndRole(
		tp.srv.Auth(),
		"cat",
		[]string{},
		[]types.Rule{
			types.NewRule(
				types.KindWorkloadIdentity,
				[]string{types.VerbRead, types.VerbList},
			),
		},
		auth.WithRoleMutator(func(role types.Role) {
			role.SetWorkloadIdentityLabels(types.Allow, types.Labels{
				"access": []string{"yes"},
			})
		}),
	)
	require.NoError(t, err)
	client, err := tp.srv.NewClient(auth.TestUser(user.GetName()))
	require.NoError(t, err)

	// Generate a keypair to generate x509 SVIDs for.
	workloadKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	workloadKeyPubBytes, err := x509.MarshalPKIXPublicKey(workloadKey.Public())
	require.NoError(t, err)

	// Create some WorkloadIdentity resources
	_, err = tp.srv.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "bar-labeled",
			Labels: map[string]string{
				"foo":    "bar",
				"access": "yes",
			},
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Rules: &workloadidentityv1pb.WorkloadIdentityRules{
				Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
					{
						Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
							{
								Attribute: "workload.kubernetes.namespace",
								Operator: &workloadidentityv1pb.WorkloadIdentityCondition_Eq{
									Eq: &workloadidentityv1pb.WorkloadIdentityConditionEq{
										Value: "default",
									},
								},
							},
						},
					},
				},
			},
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id:   "/example/{{user.name}}/{{ workload.kubernetes.namespace }}/{{ workload.kubernetes.service_account }}",
				Hint: "Wow - what a lovely hint, {{user.name}}!",
			},
		},
	})
	require.NoError(t, err)
	_, err = tp.srv.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "buzz-labeled",
			Labels: map[string]string{
				"foo":    "buzz",
				"access": "yes",
			},
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Rules: &workloadidentityv1pb.WorkloadIdentityRules{
				Allow: []*workloadidentityv1pb.WorkloadIdentityRule{
					{
						Conditions: []*workloadidentityv1pb.WorkloadIdentityCondition{
							{
								Attribute: "workload.kubernetes.namespace",
								Operator: &workloadidentityv1pb.WorkloadIdentityCondition_Eq{
									Eq: &workloadidentityv1pb.WorkloadIdentityConditionEq{
										Value: "default",
									},
								},
							},
						},
					},
				},
			},
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id:   "/example/{{user.name}}/{{ workload.kubernetes.namespace }}/{{ workload.kubernetes.service_account }}",
				Hint: "Wow - what a lovely hint, {{user.name}}!",
			},
		},
	})
	require.NoError(t, err)

	_, err = tp.srv.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
		Kind:    types.KindWorkloadIdentity,
		Version: types.V1,
		Metadata: &headerv1.Metadata{
			Name: "inaccessible",
			Labels: map[string]string{
				"foo":    "bar",
				"access": "no",
			},
		},
		Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
			Rules: &workloadidentityv1pb.WorkloadIdentityRules{},
			Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
				Id: "/example",
			},
		},
	})
	require.NoError(t, err)

	// Make enough to trip the "too many" error
	for i := range 12 {
		_, err := tp.srv.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: fmt.Sprintf("%d", i),
				Labels: map[string]string{
					"error":  "too-many",
					"access": "yes",
				},
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Rules: &workloadidentityv1pb.WorkloadIdentityRules{},
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/exampled",
				},
			},
		})
		require.NoError(t, err)
	}

	workloadAttrs := func(f func(attrs *workloadidentityv1pb.WorkloadAttrs)) *workloadidentityv1pb.WorkloadAttrs {
		attrs := &workloadidentityv1pb.WorkloadAttrs{
			Kubernetes: &workloadidentityv1pb.WorkloadAttrsKubernetes{
				Attested:       true,
				Namespace:      "default",
				PodName:        "test",
				ServiceAccount: "bar",
			},
		}
		if f != nil {
			f(attrs)
		}
		return attrs
	}
	tests := []struct {
		name       string
		client     *authclient.Client
		req        *workloadidentityv1pb.IssueWorkloadIdentitiesRequest
		requireErr require.ErrorAssertionFunc
		assert     func(*testing.T, *workloadidentityv1pb.IssueWorkloadIdentitiesResponse)
	}{
		{
			name:   "jwt svid",
			client: client,
			req: &workloadidentityv1pb.IssueWorkloadIdentitiesRequest{
				LabelSelectors: []*workloadidentityv1pb.LabelSelector{
					{
						Key:    "foo",
						Values: []string{"bar", "buzz"},
					},
				},
				Credential: &workloadidentityv1pb.IssueWorkloadIdentitiesRequest_JwtSvidParams{
					JwtSvidParams: &workloadidentityv1pb.JWTSVIDParams{
						Audiences: []string{"example.com", "test.example.com"},
					},
				},
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentitiesResponse) {
				workloadIdentitiesIssued := []string{}
				for _, cred := range res.Credentials {
					workloadIdentitiesIssued = append(workloadIdentitiesIssued, cred.WorkloadIdentityName)

					// Check a credential was actually included and is valid.
					parsed, err := jwt.ParseSigned(cred.GetJwtSvid().GetJwt())
					require.NoError(t, err)
					claims := jwt.Claims{}
					err = parsed.Claims(tp.spiffeJWTSigner.Public(), &claims)
					require.NoError(t, err)
				}
				require.EqualValues(t, []string{"bar-labeled", "buzz-labeled"}, workloadIdentitiesIssued)
			},
		},
		{
			name:   "x509 svid",
			client: client,
			req: &workloadidentityv1pb.IssueWorkloadIdentitiesRequest{
				LabelSelectors: []*workloadidentityv1pb.LabelSelector{
					{
						Key:    "foo",
						Values: []string{"bar", "buzz"},
					},
				},
				Credential: &workloadidentityv1pb.IssueWorkloadIdentitiesRequest_X509SvidParams{
					X509SvidParams: &workloadidentityv1pb.X509SVIDParams{
						PublicKey: workloadKeyPubBytes,
					},
				},
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentitiesResponse) {
				workloadIdentitiesIssued := []string{}
				for _, cred := range res.Credentials {
					workloadIdentitiesIssued = append(workloadIdentitiesIssued, cred.WorkloadIdentityName)
					// Check X509 cert actually included and signed.
					cert, err := x509.ParseCertificate(cred.GetX509Svid().GetCert())
					require.NoError(t, err)
					// Check included public key matches
					require.Equal(t, workloadKey.Public(), cert.PublicKey)
					_, err = cert.Verify(x509.VerifyOptions{
						Roots:       tp.spiffeX509CAPool,
						CurrentTime: tp.srv.Auth().GetClock().Now(),
					})
					require.NoError(t, err)
				}
				require.EqualValues(t, []string{"bar-labeled", "buzz-labeled"}, workloadIdentitiesIssued)
			},
		},
		{
			name:   "rules prevent issuing",
			client: client,
			req: &workloadidentityv1pb.IssueWorkloadIdentitiesRequest{
				LabelSelectors: []*workloadidentityv1pb.LabelSelector{
					{
						Key:    "foo",
						Values: []string{"bar", "buzz"},
					},
				},
				Credential: &workloadidentityv1pb.IssueWorkloadIdentitiesRequest_JwtSvidParams{
					JwtSvidParams: &workloadidentityv1pb.JWTSVIDParams{
						Audiences: []string{"example.com", "test.example.com"},
					},
				},
				WorkloadAttrs: workloadAttrs(func(attrs *workloadidentityv1pb.WorkloadAttrs) {
					attrs.Kubernetes.Namespace = "not-default"
				}),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentitiesResponse) {
				require.Empty(t, res.Credentials)
			},
		},
		{
			name:   "no matching labels",
			client: client,
			req: &workloadidentityv1pb.IssueWorkloadIdentitiesRequest{
				LabelSelectors: []*workloadidentityv1pb.LabelSelector{
					{
						Key:    "foo",
						Values: []string{"muahah"},
					},
				},
				Credential: &workloadidentityv1pb.IssueWorkloadIdentitiesRequest_JwtSvidParams{
					JwtSvidParams: &workloadidentityv1pb.JWTSVIDParams{
						Audiences: []string{"example.com", "test.example.com"},
					},
				},
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentitiesResponse) {
				require.Empty(t, res.Credentials)
			},
		},
		{
			name:   "too many to issue",
			client: client,
			req: &workloadidentityv1pb.IssueWorkloadIdentitiesRequest{
				LabelSelectors: []*workloadidentityv1pb.LabelSelector{
					{
						Key:    "error",
						Values: []string{"too-many"},
					},
				},
				Credential: &workloadidentityv1pb.IssueWorkloadIdentitiesRequest_JwtSvidParams{
					JwtSvidParams: &workloadidentityv1pb.JWTSVIDParams{
						Audiences: []string{"example.com", "test.example.com"},
					},
				},
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: func(t require.TestingT, err error, i ...any) {
				require.ErrorContains(t, err, "number of identities that would be issued exceeds maximum permitted (max = 10), use more specific labels")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tp.eventRecorder.Reset()
			c := workloadidentityv1pb.NewWorkloadIdentityIssuanceServiceClient(
				tt.client.GetConnection(),
			)
			res, err := c.IssueWorkloadIdentities(ctx, tt.req)
			tt.requireErr(t, err)
			if tt.assert != nil {
				tt.assert(t, res)
			}
		})
	}
}

func TestResourceService_CreateWorkloadIdentity(t *testing.T) {
	t.Parallel()
	srv, eventRecorder := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentity},
				Verbs:     []string{types.VerbCreate},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identity
	preExisting, err := srv.Auth().CreateWorkloadIdentity(
		ctx,
		&workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "preexisting",
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/example",
				},
			},
		})
	require.NoError(t, err)

	tests := []struct {
		name                string
		client              *authclient.Client
		req                 *workloadidentityv1pb.CreateWorkloadIdentityRequest
		requireError        require.ErrorAssertionFunc
		checkResultReturned bool
		requireEvent        *events.WorkloadIdentityCreate
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.CreateWorkloadIdentityRequest{
				WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "new",
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "/example",
						},
					},
				},
			},
			requireError:        require.NoError,
			checkResultReturned: true,
			requireEvent: &events.WorkloadIdentityCreate{
				Metadata: events.Metadata{
					Code: libevents.WorkloadIdentityCreateCode,
					Type: libevents.WorkloadIdentityCreateEvent,
				},
				ResourceMetadata: events.ResourceMetadata{
					Name: "new",
				},
				UserMetadata: events.UserMetadata{
					User:     authorizedUser.GetName(),
					UserKind: events.UserKind_USER_KIND_HUMAN,
				},
			},
		},
		{
			name:   "pre-existing",
			client: authorizedClient,
			req: &workloadidentityv1pb.CreateWorkloadIdentityRequest{
				WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: preExisting.GetMetadata().GetName(),
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "/example",
						},
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAlreadyExists(err))
			},
		},
		{
			name:   "validation fail",
			client: authorizedClient,
			req: &workloadidentityv1pb.CreateWorkloadIdentityRequest{
				WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "new",
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "",
						},
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "spec.spiffe.id: is required")
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.CreateWorkloadIdentityRequest{
				WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "unauthorized",
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "/example",
						},
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRecorder.Reset()
			client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
				tt.client.GetConnection(),
			)
			res, err := client.CreateWorkloadIdentity(ctx, tt.req)
			tt.requireError(t, err)

			if tt.checkResultReturned {
				require.NotEmpty(t, res.Metadata.Revision)
				// Expect returned result to match request, but also have a
				// revision
				require.Empty(
					t,
					cmp.Diff(
						res,
						tt.req.WorkloadIdentity,
						protocmp.Transform(),
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
					),
				)
				// Expect the value fetched from the store to match returned
				// item.
				fetched, err := srv.Auth().GetWorkloadIdentity(ctx, res.Metadata.Name)
				require.NoError(t, err)
				require.Empty(
					t,
					cmp.Diff(
						res,
						fetched,
						protocmp.Transform(),
					),
				)
			}
			if tt.requireEvent != nil {
				evt, ok := eventRecorder.LastEvent().(*events.WorkloadIdentityCreate)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Empty(t, cmp.Diff(
					evt,
					tt.requireEvent,
					cmpopts.IgnoreFields(events.WorkloadIdentityCreate{}, "ConnectionMetadata", "WorkloadIdentityData"),
				))
			}
		})
	}
}

func TestResourceService_DeleteWorkloadIdentity(t *testing.T) {
	t.Parallel()
	srv, eventRecorder := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentity},
				Verbs:     []string{types.VerbDelete},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identity
	preExisting, err := srv.Auth().CreateWorkloadIdentity(
		ctx,
		&workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "preexisting",
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/example",
				},
			},
		})
	require.NoError(t, err)

	tests := []struct {
		name             string
		client           *authclient.Client
		req              *workloadidentityv1pb.DeleteWorkloadIdentityRequest
		requireError     require.ErrorAssertionFunc
		checkNonExisting bool
		requireEvent     *events.WorkloadIdentityDelete
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
				Name: preExisting.GetMetadata().GetName(),
			},
			requireError:     require.NoError,
			checkNonExisting: true,
			requireEvent: &events.WorkloadIdentityDelete{
				Metadata: events.Metadata{
					Code: libevents.WorkloadIdentityDeleteCode,
					Type: libevents.WorkloadIdentityDeleteEvent,
				},
				ResourceMetadata: events.ResourceMetadata{
					Name: preExisting.GetMetadata().GetName(),
				},
				UserMetadata: events.UserMetadata{
					User:     authorizedUser.GetName(),
					UserKind: events.UserKind_USER_KIND_HUMAN,
				},
			},
		},
		{
			name:   "non-existing",
			client: authorizedClient,
			req: &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
				Name: "i-do-not-exist",
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:   "validation fail",
			client: authorizedClient,
			req: &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
				Name: "",
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "name: must be non-empty")
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
				Name: "unauthorized",
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRecorder.Reset()
			client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
				tt.client.GetConnection(),
			)
			_, err := client.DeleteWorkloadIdentity(ctx, tt.req)
			tt.requireError(t, err)

			if tt.checkNonExisting {
				_, err := srv.Auth().GetWorkloadIdentity(ctx, tt.req.Name)
				require.True(t, trace.IsNotFound(err))
			}
			if tt.requireEvent != nil {
				evt, ok := eventRecorder.LastEvent().(*events.WorkloadIdentityDelete)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Empty(t, cmp.Diff(
					tt.requireEvent,
					evt,
					cmpopts.IgnoreFields(events.WorkloadIdentityDelete{}, "ConnectionMetadata"),
				))
			}
		})
	}
}

func TestResourceService_GetWorkloadIdentity(t *testing.T) {
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentity},
				Verbs:     []string{types.VerbRead},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identity
	preExisting, err := srv.Auth().CreateWorkloadIdentity(
		ctx,
		&workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "preexisting",
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/example",
				},
			},
		})
	require.NoError(t, err)

	tests := []struct {
		name         string
		client       *authclient.Client
		req          *workloadidentityv1pb.GetWorkloadIdentityRequest
		wantRes      *workloadidentityv1pb.WorkloadIdentity
		requireError require.ErrorAssertionFunc
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.GetWorkloadIdentityRequest{
				Name: preExisting.GetMetadata().GetName(),
			},
			wantRes:      preExisting,
			requireError: require.NoError,
		},
		{
			name:   "non-existing",
			client: authorizedClient,
			req: &workloadidentityv1pb.GetWorkloadIdentityRequest{
				Name: "i-do-not-exist",
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:   "validation fail",
			client: authorizedClient,
			req: &workloadidentityv1pb.GetWorkloadIdentityRequest{
				Name: "",
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "name: must be non-empty")
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.GetWorkloadIdentityRequest{
				Name: "unauthorized",
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
				tt.client.GetConnection(),
			)
			got, err := client.GetWorkloadIdentity(ctx, tt.req)
			tt.requireError(t, err)

			if tt.wantRes != nil {
				require.Empty(
					t,
					cmp.Diff(
						tt.wantRes,
						got,
						protocmp.Transform(),
					),
				)
			}
		})
	}
}

func TestResourceService_ListWorkloadIdentities(t *testing.T) {
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentity},
				Verbs:     []string{types.VerbRead, types.VerbList},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identities
	// Two complete pages of ten, plus one incomplete page of nine
	created := []*workloadidentityv1pb.WorkloadIdentity{}
	for i := range 29 {
		r, err := srv.Auth().CreateWorkloadIdentity(
			ctx,
			&workloadidentityv1pb.WorkloadIdentity{
				Kind:    types.KindWorkloadIdentity,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name: fmt.Sprintf("preexisting-%d", i),
				},
				Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
					Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
						Id: "/example",
					},
				},
			})
		require.NoError(t, err)
		created = append(created, r)
	}

	t.Run("unauthorized", func(t *testing.T) {
		client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
			unauthorizedClient.GetConnection(),
		)

		_, err := client.ListWorkloadIdentities(ctx, &workloadidentityv1pb.ListWorkloadIdentitiesRequest{})
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("success - default page", func(t *testing.T) {
		client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
			authorizedClient.GetConnection(),
		)

		// For the default page size, we expect to get all results in one page
		res, err := client.ListWorkloadIdentities(ctx, &workloadidentityv1pb.ListWorkloadIdentitiesRequest{})
		require.NoError(t, err)
		require.Len(t, res.WorkloadIdentities, 29)
		require.Empty(t, res.NextPageToken)
		for _, created := range created {
			require.True(t, slices.ContainsFunc(res.WorkloadIdentities, func(resource *workloadidentityv1pb.WorkloadIdentity) bool {
				return proto.Equal(created, resource)
			}))
		}
	})

	t.Run("success - page size 10", func(t *testing.T) {
		client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
			authorizedClient.GetConnection(),
		)

		fetched := []*workloadidentityv1pb.WorkloadIdentity{}
		token := ""
		iterations := 0
		for {
			iterations++
			res, err := client.ListWorkloadIdentities(ctx, &workloadidentityv1pb.ListWorkloadIdentitiesRequest{
				PageSize:  10,
				PageToken: token,
			})
			require.NoError(t, err)
			fetched = append(fetched, res.WorkloadIdentities...)
			if res.NextPageToken == "" {
				break
			}
			token = res.NextPageToken
		}

		require.Len(t, fetched, 29)
		require.Equal(t, 3, iterations)
		for _, created := range created {
			require.True(t, slices.ContainsFunc(fetched, func(resource *workloadidentityv1pb.WorkloadIdentity) bool {
				return proto.Equal(created, resource)
			}))
		}
	})
}

func TestResourceService_UpdateWorkloadIdentity(t *testing.T) {
	t.Parallel()
	srv, eventRecorder := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentity},
				Verbs:     []string{types.VerbUpdate},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identity
	preExisting, err := srv.Auth().CreateWorkloadIdentity(
		ctx,
		&workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "preexisting",
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/example",
				},
			},
		})
	require.NoError(t, err)
	preExisting2, err := srv.Auth().CreateWorkloadIdentity(
		ctx,
		&workloadidentityv1pb.WorkloadIdentity{
			Kind:    types.KindWorkloadIdentity,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "preexisting-2",
			},
			Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
				Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
					Id: "/example",
				},
			},
		})
	require.NoError(t, err)

	tests := []struct {
		name                string
		client              *authclient.Client
		req                 *workloadidentityv1pb.UpdateWorkloadIdentityRequest
		requireError        require.ErrorAssertionFunc
		checkResultReturned bool
		requireEvent        *events.WorkloadIdentityUpdate
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.UpdateWorkloadIdentityRequest{
				WorkloadIdentity: preExisting,
			},
			requireError:        require.NoError,
			checkResultReturned: true,
			requireEvent: &events.WorkloadIdentityUpdate{
				Metadata: events.Metadata{
					Code: libevents.WorkloadIdentityUpdateCode,
					Type: libevents.WorkloadIdentityUpdateEvent,
				},
				ResourceMetadata: events.ResourceMetadata{
					Name: preExisting.GetMetadata().GetName(),
				},
				UserMetadata: events.UserMetadata{
					User:     authorizedUser.GetName(),
					UserKind: events.UserKind_USER_KIND_HUMAN,
				},
			},
		},
		{
			name:   "incorrect revision",
			client: authorizedClient,
			req: (func() *workloadidentityv1pb.UpdateWorkloadIdentityRequest {
				preExisting2.Metadata.Revision = "incorrect"
				return &workloadidentityv1pb.UpdateWorkloadIdentityRequest{
					WorkloadIdentity: preExisting2,
				}
			})(),
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsCompareFailed(err))
			},
		},
		{
			name:   "not existing",
			client: authorizedClient,
			req: &workloadidentityv1pb.UpdateWorkloadIdentityRequest{
				WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "new",
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "/test",
						},
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err)
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.UpdateWorkloadIdentityRequest{
				WorkloadIdentity: preExisting,
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRecorder.Reset()
			client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
				tt.client.GetConnection(),
			)
			res, err := client.UpdateWorkloadIdentity(ctx, tt.req)
			tt.requireError(t, err)

			if tt.checkResultReturned {
				require.NotEmpty(t, res.Metadata.Revision)
				require.NotEqual(t, tt.req.WorkloadIdentity.GetMetadata().GetRevision(), res.Metadata.Revision)
				// Expect returned result to match request, but also have a
				// revision
				require.Empty(
					t,
					cmp.Diff(
						res,
						tt.req.WorkloadIdentity,
						protocmp.Transform(),
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
					),
				)
				// Expect the value fetched from the store to match returned
				// item.
				fetched, err := srv.Auth().GetWorkloadIdentity(ctx, res.Metadata.Name)
				require.NoError(t, err)
				require.Empty(
					t,
					cmp.Diff(
						res,
						fetched,
						protocmp.Transform(),
					),
				)
			}
			if tt.requireEvent != nil {
				evt, ok := eventRecorder.LastEvent().(*events.WorkloadIdentityUpdate)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Empty(t, cmp.Diff(
					evt,
					tt.requireEvent,
					cmpopts.IgnoreFields(events.WorkloadIdentityUpdate{}, "ConnectionMetadata", "WorkloadIdentityData"),
				))
			}
		})
	}
}

func TestResourceService_UpsertWorkloadIdentity(t *testing.T) {
	t.Parallel()
	srv, eventRecorder := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentity},
				Verbs:     []string{types.VerbCreate, types.VerbUpdate},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	tests := []struct {
		name                string
		client              *authclient.Client
		req                 *workloadidentityv1pb.UpsertWorkloadIdentityRequest
		requireError        require.ErrorAssertionFunc
		checkResultReturned bool
		requireEvent        *events.WorkloadIdentityCreate
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.UpsertWorkloadIdentityRequest{
				WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "new",
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "/example",
						},
					},
				},
			},
			requireError:        require.NoError,
			checkResultReturned: true,
			requireEvent: &events.WorkloadIdentityCreate{
				Metadata: events.Metadata{
					Code: libevents.WorkloadIdentityCreateCode,
					Type: libevents.WorkloadIdentityCreateEvent,
				},
				ResourceMetadata: events.ResourceMetadata{
					Name: "new",
				},
				UserMetadata: events.UserMetadata{
					User:     authorizedUser.GetName(),
					UserKind: events.UserKind_USER_KIND_HUMAN,
				},
			},
		},
		{
			name:   "validation fail",
			client: authorizedClient,
			req: &workloadidentityv1pb.UpsertWorkloadIdentityRequest{
				WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "new",
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "",
						},
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "spec.spiffe.id: is required")
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.UpsertWorkloadIdentityRequest{
				WorkloadIdentity: &workloadidentityv1pb.WorkloadIdentity{
					Kind:    types.KindWorkloadIdentity,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name: "unauthorized",
					},
					Spec: &workloadidentityv1pb.WorkloadIdentitySpec{
						Spiffe: &workloadidentityv1pb.WorkloadIdentitySPIFFE{
							Id: "/example",
						},
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRecorder.Reset()
			client := workloadidentityv1pb.NewWorkloadIdentityResourceServiceClient(
				tt.client.GetConnection(),
			)
			res, err := client.UpsertWorkloadIdentity(ctx, tt.req)
			tt.requireError(t, err)

			if tt.checkResultReturned {
				require.NotEmpty(t, res.Metadata.Revision)
				// Expect returned result to match request, but also have a
				// revision
				require.Empty(
					t,
					cmp.Diff(
						res,
						tt.req.WorkloadIdentity,
						protocmp.Transform(),
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
					),
				)
				// Expect the value fetched from the store to match returned
				// item.
				fetched, err := srv.Auth().GetWorkloadIdentity(ctx, res.Metadata.Name)
				require.NoError(t, err)
				require.Empty(
					t,
					cmp.Diff(
						res,
						fetched,
						protocmp.Transform(),
					),
				)
			}
			if tt.requireEvent != nil {
				evt, ok := eventRecorder.LastEvent().(*events.WorkloadIdentityCreate)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Empty(t, cmp.Diff(
					evt,
					tt.requireEvent,
					cmpopts.IgnoreFields(events.WorkloadIdentityCreate{}, "ConnectionMetadata", "WorkloadIdentityData"),
				))
			}
		})
	}
}

func TestRevocationService_CreateWorkloadIdentityX509Revocation(t *testing.T) {
	t.Parallel()
	srv, eventRecorder := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentityX509Revocation},
				Verbs:     []string{types.VerbCreate},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identity revocation
	preExisting, err := srv.Auth().CreateWorkloadIdentityX509Revocation(
		ctx,
		&workloadidentityv1pb.WorkloadIdentityX509Revocation{
			Kind:    types.KindWorkloadIdentityX509Revocation,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:    "aabbccdd",
				Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
			},
			Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
				Reason:    "compromised",
				RevokedAt: timestamppb.New(srv.Clock().Now()),
			},
		})
	require.NoError(t, err)

	tests := []struct {
		name                string
		client              *authclient.Client
		req                 *workloadidentityv1pb.CreateWorkloadIdentityX509RevocationRequest
		requireError        require.ErrorAssertionFunc
		checkResultReturned bool
		requireEvent        *events.WorkloadIdentityX509RevocationCreate
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.CreateWorkloadIdentityX509RevocationRequest{
				WorkloadIdentityX509Revocation: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
					Kind:    types.KindWorkloadIdentityX509Revocation,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:    "aa",
						Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
					},
					Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
						Reason:    "compromised",
						RevokedAt: timestamppb.New(srv.Clock().Now()),
					},
				},
			},
			requireError:        require.NoError,
			checkResultReturned: true,
			requireEvent: &events.WorkloadIdentityX509RevocationCreate{
				Metadata: events.Metadata{
					Code: libevents.WorkloadIdentityX509RevocationCreateCode,
					Type: libevents.WorkloadIdentityX509RevocationCreateEvent,
				},
				ResourceMetadata: events.ResourceMetadata{
					Name: "aa",
				},
				UserMetadata: events.UserMetadata{
					User:     authorizedUser.GetName(),
					UserKind: events.UserKind_USER_KIND_HUMAN,
				},
				Reason: "compromised",
			},
		},
		{
			name:   "pre-existing",
			client: authorizedClient,
			req: &workloadidentityv1pb.CreateWorkloadIdentityX509RevocationRequest{
				WorkloadIdentityX509Revocation: preExisting,
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAlreadyExists(err))
			},
		},
		{
			name:   "validation fail",
			client: authorizedClient,
			req: &workloadidentityv1pb.CreateWorkloadIdentityX509RevocationRequest{
				WorkloadIdentityX509Revocation: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
					Kind:    types.KindWorkloadIdentityX509Revocation,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:    "bb",
						Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
					},
					Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
						Reason:    "",
						RevokedAt: timestamppb.New(srv.Clock().Now()),
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "spec.reason: is required")
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.CreateWorkloadIdentityX509RevocationRequest{
				WorkloadIdentityX509Revocation: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
					Kind:    types.KindWorkloadIdentityX509Revocation,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:    "cc",
						Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
					},
					Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
						Reason:    "compromised",
						RevokedAt: timestamppb.New(srv.Clock().Now()),
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRecorder.Reset()
			client := workloadidentityv1pb.NewWorkloadIdentityRevocationServiceClient(
				tt.client.GetConnection(),
			)
			res, err := client.CreateWorkloadIdentityX509Revocation(ctx, tt.req)
			tt.requireError(t, err)

			if tt.checkResultReturned {
				require.NotEmpty(t, res.Metadata.Revision)
				// Expect returned result to match request, but also have a
				// revision
				require.Empty(
					t,
					cmp.Diff(
						res,
						tt.req.WorkloadIdentityX509Revocation,
						protocmp.Transform(),
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
					),
				)
				// Expect the value fetched from the store to match returned
				// item.
				fetched, err := srv.Auth().GetWorkloadIdentityX509Revocation(ctx, res.Metadata.Name)
				require.NoError(t, err)
				require.Empty(
					t,
					cmp.Diff(
						res,
						fetched,
						protocmp.Transform(),
					),
				)
			}
			if tt.requireEvent != nil {
				evt, ok := eventRecorder.LastEvent().(*events.WorkloadIdentityX509RevocationCreate)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Empty(t, cmp.Diff(
					evt,
					tt.requireEvent,
					cmpopts.IgnoreFields(events.WorkloadIdentityX509RevocationCreate{}, "ConnectionMetadata"),
				))
			}
		})
	}
}

func TestRevocationService_DeleteWorkloadIdentityX509Revocation(t *testing.T) {
	t.Parallel()
	srv, eventRecorder := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentityX509Revocation},
				Verbs:     []string{types.VerbDelete},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identity revocation
	preExisting, err := srv.Auth().CreateWorkloadIdentityX509Revocation(
		ctx,
		&workloadidentityv1pb.WorkloadIdentityX509Revocation{
			Kind:    types.KindWorkloadIdentityX509Revocation,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:    "aabbccdd",
				Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
			},
			Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
				Reason:    "compromised",
				RevokedAt: timestamppb.New(srv.Clock().Now()),
			},
		})
	require.NoError(t, err)

	tests := []struct {
		name             string
		client           *authclient.Client
		req              *workloadidentityv1pb.DeleteWorkloadIdentityX509RevocationRequest
		requireError     require.ErrorAssertionFunc
		checkNonExisting bool
		requireEvent     *events.WorkloadIdentityX509RevocationDelete
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.DeleteWorkloadIdentityX509RevocationRequest{
				Name: preExisting.GetMetadata().GetName(),
			},
			requireError:     require.NoError,
			checkNonExisting: true,
			requireEvent: &events.WorkloadIdentityX509RevocationDelete{
				Metadata: events.Metadata{
					Code: libevents.WorkloadIdentityX509RevocationDeleteCode,
					Type: libevents.WorkloadIdentityX509RevocationDeleteEvent,
				},
				ResourceMetadata: events.ResourceMetadata{
					Name: preExisting.GetMetadata().GetName(),
				},
				UserMetadata: events.UserMetadata{
					User:     authorizedUser.GetName(),
					UserKind: events.UserKind_USER_KIND_HUMAN,
				},
			},
		},
		{
			name:   "non-existing",
			client: authorizedClient,
			req: &workloadidentityv1pb.DeleteWorkloadIdentityX509RevocationRequest{
				Name: "i-do-not-exist",
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:   "validation fail",
			client: authorizedClient,
			req: &workloadidentityv1pb.DeleteWorkloadIdentityX509RevocationRequest{
				Name: "",
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "name: must be non-empty")
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.DeleteWorkloadIdentityX509RevocationRequest{
				Name: "unauthorized",
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRecorder.Reset()
			client := workloadidentityv1pb.NewWorkloadIdentityRevocationServiceClient(
				tt.client.GetConnection(),
			)
			_, err := client.DeleteWorkloadIdentityX509Revocation(ctx, tt.req)
			tt.requireError(t, err)

			if tt.checkNonExisting {
				_, err := srv.Auth().GetWorkloadIdentityX509Revocation(ctx, tt.req.Name)
				require.True(t, trace.IsNotFound(err))
			}
			if tt.requireEvent != nil {
				evt, ok := eventRecorder.LastEvent().(*events.WorkloadIdentityX509RevocationDelete)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Empty(t, cmp.Diff(
					tt.requireEvent,
					evt,
					cmpopts.IgnoreFields(events.WorkloadIdentityX509RevocationDelete{}, "ConnectionMetadata"),
				))
			}
		})
	}
}

func TestRevocationService_GetWorkloadIdentityX509Revocation(t *testing.T) {
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentityX509Revocation},
				Verbs:     []string{types.VerbRead},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identity revocation
	preExisting, err := srv.Auth().CreateWorkloadIdentityX509Revocation(
		ctx,
		&workloadidentityv1pb.WorkloadIdentityX509Revocation{
			Kind:    types.KindWorkloadIdentityX509Revocation,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:    "aabbccdd",
				Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
			},
			Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
				Reason:    "compromised",
				RevokedAt: timestamppb.New(srv.Clock().Now()),
			},
		})
	require.NoError(t, err)

	tests := []struct {
		name         string
		client       *authclient.Client
		req          *workloadidentityv1pb.GetWorkloadIdentityX509RevocationRequest
		wantRes      *workloadidentityv1pb.WorkloadIdentityX509Revocation
		requireError require.ErrorAssertionFunc
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.GetWorkloadIdentityX509RevocationRequest{
				Name: preExisting.GetMetadata().GetName(),
			},
			wantRes:      preExisting,
			requireError: require.NoError,
		},
		{
			name:   "non-existing",
			client: authorizedClient,
			req: &workloadidentityv1pb.GetWorkloadIdentityX509RevocationRequest{
				Name: "i-do-not-exist",
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:   "validation fail",
			client: authorizedClient,
			req: &workloadidentityv1pb.GetWorkloadIdentityX509RevocationRequest{
				Name: "",
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "name: must be non-empty")
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.GetWorkloadIdentityX509RevocationRequest{
				Name: "unauthorized",
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := workloadidentityv1pb.NewWorkloadIdentityRevocationServiceClient(
				tt.client.GetConnection(),
			)
			got, err := client.GetWorkloadIdentityX509Revocation(ctx, tt.req)
			tt.requireError(t, err)

			if tt.wantRes != nil {
				require.Empty(
					t,
					cmp.Diff(
						tt.wantRes,
						got,
						protocmp.Transform(),
					),
				)
			}
		})
	}
}

func TestRevocationService_ListWorkloadIdentityX509Revocations(t *testing.T) {
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentityX509Revocation},
				Verbs:     []string{types.VerbRead, types.VerbList},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identitie revocations
	// Two complete pages of ten, plus one incomplete page of nine
	created := []*workloadidentityv1pb.WorkloadIdentityX509Revocation{}
	for i := range 29 {
		r, err := srv.Auth().CreateWorkloadIdentityX509Revocation(
			ctx,
			&workloadidentityv1pb.WorkloadIdentityX509Revocation{
				Kind:    types.KindWorkloadIdentityX509Revocation,
				Version: types.V1,
				Metadata: &headerv1.Metadata{
					Name:    fmt.Sprintf("%d%d", i, i),
					Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
				},
				Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
					Reason:    "compromised",
					RevokedAt: timestamppb.New(srv.Clock().Now()),
				},
			})
		require.NoError(t, err)
		created = append(created, r)
	}

	t.Run("unauthorized", func(t *testing.T) {
		client := workloadidentityv1pb.NewWorkloadIdentityRevocationServiceClient(
			unauthorizedClient.GetConnection(),
		)

		_, err := client.ListWorkloadIdentityX509Revocations(
			ctx,
			&workloadidentityv1pb.ListWorkloadIdentityX509RevocationsRequest{},
		)
		require.True(t, trace.IsAccessDenied(err))
	})

	t.Run("success - default page", func(t *testing.T) {
		client := workloadidentityv1pb.NewWorkloadIdentityRevocationServiceClient(
			authorizedClient.GetConnection(),
		)

		// For the default page size, we expect to get all results in one page
		res, err := client.ListWorkloadIdentityX509Revocations(ctx, &workloadidentityv1pb.ListWorkloadIdentityX509RevocationsRequest{})
		require.NoError(t, err)
		require.Len(t, res.WorkloadIdentityX509Revocations, 29)
		require.Empty(t, res.NextPageToken)
		for _, created := range created {
			require.True(t, slices.ContainsFunc(res.WorkloadIdentityX509Revocations, func(resource *workloadidentityv1pb.WorkloadIdentityX509Revocation) bool {
				return proto.Equal(created, resource)
			}))
		}
	})

	t.Run("success - page size 10", func(t *testing.T) {
		client := workloadidentityv1pb.NewWorkloadIdentityRevocationServiceClient(
			authorizedClient.GetConnection(),
		)

		fetched := []*workloadidentityv1pb.WorkloadIdentityX509Revocation{}
		token := ""
		iterations := 0
		for {
			iterations++
			res, err := client.ListWorkloadIdentityX509Revocations(ctx, &workloadidentityv1pb.ListWorkloadIdentityX509RevocationsRequest{
				PageSize:  10,
				PageToken: token,
			})
			require.NoError(t, err)
			fetched = append(fetched, res.WorkloadIdentityX509Revocations...)
			if res.NextPageToken == "" {
				break
			}
			token = res.NextPageToken
		}

		require.Len(t, fetched, 29)
		require.Equal(t, 3, iterations)
		for _, created := range created {
			require.True(t, slices.ContainsFunc(fetched, func(resource *workloadidentityv1pb.WorkloadIdentityX509Revocation) bool {
				return proto.Equal(created, resource)
			}))
		}
	})
}

func TestRevocationService_UpdateWorkloadIdentityX509Revocation(t *testing.T) {
	t.Parallel()
	srv, eventRecorder := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentityX509Revocation},
				Verbs:     []string{types.VerbUpdate},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	// Create a pre-existing workload identity revocation
	preExisting, err := srv.Auth().CreateWorkloadIdentityX509Revocation(
		ctx,
		&workloadidentityv1pb.WorkloadIdentityX509Revocation{
			Kind:    types.KindWorkloadIdentityX509Revocation,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:    "aabbccdd",
				Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
			},
			Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
				Reason:    "compromised",
				RevokedAt: timestamppb.New(srv.Clock().Now()),
			},
		})
	require.NoError(t, err)
	// Create a pre-existing workload identity revocation
	preExisting2, err := srv.Auth().CreateWorkloadIdentityX509Revocation(
		ctx,
		&workloadidentityv1pb.WorkloadIdentityX509Revocation{
			Kind:    types.KindWorkloadIdentityX509Revocation,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name:    "aabbccee",
				Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
			},
			Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
				Reason:    "compromised",
				RevokedAt: timestamppb.New(srv.Clock().Now()),
			},
		})
	require.NoError(t, err)

	tests := []struct {
		name                string
		client              *authclient.Client
		req                 *workloadidentityv1pb.UpdateWorkloadIdentityX509RevocationRequest
		requireError        require.ErrorAssertionFunc
		checkResultReturned bool
		requireEvent        *events.WorkloadIdentityX509RevocationUpdate
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.UpdateWorkloadIdentityX509RevocationRequest{
				WorkloadIdentityX509Revocation: preExisting,
			},
			requireError:        require.NoError,
			checkResultReturned: true,
			requireEvent: &events.WorkloadIdentityX509RevocationUpdate{
				Metadata: events.Metadata{
					Code: libevents.WorkloadIdentityX509RevocationUpdateCode,
					Type: libevents.WorkloadIdentityX509RevocationUpdateEvent,
				},
				ResourceMetadata: events.ResourceMetadata{
					Name: preExisting.GetMetadata().GetName(),
				},
				UserMetadata: events.UserMetadata{
					User:     authorizedUser.GetName(),
					UserKind: events.UserKind_USER_KIND_HUMAN,
				},
				Reason: "compromised",
			},
		},
		{
			name:   "incorrect revision",
			client: authorizedClient,
			req: (func() *workloadidentityv1pb.UpdateWorkloadIdentityX509RevocationRequest {
				preExisting2.Metadata.Revision = "incorrect"
				return &workloadidentityv1pb.UpdateWorkloadIdentityX509RevocationRequest{
					WorkloadIdentityX509Revocation: preExisting2,
				}
			})(),
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsCompareFailed(err))
			},
		},
		{
			name:   "not existing",
			client: authorizedClient,
			req: &workloadidentityv1pb.UpdateWorkloadIdentityX509RevocationRequest{
				WorkloadIdentityX509Revocation: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
					Kind:    types.KindWorkloadIdentityX509Revocation,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:    "aabbccdd404",
						Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
					},
					Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
						Reason:    "compromised",
						RevokedAt: timestamppb.New(srv.Clock().Now()),
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.Error(t, err)
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.UpdateWorkloadIdentityX509RevocationRequest{
				WorkloadIdentityX509Revocation: preExisting,
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRecorder.Reset()
			client := workloadidentityv1pb.NewWorkloadIdentityRevocationServiceClient(
				tt.client.GetConnection(),
			)
			res, err := client.UpdateWorkloadIdentityX509Revocation(ctx, tt.req)
			tt.requireError(t, err)

			if tt.checkResultReturned {
				require.NotEmpty(t, res.Metadata.Revision)
				require.NotEqual(t, tt.req.WorkloadIdentityX509Revocation.GetMetadata().GetRevision(), res.Metadata.Revision)
				// Expect returned result to match request, but also have a
				// revision
				require.Empty(
					t,
					cmp.Diff(
						res,
						tt.req.WorkloadIdentityX509Revocation,
						protocmp.Transform(),
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
					),
				)
				// Expect the value fetched from the store to match returned
				// item.
				fetched, err := srv.Auth().GetWorkloadIdentityX509Revocation(ctx, res.Metadata.Name)
				require.NoError(t, err)
				require.Empty(
					t,
					cmp.Diff(
						res,
						fetched,
						protocmp.Transform(),
					),
				)
			}
			if tt.requireEvent != nil {
				evt, ok := eventRecorder.LastEvent().(*events.WorkloadIdentityX509RevocationUpdate)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Empty(t, cmp.Diff(
					evt,
					tt.requireEvent,
					cmpopts.IgnoreFields(events.WorkloadIdentityX509RevocationUpdate{}, "ConnectionMetadata"),
				))
			}
		})
	}
}

func TestRevocationService_UpsertWorkloadIdentityX509Revocation(t *testing.T) {
	t.Parallel()
	srv, eventRecorder := newTestTLSServer(t)
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentityX509Revocation},
				Verbs:     []string{types.VerbCreate, types.VerbUpdate},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	unauthorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"unauthorized",
		[]string{},
		[]types.Rule{},
	)
	require.NoError(t, err)
	unauthorizedClient, err := srv.NewClient(auth.TestUser(unauthorizedUser.GetName()))
	require.NoError(t, err)

	tests := []struct {
		name                string
		client              *authclient.Client
		req                 *workloadidentityv1pb.UpsertWorkloadIdentityX509RevocationRequest
		requireError        require.ErrorAssertionFunc
		checkResultReturned bool
		requireEvent        *events.WorkloadIdentityX509RevocationCreate
	}{
		{
			name:   "success",
			client: authorizedClient,
			req: &workloadidentityv1pb.UpsertWorkloadIdentityX509RevocationRequest{
				WorkloadIdentityX509Revocation: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
					Kind:    types.KindWorkloadIdentityX509Revocation,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:    "aabbccdd",
						Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
					},
					Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
						Reason:    "compromised",
						RevokedAt: timestamppb.New(srv.Clock().Now()),
					},
				},
			},
			requireError:        require.NoError,
			checkResultReturned: true,
			requireEvent: &events.WorkloadIdentityX509RevocationCreate{
				Metadata: events.Metadata{
					Code: libevents.WorkloadIdentityX509RevocationCreateCode,
					Type: libevents.WorkloadIdentityX509RevocationCreateEvent,
				},
				ResourceMetadata: events.ResourceMetadata{
					Name: "aabbccdd",
				},
				UserMetadata: events.UserMetadata{
					User:     authorizedUser.GetName(),
					UserKind: events.UserKind_USER_KIND_HUMAN,
				},
				Reason: "compromised",
			},
		},
		{
			name:   "validation fail",
			client: authorizedClient,
			req: &workloadidentityv1pb.UpsertWorkloadIdentityX509RevocationRequest{
				WorkloadIdentityX509Revocation: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
					Kind:    types.KindWorkloadIdentityX509Revocation,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:    "aabbccdd",
						Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
					},
					Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
						RevokedAt: timestamppb.New(srv.Clock().Now()),
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "spec.reason: is required")
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.UpsertWorkloadIdentityX509RevocationRequest{
				WorkloadIdentityX509Revocation: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
					Kind:    types.KindWorkloadIdentityX509Revocation,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:    "aabbccdd",
						Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
					},
					Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
						Reason:    "compromised",
						RevokedAt: timestamppb.New(srv.Clock().Now()),
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRecorder.Reset()
			client := workloadidentityv1pb.NewWorkloadIdentityRevocationServiceClient(
				tt.client.GetConnection(),
			)
			res, err := client.UpsertWorkloadIdentityX509Revocation(ctx, tt.req)
			tt.requireError(t, err)

			if tt.checkResultReturned {
				require.NotEmpty(t, res.Metadata.Revision)
				// Expect returned result to match request, but also have a
				// revision
				require.Empty(
					t,
					cmp.Diff(
						res,
						tt.req.WorkloadIdentityX509Revocation,
						protocmp.Transform(),
						protocmp.IgnoreFields(&headerv1.Metadata{}, "revision"),
					),
				)
				// Expect the value fetched from the store to match returned
				// item.
				fetched, err := srv.Auth().GetWorkloadIdentityX509Revocation(ctx, res.Metadata.Name)
				require.NoError(t, err)
				require.Empty(
					t,
					cmp.Diff(
						res,
						fetched,
						protocmp.Transform(),
					),
				)
			}
			if tt.requireEvent != nil {
				evt, ok := eventRecorder.LastEvent().(*events.WorkloadIdentityX509RevocationCreate)
				require.True(t, ok)
				require.NotEmpty(t, evt.ConnectionMetadata.RemoteAddr)
				require.Empty(t, cmp.Diff(
					evt,
					tt.requireEvent,
					cmpopts.IgnoreFields(events.WorkloadIdentityX509RevocationCreate{}, "ConnectionMetadata"),
				))
			}
		})
	}
}

func TestRevocationService_CRL(t *testing.T) {
	t.Parallel()
	revocationsEventCh := make(chan struct{})
	fakeClock := clockwork.NewFakeClock()
	srv, _ := newTestTLSServer(t, func(config *auth.TestTLSServerConfig) {
		config.APIConfig.MutateRevocationsServiceConfig = func(config *workloadidentityv1.RevocationServiceConfig) {
			config.RevocationsEventProcessedCh = revocationsEventCh
			config.Clock = fakeClock
		}
	})
	ctx := context.Background()

	authorizedUser, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"authorized",
		[]string{},
		[]types.Rule{
			{
				Resources: []string{types.KindWorkloadIdentityX509Revocation},
				Verbs: []string{
					types.VerbRead,
					types.VerbList,
					types.VerbCreate,
					types.VerbUpdate,
					types.VerbDelete,
				},
			},
		})
	require.NoError(t, err)
	authorizedClient, err := srv.NewClient(auth.TestUser(authorizedUser.GetName()))
	require.NoError(t, err)
	revocationsClient := authorizedClient.WorkloadIdentityRevocationServiceClient()

	// Fetch the SPIFFE CA so we can validate CRL signature.
	ca, err := srv.Auth().GetCertAuthority(ctx, types.CertAuthID{
		Type:       types.SPIFFECA,
		DomainName: srv.ClusterName(),
	}, false)
	require.NoError(t, err)
	caCert, err := tlsca.ParseCertificatePEM(ca.GetActiveKeys().TLS[0].Cert)
	require.NoError(t, err)

	checkCRL := func(
		t *testing.T,
		crlBytes []byte,
		wantEntries []x509.RevocationListEntry,
	) {
		require.NotEmpty(t, crlBytes)

		// Expect a DER encoded CRL directly (e.g no PEM)
		parsed, err := x509.ParseRevocationList(crlBytes)
		require.NoError(t, err)

		// Check CRL has a valid signature
		require.NoError(t, parsed.CheckSignatureFrom(caCert))

		diff := cmp.Diff(
			wantEntries,
			parsed.RevokedCertificateEntries,
			cmp.Comparer(func(a, b *big.Int) bool {
				return a.Cmp(b) == 0
			}),
			cmpopts.IgnoreFields(x509.RevocationListEntry{}, "Raw"),
			cmpopts.SortSlices(func(a, b x509.RevocationListEntry) bool {
				return a.SerialNumber.Cmp(b.SerialNumber) < 0
			}),
		)
		require.Empty(t, diff)
	}

	revokedAt := srv.Clock().Now()
	createRevocation := func(t *testing.T, name string) {
		_, err = revocationsClient.CreateWorkloadIdentityX509Revocation(
			ctx,
			&workloadidentityv1pb.CreateWorkloadIdentityX509RevocationRequest{
				WorkloadIdentityX509Revocation: &workloadidentityv1pb.WorkloadIdentityX509Revocation{
					Kind:    types.KindWorkloadIdentityX509Revocation,
					Version: types.V1,
					Metadata: &headerv1.Metadata{
						Name:    name,
						Expires: timestamppb.New(srv.Clock().Now().Add(time.Hour)),
					},
					Spec: &workloadidentityv1pb.WorkloadIdentityX509RevocationSpec{
						Reason:    "compromised",
						RevokedAt: timestamppb.New(revokedAt),
					},
				},
			},
		)
		require.NoError(t, err)
		// Wait for the revocation event to be processed.
		select {
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for revocation event to be processed")
		case <-revocationsEventCh:
		}
	}
	deleteRevocation := func(t *testing.T, name string) {
		_, err = revocationsClient.DeleteWorkloadIdentityX509Revocation(
			ctx,
			&workloadidentityv1pb.DeleteWorkloadIdentityX509RevocationRequest{
				Name: name,
			},
		)
		require.NoError(t, err)
		// Wait for the revocation event to be processed.
		select {
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for revocation event to be processed")
		case <-revocationsEventCh:
		}
	}

	// Fetch the initial, empty, CRL
	stream, err := revocationsClient.StreamSignedCRL(
		ctx, &workloadidentityv1pb.StreamSignedCRLRequest{},
	)
	require.NoError(t, err)
	res, err := stream.Recv()
	require.NoError(t, err)
	checkCRL(t, res.Crl, nil)

	// Create new revocations
	createRevocation(t, "ff")
	createRevocation(t, "aa")
	require.NoError(t, fakeClock.BlockUntilContext(ctx, 2))
	t.Log("Advancing fake clock to pass debounce period")
	fakeClock.Advance(6 * time.Second)
	// The client should now receive a new CRL
	res, err = stream.Recv()
	require.NoError(t, err)
	checkCRL(t, res.Crl, []x509.RevocationListEntry{
		{
			SerialNumber:   big.NewInt(170),
			RevocationTime: revokedAt,
		},
		{
			SerialNumber:   big.NewInt(255),
			RevocationTime: revokedAt,
		},
	})

	// Add another revocation, delete one revocation
	createRevocation(t, "bb")
	deleteRevocation(t, "aa")
	require.NoError(t, fakeClock.BlockUntilContext(ctx, 2))
	t.Log("Advancing fake clock to pass debounce period")
	fakeClock.Advance(6 * time.Second)
	// The client should now receive a new CRL
	res, err = stream.Recv()
	require.NoError(t, err)
	checkCRL(t, res.Crl, []x509.RevocationListEntry{
		{
			SerialNumber:   big.NewInt(255),
			RevocationTime: revokedAt,
		},
		{
			SerialNumber:   big.NewInt(187),
			RevocationTime: revokedAt,
		},
	})

	// Delete all remaining CRL
	deleteRevocation(t, "bb")
	deleteRevocation(t, "ff")
	require.NoError(t, fakeClock.BlockUntilContext(ctx, 2))
	t.Log("Advancing fake clock to pass debounce period")
	fakeClock.Advance(6 * time.Second)
	// The client should now receive a new CRL
	res, err = stream.Recv()
	require.NoError(t, err)
	checkCRL(t, res.Crl, nil)

	// Wait ten minutes to see if the periodic CRL is sent.
	t.Log("Advancing fake clock to pass the periodic timer")
	fakeClock.Advance(11 * time.Minute)
	res, err = stream.Recv()
	require.NoError(t, err)
	checkCRL(t, res.Crl, nil)
}
