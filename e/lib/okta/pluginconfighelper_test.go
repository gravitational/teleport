package okta

import (
	"context"
	"testing"

	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"
)

func TestGetOktaGroups(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		groups    []*okta.Group
		filters   []string
		expectErr require.ErrorAssertionFunc
		expect    []*PluginConfigOktaGroup
	}{
		{
			name:      "get empty groups with no filters",
			expectErr: require.NoError,
		},
		{
			name: "get empty groups with filters",
			filters: []string{
				"group*",
			},
			expectErr: require.NoError,
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
			expect: []*PluginConfigOktaGroup{
				{Name: "group1", Description: "description"},
				{Name: "admin-group2"},
				{Name: "dev-group3"},
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
			expect: []*PluginConfigOktaGroup{
				{Name: "group1", Description: "description"},
				{Name: "dev-group3"},
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
			expect: []*PluginConfigOktaGroup{
				{Name: "admin-group2"},
				{Name: "dev-group3"},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			c := initPluginConfigHelper(t)

			for _, group := range test.groups {
				c.client.addOktaGroupToMapping(group)
			}

			ctx := context.Background()
			groups, err := c.svc.GetOktaGroups(ctx, "orgURL", "token", test.filters)
			test.expectErr(t, err)
			if test.expect == nil {
				require.Nil(t, groups)
			} else {
				require.Equal(t, test.expect, groups)
			}
		})
	}
}

func TestGetOktaApps(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		apps      []okta.App
		filters   []string
		expectErr require.ErrorAssertionFunc
		expect    []*PluginConfigOktaApp
	}{
		{
			name:      "get empty apps with no filters",
			expectErr: require.NoError,
		},
		{
			name: "get empty apps with filters",
			filters: []string{
				"group*",
			},
			expectErr: require.NoError,
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
			expect: []*PluginConfigOktaApp{
				{Name: "app1"},
				{Name: "admin-app2"},
				{Name: "dev-app3"},
			},
		},
		{
			name: "get apps with no filters, one not an app instances",
			apps: []okta.App{
				&okta.Application{Id: "1", Label: "app1"},
				&dummyOktaApp{},
				&okta.Application{Id: "3", Label: "dev-app3"},
			},
			expectErr: require.NoError,
			expect: []*PluginConfigOktaApp{
				{Name: "app1"},
				{Name: "dev-app3"},
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
			expect: []*PluginConfigOktaApp{
				{Name: "admin-app2"},
				{Name: "dev-app3"},
			},
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			c := initPluginConfigHelper(t)

			for _, app := range test.apps {
				c.client.addOktaApplicationToMapping(app)
			}

			ctx := context.Background()
			groups, err := c.svc.GetOktaApps(ctx, "orgURL", "token", test.filters)
			test.expectErr(t, err)
			if test.expect == nil {
				require.Nil(t, groups)
			} else {
				require.Equal(t, test.expect, groups)
			}
		})
	}
}

type pluginConfigHelperComponents struct {
	svc    *PluginConfigHelper
	client *testOktaClient
}

func initPluginConfigHelper(t *testing.T) *pluginConfigHelperComponents {
	t.Helper()

	testClient := newTestClient()
	svc, err := NewPluginConfigHelper(PluginConfigHelperConfig{
		oktaClientCreator: creatorFromTestClient(testClient),
	})
	require.NoError(t, err)

	return &pluginConfigHelperComponents{
		svc:    svc,
		client: testClient,
	}
}
