package okta

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	"github.com/gravitational/teleport"
	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	oktaservice "github.com/gravitational/teleport/e/lib/okta/service"
	common "github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils/set"
)

// TestPluginEnrollmentFullIntegration tests the full integration of the Okta plugin.
// Where all the features are enabled and the plugin is fully integrated with Teleport.
// Additionally, it tests the filtering of apps and groups.
func TestPluginEnrollmentFullIntegration(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fakeOkta1 := newFakeOktaServer(
		withAppCount(1),
		withGroupCount(1),
	)
	t.Cleanup(fakeOkta1.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta1.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta1.Client().Transport),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	pluginClient := pluginsv1.NewPluginServiceClient(sut.GetAuthServiceGRPCConn(t, "alice-admin"))

	t.Run("filter apps", func(t *testing.T) {
		var resp, err = oktaClient.GetApps(ctx, oktav1.GetAppsRequest_builder{
			OktaOrganizationUrl: fakeOkta1.URL(),
			ApiCredentials:      apiCredentials,
			Filters:             nil,
		}.Build())
		require.NoError(t, err)
		require.Len(t, resp.GetApps(), len(fakeOkta1.provisionedApps))
	})

	t.Run("filters groups", func(t *testing.T) {
		resp := mustFilterGroups(t, oktaClient, fakeOkta1.URL())
		require.Len(t, resp.GetGroups(), len(fakeOkta1.provisionedGroups))

		resp = mustFilterGroups(t, oktaClient, fakeOkta1.URL(), "no-existing-group")
		require.Empty(t, resp.GetGroups())

		resp = mustFilterGroups(t, oktaClient, fakeOkta1.URL(), "*")
		require.Len(t, resp.GetGroups(), len(fakeOkta1.provisionedGroups))

		resp = mustFilterGroups(t, oktaClient, fakeOkta1.URL(), "group-0*")
		require.True(t, strings.HasPrefix(resp.GetGroups()[0].GetName(), "group-0"))
	})

	fakeOkta2 := newFakeOktaServer()
	t.Cleanup(fakeOkta2.Stop)

	app := fakeOkta2.CreateBasicApp("example_test-okta-app-name") // matches the app name from the idp.EntityDescriptor

	resp, err := oktaClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
		OktaOrganizationUrl:       fakeOkta2.URL(),
		ApiCredentials:            apiCredentials,
		EnableAccessListSync:      true,
		EnableAppGroupSync:        true,
		EnableBidirectionalSync:   true,
		EnableUserSync:            true,
		DisableAssignDefaultRoles: false,
		AccessListSettings: oktav1.AccessListSettings_builder{
			DefaultOwner: []string{"alice-admin"},
		}.Build(),
		SsoMetadataUrl: fakeOkta2.URL() + "/sso/saml/metadata",
	}.Build())
	require.NoError(t, err)

	oktaPlugin, err := pluginClient.GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{
		Name: types.PluginTypeOkta,
	}.Build())
	require.NoError(t, err)

	expectedOktaPluginSettings := &types.PluginOktaSettings{
		OrgUrl: fakeOkta2.URL(),
		SyncSettings: &types.PluginOktaSyncSettings{
			SsoConnectorId:           "okta",
			AppId:                    app.Id,
			AppName:                  app.Label,
			SyncUsers:                true,
			UserSyncSource:           "saml_app",
			DisableSyncAppGroups:     false,
			DisableBidirectionalSync: false,
			SyncAccessLists:          true,
			DefaultOwners:            []string{"alice-admin"},
		},
		CredentialsInfo: &types.PluginOktaCredentialsInfo{
			HasOauthCredentials: true,
		},
	}
	require.Equal(t, expectedOktaPluginSettings, oktaPlugin.Spec.GetOkta())

	require.NotEmpty(t, resp.GetConnectorInfo().GetOktaAppId())
	_, ok := fakeOkta2.Application(resp.GetConnectorInfo().GetOktaAppId())
	require.True(t, ok)
	samlConnector, err := sut.Teleport.Process.GetAuthServer().GetSAMLConnector(ctx, "okta", false)
	require.NoError(t, err)
	require.Equal(t, fakeOkta2.URL(), samlConnector.GetMetadata().Labels[types.OktaOrgURLLabel])
}

