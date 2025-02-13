// Copyright 2023 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
	checker *fakeChecker
}

func (f *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	return &authz.Context{
		Checker: f.checker,
	}, nil
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
					cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
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
				require.Empty(t, cmp.Diff(authority, ca, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
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
					cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision"),
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
				require.Empty(t, cmp.Diff(expectedCAs, resp.CertAuthoritiesV2, cmpopts.IgnoreFields(types.Metadata{}, "ID", "Revision")))
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
				require.True(t, trace.IsNotFound(err))
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
