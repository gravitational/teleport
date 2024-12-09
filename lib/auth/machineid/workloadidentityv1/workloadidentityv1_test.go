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
	"crypto/x509"
	"errors"
	"fmt"
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
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/durationpb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	workloadidentityv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/workloadidentity/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	libevents "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/events/eventstest"
	libjwt "github.com/gravitational/teleport/lib/jwt"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

func TestMain(m *testing.M) {
	modules.SetInsecureTestMode(true)
	os.Exit(m.Run())
}

func newTestTLSServer(t testing.TB) (*auth.TestTLSServer, *eventstest.MockRecorderEmitter) {
	as, err := auth.NewTestAuthServer(auth.TestAuthServerConfig{
		Dir:   t.TempDir(),
		Clock: clockwork.NewFakeClockAt(time.Now().Round(time.Second).UTC()),
	})
	require.NoError(t, err)

	emitter := &eventstest.MockRecorderEmitter{}
	srv, err := as.NewTestTLSServer(func(config *auth.TestTLSServerConfig) {
		config.APIConfig.Emitter = emitter
	})
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

func TestIssueWorkloadIdentity(t *testing.T) {
	t.Parallel()
	srv, eventRecorder := newTestTLSServer(t)
	ctx := context.Background()

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

	wildcardAccess, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"dog",
		[]string{},
		[]types.Rule{},
		auth.WithRoleMutator(func(role types.Role) {
			role.SetWorkloadIdentityLabels(types.Allow, types.Labels{
				types.Wildcard: []string{types.Wildcard},
			})
		}),
	)
	require.NoError(t, err)
	wilcardAccessClient, err := srv.NewClient(auth.TestUser(wildcardAccess.GetName()))
	require.NoError(t, err)

	specificAccess, _, err := auth.CreateUserAndRole(
		srv.Auth(),
		"cat",
		[]string{},
		[]types.Rule{},
		auth.WithRoleMutator(func(role types.Role) {
			role.SetWorkloadIdentityLabels(types.Allow, types.Labels{
				"foo": []string{"bar"},
			})
		}),
	)
	require.NoError(t, err)
	specificAccessClient, err := srv.NewClient(auth.TestUser(specificAccess.GetName()))
	require.NoError(t, err)

	// Generate a keypair to generate x509 SVIDs for.
	workloadKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	workloadKeyPubBytes, err := x509.MarshalPKIXPublicKey(workloadKey.Public())
	require.NoError(t, err)

	// Create some WorkloadIdentity resources
	full, err := srv.Auth().CreateWorkloadIdentity(ctx, &workloadidentityv1pb.WorkloadIdentity{
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
								Equals:    "dog",
							},
							{
								Attribute: "workload.kubernetes.namespace",
								Equals:    "default",
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
				WorkloadAttrs: workloadAttrs(nil),
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentityResponse) {
				cred := res.Credential
				require.NotNil(t, res.Credential)

				wantTTL := time.Hour
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
						"expiry",
					),
					protocmp.IgnoreOneofs(
						&workloadidentityv1pb.Credential{},
						"credential",
					),
				))
				// Check expiry makes sense
				require.WithinDuration(t, time.Now().Add(wantTTL), cred.GetExpiresAt().AsTime(), time.Second)

				// Check the JWT
				parsed, err := jwt.ParseSigned(cred.GetJwtSvid().GetJwt())
				require.NoError(t, err)

				claims := jwt.Claims{}
				err = parsed.Claims(jwtSigner.Public(), &claims)
				require.NoError(t, err)
				// Check headers
				require.Len(t, parsed.Headers, 1)
				require.Equal(t, kid, parsed.Headers[0].KeyID)
				// Check claims
				require.Equal(t, wantSPIFFEID, claims.Subject)
				require.NotEmpty(t, claims.ID)
				require.Equal(t, jwt.Audience{"example.com", "test.example.com"}, claims.Audience)
				require.Equal(t, wantIssuer, claims.Issuer)
				require.WithinDuration(t, time.Now().Add(wantTTL), claims.Expiry.Time(), 5*time.Second)
				require.WithinDuration(t, time.Now(), claims.IssuedAt.Time(), 5*time.Second)

				// Check audit log event
				evt, ok := eventRecorder.LastEvent().(*events.SPIFFESVIDIssued)
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
					),
				))
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
			},
			requireErr: require.NoError,
			assert: func(t *testing.T, res *workloadidentityv1pb.IssueWorkloadIdentityResponse) {
				cred := res.Credential
				require.NotNil(t, res.Credential)

				wantSPIFFEID := "spiffe://localhost/example/dog/default/bar"
				wantTTL := time.Hour
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
						"expiry",
					),
					protocmp.IgnoreOneofs(
						&workloadidentityv1pb.Credential{},
						"credential",
					),
				))
				// Check expiry makes sense
				require.WithinDuration(t, time.Now().Add(wantTTL), cred.GetExpiresAt().AsTime(), time.Second)

				// Check the X509
				cert, err := x509.ParseCertificate(cred.GetX509Svid().GetCert())
				require.NoError(t, err)
				// Check included public key matches
				require.Equal(t, workloadKey.Public(), cert.PublicKey)
				// Check cert expiry
				require.WithinDuration(t, time.Now().Add(wantTTL), cert.NotAfter, time.Second)
				// Check cert nbf
				require.WithinDuration(t, time.Now().Add(-1*time.Minute), cert.NotBefore, time.Second)
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

				// Check cert signature is valid
				_, err = cert.Verify(x509.VerifyOptions{
					Roots: spiffeX509CAPool,
				})
				require.NoError(t, err)

				// Check audit log event
				evt, ok := eventRecorder.LastEvent().(*events.SPIFFESVIDIssued)
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
					},
					cmpopts.IgnoreFields(
						events.SPIFFESVIDIssued{},
						"ConnectionMetadata",
						"SerialNumber",
					),
				))
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
			requireErr: func(t require.TestingT, err error, i ...interface{}) {
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
			requireErr: func(t require.TestingT, err error, i ...interface{}) {
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
			requireErr: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eventRecorder.Reset()
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:   "validation fail",
			client: authorizedClient,
			req: &workloadidentityv1pb.DeleteWorkloadIdentityRequest{
				Name: "",
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
		},
		{
			name:   "validation fail",
			client: authorizedClient,
			req: &workloadidentityv1pb.GetWorkloadIdentityRequest{
				Name: "",
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
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
	for i := 0; i < 29; i++ {
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
			slices.ContainsFunc(res.WorkloadIdentities, func(resource *workloadidentityv1pb.WorkloadIdentity) bool {
				return proto.Equal(created, resource)
			})
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
			slices.ContainsFunc(fetched, func(resource *workloadidentityv1pb.WorkloadIdentity) bool {
				return proto.Equal(created, resource)
			})
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.Error(t, err)
			},
		},
		{
			name:   "unauthorized",
			client: unauthorizedClient,
			req: &workloadidentityv1pb.UpdateWorkloadIdentityRequest{
				WorkloadIdentity: preExisting,
			},
			requireError: func(t require.TestingT, err error, i ...interface{}) {
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
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
			requireError: func(t require.TestingT, err error, i ...interface{}) {
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