// TestPluginEnrollmentSSOMetadataURL tests the enrollment of the Okta plugin where the SSO metadata URL is provided
// and the SAML connector is created based on the provided metadata URL.
func TestPluginEnrollmentSSOMetadataURLOnly(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fakeOkta := newFakeOktaServer()
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)

	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	_, err := oktaClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
		ScimToken:                 "12345",
		SsoMetadataUrl:            fakeOkta.URL() + "/sso/saml/metadata",
		DisableAssignDefaultRoles: false,
	}.Build())
	require.NoError(t, err)
	resp, err := sut.Teleport.Process.GetAuthServer().GetSAMLConnector(ctx, "okta", false)
	require.NoError(t, err)
	require.Equal(t, fakeOkta.URL(), resp.GetMetadata().Labels[types.OktaOrgURLLabel])
}

func oktaUserToSCIMUser(oktaUser *okta.User) *oktaSCIMUser {
	login := oktaUserLogin(oktaUser)
	return &oktaSCIMUser{
		ExternalID: oktaUser.Id,
		ID:         login,
		UserName:   login,
		Groups:     []string{},
	}
}

type oktaSCIMUser struct {
	ExternalID string `json:"externalId"`
	ID         string `json:"id"`
	Meta       struct {
		Created  time.Time `json:"created"`
		Location string    `json:"location"`
		Version  string    `json:"version"`
	} `json:"meta"`
	Schemas  []string `json:"schemas"`
	UserName string   `json:"userName"`
	Name     struct {
		GivenName  string `json:"givenName"`
		FamilyName string `json:"familyName"`
	} `json:"name"`
	Emails []struct {
		Primary bool   `json:"primary"`
		Value   string `json:"value"`
		Type    string `json:"type"`
	} `json:"emails"`
	DisplayName string   `json:"displayName"`
	Locale      string   `json:"locale"`
	Groups      []string `json:"groups"`
}

func pushSCIMUserCreate(t *testing.T, sut *common.SUT, user *okta.User, scimToken string) int {
	httpClient := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	u := url.URL{
		Scheme: "https",
		Path:   "/v1/webapi/scim/okta/Users",
		Host:   sut.ProxyAddr,
	}
	buff, err := json.Marshal(oktaUserToSCIMUser(user))
	require.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, u.String(), bytes.NewReader(buff))
	require.NoError(t, err)
	req.Header.Add("Authorization", "Bearer "+scimToken)

	resp, err := httpClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	return resp.StatusCode
}

