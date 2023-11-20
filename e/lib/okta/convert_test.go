package okta

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	eteleport "github.com/gravitational/teleport/e/lib/teleport"
)

func TestBase36Encode(t *testing.T) {
	charset := "0123456789abcdefghijklmnopqrstuvwxyz"

	for i := 0; i < 36; i++ {
		require.Equal(t, string(charset[i]), base36Encode([]byte{byte(i)}))
	}

	require.Equal(t, "1z", base36Encode(big.NewInt(int64(36+35)).Bytes()))
}

func TestOktaGroupToUserGroup(t *testing.T) {
	oktaGroup := &okta.Group{
		Id: "okta-group-id",
	}

	ap := newTestAccessPoint(t, clockwork.NewRealClock())
	service, _, _ := newTestService(t, ap)
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
			types.OriginLabel:          types.OriginOkta,
			eteleport.OktaOrgURLLabel:  service.orgURL,
			eteleport.OktaGroupIDLabel: "okta-group-id",
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
			types.OriginLabel:          types.OriginOkta,
			eteleport.OktaOrgURLLabel:  service.orgURL,
			eteleport.OktaGroupIDLabel: "okta-group-id",
		},
	}, types.UserGroupSpecV1{
		Applications: []string{"app1", "app2"},
	})
	require.NoError(t, err)
	require.Equal(t, expected, userGroup)
}

