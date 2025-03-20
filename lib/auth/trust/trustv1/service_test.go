/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
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

package trustv1

import (
	"context"
	"crypto/x509/pkix"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	trustpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/trust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/testauthority"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/tlsca"
)

type testPack struct {
	clock clockwork.FakeClock
	mem   *memory.Memory
}

func newTestPack(t *testing.T) *testPack {
	t.Helper()

	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	return &testPack{
		clock: clock,
		mem:   mem,
	}
}

type fakeAuthorizer struct {
	authzCtx *authz.Context
	checker  *fakeChecker
}

func (f *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	if f.authzCtx != nil {
		return f.authzCtx, nil
	}

	return &authz.Context{
		Checker:              f.checker,
		AdminActionAuthState: authz.AdminActionAuthMFAVerified,
	}, nil
}

type fakeAuthServer struct {
	clusterName          types.ClusterName
	generateHostCertData map[string]struct {
		cert []byte
		err  error
	}
	rotateCertAuthorityData map[string]error
}

func (f *fakeAuthServer) GetClusterName(opts ...services.MarshalOption) (types.ClusterName, error) {
	return f.clusterName, nil
}

func (f *fakeAuthServer) GenerateHostCert(ctx context.Context, hostPublicKey []byte, hostID, nodeName string, principals []string, clusterName string, role types.SystemRole, ttl time.Duration) ([]byte, error) {
	data := f.generateHostCertData[hostID]
	return data.cert, data.err
}

func (f *fakeAuthServer) RotateCertAuthority(ctx context.Context, req types.RotateRequest) error {
	return f.rotateCertAuthorityData[string(req.Type)]
}

func (f *fakeAuthServer) UpsertTrustedClusterV2(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error) {
	return tc, nil
}

func (f *fakeAuthServer) CreateTrustedCluster(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error) {
	return tc, nil
}

func (f *fakeAuthServer) UpdateTrustedCluster(ctx context.Context, tc types.TrustedCluster) (types.TrustedCluster, error) {
	return tc, nil
}

type fakeChecker struct {
	services.AccessChecker
	allow  map[check]bool
	checks []check
}

func (f *fakeChecker) CheckAccessToRule(context services.RuleContext, namespace string, rule string, verb string) error {
	c := check{rule, verb}
	f.checks = append(f.checks, c)
	if f.allow[c] {
		return nil
	}
	return trace.AccessDenied("access to %s with verb %s is not allowed", rule, verb)
}

type check struct {
	rule, verb string
}

func newCertAuthority(t *testing.T, caType types.CertAuthType, domain string) types.CertAuthority {
	t.Helper()

	ta := testauthority.New()
	priv, pub, err := ta.GenerateKeyPair()
	require.NoError(t, err)

	key, cert, err := tlsca.GenerateSelfSignedCA(pkix.Name{CommonName: domain, Organization: []string{domain}}, nil, time.Hour)
	require.NoError(t, err)

	ca, err := types.NewCertAuthority(types.CertAuthoritySpecV2{
		Type:        caType,
		ClusterName: domain,
		ActiveKeys: types.CAKeySet{
			SSH: []*types.SSHKeyPair{{
				PrivateKey:     priv,
				PrivateKeyType: types.PrivateKeyType_RAW,
				PublicKey:      pub,
			}},
			TLS: []*types.TLSKeyPair{{
				Cert: cert,
				Key:  key,
			}},
			JWT: []*types.JWTKeyPair{{
				PublicKey:      pub,
				PrivateKey:     priv,
				PrivateKeyType: types.PrivateKeyType_RAW,
			}},
		},
	})
	require.NoError(t, err)

	return ca
}

