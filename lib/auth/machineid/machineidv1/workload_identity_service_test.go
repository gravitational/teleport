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

package machineidv1_test

import (
	"context"
	"crypto/x509"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	machineidv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/cryptosuites"
	libjwt "github.com/gravitational/teleport/lib/jwt"
)

// TestWorkloadIdentityService_SignX509SVIDs is an integration test that uses a
// real gRPC client/server.
func TestWorkloadIdentityService_SignX509SVIDs(t *testing.T) {
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	nothingRole, err := types.NewRole("nothing", types.RoleSpecV6{})
	require.NoError(t, err)
	role, err := types.NewRole("svid-issuer", types.RoleSpecV6{
		Allow: types.RoleConditions{
			SPIFFE: []*types.SPIFFERoleCondition{
				{
					Path:    "/alpha/*",
					DNSSANs: []string{"*.alpha.example.com"},
					IPSANs: []string{
						"10.0.0.1/8",
					},
				},
				{
					Path:    "/bravo/foo",
					DNSSANs: []string{"foo.bravo.example.com"},
					IPSANs: []string{
						"11.0.0.1/8",
					},
				},
			},
		},
		Deny: types.RoleConditions{
			SPIFFE: []*types.SPIFFERoleCondition{
				{
					Path:    "/alpha/forbidden",
					DNSSANs: []string{"*"},
					IPSANs: []string{
						"0.0.0.0/0",
					},
				},
			},
		},
	})
	require.NoError(t, err)
	authorizedUser, err := auth.CreateUser(
		ctx,
		srv.Auth(),
		"authorized",
		role,
	)
	require.NoError(t, err)
	unauthorizedUser, err := auth.CreateUser(
		ctx,
		srv.Auth(),
		"unauthorized",
		// Nothing role necessary as otherwise authz engine gets confused.
		nothingRole,
	)
	require.NoError(t, err)

	privateKey, err := cryptosuites.GenerateKeyWithAlgorithm(cryptosuites.ECDSAP256)
	require.NoError(t, err)
	pubBytes, err := x509.MarshalPKIXPublicKey(privateKey.Public())
	require.NoError(t, err)

	tests := []struct {
		name           string
		user           string
		req            *machineidv1pb.SignX509SVIDsRequest
		requireError   require.ErrorAssertionFunc
		assertResponse func(*testing.T, *machineidv1pb.SignX509SVIDsResponse)
	}{
		{
			name: "success",
			user: authorizedUser.GetName(),
			req: &machineidv1pb.SignX509SVIDsRequest{
				Svids: []*machineidv1pb.SVIDRequest{
					{
						SpiffeIdPath: "/alpha/foo",
						PublicKey:    pubBytes,
						Hint:         "llamas",
						Ttl:          durationpb.New(30 * time.Minute),
						DnsSans: []string{
							"foo.alpha.example.com",
							"bar.alpha.example.com",
						},
						IpSans: []string{"10.42.42.42"},
					},
				},
			},
			requireError: require.NoError,
			assertResponse: func(t *testing.T, resp *machineidv1pb.SignX509SVIDsResponse) {
				wantSPIFFEID := "spiffe://localhost/alpha/foo"

				// Parse response
				require.Len(t, resp.Svids, 1)
				svid := resp.Svids[0]
				require.Equal(t, "llamas", svid.Hint)
				require.Equal(t, wantSPIFFEID, svid.SpiffeId)
				cert, err := x509.ParseCertificate(svid.Certificate)
				require.NoError(t, err)

				// Check TTL
				require.WithinDuration(t, time.Now().Add(30*time.Minute), cert.NotAfter, 5*time.Second)

				// Check included public key matches
				require.Equal(t, privateKey.Public(), cert.PublicKey)

				// Check against SPIFFE SPEC
				// References are to https://github.com/spiffe/spiffe/blob/main/standards/X509-SVID.md
				// 2: An X.509 SVID MUST contain exactly one URI SAN, and by extension, exactly one SPIFFE ID
				require.Len(t, cert.URIs, 1)
				require.Equal(t, wantSPIFFEID, cert.URIs[0].String())
				// 2: An X.509 SVID MAY contain any number of other SAN field types, including DNS SANs.
				// Here we validate against what was requested
				require.Len(t, cert.DNSNames, 2)
				require.Equal(t, "foo.alpha.example.com", cert.DNSNames[0])
				require.Equal(t, "bar.alpha.example.com", cert.DNSNames[1])
				require.Len(t, cert.IPAddresses, 1)
				require.Equal(t, "10.42.42.42", cert.IPAddresses[0].String())
				// 4.1: leaf certificates MUST set the cA field to false.
				require.False(t, cert.IsCA)
				// 4.3: Leaf SVIDs MUST set digitalSignature.
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

				// Check that the Common Name is set to the first DNS SAN.
				require.Equal(t, "foo.alpha.example.com", cert.Subject.CommonName)
			},
		},
		{
			name: "forbidden svid",
			user: authorizedUser.GetName(),
			req: &machineidv1pb.SignX509SVIDsRequest{
				Svids: []*machineidv1pb.SVIDRequest{
					// Include an ok SVID first to ensure we check perms for all
					// SVIDs.
					{
						SpiffeIdPath: "/alpha/foo",
						PublicKey:    pubBytes,
					},
					{
						SpiffeIdPath: "/alpha/forbidden",
						PublicKey:    pubBytes,
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "no svids",
			user: authorizedUser.GetName(),
			req:  &machineidv1pb.SignX509SVIDsRequest{},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "svids: must be non-empty")
			},
		},
		{
			name: "no permissions",
			user: unauthorizedUser.GetName(),
			req: &machineidv1pb.SignX509SVIDsRequest{
				Svids: []*machineidv1pb.SVIDRequest{
					{
						SpiffeIdPath: "/alpha/foo",
						PublicKey:    pubBytes,
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
			client, err := srv.NewClient(auth.TestUser(tt.user))
			require.NoError(t, err)

			res, err := client.WorkloadIdentityServiceClient().
				SignX509SVIDs(ctx, tt.req)
			tt.requireError(t, err)
			if tt.assertResponse != nil {
				tt.assertResponse(t, res)
			}
		})
	}
}

// TestWorkloadIdentityService_SignJWTSVIDs is an integration test that uses a
// real gRPC client/server.
func TestWorkloadIdentityService_SignJWTSVIDs(t *testing.T) {
	t.Parallel()
	srv, _ := newTestTLSServer(t)
	ctx := context.Background()

	nothingRole, err := types.NewRole("nothing", types.RoleSpecV6{})
	require.NoError(t, err)
	role, err := types.NewRole("svid-issuer", types.RoleSpecV6{
		Allow: types.RoleConditions{
			SPIFFE: []*types.SPIFFERoleCondition{
				{
					Path: "/alpha/*",
				},
				{
					Path: "/bravo/foo",
				},
			},
		},
		Deny: types.RoleConditions{
			SPIFFE: []*types.SPIFFERoleCondition{
				{
					Path: "/alpha/forbidden",
				},
			},
		},
	})
	require.NoError(t, err)
	authorizedUser, err := auth.CreateUser(
		ctx,
		srv.Auth(),
		"authorized",
		role,
	)
	require.NoError(t, err)
	unauthorizedUser, err := auth.CreateUser(
		ctx,
		srv.Auth(),
		"unauthorized",
		// Nothing role necessary as otherwise authz engine gets confused.
		nothingRole,
	)
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

	// Upsert a fake proxy to ensure we have a public address to use for the
	// issuer.
	proxy, err := types.NewServer("proxy", types.KindProxy, types.ServerSpecV2{
		PublicAddrs: []string{"teleport.example.com"},
	})
	require.NoError(t, err)
	err = srv.Auth().UpsertProxy(ctx, proxy)
	require.NoError(t, err)
	wantIssuer := "https://teleport.example.com/workload-identity"

	tests := []struct {
		name           string
		user           string
		req            *machineidv1pb.SignJWTSVIDsRequest
		requireError   require.ErrorAssertionFunc
		assertResponse func(*testing.T, *machineidv1pb.SignJWTSVIDsResponse)
	}{
		{
			name: "success",
			user: authorizedUser.GetName(),
			req: &machineidv1pb.SignJWTSVIDsRequest{
				Svids: []*machineidv1pb.JWTSVIDRequest{
					{
						SpiffeIdPath: "/alpha/foo",
						Hint:         "llamas",
						Ttl:          durationpb.New(30 * time.Minute),
						Audiences:    []string{"example.com"},
					},
				},
			},
			requireError: require.NoError,
			assertResponse: func(t *testing.T, resp *machineidv1pb.SignJWTSVIDsResponse) {
				require.Len(t, resp.Svids, 1)

				svid := resp.Svids[0]
				wantSPIFFEID := "spiffe://localhost/alpha/foo"
				require.Equal(t, wantSPIFFEID, svid.SpiffeId)
				require.Equal(t, "llamas", svid.Hint)
				require.Equal(t, []string{"example.com"}, svid.Audiences)
				require.NotEmpty(t, svid.Jti)
				require.NotEmpty(t, svid.Jwt)

				parsed, err := jwt.ParseSigned(svid.Jwt)
				require.NoError(t, err)

				claims := jwt.Claims{}
				err = parsed.Claims(jwtSigner.Public(), &claims)
				require.NoError(t, err)

				// Check headers
				require.Len(t, parsed.Headers, 1)
				require.Equal(t, kid, parsed.Headers[0].KeyID)

				// Check claims
				require.Equal(t, wantSPIFFEID, claims.Subject)
				require.Equal(t, svid.Jti, claims.ID)
				require.Equal(t, "example.com", claims.Audience[0])
				require.Equal(t, wantIssuer, claims.Issuer)
				require.WithinDuration(t, time.Now().Add(30*time.Minute), claims.Expiry.Time(), 5*time.Second)
				require.WithinDuration(t, time.Now(), claims.IssuedAt.Time(), 5*time.Second)
			},
		},
		{
			name: "forbidden svid",
			user: authorizedUser.GetName(),
			req: &machineidv1pb.SignJWTSVIDsRequest{
				Svids: []*machineidv1pb.JWTSVIDRequest{
					// Include an ok SVID first to ensure we check perms for all
					// SVIDs.
					{
						SpiffeIdPath: "/alpha/foo",
						Audiences:    []string{"example.com"},
					},
					{
						SpiffeIdPath: "/alpha/forbidden",
						Audiences:    []string{"example.com"},
					},
				},
			},
			requireError: func(t require.TestingT, err error, i ...any) {
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name: "no permissions",
			user: unauthorizedUser.GetName(),
			req: &machineidv1pb.SignJWTSVIDsRequest{
				Svids: []*machineidv1pb.JWTSVIDRequest{
					{
						SpiffeIdPath: "/alpha/foo",
						Audiences:    []string{"example.com"},
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
			client, err := srv.NewClient(auth.TestUser(tt.user))
			require.NoError(t, err)

			res, err := client.WorkloadIdentityServiceClient().
				SignJWTSVIDs(ctx, tt.req)
			tt.requireError(t, err)
			if tt.assertResponse != nil {
				tt.assertResponse(t, res)
			}
		})
	}
}