func TestPluginEnrollmentPartialSteps(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fakeOkta := newFakeOktaServer(
		withAppCount(2),
		withUserCount(3),
		withGroupCount(3),
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	for _, user := range fakeOkta.provisionedUsers {
		fakeOkta.AddUserToGroup(fakeOkta.provisionedGroups[0].Id, user.Id)
	}

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	pluginClient := pluginsv1.NewPluginServiceClient(sut.GetAuthServiceGRPCConn(t, "alice-admin"))
	pluginWatcher := sut.NewResourceWatcher(t, types.KindPlugin)
	accessListWatcher := sut.NewResourceWatcher(t, types.KindAccessList)
	userGroupWatcher := sut.NewResourceWatcher(t, types.KindUserGroup)

	everyoneGroup := fakeOkta.CreateBuiltInGroup("Everyone")

	for _, user := range fakeOkta.provisionedUsers {
		fakeOkta.AddUserToGroup(everyoneGroup.Id, user.Id)
	}

	// Assign all users to this app for user sync.
	for _, u := range fakeOkta.provisionedUsers {
		err := fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, u.Id)
		require.NoError(t, err)
	}

	scimToken := uuid.NewString()
	t.Run("enroll okta integration with SCIM only", func(t *testing.T) {
		_, err := oktaClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
			ScimToken:                 scimToken,
			EnableAccessListSync:      false,
			EnableAppGroupSync:        false,
			EnableBidirectionalSync:   false,
			EnableUserSync:            false,
			DisableAssignDefaultRoles: false,
			ReuseConnector:            "okta-pre-created-test",
		}.Build())
		require.NoError(t, err)

		oktaPlugin, err := pluginClient.GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{
			Name: types.PluginTypeOkta,
		}.Build())
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: fakeOkta.URL(),
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           "okta-pre-created-test",
				UserSyncSource:           "saml_app",
				DisableSyncAppGroups:     true,
				DisableBidirectionalSync: true,
			},
			CredentialsInfo: &types.PluginOktaCredentialsInfo{
				HasScimToken: true,
			},
		}
		require.Equal(t, expectedOktaPluginSettings, oktaPlugin.Spec.GetOkta())

		common.WaitForPutEvent(t, pluginWatcher, func(p types.Plugin) bool {
			return p.GetName() == types.PluginTypeOkta && p.GetStatus().GetCode() == types.PluginStatusCode_RUNNING
		})

		pushSCIMUserCreate(t, sut, fakeOkta.provisionedUsers[0], scimToken)
	})

	t.Run("extend okta integration and enable user sync", func(t *testing.T) {
		from := time.Now()
		mustUpdateOktaIntegration(ctx, t, oktaClient, oktav1.UpdateIntegrationRequest_builder{
			ApiCredentials:            apiCredentials,
			EnableUserSync:            true,
			DisableAssignDefaultRoles: false,
		}.Build())

		oktaPlugin, err := pluginClient.GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{
			Name: types.PluginTypeOkta,
		}.Build())
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: fakeOkta.URL(),
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           "okta-pre-created-test",
				AppId:                    fakeOkta.provisionedSAMLApp.Id,
				AppName:                  fakeOkta.provisionedSAMLApp.Name,
				SyncUsers:                true,
				UserSyncSource:           "saml_app",
				DisableSyncAppGroups:     true,
				DisableBidirectionalSync: true,
			},
			CredentialsInfo: &types.PluginOktaCredentialsInfo{
				HasOauthCredentials: true,
				HasScimToken:        true,
			},
		}
		require.Equal(t, expectedOktaPluginSettings, oktaPlugin.Spec.GetOkta())

		mustWaitForEvent(t, sut, events.OktaUserSyncEvent, withTimePoint(from))
		userExistInTeleportAndIsNotLocked(t, ctx, sut.Teleport.Process.GetAuthServer(), oktaUserLogin(fakeOkta.provisionedUsers[0]))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			accessLists, err := sut.Teleport.Process.GetAuthServer().GetAccessLists(ctx)
			require.NoError(t, err)
			require.Empty(t, accessLists)

			usersGroups, _, err := sut.Teleport.Process.GetAuthServer().ListUserGroups(ctx, 0, "")
			require.NoError(t, err)
			require.Empty(t, usersGroups)

			apps, err := sut.Teleport.Process.GetAuthServer().GetApps(ctx)
			require.NoError(t, err)
			require.Empty(t, apps)
		}, time.Second*2, time.Millisecond*50)
	})

	t.Run("update integration setting and enable user sync and app groups sync", func(t *testing.T) {
		from := time.Now()
		mustUpdateOktaIntegration(ctx, t, oktaClient, oktav1.UpdateIntegrationRequest_builder{
			ApiCredentials:            apiCredentials,
			EnableUserSync:            true,
			DisableAssignDefaultRoles: false,
			EnableAppGroupSync:        true,
			EnableAccessListSync:      false,
			EnableBidirectionalSync:   true,
			AccessListSettings: oktav1.AccessListSettings_builder{
				DefaultOwner: []string{"alice-admin"},
			}.Build(),
		}.Build())

		oktaPlugin, err := pluginClient.GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{
			Name: types.PluginTypeOkta,
		}.Build())
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: fakeOkta.URL(),
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           "okta-pre-created-test",
				AppId:                    fakeOkta.provisionedSAMLApp.Id,
				AppName:                  fakeOkta.provisionedSAMLApp.Name,
				SyncUsers:                true,
				UserSyncSource:           "saml_app",
				SyncAccessLists:          false,
				DisableSyncAppGroups:     false,
				DefaultOwners:            []string{"alice-admin"},
				DisableBidirectionalSync: false,
			},
			CredentialsInfo: &types.PluginOktaCredentialsInfo{
				HasOauthCredentials: true,
				HasScimToken:        true,
			},
		}
		require.Equal(t, expectedOktaPluginSettings, oktaPlugin.Spec.GetOkta())

		mustWaitForEvent(t, sut, events.OktaGroupsUpdateEvent, withTimePoint(from))
		mustWaitForEvent(t, sut, events.OktaApplicationsUpdateEvent, withTimePoint(from))

		waitForResourceCount(t, userGroupWatcher, len(fakeOkta.provisionedGroups), func(types.UserGroup) bool { return true })

		accessLists, err := sut.Teleport.Process.GetAuthServer().GetAccessLists(ctx)
		require.NoError(t, err)
		require.Empty(t, accessLists)
	})

	t.Run("enabled full integration by turning on access list sync", func(t *testing.T) {
		from := time.Now()
		mustUpdateOktaIntegration(ctx, t, oktaClient, oktav1.UpdateIntegrationRequest_builder{
			EnableUserSync:            true,
			DisableAssignDefaultRoles: false,
			EnableAppGroupSync:        true,
			EnableAccessListSync:      true,
			EnableBidirectionalSync:   true,
			AccessListSettings: oktav1.AccessListSettings_builder{
				DefaultOwner: []string{"alice-admin"},
			}.Build(),
		}.Build())
		mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent, withTimePoint(from))

		oktaPlugin, err := pluginClient.GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{
			Name: types.PluginTypeOkta,
		}.Build())
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: fakeOkta.URL(),
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           "okta-pre-created-test",
				AppId:                    fakeOkta.provisionedSAMLApp.Id,
				AppName:                  fakeOkta.provisionedSAMLApp.Name,
				SyncUsers:                true,
				UserSyncSource:           "saml_app",
				DisableSyncAppGroups:     false,
				DisableBidirectionalSync: false,
				SyncAccessLists:          true,
				DefaultOwners:            []string{"alice-admin"},
			},
			CredentialsInfo: &types.PluginOktaCredentialsInfo{
				HasOauthCredentials: true,
				HasScimToken:        true,
			},
		}
		require.Equal(t, expectedOktaPluginSettings, oktaPlugin.Spec.GetOkta())

		waitForResourceCount(t, accessListWatcher, len(fakeOkta.provisionedGroups), func(*accesslist.AccessList) bool { return true })
	})

	t.Run("update integration setting and enable user sync and app groups sync", func(t *testing.T) {
		mustUpdateOktaIntegration(ctx, t, oktaClient, oktav1.UpdateIntegrationRequest_builder{
			EnableUserSync:            true,
			DisableAssignDefaultRoles: false,
			EnableAppGroupSync:        true,
			EnableAccessListSync:      true,
			EnableBidirectionalSync:   true,
			AccessListSettings: oktav1.AccessListSettings_builder{
				DefaultOwner: []string{"alice-admin"},
				GroupFilters: []string{fakeOkta.provisionedGroups[0].Profile.Name},
				AppFilters:   []string{"__none__"},
			}.Build(),
		}.Build())

		oktaPlugin, err := pluginClient.GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{
			Name: types.PluginTypeOkta,
		}.Build())
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: fakeOkta.URL(),
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           "okta-pre-created-test",
				AppId:                    fakeOkta.provisionedSAMLApp.Id,
				AppName:                  fakeOkta.provisionedSAMLApp.Name,
				SyncUsers:                true,
				UserSyncSource:           "saml_app",
				DisableSyncAppGroups:     false,
				DisableBidirectionalSync: false,
				SyncAccessLists:          true,
				GroupFilters:             []string{fakeOkta.provisionedGroups[0].Profile.Name},
				AppFilters:               []string{"__none__"},
				DefaultOwners:            []string{"alice-admin"},
			},
			CredentialsInfo: &types.PluginOktaCredentialsInfo{
				HasOauthCredentials: true,
				HasScimToken:        true,
			},
		}
		require.Equal(t, expectedOktaPluginSettings, oktaPlugin.Spec.GetOkta())

		mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent, withTimePoint(time.Now()))

		// The group filter now only matches provisionedGroups[0], so the access
		// lists synced for the other groups should be removed. Both deletions
		// are awaited by a single predicate (rather than one WaitForDeleteEvent
		// call per group) because the events can arrive in either order, and a
		// call waiting on a fixed ID would silently drop the other group's event.
		remaining := set.New(fakeOkta.provisionedGroups[1].Id, fakeOkta.provisionedGroups[2].Id)
		for remaining.Len() > 0 {
			common.WaitForDeleteEvent(t, accessListWatcher, func(r types.Resource) bool {
				if !remaining.Contains(r.GetName()) {
					return false
				}
				remaining.Remove(r.GetName())
				return true
			})
		}

		accessLists, err := sut.Teleport.Process.GetAuthServer().GetAccessLists(ctx)
		require.NoError(t, err)
		require.Len(t, accessLists, 1)
	})
}