func TestRBAC(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	ca := newCertAuthority(t, types.HostCA, "test")

	tests := []struct {
		desc         string
		f            func(t *testing.T, service *Service)
		authorizer   fakeAuthorizer
		expectChecks []check
	}{
		{
			desc: "get no access",
			f: func(t *testing.T, service *Service) {
				_, err := service.GetCertAuthority(ctx, &trustpb.GetCertAuthorityRequest{
					Type:   string(ca.GetType()),
					Domain: ca.GetClusterName(),
				})

				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindCertAuthority, types.VerbReadNoSecrets}: false,
					},
				},
			},
			expectChecks: []check{{types.KindCertAuthority, types.VerbReadNoSecrets}},
		},
		{
			desc: "get no secrets",
			f: func(t *testing.T, service *Service) {
				_, err := service.GetCertAuthority(ctx, &trustpb.GetCertAuthorityRequest{
					Type:   string(ca.GetType()),
					Domain: ca.GetClusterName(),
				})

				require.NoError(t, err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindCertAuthority, types.VerbReadNoSecrets}: true,
					},
				},
			},
			expectChecks: []check{
				{types.KindCertAuthority, types.VerbReadNoSecrets}, // initial rbac check prior to getting CA
				{types.KindCertAuthority, types.VerbReadNoSecrets},
			},
		},
		{
			desc: "get with secrets",
			f: func(t *testing.T, service *Service) {
				_, err := service.GetCertAuthority(ctx, &trustpb.GetCertAuthorityRequest{
					Type:       string(ca.GetType()),
					Domain:     ca.GetClusterName(),
					IncludeKey: true,
				})

				require.NoError(t, err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindCertAuthority, types.VerbRead}: true,
					},
				},
			},
			expectChecks: []check{
				{types.KindCertAuthority, types.VerbRead}, // initial rbac check prior to getting CA
				{types.KindCertAuthority, types.VerbRead},
			},
		},
		{
			desc: "get authorities no access",
			f: func(t *testing.T, service *Service) {
				_, err := service.GetCertAuthorities(ctx, &trustpb.GetCertAuthoritiesRequest{
					Type: string(ca.GetType()),
				})

				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindCertAuthority, types.VerbList}: false,
					},
				},
			},
			expectChecks: []check{
				{types.KindCertAuthority, types.VerbList},
				{types.KindCertAuthority, types.VerbReadNoSecrets},
			},
		},
		{
			desc: "get authorities no secrets",
			f: func(t *testing.T, service *Service) {
				_, err := service.GetCertAuthorities(ctx, &trustpb.GetCertAuthoritiesRequest{
					Type: string(ca.GetType()),
				})

				require.NoError(t, err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindCertAuthority, types.VerbList}:          true,
						{types.KindCertAuthority, types.VerbReadNoSecrets}: true,
					},
				},
			},
			expectChecks: []check{
				{types.KindCertAuthority, types.VerbList},
				{types.KindCertAuthority, types.VerbReadNoSecrets},
			},
		},
		{
			desc: "get authorities with secrets",
			f: func(t *testing.T, service *Service) {
				_, err := service.GetCertAuthorities(ctx, &trustpb.GetCertAuthoritiesRequest{
					Type:       string(ca.GetType()),
					IncludeKey: true,
				})

				require.NoError(t, err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindCertAuthority, types.VerbList}:          true,
						{types.KindCertAuthority, types.VerbReadNoSecrets}: true,
						{types.KindCertAuthority, types.VerbRead}:          true,
					},
				},
			},
			expectChecks: []check{
				{types.KindCertAuthority, types.VerbList},
				{types.KindCertAuthority, types.VerbReadNoSecrets},
				{types.KindCertAuthority, types.VerbRead},
			},
		},
		{
			desc: "delete no access",
			f: func(t *testing.T, service *Service) {
				_, err := service.DeleteCertAuthority(ctx, &trustpb.DeleteCertAuthorityRequest{
					Type:   string(ca.GetType()),
					Domain: ca.GetClusterName(),
				})

				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindCertAuthority, types.VerbDelete}: false,
					},
				},
			},
			expectChecks: []check{{types.KindCertAuthority, types.VerbDelete}},
		},
		{
			desc: "delete",
			f: func(t *testing.T, service *Service) {
				_, err := service.DeleteCertAuthority(ctx, &trustpb.DeleteCertAuthorityRequest{
					Type:   string(ca.GetType()),
					Domain: ca.GetClusterName(),
				})

				require.NoError(t, err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindCertAuthority, types.VerbDelete}: true,
					},
				},
			},
			expectChecks: []check{{types.KindCertAuthority, types.VerbDelete}},
		},
		{
			desc: "upsert without create",
			f: func(t *testing.T, service *Service) {
				_, err := service.UpsertCertAuthority(ctx, &trustpb.UpsertCertAuthorityRequest{
					CertAuthority: newCertAuthority(t, types.UserCA, "user").(*types.CertAuthorityV2),
				})

				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindCertAuthority, types.VerbCreate}: false,
						{types.KindCertAuthority, types.VerbUpdate}: true,
					},
				},
			},
			expectChecks: []check{
				{types.KindCertAuthority, types.VerbCreate},
				{types.KindCertAuthority, types.VerbUpdate},
			},
		},
		{
			desc: "upsert without update",
			f: func(t *testing.T, service *Service) {
				_, err := service.UpsertCertAuthority(ctx, &trustpb.UpsertCertAuthorityRequest{
					CertAuthority: newCertAuthority(t, types.UserCA, "user").(*types.CertAuthorityV2),
				})

				require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindCertAuthority, types.VerbCreate}: true,
						{types.KindCertAuthority, types.VerbUpdate}: false,
					},
				},
			},
			expectChecks: []check{
				{types.KindCertAuthority, types.VerbCreate},
				{types.KindCertAuthority, types.VerbUpdate},
			},
		},
		{
			desc: "upsert",
			f: func(t *testing.T, service *Service) {
				ca, err := service.UpsertCertAuthority(ctx, &trustpb.UpsertCertAuthorityRequest{
					CertAuthority: newCertAuthority(t, types.UserCA, "user").(*types.CertAuthorityV2),
				})
				require.NoError(t, err)
				require.NotNil(t, ca)
			},
			authorizer: fakeAuthorizer{
				checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindCertAuthority, types.VerbCreate}: true,
						{types.KindCertAuthority, types.VerbUpdate}: true,
					},
				},
			},
			expectChecks: []check{
				{types.KindCertAuthority, types.VerbCreate},
				{types.KindCertAuthority, types.VerbUpdate},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			p := newTestPack(t)

			trust := local.NewCAService(p.mem)
			cfg := &ServiceConfig{
				Cache:      trust,
				Backend:    trust,
				Authorizer: &test.authorizer,
				AuthServer: &fakeAuthServer{},
			}

			service, err := NewService(cfg)
			require.NoError(t, err)

			require.NoError(t, trust.CreateCertAuthority(ctx, ca))

			test.f(t, service)
			require.ElementsMatch(t, test.expectChecks, test.authorizer.checker.checks)
		})
	}
}

