package okta

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

func TestBase36Encode(t *testing.T) {
	t.Parallel()
	charset := "0123456789abcdefghijklmnopqrstuvwxyz"

	for i := range 36 {
		require.Equal(t, string(charset[i]), base36Encode([]byte{byte(i)}))
	}

	require.Equal(t, "1z", base36Encode(big.NewInt(int64(36+35)).Bytes()))
}

func TestOktaGroupToUserGroup(t *testing.T) {
	t.Parallel()
	oktaGroup := &okta.Group{
		Id: "okta-group-id",
	}

	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	service, _ := newTestService(t, ap, newTestOktaClient())
	_, err := service.oktaGroupToUserGroup(oktaGroup, nil)
	require.ErrorIs(t, trace.BadParameter("the okta group okta-group-id has no profile"), err)

	oktaGroup = &okta.Group{
		Id: "okta-group-id",
		Profile: &okta.GroupProfile{
			Name:        "group name",
			Description: "group description",
		},
	}

	userGroup, err := service.oktaGroupToUserGroup(oktaGroup, nil)
	require.NoError(t, err)

	expected, err := types.NewUserGroup(types.Metadata{
		Name:        "okta-group-id",
		Description: "group name (group description)",
		Labels: map[string]string{
			types.OriginLabel:               types.OriginOkta,
			types.OktaGroupNameLabel:        "group name",
			types.OktaGroupDescriptionLabel: "group description",
			eteleport.OktaOrgURLLabel:       service.orgURL,
			eteleport.OktaGroupIDLabel:      "okta-group-id",
		},
	}, types.UserGroupSpecV1{})
	require.NoError(t, err)
	require.Equal(t, expected, userGroup)

	userGroup, err = service.oktaGroupToUserGroup(oktaGroup, []string{"app1", "app2"})
	require.NoError(t, err)

	expected, err = types.NewUserGroup(types.Metadata{
		Name:        "okta-group-id",
		Description: "group name (group description)",
		Labels: map[string]string{
			types.OriginLabel:               types.OriginOkta,
			types.OktaGroupNameLabel:        "group name",
			types.OktaGroupDescriptionLabel: "group description",
			eteleport.OktaOrgURLLabel:       service.orgURL,
			eteleport.OktaGroupIDLabel:      "okta-group-id",
		},
	}, types.UserGroupSpecV1{
		Applications: []string{"app1", "app2"},
	})
	require.NoError(t, err)
	require.Equal(t, expected, userGroup)
}

func TestIsGroupValid(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name             string
		group            *okta.Group
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			name: "is valid",
			group: &okta.Group{
				Id: "group-id",
				Profile: &okta.GroupProfile{
					Name: "group",
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "no profile",
			group: &okta.Group{
				Id: "group-id",
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("the okta group group-id has no profile"))
			},
		},
		{
			name: "everyone",
			group: &okta.Group{
				Id: "group-id",
				Profile: &okta.GroupProfile{
					Name: oktaapi.OktaGroupEveryone,
				},
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("group group-id is Everyone"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.errAssertionFunc(t, isGroupValid(test.group))
		})
	}
}

type dummyOktaApp struct{}

func (d *dummyOktaApp) IsApplicationInstance() bool {
	return false
}