func TestPluginEnrollment_OktaRequester_Role(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fakeOkta := newFakeOktaServer(
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	// Create and assign Okta SAML app users.
	user1 := fakeOkta.CreateUser("bob")
	user2 := fakeOkta.CreateUser("alice")

	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, user1.Id))
	require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, user2.Id))

	// Setup Teleport.
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaAuthClient := sut.GetOktaAuthClient(t, "alice-admin")

	t.Run("New integration with okta-requester role assignment disabled and non-existing SAML connector fails", func(t *testing.T) {
		_, err := oktaAuthClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
			ReuseConnector:            "test-connector-does-not-exist",
			SsoMetadataUrl:            fakeOkta.URL() + "/sso/saml/metadata",
			ApiCredentials:            apiCredentials,
			EnableUserSync:            true,
			DisableAssignDefaultRoles: true,
		}.Build())
		require.ErrorIs(t, err, oktaservice.DefaultRolesAssignmentDisabledError)
	})

	t.Run("New integration with okta-requester role assignment disabled", func(t *testing.T) {
		userWatcher := sut.NewResourceWatcher(t, types.KindUser)
		defer userWatcher.Close()

		_, err := oktaAuthClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
			ReuseConnector:            "okta-pre-created-test",
			ApiCredentials:            apiCredentials,
			EnableUserSync:            true,
			DisableAssignDefaultRoles: true,
		}.Build())
		require.NoError(t, err)
		updateOktaDelays(t, sut, delays{
			timeBetweenImports:                1 * time.Second,
			timeBetweenAssignmentProcessLoops: 1 * time.Second,
		})

		// Verify users don't have okta-requester assigned.
		waitForOktaOriginatedUsers(t, userWatcher, 2, func(roles []string) bool {
			return !slices.Contains(roles, teleport.SystemOktaRequesterRoleName)
		})
	})

	t.Run("Enable okta-requester role assignment", func(t *testing.T) {
		userWatcher := sut.NewResourceWatcher(t, types.KindUser)
		defer userWatcher.Close()

		mustUpdateOktaIntegration(ctx, t, oktaAuthClient,
			oktav1.UpdateIntegrationRequest_builder{
				EnableUserSync:            true,
				DisableAssignDefaultRoles: false,
			}.Build())

		// UpdateIntegration resets TimeBetweenImports to default
		// so the delays have to be re-applied to keep the plugin
		// syncing on a test-friendly interval.
		updateOktaDelays(t, sut, delays{
			timeBetweenImports:                1 * time.Second,
			timeBetweenAssignmentProcessLoops: 1 * time.Second,
		})

		// Verify have okta-requester assigned.
		waitForOktaOriginatedUsers(t, userWatcher, 2, func(roles []string) bool {
			return slices.Contains(roles, teleport.SystemOktaRequesterRoleName)
		})
	})

	t.Run("Disable okta-requester role assignment again", func(t *testing.T) {
		userWatcher := sut.NewResourceWatcher(t, types.KindUser)
		defer userWatcher.Close()

		mustUpdateOktaIntegration(ctx, t, oktaAuthClient,
			oktav1.UpdateIntegrationRequest_builder{
				EnableUserSync:            true,
				DisableAssignDefaultRoles: true,
			}.Build())

		updateOktaDelays(t, sut, delays{
			timeBetweenImports:                1 * time.Second,
			timeBetweenAssignmentProcessLoops: 1 * time.Second,
		})

		// Verify don't have okta-requester assigned.
		waitForOktaOriginatedUsers(t, userWatcher, 2, func(roles []string) bool {
			return !slices.Contains(roles, teleport.SystemOktaRequesterRoleName)
		})
	})
}