func TestGetCertAuthority(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	authorizer := &fakeAuthorizer{
		checker: &fakeChecker{
			allow: map[check]bool{
				{types.KindCertAuthority, types.VerbReadNoSecrets}: true,
				{types.KindCertAuthority, types.VerbRead}:          true,
			},
		},
	}

	trust := local.NewCAService(p.mem)
	cfg := &ServiceConfig{
		Cache:      trust,
		Backend:    trust,
		Authorizer: authorizer,
		AuthServer: &fakeAuthServer{},
	}

	service, err := NewService(cfg)
	require.NoError(t, err)

	// bootstrap a CA
	ca := newCertAuthority(t, types.HostCA, "test")
	require.NoError(t, trust.CreateCertAuthority(ctx, ca))

	tests := []struct {
		name      string
		request   *trustpb.GetCertAuthorityRequest
		assertion func(t *testing.T, authority types.CertAuthority, err error)
	}{
		{
			name: "ca not found",
			request: &trustpb.GetCertAuthorityRequest{
				Type:   string(types.SAMLIDPCA),
				Domain: "unknown",
			},
			assertion: func(t *testing.T, authority types.CertAuthority, err error) {
				require.True(t, trace.IsNotFound(err))
				require.Nil(t, authority)
			},
		},
		{
			name: "ca found without secrets",
			request: &trustpb.GetCertAuthorityRequest{
				Type:   string(types.HostCA),
				Domain: "test",
			},
			assertion: func(t *testing.T, authority types.CertAuthority, err error) {
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(authority, ca,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
					cmpopts.IgnoreFields(types.SSHKeyPair{}, "PrivateKey"),
					cmpopts.IgnoreFields(types.TLSKeyPair{}, "Key"),
					cmpopts.IgnoreFields(types.JWTKeyPair{}, "PrivateKey"),
				))
				keys := authority.GetActiveKeys()
				require.Nil(t, keys.TLS[0].Key)
				require.Nil(t, keys.SSH[0].PrivateKey)
				require.Nil(t, keys.JWT[0].PrivateKey)
			},
		},
		{
			name: "ca found with secrets",
			request: &trustpb.GetCertAuthorityRequest{
				Type:       string(types.HostCA),
				Domain:     "test",
				IncludeKey: true,
			},
			assertion: func(t *testing.T, authority types.CertAuthority, err error) {
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(authority, ca, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ca, err := service.GetCertAuthority(ctx, test.request)
			test.assertion(t, ca, err)
		})
	}
}