func TestOktaAppToApplications(t *testing.T) {
	t.Parallel()
	trueBool := true
	tests := []struct {
		name             string
		oktaApp          *okta.Application
		groupIDs         []string
		errAssertionFunc require.ErrorAssertionFunc
		expected         []types.AppServer
	}{
		{
			name: "happy path (with group IDs)",
			oktaApp: &okta.Application{
				Id:     "app-id",
				Name:   "app-name",
				Status: "ACTIVE",
				Label:  "app label",
				Links: map[string]any{
					"appLinks": []any{
						map[string]any{
							"name": "applink-name1",
							"href": "https://www.link1.com",
						},
						map[string]any{
							"name": "applink-name2",
							"href": "https://www.link2.com",
						},
					},
				},
			},
			groupIDs:         []string{"group1", "group2", "group3"},
			errAssertionFunc: require.NoError,
			expected: []types.AppServer{
				newAppServer(t,
					types.Metadata{
						Name:        "3cjffnnvq17sgg",
						Description: "app label",
						Labels: map[string]string{
							types.OriginLabel:             types.OriginOkta,
							types.OktaAppNameLabel:        "app label",
							types.OktaAppDescriptionLabel: "applink-name1",
							eteleport.OktaOrgURLLabel:     testOrgURL,
							eteleport.OktaAppIDLabel:      "app-id",
						},
					},
					types.AppSpecV3{
						URI:        "https://www.link1.com",
						PublicAddr: fmt.Sprintf("3cjffnnvq17sgg.%s", testClusterName),
						UserGroups: []string{"group1", "group2", "group3"},
					},
				),
				newAppServer(t,
					types.Metadata{
						Name:        "4nmi1dlgr9wc9z",
						Description: "app label",
						Labels: map[string]string{
							types.OriginLabel:             types.OriginOkta,
							types.OktaAppNameLabel:        "app label",
							types.OktaAppDescriptionLabel: "applink-name2",
							eteleport.OktaOrgURLLabel:     testOrgURL,
							eteleport.OktaAppIDLabel:      "app-id",
						},
					},
					types.AppSpecV3{
						URI:        "https://www.link2.com",
						PublicAddr: fmt.Sprintf("4nmi1dlgr9wc9z.%s", testClusterName),
						UserGroups: []string{"group1", "group2", "group3"},
					},
				),
			},
		},
		{
			name: "happy path (no group IDs)",
			oktaApp: &okta.Application{
				Id:     "app-id",
				Name:   "app-name",
				Status: "ACTIVE",
				Label:  "app label",
				Links: map[string]any{
					"appLinks": []any{
						map[string]any{
							"name": "applink-name1",
							"href": "https://www.link1.com",
						},
						map[string]any{
							"name": "applink-name2",
							"href": "https://www.link2.com",
						},
					},
				},
			},
			errAssertionFunc: require.NoError,
			expected: []types.AppServer{
				newAppServer(t,
					types.Metadata{
						Name:        "3cjffnnvq17sgg",
						Description: "app label",
						Labels: map[string]string{
							types.OriginLabel:             types.OriginOkta,
							types.OktaAppNameLabel:        "app label",
							types.OktaAppDescriptionLabel: "applink-name1",
							eteleport.OktaOrgURLLabel:     testOrgURL,
							eteleport.OktaAppIDLabel:      "app-id",
						},
					},
					types.AppSpecV3{
						URI:        "https://www.link1.com",
						PublicAddr: fmt.Sprintf("3cjffnnvq17sgg.%s", testClusterName),
					},
				),
				newAppServer(t,
					types.Metadata{
						Name:        "4nmi1dlgr9wc9z",
						Description: "app label",
						Labels: map[string]string{
							types.OriginLabel:             types.OriginOkta,
							types.OktaAppNameLabel:        "app label",
							types.OktaAppDescriptionLabel: "applink-name2",
							eteleport.OktaOrgURLLabel:     testOrgURL,
							eteleport.OktaAppIDLabel:      "app-id",
						},
					},
					types.AppSpecV3{
						URI:        "https://www.link2.com",
						PublicAddr: fmt.Sprintf("4nmi1dlgr9wc9z.%s", testClusterName),
					},
				),
			},
		},
		{
			name: "not active",
			oktaApp: &okta.Application{
				Id:     "app-id",
				Status: "INACTIVE",
				Label:  "app label",
				Links: map[string]any{
					"appLinks": []any{
						map[string]any{
							"name": "applink-name",
							"href": "https://wwww.link.com",
						},
					},
				},
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("application app-id (app label) is not active"))
			},
		},
		{
			name: "empty links",
			oktaApp: &okta.Application{
				Id:     "app-id",
				Status: "ACTIVE",
				Label:  "app label",
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("links is missing in okta application object app-id (app label)"))
			},
		},
		{
			name: "no app links",
			oktaApp: &okta.Application{
				Id:     "app-id",
				Status: "ACTIVE",
				Label:  "app label",
				Links: map[string]any{
					"appLinks": []any{},
				},
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("app links is empty in okta application object app-id (app label)"))
			},
		},
		{
			name: "hidden app",
			oktaApp: &okta.Application{
				Id:     "app-id",
				Status: "ACTIVE",
				Label:  "app label",
				Links: map[string]any{
					"appLinks": []any{
						map[string]any{
							"name": "applink-name",
							"href": "https://www.link.com",
						},
					},
				},
				Visibility: &okta.ApplicationVisibility{Hide: &okta.ApplicationVisibilityHide{Web: &trueBool}},
			},
			errAssertionFunc: require.NoError,
			expected: []types.AppServer{
				newAppServer(t,
					types.Metadata{
						Name:        "33fv66f9ju37a6",
						Description: "app label",
						Labels: map[string]string{
							types.OriginLabel:             types.OriginOkta,
							types.OktaAppNameLabel:        "app label",
							types.OktaAppDescriptionLabel: "applink-name",
							eteleport.OktaOrgURLLabel:     testOrgURL,
							eteleport.OktaAppIDLabel:      "app-id",
							eteleport.OktaAppHiddenLabel:  "true",
						},
					},
					types.AppSpecV3{
						URI:        "https://www.link.com",
						PublicAddr: fmt.Sprintf("33fv66f9ju37a6.%s", testClusterName),
					},
				),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ap := newTestAccessPoint(t, clockwork.NewRealClock())
			service, _ := newTestService(t, ap, newTestOktaClient())
			apps, err := service.oktaAppToAppServers(t.Context(), test.oktaApp, test.groupIDs)
			test.errAssertionFunc(t, err)
			require.Equal(t, test.expected, apps)
		})
	}
}