// waitForOktaOriginatedUsers blocks until the watcher reports the expected
// number of distinct kta-originated users whose roles satisfy the predicate.
func waitForOktaOriginatedUsers(t *testing.T, watcher types.Watcher, userCount int, rolesOK func(roles []string) bool) {
	t.Helper()

	seen := make(map[string]struct{})
	common.WaitForPutEvent(t, watcher, func(u types.User) bool {
		if v, _ := u.GetLabel(types.OriginLabel); v != types.OriginOkta {
			return false
		}
		if rolesOK(u.GetRoles()) {
			seen[u.GetName()] = struct{}{}
		} else {
			delete(seen, u.GetName())
		}
		return len(seen) >= userCount
	})
}

func TestPluginEnrollmentErrors(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	scimToken := uuid.NewString()

	fakeOkta := newFakeOktaServer(
		withAppCount(3),
		withUserCount(3),
		withGroupCount(3),
	)
	t.Cleanup(fakeOkta.Stop)

	fakeOkta.AddUserToGroup(fakeOkta.provisionedGroups[0].Id, fakeOkta.provisionedUsers[0].Id)
	fakeOkta.AddUserToGroup(fakeOkta.provisionedGroups[1].Id, fakeOkta.provisionedUsers[1].Id)
	fakeOkta.AddUserToGroup(fakeOkta.provisionedGroups[2].Id, fakeOkta.provisionedUsers[2].Id)

	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")

	everyoneGroup := fakeOkta.CreateBuiltInGroup("Everyone")

	for _, user := range fakeOkta.provisionedUsers {
		fakeOkta.AddUserToGroup(everyoneGroup.Id, user.Id)
	}

	t.Run("try to configure scim integration without any okta connector", func(t *testing.T) {
		_, err := oktaClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
			OktaOrganizationUrl:     fakeOkta.URL(),
			ScimToken:               scimToken,
			EnableAccessListSync:    false,
			EnableAppGroupSync:      false,
			EnableUserSync:          false,
			EnableBidirectionalSync: false,
		}.Build())
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("okta client is missing permission to create okta SAML application", func(t *testing.T) {
		fakeOkta.SetScopes([]string{oktaapi.ScopeAppsRead, oktaapi.ScopeGroupsRead, oktaapi.ScopeUserRead})
		_, err := oktaClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
			ApiCredentials:          oktav1.OktaAPICredentials_builder{OauthId: proto.String("12345")}.Build(),
			OktaOrganizationUrl:     fakeOkta.URL(),
			ScimToken:               scimToken,
			EnableAccessListSync:    false,
			EnableAppGroupSync:      false,
			EnableUserSync:          false,
			EnableBidirectionalSync: false,
		}.Build())
		require.Error(t, err)
	})
}