func TestGetCertAuthorities(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	authorizer := &fakeAuthorizer{
		checker: &fakeChecker{
			allow: map[check]bool{
				{types.KindCertAuthority, types.VerbReadNoSecrets}: true,
				{types.KindCertAuthority, types.VerbRead}:          true,
				{types.KindCertAuthority, types.VerbList}:          true,
			},
		},
	}

	trust := local.NewCAService(p.mem)
	cfg := &ServiceConfig{
		Cache:      trust,
		Backend:    trust,
		Authorizer: authorizer,
		AuthServer: &fakeAuthServer{},
	}

	service, err := NewService(cfg)
	require.NoError(t, err)

	// bootstrap CAs
	ca1 := newCertAuthority(t, types.HostCA, "test")
	require.NoError(t, trust.CreateCertAuthority(ctx, ca1))

	ca2 := newCertAuthority(t, types.HostCA, "test2")
	require.NoError(t, trust.CreateCertAuthority(ctx, ca2))

	expectedCAs := []*types.CertAuthorityV2{ca1.(*types.CertAuthorityV2), ca2.(*types.CertAuthorityV2)}

	tests := []struct {
		name      string
		request   *trustpb.GetCertAuthoritiesRequest
		assertion func(t *testing.T, resp *trustpb.GetCertAuthoritiesResponse, err error)
	}{
		{
			name: "ca type does not exist",
			request: &trustpb.GetCertAuthoritiesRequest{
				Type: string(types.SAMLIDPCA),
			},
			assertion: func(t *testing.T, resp *trustpb.GetCertAuthoritiesResponse, err error) {
				require.NoError(t, err)
				require.NotNil(t, resp)
				require.Empty(t, resp.CertAuthoritiesV2)
			},
		},
		{
			name: "ca found without secrets",
			request: &trustpb.GetCertAuthoritiesRequest{
				Type: string(types.HostCA),
			},
			assertion: func(t *testing.T, resp *trustpb.GetCertAuthoritiesResponse, err error) {
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(expectedCAs, resp.CertAuthoritiesV2,
					cmpopts.IgnoreFields(types.Metadata{}, "Revision"),
					cmpopts.IgnoreFields(types.SSHKeyPair{}, "PrivateKey"),
					cmpopts.IgnoreFields(types.TLSKeyPair{}, "Key"),
					cmpopts.IgnoreFields(types.JWTKeyPair{}, "PrivateKey"),
				))

				for _, ca := range resp.CertAuthoritiesV2 {
					keys := ca.GetActiveKeys()
					require.Nil(t, keys.TLS[0].Key)
					require.Nil(t, keys.SSH[0].PrivateKey)
					require.Nil(t, keys.JWT[0].PrivateKey)
				}
			},
		},
		{
			name: "ca found with secrets",
			request: &trustpb.GetCertAuthoritiesRequest{
				Type:       string(types.HostCA),
				IncludeKey: true,
			},
			assertion: func(t *testing.T, resp *trustpb.GetCertAuthoritiesResponse, err error) {
				require.NoError(t, err)
				require.Empty(t, cmp.Diff(expectedCAs, resp.CertAuthoritiesV2, cmpopts.IgnoreFields(types.Metadata{}, "Revision")))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			resp, err := service.GetCertAuthorities(ctx, test.request)
			test.assertion(t, resp, err)
		})
	}
}