func TestIsAppValid(t *testing.T) {
	t.Parallel()
	trueBool := true
	falseBool := false

	tests := []struct {
		name             string
		app              *okta.Application
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			name: "is valid",
			app: &okta.Application{
				Id:         "app-id",
				Status:     oktaapi.OktaActive,
				Visibility: &okta.ApplicationVisibility{Hide: &okta.ApplicationVisibilityHide{Web: &falseBool}},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "not active",
			app: &okta.Application{
				Id:     "app-id",
				Label:  "app label",
				Status: "INACTIVE",
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("application app-id (app label) is not active"))
			},
		},
		{
			name: "okta admin console",
			app: &okta.Application{
				Id:     "app-id",
				Label:  "Okta Admin Console",
				Status: oktaapi.OktaActive,
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(t, err, trace.BadParameter("application app-id is the Okta admin console"))
			},
		},
		{
			name: "hidden",
			app: &okta.Application{
				Id:         "app-id",
				Label:      "app label",
				Status:     oktaapi.OktaActive,
				Visibility: &okta.ApplicationVisibility{Hide: &okta.ApplicationVisibilityHide{Web: &trueBool}},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "only visibility present",
			app: &okta.Application{
				Id:         "app-id",
				Status:     oktaapi.OktaActive,
				Visibility: &okta.ApplicationVisibility{},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "only hide present",
			app: &okta.Application{
				Id:         "app-id",
				Status:     oktaapi.OktaActive,
				Visibility: &okta.ApplicationVisibility{Hide: &okta.ApplicationVisibilityHide{}},
			},
			errAssertionFunc: require.NoError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.errAssertionFunc(t, isAppValid(test.app))
		})
	}
}

func TestShortenedEncodedID(t *testing.T) {
	t.Parallel()

	length := 14

	tests := []struct {
		name     string
		id       []byte
		expected string
	}{
		{
			name:     "greater than 14",
			id:       []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1},
			expected: "47zbadltmrac8p",
		},
		{
			name:     "less than than 14",
			id:       []byte{1, 1, 1, 1, 1, 1},
			expected: "e337z3sx",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expected, shortenedEncodedID(test.id, length))
		})
	}
}