func mustFilterGroups(t *testing.T, oktaClient oktav1.OktaServiceClient, orgURL string, filters ...string) *oktav1.GetGroupsResponse {
	resp, err := oktaClient.GetGroups(t.Context(), oktav1.GetGroupsRequest_builder{
		OktaOrganizationUrl: orgURL,
		ApiCredentials:      apiCredentials,
		Filters:             filters,
	}.Build())
	require.NoError(t, err)
	return resp
}

func TestEnrollmentPartialStepsFromLegacyConnector(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	const legacyConnectorName = "connector1"

	fakeOkta := newFakeOktaServer(
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	pluginClient := pluginsv1.NewPluginServiceClient(sut.GetAuthServiceGRPCConn(t, "alice-admin"))

	// Create a legacy SAML connector that doesn't have any label
	// The Legacy SAML connector is created manually by a user following
	// the OKTA SSO integration guide.
	connector := mustUnmarshalSAMLConnector(t, idp.TestOktaSAMLConnector(fakeOkta.URL()))

	meta := connector.GetMetadata()
	meta.Labels = map[string]string{}
	meta.Name = legacyConnectorName
	connector.SetMetadata(meta)

	_, err := sut.Teleport.Process.GetAuthServer().CreateSAMLConnector(ctx, connector)
	require.NoError(t, err)

	t.Run("enroll okta integration with user sync and legacy credentials", func(t *testing.T) {
		resp, err := oktaClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
			ApiCredentials:            oktav1.OktaAPICredentials_builder{SswsBearerToken: proto.String("token")}.Build(),
			EnableUserSync:            true,
			DisableAssignDefaultRoles: false,
			ReuseConnector:            legacyConnectorName,
		}.Build())
		require.NoError(t, err)
		require.Equal(t, resp.GetConnectorInfo().GetOktaAppId(), fakeOkta.provisionedSAMLApp.Id)

		oktaPlugin, err := pluginClient.GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{Name: types.PluginTypeOkta}.Build())
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: fakeOkta.URL(),
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           legacyConnectorName,
				AppId:                    fakeOkta.provisionedSAMLApp.Id,
				AppName:                  fakeOkta.provisionedSAMLApp.Name,
				SyncUsers:                true,
				UserSyncSource:           "saml_app",
				DisableSyncAppGroups:     true,
				DisableBidirectionalSync: true,
			},
			CredentialsInfo: &types.PluginOktaCredentialsInfo{
				HasSsmToken: true,
			},
		}
		require.Equal(t, expectedOktaPluginSettings, oktaPlugin.Spec.GetOkta())

		cred, err := sut.Teleport.Process.GetAuthServer().GetPluginStaticCredentials(ctx, types.PluginTypeOkta)
		require.NoError(t, err)
		purpose, ok := cred.GetLabel("okta/purpose")
		require.True(t, ok)
		require.Equal(t, "okta-auth", purpose)
		require.Equal(t, "token", cred.GetAPIToken())
	})

	t.Run("update credentials", func(t *testing.T) {
		mustUpdateOktaIntegration(ctx, t, oktaClient, oktav1.UpdateIntegrationRequest_builder{
			ApiCredentials:            oktav1.OktaAPICredentials_builder{OauthId: proto.String("test_client_id_vSHak23")}.Build(),
			EnableUserSync:            true,
			DisableAssignDefaultRoles: false,
		}.Build())

		oktaPlugin, err := pluginClient.GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{Name: types.PluginTypeOkta}.Build())
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: fakeOkta.URL(),
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           legacyConnectorName,
				AppId:                    fakeOkta.provisionedSAMLApp.Id,
				AppName:                  fakeOkta.provisionedSAMLApp.Name,
				SyncUsers:                true,
				UserSyncSource:           "saml_app",
				DisableSyncAppGroups:     true,
				DisableBidirectionalSync: true,
			},
			CredentialsInfo: &types.PluginOktaCredentialsInfo{
				HasOauthCredentials: true,
			},
		}
		require.Equal(t, expectedOktaPluginSettings, oktaPlugin.Spec.GetOkta())

		cred, err := sut.Teleport.Process.GetAuthServer().GetPluginStaticCredentials(ctx, types.PluginTypeOkta)
		require.NoError(t, err)
		purpose, ok := cred.GetLabel("okta/purpose")
		require.True(t, ok)
		require.Equal(t, "okta-oauth-client-id", purpose)
		clientID, _ := cred.GetOAuthClientSecret()
		require.Equal(t, "test_client_id_vSHak23", clientID)
	})

	t.Run("fall back to org user source while updating legacy plugins", func(t *testing.T) {
		// Now let's pretend this is a legacy plugin by unsetting and app ID.
		oktaPlugin, err := pluginClient.GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{Name: types.PluginTypeOkta}.Build())
		require.NoError(t, err)
		oktaPlugin.Spec.GetOkta().GetSyncSettings().UserSyncSource = ""
		oktaPlugin.Spec.GetOkta().GetSyncSettings().AppId = ""
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			oktaPlugin, err = pluginClient.UpdatePlugin(ctx, pluginsv1.UpdatePluginRequest_builder{Plugin: oktaPlugin}.Build())
			require.NoError(t, err)
		}, time.Second*5, time.Millisecond*60)
		require.Equal(t, "unknown", oktaPlugin.Spec.GetOkta().GetSyncSettings().UserSyncSource)
		require.Empty(t, oktaPlugin.Spec.GetOkta().GetSyncSettings().AppId)

		// Now updating integration should populate back the app ID and set user sync source to
		// "org" (because app ID was not set).
		mustUpdateOktaIntegration(ctx, t, oktaClient, oktav1.UpdateIntegrationRequest_builder{
			EnableUserSync:            true,
			DisableAssignDefaultRoles: false,
		}.Build())

		oktaPlugin, err = pluginClient.GetPlugin(ctx, pluginsv1.GetPluginRequest_builder{Name: types.PluginTypeOkta}.Build())
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: fakeOkta.URL(),
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           legacyConnectorName,
				AppId:                    fakeOkta.provisionedSAMLApp.Id,
				AppName:                  fakeOkta.provisionedSAMLApp.Name,
				SyncUsers:                true,
				UserSyncSource:           "org",
				DisableSyncAppGroups:     true,
				DisableBidirectionalSync: true,
			},
			CredentialsInfo: &types.PluginOktaCredentialsInfo{
				HasOauthCredentials: true,
			},
		}
		require.Equal(t, expectedOktaPluginSettings, oktaPlugin.Spec.GetOkta())
	})
}