func TestDeleteCertAuthority(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	authorizer := &fakeAuthorizer{
		checker: &fakeChecker{
			allow: map[check]bool{
				{types.KindCertAuthority, types.VerbReadNoSecrets}: true,
				{types.KindCertAuthority, types.VerbDelete}:        true,
			},
		},
	}

	trust := local.NewCAService(p.mem)
	cfg := &ServiceConfig{
		Cache:      trust,
		Backend:    trust,
		Authorizer: authorizer,
		AuthServer: &fakeAuthServer{},
	}

	service, err := NewService(cfg)
	require.NoError(t, err)

	// bootstrap a CA
	ca := newCertAuthority(t, types.HostCA, "test")
	require.NoError(t, trust.CreateCertAuthority(ctx, ca))

	tests := []struct {
		name      string
		request   *trustpb.DeleteCertAuthorityRequest
		assertion func(t *testing.T, err error)
	}{
		{
			name: "ca not found",
			request: &trustpb.DeleteCertAuthorityRequest{
				Type:   string(types.SAMLIDPCA),
				Domain: "unknown",
			},
			assertion: func(t *testing.T, err error) {
				// ca deletion doesn't generate not found errors. this is a quirk of
				// the fact that deleting active and inactive CAs simultanesouly
				// is difficult to do conditionally without introducing odd edge
				// cases (e.g. having a delete fail while appearing to succeed if it
				// races with a concurrent activation/deactivation).
				require.NoError(t, err)
			},
		},
		{
			name: "ca deleted",
			request: &trustpb.DeleteCertAuthorityRequest{
				Type:   string(types.HostCA),
				Domain: "test",
			},
			assertion: func(t *testing.T, err error) {
				require.NoError(t, err)

				ca, err := service.GetCertAuthority(ctx, &trustpb.GetCertAuthorityRequest{Domain: "test", Type: string(types.HostCA)})
				require.True(t, trace.IsNotFound(err), "got unexpected error retrieving a deleted ca: %v", err)
				require.Nil(t, ca)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.DeleteCertAuthority(ctx, test.request)
			test.assertion(t, err)
		})
	}
}