func TestIsGroupValid(t *testing.T) {
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
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("the okta group group-id has no profile"))
			},
		},
		{
			name: "everyone",
			group: &okta.Group{
				Id: "group-id",
				Profile: &okta.GroupProfile{
					Name: oktaGroupEveryone,
				},
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
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
	tests := []struct {
		name             string
		oktaApp          *okta.Application
		groupIDs         []string
		errAssertionFunc require.ErrorAssertionFunc
		expected         []*types.AppV3
	}{
		{
			name: "happy path (with group IDs)",
			oktaApp: &okta.Application{
				Id:     "app-id",
				Name:   "app-name",
				Status: "ACTIVE",
				Label:  "app label",
				Links: map[string]interface{}{
					"appLinks": []interface{}{
						map[string]interface{}{
							"name": "applink-name1",
							"href": "https://www.link1.com",
						},
						map[string]interface{}{
							"name": "applink-name2",
							"href": "https://www.link2.com",
						},
					},
				},
			},
			groupIDs:         []string{"group1", "group2", "group3"},
			errAssertionFunc: require.NoError,
			expected: []*types.AppV3{
				newApp(t,
					types.Metadata{
						Name:        "3cjffnnvq17sgg",
						Description: "app label",
						Labels: map[string]string{
							types.OriginLabel:         types.OriginOkta,
							eteleport.OktaOrgURLLabel: testOrgURL,
							eteleport.OktaAppIDLabel:  "app-id",
						},
					},
					types.AppSpecV3{
						URI:        "https://www.link1.com",
						PublicAddr: fmt.Sprintf("3cjffnnvq17sgg.%s", testClusterName),
						UserGroups: []string{"group1", "group2", "group3"},
					},
				),
				newApp(t,
					types.Metadata{
						Name:        "4nmi1dlgr9wc9z",
						Description: "app label",
						Labels: map[string]string{
							types.OriginLabel:         types.OriginOkta,
							eteleport.OktaOrgURLLabel: testOrgURL,
							eteleport.OktaAppIDLabel:  "app-id",
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
				Links: map[string]interface{}{
					"appLinks": []interface{}{
						map[string]interface{}{
							"name": "applink-name1",
							"href": "https://www.link1.com",
						},
						map[string]interface{}{
							"name": "applink-name2",
							"href": "https://www.link2.com",
						},
					},
				},
			},
			errAssertionFunc: require.NoError,
			expected: []*types.AppV3{
				newApp(t,
					types.Metadata{
						Name:        "3cjffnnvq17sgg",
						Description: "app label",
						Labels: map[string]string{
							types.OriginLabel:         types.OriginOkta,
							eteleport.OktaOrgURLLabel: testOrgURL,
							eteleport.OktaAppIDLabel:  "app-id",
						},
					},
					types.AppSpecV3{
						URI:        "https://www.link1.com",
						PublicAddr: fmt.Sprintf("3cjffnnvq17sgg.%s", testClusterName),
					},
				),
				newApp(t,
					types.Metadata{
						Name:        "4nmi1dlgr9wc9z",
						Description: "app label",
						Labels: map[string]string{
							types.OriginLabel:         types.OriginOkta,
							eteleport.OktaOrgURLLabel: testOrgURL,
							eteleport.OktaAppIDLabel:  "app-id",
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
				Links: map[string]interface{}{
					"appLinks": []interface{}{
						map[string]interface{}{
							"name": "applink-name",
							"href": "https://wwww.link.com",
						},
					},
				},
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
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
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("links is missing in okta application object app-id (app label)"))
			},
		},
		{
			name: "no app links",
			oktaApp: &okta.Application{
				Id:     "app-id",
				Status: "ACTIVE",
				Label:  "app label",
				Links: map[string]interface{}{
					"appLinks": []interface{}{},
				},
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("app links is empty in okta application object app-id (app label)"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ap := newTestAccessPoint(t, clockwork.NewRealClock())
			service, _, _ := newTestService(t, ap)
			apps, err := service.oktaAppToApp(test.oktaApp, test.groupIDs)
			test.errAssertionFunc(t, err)
			require.Equal(t, test.expected, apps)
		})
	}
}

func TestIsAppValid(t *testing.T) {
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
				Status:     oktaActive,
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
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("application app-id (app label) is not active"))
			},
		},
		{
			name: "okta admin console",
			app: &okta.Application{
				Id:     "app-id",
				Label:  "Okta Admin Console",
				Status: oktaActive,
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("application app-id is the Okta admin console"))
			},
		},
		{
			name: "hidden",
			app: &okta.Application{
				Id:         "app-id",
				Label:      "app label",
				Status:     oktaActive,
				Visibility: &okta.ApplicationVisibility{Hide: &okta.ApplicationVisibilityHide{Web: &trueBool}},
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("application app-id (app label) is hidden from the web"))
			},
		},
		{
			name: "only visibility present",
			app: &okta.Application{
				Id:         "app-id",
				Status:     oktaActive,
				Visibility: &okta.ApplicationVisibility{},
			},
			errAssertionFunc: require.NoError,
		},
		{
			name: "only hide present",
			app: &okta.Application{
				Id:         "app-id",
				Status:     oktaActive,
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

func TestUserConversion(t *testing.T) {
	const (
		loginName  = "scooby@the-mystery-machine.com"
		oktaUserID = "SCOOBY"
		oktaOrgURL = "https://example.okta.com"
	)

	oktaUser := &okta.User{
		Id:     oktaUserID,
		Status: "ACTIVE",
		Profile: &okta.UserProfile{
			"firstName": "Scoobert",
			"lastName":  "Doo",
			"nickName":  "Scooby",
			"login":     loginName,

			// not sure Okta profile elements can be anything other than strings,
			// but let's throw in a list of strings just to ensure we can handle
			// it
			"email": []string{loginName, "SnackFan0154@hotmail.com"},
		},
	}

	ctime := time.Date(1986, time.August, 6, 23, 58, 0, 0, time.UTC)
	mockClock := clockwork.NewFakeClockAt(ctime)
	convert := makeUserConverter(mockClock, "sso-connector", oktaOrgURL)

	// Expect the conversion to succeed
	teleportUser, err := convert(oktaUser)
	require.NoError(t, err)
	require.Equal(t, loginName, teleportUser.GetName())

	// Expect that the user is marked as an Okta-sourced user
	labels := teleportUser.GetMetadata().Labels
	require.Equal(t, types.OriginOkta, labels[types.OriginLabel])
	require.Equal(t, oktaOrgURL, labels[eteleport.OktaOrgURLLabel])
	require.Equal(t, oktaUserID, labels[eteleport.OktaUserIDLabel])

	// Expect that the user has been given the requester role by default
	require.Equal(t, []string{teleport.PresetRequesterRoleName},
		teleportUser.GetRoles())

	// Expect that the okta user profile has been converted to traits
	traits := teleportUser.GetTraits()
	require.Equal(t, []string{"Scoobert"}, traits["okta/firstName"])
	require.Equal(t, []string{"Doo"}, traits["okta/lastName"])
	require.Equal(t, []string{"Scooby"}, traits["okta/nickName"])
	require.ElementsMatch(t, []string{loginName, "SnackFan0154@hotmail.com"},
		traits["okta/email"])

	// Expect that the well-known profile elements have been filtered out...
	require.NotContains(t, traits, "okta/login")

	// Expect that the creation date has been set
	cretator := teleportUser.GetCreatedBy()
	require.Equal(t, ctime, cretator.Time)
}