// Tests OAuth scopes validation during the Create/UpdateIntegration reqeusts.
func Test_PluginEnrollment_OAuthScopes(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	fakeOkta := newFakeOktaServer(
		withSAMLApp(),
	)
	t.Cleanup(fakeOkta.Stop)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.TestOktaSAMLConnector(fakeOkta.URL())),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(fakeOkta.Client().Transport),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")

	// Try (and fail) creating an integration with insufficient OAuth scopes
	fakeOkta.SetScopes([]string{oktaapi.ScopeUserRead})

	_, err := oktaClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
		ReuseConnector:      "okta-pre-created-test",
		OktaOrganizationUrl: fakeOkta.URL(),
		ApiCredentials:      apiCredentials,
		EnableUserSync:      true,
		EnableAppGroupSync:  true,
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, `Okta OAuth scopes verification failed: scope "okta.apps.read" missing, scope "okta.groups.read" missing`)

	// Create the integration (to proceed with other tests)
	fakeOkta.SetScopes([]string{
		oktaapi.ScopeUserRead,
		oktaapi.ScopeAppsRead,
		oktaapi.ScopeGroupsRead,
	})

	_, err = oktaClient.CreateIntegration(ctx, oktav1.CreateIntegrationRequest_builder{
		ReuseConnector:      "okta-pre-created-test",
		OktaOrganizationUrl: fakeOkta.URL(),
		ApiCredentials:      apiCredentials,
		EnableUserSync:      true,
		EnableAppGroupSync:  true,
	}.Build())
	require.NoError(t, err)

	// Try (and fail) updating the integration with insufficient OAuth scopes
	fakeOkta.SetScopes([]string{
		oktaapi.ScopeUserRead,
		oktaapi.ScopeAppsRead,
	})

	_, err = oktaClient.UpdateIntegration(ctx, oktav1.UpdateIntegrationRequest_builder{
		EnableUserSync:       true,
		EnableAppGroupSync:   true,
		EnableAccessListSync: true,
		AccessListSettings: oktav1.AccessListSettings_builder{
			DefaultOwner: []string{"alice-admin"},
		}.Build(),
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, `Okta OAuth scopes verification failed: scope "okta.groups.read" missing`)

	// Try (and fail) updating the integration with even more insufficient OAuth scopes
	// (because of bidirectional sync)
	fakeOkta.SetScopes([]string{
		oktaapi.ScopeUserRead,
		oktaapi.ScopeAppsRead,
		oktaapi.ScopeGroupsRead,
	})

	_, err = oktaClient.UpdateIntegration(ctx, oktav1.UpdateIntegrationRequest_builder{
		EnableUserSync:       true,
		EnableAppGroupSync:   true,
		EnableAccessListSync: true,
		AccessListSettings: oktav1.AccessListSettings_builder{
			DefaultOwner: []string{"alice-admin"},
		}.Build(),
		EnableBidirectionalSync: true,
	}.Build())
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "Okta OAuth scopes verification failed: scope \"okta.apps.manage\" missing, scope \"okta.groups.manage\" missing")

	// Disabling whole sync and credentials with no scopes is ok.
	fakeOkta.SetScopes([]string{})

	mustUpdateOktaIntegration(ctx, t, oktaClient, oktav1.UpdateIntegrationRequest_builder{
		EnableUserSync:       false,
		EnableAppGroupSync:   false,
		EnableAccessListSync: false,
	}.Build())
}

func mustUnmarshalSAMLConnector(t *testing.T, input string) types.SAMLConnector {
	decoder := kyaml.NewYAMLOrJSONDecoder(strings.NewReader(input), defaults.LookaheadBufSize)
	var raw services.UnknownResource
	err := decoder.Decode(&raw)
	require.NoError(t, err)

	connector, err := services.UnmarshalSAMLConnector(raw.Raw)
	require.NoError(t, err)
	return connector
}