func TestUpsertCertAuthority(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	authorizer := &fakeAuthorizer{
		checker: &fakeChecker{
			allow: map[check]bool{
				{types.KindCertAuthority, types.VerbCreate}: true,
				{types.KindCertAuthority, types.VerbUpdate}: true,
			},
		},
	}

	trust := local.NewCAService(p.mem)
	cfg := &ServiceConfig{
		Cache:      trust,
		Backend:    trust,
		Authorizer: authorizer,
		AuthServer: &fakeAuthServer{},
	}

	service, err := NewService(cfg)
	require.NoError(t, err)

	hostCA := newCertAuthority(t, types.HostCA, "test").(*types.CertAuthorityV2)

	tests := []struct {
		name      string
		ca        func(ca *types.CertAuthorityV2) *types.CertAuthorityV2
		assertion func(t *testing.T, ca *types.CertAuthorityV2, err error)
	}{
		{
			name: "create ca",
			ca: func(ca *types.CertAuthorityV2) *types.CertAuthorityV2 {
				return ca
			},
			assertion: func(t *testing.T, ca *types.CertAuthorityV2, err error) {
				require.NoError(t, err)
				// Since the ca was created there should
				// be no differences returned.
				require.Empty(t, cmp.Diff(hostCA, ca))
			},
		},
		{
			name: "update ca",
			ca: func(ca *types.CertAuthorityV2) *types.CertAuthorityV2 {
				rotated := ca.Clone().(*types.CertAuthorityV2)

				rotated.Spec.Rotation = &types.Rotation{LastRotated: time.Now().UTC()}

				return rotated
			},
			assertion: func(t *testing.T, ca *types.CertAuthorityV2, err error) {
				require.NoError(t, err)

				// The ca was altered and updated so it shouldn't
				// match the original ca.
				require.NotEmpty(t, cmp.Diff(hostCA, ca))
				// Validate that only the rotation was changed
				require.Nil(t, hostCA.Spec.Rotation)
				require.NotNil(t, ca.Spec.Rotation)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ca, err := service.UpsertCertAuthority(ctx, &trustpb.UpsertCertAuthorityRequest{
				CertAuthority: test.ca(hostCA),
			})
			test.assertion(t, ca, err)
		})
	}
}

func TestRotateCertAuthority(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	authorizer := &fakeAuthorizer{
		checker: &fakeChecker{
			allow: map[check]bool{
				{types.KindCertAuthority, types.VerbCreate}: true,
				{types.KindCertAuthority, types.VerbUpdate}: true,
			},
		},
	}

	fakeErr := trace.BadParameter("bad thing happened")
	authServer := &fakeAuthServer{
		rotateCertAuthorityData: map[string]error{
			"success": nil,
			"fail":    fakeErr,
		},
	}

	trust := local.NewCAService(p.mem)
	cfg := &ServiceConfig{
		Cache:      trust,
		Backend:    trust,
		Authorizer: authorizer,
		AuthServer: authServer,
	}

	tests := []struct {
		name    string
		req     *trustpb.RotateCertAuthorityRequest
		wantErr error
	}{
		{
			name: "success",
			req: &trustpb.RotateCertAuthorityRequest{
				Type: "success",
			},
		},
		{
			name: "fail",
			req: &trustpb.RotateCertAuthorityRequest{
				Type: "fail",
			},
			wantErr: fakeErr,
		},
	}

	service, err := NewService(cfg)
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := service.RotateCertAuthority(ctx, test.req)
			if test.wantErr != nil {
				require.ErrorIs(t, err, test.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRotateExternalCertAuthority(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)
	trust := local.NewCAService(p.mem)

	localCA := newCertAuthority(t, types.HostCA, "local").(*types.CertAuthorityV2)
	externalCA := newCertAuthority(t, types.HostCA, "external").(*types.CertAuthorityV2)
	require.NoError(t, trust.UpsertCertAuthority(ctx, externalCA))

	authorizedCtx := &authz.Context{
		UnmappedIdentity: authz.BuiltinRole{},
		Checker: &fakeChecker{
			allow: map[check]bool{
				{types.KindCertAuthority, types.VerbRotate}: true,
			},
		},
	}
	remoteUserCtx := &authz.Context{
		UnmappedIdentity: authz.BuiltinRole{},
		Checker: &fakeChecker{
			allow: map[check]bool{
				{types.KindCertAuthority, types.VerbRotate}: true,
			},
		},
		Identity: authz.RemoteUser{
			Identity: tlsca.Identity{
				TeleportCluster: "external",
			},
		},
	}

	tests := []struct {
		name        string
		authzCtx    *authz.Context
		ca          *types.CertAuthorityV2
		assertError require.ErrorAssertionFunc
	}{
		{
			name: "NOK unauthorized user",
			authzCtx: &authz.Context{
				UnmappedIdentity: authz.LocalUser{},
				Checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindCertAuthority, types.VerbRotate}: true,
					},
				},
			},
			ca: externalCA,
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		}, {
			name: "NOK unauthorized service",
			authzCtx: &authz.Context{
				UnmappedIdentity: authz.BuiltinRole{},
				Checker: &fakeChecker{
					allow: map[check]bool{
						{types.KindCertAuthority, types.VerbRotate}: false,
					},
				},
			},
			ca: externalCA,
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsAccessDenied(err), "expected access denied error but got %v", err)
			},
		}, {
			name:     "NOK no ca",
			authzCtx: authorizedCtx,
			ca:       nil,
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsBadParameter(err))
			},
		}, {
			name:     "NOK invalid ca",
			authzCtx: authorizedCtx,
			ca:       &types.CertAuthorityV2{},
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsBadParameter(err))
			},
		}, {
			name:     "NOK rotate local ca",
			authzCtx: remoteUserCtx,
			ca:       localCA,
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsBadParameter(err))
			},
		}, {
			name:     "NOK nonexistent ca",
			authzCtx: remoteUserCtx,
			ca:       newCertAuthority(t, types.HostCA, "na").(*types.CertAuthorityV2),
			assertError: func(tt require.TestingT, err error, i ...interface{}) {
				require.True(tt, trace.IsBadParameter(err))
			},
		}, {
			name:        "OK rotate external ca",
			authzCtx:    remoteUserCtx,
			ca:          newCertAuthority(t, types.HostCA, "external").(*types.CertAuthorityV2),
			assertError: require.NoError,
		}, {
			name:        "OK equivalent external ca",
			authzCtx:    remoteUserCtx,
			ca:          externalCA,
			assertError: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cfg := &ServiceConfig{
				Cache:   trust,
				Backend: trust,
				Authorizer: &fakeAuthorizer{
					authzCtx: test.authzCtx,
				},
				AuthServer: &fakeAuthServer{
					clusterName: &types.ClusterNameV2{
						Spec: types.ClusterNameSpecV2{
							ClusterName: "local",
						},
					},
				},
			}

			service, err := NewService(cfg)
			require.NoError(t, err)

			_, err = service.RotateExternalCertAuthority(ctx, &trustpb.RotateExternalCertAuthorityRequest{
				CertAuthority: test.ca,
			})
			test.assertError(t, err, "RotateExternalCertAuthority error mismatch")
		})
	}
}

