package oktaservice

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"

	oktapb "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/okta/api"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

func TestGetOktaGroups(t *testing.T) {
	t.Parallel()
	oktaURL := "https://example.com"
	tests := []struct {
		name      string
		groups    []*okta.Group
		filters   []string
		expectErr require.ErrorAssertionFunc
		expect    *oktapb.GetGroupsResponse
	}{
		{
			name:      "get empty groups with no filters",
			expectErr: require.NoError,
			expect:    &oktapb.GetGroupsResponse{},
		},
		{
			name: "get empty groups with filters",
			filters: []string{
				"group*",
			},
			expectErr: require.NoError,
			expect:    &oktapb.GetGroupsResponse{},
		},
		{
			name: "get empty groups with bad filters",
			filters: []string{
				"^admin-.[[[*$",
			},
			expectErr: require.Error,
		},
		{
			name: "get groups with no filters",
			groups: []*okta.Group{
				{Id: "1", Profile: &okta.GroupProfile{Name: "group1", Description: "description"}},
				{Id: "2", Profile: &okta.GroupProfile{Name: "admin-group2"}},
				{Id: "3", Profile: &okta.GroupProfile{Name: "dev-group3"}},
			},
			expectErr: require.NoError,
			expect: &oktapb.GetGroupsResponse{
				Groups: []*oktapb.GetGroupsResponse_Group{
					{Name: "group1", Description: "description"},
					{Name: "admin-group2"},
					{Name: "dev-group3"},
				},
			},
		},
		{
			name: "get groups with no filters, one missing profile",
			groups: []*okta.Group{
				{Id: "1", Profile: &okta.GroupProfile{Name: "group1", Description: "description"}},
				{Id: "2"},
				{Id: "3", Profile: &okta.GroupProfile{Name: "dev-group3"}},
			},
			expectErr: require.NoError,
			expect: &oktapb.GetGroupsResponse{
				Groups: []*oktapb.GetGroupsResponse_Group{
					{Name: "group1", Description: "description"},
					{Name: "dev-group3"},
				},
			},
		},
		{
			name: "get groups with filters, don't duplicate",
			groups: []*okta.Group{
				{Id: "1", Profile: &okta.GroupProfile{Name: "group1", Description: "description"}},
				{Id: "2", Profile: &okta.GroupProfile{Name: "admin-group2"}},
				{Id: "3", Profile: &okta.GroupProfile{Name: "dev-group3"}},
			},
			filters: []string{
				"*-group*",
				"*-*",
			},
			expectErr: require.NoError,
			expect: &oktapb.GetGroupsResponse{
				Groups: []*oktapb.GetGroupsResponse_Group{
					{Name: "admin-group2"},
					{Name: "dev-group3"},
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			testClient := api.NewTestClient()
			cache, err := utils.NewFnCache(utils.FnCacheConfig{TTL: time.Minute})
			require.NoError(t, err)
			svc := Service{
				cache: cache,
				apiClientProviderFn: func(ctx context.Context, cfg api.ClientConfig) (api.Client, error) {
					return testClient, nil
				},
				logger: slog.Default(),
				authorizer: &fakeAuthorizer{
					checker: &fakeChecker{
						allow: map[check]bool{
							{types.KindPlugin, types.VerbCreate}: true,
						},
					},
				},
			}
			for _, groups := range test.groups {
				testClient.AddOktaGroupToMapping(groups)
			}

			ctx := context.Background()
			got, err := svc.GetGroups(ctx, &oktapb.GetGroupsRequest{
				ApiCredentials: &oktapb.OktaAPICredentials{
					Auth: &oktapb.OktaAPICredentials_SswsBearerToken{SswsBearerToken: "token"},
				},
				OktaOrganizationUrl: oktaURL,
				Filters:             test.filters,
			})
			test.expectErr(t, err)
			if test.expect == nil {
				require.Nil(t, got)
			} else {
				require.Equal(t, test.expect, got)
			}
		})
	}
}

func TestGetOktaApps(t *testing.T) {
	t.Parallel()
	oktaURL := "https://example.com"
	tests := []struct {
		name      string
		apps      []okta.App
		filters   []string
		expectErr require.ErrorAssertionFunc
		expect    *oktapb.GetAppsResponse
	}{
		{
			name:      "get empty apps with no filters",
			expectErr: require.NoError,
			expect:    &oktapb.GetAppsResponse{},
		},
		{
			name: "get empty apps with filters",
			filters: []string{
				"group*",
			},
			expectErr: require.NoError,
			expect:    &oktapb.GetAppsResponse{},
		},
		{
			name: "get empty apps with bad filters",
			filters: []string{
				"^admin-.[[[*$",
			},
			expectErr: require.Error,
		},
		{
			name: "get apps with no filters",
			apps: []okta.App{
				&okta.Application{Id: "1", Label: "app1"},
				&okta.Application{Id: "2", Label: "admin-app2"},
				&okta.Application{Id: "3", Label: "dev-app3"},
			},
			expectErr: require.NoError,
			expect: &oktapb.GetAppsResponse{
				Apps: []*oktapb.GetAppsResponse_App{
					{Name: "app1"},
					{Name: "admin-app2"},
					{Name: "dev-app3"},
				},
			},
		},
		{
			name: "get apps with no filters, one not an app instances",
			apps: []okta.App{
				&okta.Application{Id: "1", Label: "app1"},
				&api.DummyOktaApp{},
				&okta.Application{Id: "3", Label: "dev-app3"},
			},
			expectErr: require.NoError,
			expect: &oktapb.GetAppsResponse{
				Apps: []*oktapb.GetAppsResponse_App{
					{Name: "app1"},
					{Name: "dev-app3"},
				},
			},
		},
		{
			name: "get apps with filters, don't duplicate",
			apps: []okta.App{
				&okta.Application{Id: "1", Label: "app1"},
				&okta.Application{Id: "2", Label: "admin-app2"},
				&okta.Application{Id: "3", Label: "dev-app3"},
			},
			filters: []string{
				"*-app*",
				"*-*",
			},
			expectErr: require.NoError,
			expect: &oktapb.GetAppsResponse{
				Apps: []*oktapb.GetAppsResponse_App{
					{Name: "admin-app2"},
					{Name: "dev-app3"},
				},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			testClient := api.NewTestClient()
			cache, err := utils.NewFnCache(utils.FnCacheConfig{TTL: time.Minute})
			require.NoError(t, err)
			svc := Service{
				cache: cache,
				apiClientProviderFn: func(ctx context.Context, cfg api.ClientConfig) (api.Client, error) {
					return testClient, nil
				},
				logger: slog.Default(),
				authorizer: &fakeAuthorizer{
					checker: &fakeChecker{
						allow: map[check]bool{
							{types.KindPlugin, types.VerbCreate}: true,
						},
					},
				},
			}
			for _, app := range test.apps {
				testClient.AddOktaApplicationToMapping(app)
			}

			ctx := context.Background()
			got, err := svc.GetApps(ctx, &oktapb.GetAppsRequest{
				ApiCredentials: &oktapb.OktaAPICredentials{
					Auth: &oktapb.OktaAPICredentials_SswsBearerToken{SswsBearerToken: "token"},
				},
				OktaOrganizationUrl: oktaURL,
				Filters:             test.filters,
			})

			test.expectErr(t, err)
			if test.expect == nil {
				require.Nil(t, got)
			} else {
				require.Equal(t, test.expect, got)
			}
		})
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