func TestGenerateHostCert(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	authorizer := &fakeAuthorizer{
		checker: &fakeChecker{
			allow: map[check]bool{
				{types.KindHostCert, types.VerbCreate}: true,
			},
		},
	}

	hostCertSigner := &fakeAuthServer{
		generateHostCertData: map[string]struct {
			cert []byte
			err  error
		}{
			"success": {
				cert: []byte("foo"),
			},
			"fail": {
				err: trace.BadParameter("bad thing happened"),
			},
		},
	}

	trust := local.NewCAService(p.mem)
	cfg := &ServiceConfig{
		Cache:      trust,
		Backend:    trust,
		Authorizer: authorizer,
		AuthServer: hostCertSigner,
	}

	tests := []struct {
		name string
		req  *trustpb.GenerateHostCertRequest

		want    *trustpb.GenerateHostCertResponse
		wantErr string
	}{
		{
			name: "success",
			req: &trustpb.GenerateHostCertRequest{
				HostId: "success",
			},

			want: &trustpb.GenerateHostCertResponse{
				SshCertificate: []byte("foo"),
			},
		},
		{
			name: "fail",
			req: &trustpb.GenerateHostCertRequest{
				HostId: "fail",
			},

			wantErr: "bad thing happened",
		},
	}

	service, err := NewService(cfg)
	require.NoError(t, err)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := service.GenerateHostCert(ctx, test.req)
			if test.wantErr != "" {
				require.EqualError(t, err, test.wantErr)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.want, got)
		})
	}
}
