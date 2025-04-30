package okta

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	kyaml "k8s.io/apimachinery/pkg/util/yaml"

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	oktaapi "github.com/gravitational/teleport/e/lib/okta/api"
	common "github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
)

var apiCredentials = &oktav1.OktaAPICredentials{
	Auth: &oktav1.OktaAPICredentials_OauthId{
		OauthId: "12345",
	},
}

// TestPluginEnrolmentFullIntegration tests the full integration of the Okta plugin.
// Where all the features are enabled and the plugin is fully integrated with Teleport.
// Additionally, it tests the filtering of apps and groups.
func TestPluginEnrolmentFullIntegration(t *testing.T) {
	ctx := context.Background()
	oktaApiClientMock := newMockOktaAPIClient("https://trial-1234567.okta.com")
	oktaInfra := createOktaSetup(t, ctx, oktaApiClientMock, withAppsGroupsUsersCount(1, 10, 7))
	httpMock := RoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/sso/saml/metadata") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(idp.EntityDescriptor)),
				Header:     http.Header{"Content-Type": []string{"application/xml"}},
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
		}, nil
	})
	oktaApiClientMock.setRoundTripper(httpMock)

	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(httpMock),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	pluginClient := pluginsv1.NewPluginServiceClient(sut.GetAuthServiceGRPCConn(t, "alice-admin"))

	t.Run("filter apps", func(t *testing.T) {
		var resp, err = oktaClient.GetApps(ctx, &oktav1.GetAppsRequest{
			OktaOrganizationUrl: oktaApiClientMock.GetOrgUrl(),
			ApiCredentials:      apiCredentials,
			Filters:             nil,
		})
		require.NoError(t, err)
		require.Len(t, resp.GetApps(), len(oktaInfra.Apps))
	})

	t.Run("filters groups", func(t *testing.T) {
		resp := mustFilterGroups(t, oktaClient, nil)
		require.Len(t, resp.GetGroups(), len(oktaInfra.Groups))

		resp = mustFilterGroups(t, oktaClient, []string{"no-existing-group"})
		require.Empty(t, resp.GetGroups())

		resp = mustFilterGroups(t, oktaClient, []string{"*"})
		require.Len(t, resp.GetGroups(), len(oktaInfra.Groups))

		resp = mustFilterGroups(t, oktaClient, []string{"group-0*"})
		require.True(t, strings.HasPrefix(resp.GetGroups()[0].GetName(), "group-0"))
	})

	t.Run("enroll okta integration", func(t *testing.T) {
		mustCreateOktaEveryoneGroupAndAssignOktaUsers(t, oktaInfra.client, oktaInfra.Users...)

		resp, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
			OktaOrganizationUrl:     oktaApiClientMock.GetOrgUrl(),
			ApiCredentials:          apiCredentials,
			EnableUserSync:          true,
			EnableAppGroupSync:      true,
			EnableBidirectionalSync: true,
			EnableAccessListSync:    true,
			AccessListSettings: &oktav1.AccessListSettings{
				DefaultOwner: []string{"alice-admin"},
			},
		})
		require.NoError(t, err)

		oktaPlugin, err := pluginClient.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
			Name: types.PluginTypeOkta,
		})
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: oktaApiClientMock.GetOrgUrl(),
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           "okta",
				AppId:                    resp.GetConnectorInfo().GetOktaAppId(),
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
		_, _, err = oktaInfra.client.GetApplication(ctx, resp.GetConnectorInfo().GetOktaAppId(), &okta.SamlApplication{}, nil)
		require.NoError(t, err)
		_, err = sut.Teleport.Process.GetAuthServer().GetSAMLConnector(ctx, "okta", false)
		require.NoError(t, err)
	})
}

// TestPluginEnrolmentSSOMetadataURL tests the enrolment of the Okta plugin where the SSO metadata URL is provided
// and the SAML connector is created based on the provided metadata URL.
func TestPluginEnrolmentSSOMetadataURL(t *testing.T) {
	ctx := context.Background()

	oktaApiClientMock := newMockOktaAPIClient("https://trial-1234567.okta.com")
	oktaInfra := createOktaSetup(t, ctx, oktaApiClientMock, withAppsGroupsUsersCount(1, 1, 1))
	httpMock := RoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/sso/saml/metadata") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(idp.EntityDescriptor)),
				Header:     http.Header{"Content-Type": []string{"application/xml"}},
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
		}, nil
	})

	// The value from app name is  taken from idp.EntityDescriptor returned by the httpMock
	// above.
	oktaInfra.Apps[0].Name = "example_test-okta-app-name"

	oktaInfra.client.setRoundTripper(httpMock)
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(httpMock),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	pluginClient := pluginsv1.NewPluginServiceClient(sut.GetAuthServiceGRPCConn(t, "alice-admin"))

	resp, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		ApiCredentials:          apiCredentials,
		EnableAccessListSync:    true,
		EnableAppGroupSync:      true,
		EnableBidirectionalSync: true,
		EnableUserSync:          true,
		AccessListSettings: &oktav1.AccessListSettings{
			DefaultOwner: []string{"alice-admin"},
		},
		SsoMetadataUrl: "https://trial-1234567.okta.com/app/123487988/sso/saml/metadata",
	})
	require.NoError(t, err)

	oktaPlugin, err := pluginClient.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
		Name: types.PluginTypeOkta,
	})
	require.NoError(t, err)
	expectedOktaPluginSettings := &types.PluginOktaSettings{
		OrgUrl: oktaApiClientMock.GetOrgUrl(),
		SyncSettings: &types.PluginOktaSyncSettings{
			SsoConnectorId:           "okta",
			AppId:                    resp.GetConnectorInfo().GetOktaAppId(),
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
	_, _, err = oktaInfra.client.GetApplication(ctx, resp.GetConnectorInfo().GetOktaAppId(), &okta.SamlApplication{}, nil)
	require.NoError(t, err)
	samlConnector, err := sut.Teleport.Process.GetAuthServer().GetSAMLConnector(ctx, "okta", false)
	require.NoError(t, err)
	require.Equal(t, oktaApiClientMock.GetOrgUrl(), samlConnector.GetMetadata().Labels[types.OktaOrgURLLabel])
}

// TestPluginEnrolmentSSOMetadataURL tests the enrolment of the Okta plugin where the SSO metadata URL is provided
// and the SAML connector is created based on the provided metadata URL.
func TestPluginEnrolmentSSOMetadataURLOnly(t *testing.T) {
	ctx := context.Background()
	httpMock := RoundTripperFunc(func(request *http.Request) (*http.Response, error) {
		if strings.HasSuffix(request.URL.Path, "/sso/saml/metadata") {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(idp.EntityDescriptor)),
				Header:     http.Header{"Content-Type": []string{"application/xml"}},
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusNotFound,
		}, nil
	})

	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(httpMock),
	)

	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	_, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		ScimToken:      "12345",
		SsoMetadataUrl: "https://trial-7284229.okta.com/app/exkjel1ccet9biVnA697/sso/saml/metadata",
	})
	require.NoError(t, err)
	resp, err := sut.Teleport.Process.GetAuthServer().GetSAMLConnector(ctx, "okta", false)
	require.NoError(t, err)
	require.Equal(t, "https://trial-7284229.okta.com", resp.GetMetadata().Labels[types.OktaOrgURLLabel])
}

func TestPluginEnrolmentPartialSteps(t *testing.T) {
	ctx := context.Background()
	scimToken := uuid.NewString()
	oktaInfra := createOktaSetupTreeAppGroupUserAndBasicUserGroupAssignment(t, ctx, "https://trial-1234567.okta.com")
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	pluginClient := pluginsv1.NewPluginServiceClient(sut.GetAuthServiceGRPCConn(t, "alice-admin"))
	mustCreateOktaEveryoneGroupAndAssignOktaUsers(t, oktaInfra.client, oktaInfra.Users...)

	// Let's make the first app the Okta SAML app for the connector.
	samlApp := oktaInfra.Apps[0]
	// The value from app name is  taken from idp.SAMLConnector above.
	samlApp.Name = "trial-1234567_teleportsamlconnectorapp_1"
	// Assign all users to this app for user sync.
	for _, u := range oktaInfra.Users {
		_, _, err := oktaInfra.client.AssignUserToApplication(ctx, samlApp.Id, okta.AppUser{Id: u.Id})
		require.NoError(t, err)
	}

	t.Run("enroll okta integration with SCIM only", func(t *testing.T) {
		_, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
			ScimToken:               scimToken,
			EnableAccessListSync:    false,
			EnableAppGroupSync:      false,
			EnableBidirectionalSync: false,
			EnableUserSync:          false,
			ReuseConnector:          "okta-pre-created-test",
		})
		require.NoError(t, err)

		oktaPlugin, err := pluginClient.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
			Name: types.PluginTypeOkta,
		})
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: "https://trial-1234567.okta.com",
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

		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			oktaPlugin, err := pluginClient.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
				Name: types.PluginTypeOkta,
			})
			assert.NoError(collect, err)
			assert.Equal(collect, types.PluginStatusCode_RUNNING, oktaPlugin.GetStatus().GetCode())
		}, time.Second*2, time.Millisecond*100)

		pushSCIMUserCreate(t, sut, oktaInfra.Users[0], scimToken)
		userExistInTeleportAndIsNotLocked(t, ctx, sut.Teleport.Process.GetAuthServer(), oktaInfra.Users[0])
	})

	t.Run("extend okta integration and enable user sync", func(t *testing.T) {
		from := time.Now()
		_, err := oktaClient.UpdateIntegration(ctx, &oktav1.UpdateIntegrationRequest{
			ApiCredentials: apiCredentials,
			EnableUserSync: true,
		})
		require.NoError(t, err)

		oktaPlugin, err := pluginClient.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
			Name: types.PluginTypeOkta,
		})
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: "https://trial-1234567.okta.com",
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           "okta-pre-created-test",
				SyncUsers:                true,
				UserSyncSource:           "saml_app",
				DisableSyncAppGroups:     true,
				DisableBidirectionalSync: true,
				AppId:                    samlApp.Id,
			},
			CredentialsInfo: &types.PluginOktaCredentialsInfo{
				HasOauthCredentials: true,
				HasScimToken:        true,
			},
		}
		require.Equal(t, expectedOktaPluginSettings, oktaPlugin.Spec.GetOkta())

		mustWaitForEvent(t, sut, events.OktaUserSyncEvent, withTimeout(time.Second*5), withTimePoint(from))
		userExistInTeleportAndIsNotLocked(t, ctx, sut.Teleport.Process.GetAuthServer(), oktaInfra.Users[0])

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			accessLists, err := sut.Teleport.Process.GetAuthServer().GetAccessLists(ctx)
			assert.NoError(t, err)
			assert.Empty(t, accessLists)

			usersGroups, _, err := sut.Teleport.Process.GetAuthServer().ListUserGroups(ctx, 0, "")
			assert.NoError(t, err)
			assert.Empty(t, usersGroups)

			apps, err := sut.Teleport.Process.GetAuthServer().GetApps(ctx)
			assert.NoError(t, err)
			assert.Empty(t, apps)
		}, time.Second*2, time.Millisecond*50)
	})

	t.Run("update integration setting and enable user sync and app groups sync", func(t *testing.T) {
		from := time.Now()
		_, err := oktaClient.UpdateIntegration(ctx, &oktav1.UpdateIntegrationRequest{
			ApiCredentials:          apiCredentials,
			EnableUserSync:          true,
			EnableAppGroupSync:      true,
			EnableAccessListSync:    true,
			EnableBidirectionalSync: true,
			AccessListSettings: &oktav1.AccessListSettings{
				DefaultOwner: []string{"alice-admin"},
			},
		})
		require.NoError(t, err)

		oktaPlugin, err := pluginClient.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
			Name: types.PluginTypeOkta,
		})
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: "https://trial-1234567.okta.com",
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           "okta-pre-created-test",
				AppId:                    samlApp.Id,
				SyncUsers:                true,
				UserSyncSource:           "saml_app",
				SyncAccessLists:          true,
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

		mustWaitForEvent(t, sut, events.OktaGroupsUpdateEvent, withTimeout(time.Second*5), withTimePoint(from))
		mustWaitForEvent(t, sut, events.OktaApplicationsUpdateEvent, withTimeout(time.Second*10), withTimePoint(from))

		require.EventuallyWithT(t, func(collection *assert.CollectT) {
			accessLists, err := sut.Teleport.Process.GetAuthServer().GetAccessLists(ctx)
			assert.NoError(collection, err)
			assert.Empty(collection, accessLists)
			usersGroups, _, err := sut.Teleport.Process.GetAuthServer().ListUserGroups(ctx, 0, "")
			assert.NoError(collection, err)
			assert.Len(collection, usersGroups, len(oktaInfra.Groups))
		}, time.Second*2, time.Millisecond*100)
	})

	t.Run("enabled full integration by turing on access list sync", func(t *testing.T) {
		from := time.Now()
		require.EventuallyWithT(t, func(collect *assert.CollectT) {
			_, err := oktaClient.UpdateIntegration(ctx, &oktav1.UpdateIntegrationRequest{
				EnableUserSync:          true,
				EnableAppGroupSync:      true,
				EnableAccessListSync:    true,
				EnableBidirectionalSync: true,
				AccessListSettings: &oktav1.AccessListSettings{
					DefaultOwner: []string{"alice-admin"},
				},
			})
			require.NoError(t, err)
		}, time.Second, 200*time.Millisecond)
		mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent, withTimeout(time.Second*5), withTimePoint(from))

		oktaPlugin, err := pluginClient.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
			Name: types.PluginTypeOkta,
		})
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: "https://trial-1234567.okta.com",
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           "okta-pre-created-test",
				SyncUsers:                true,
				UserSyncSource:           "saml_app",
				DisableSyncAppGroups:     false,
				DisableBidirectionalSync: false,
				SyncAccessLists:          true,
				DefaultOwners:            []string{"alice-admin"},
				AppId:                    samlApp.Id,
			},
			CredentialsInfo: &types.PluginOktaCredentialsInfo{
				HasOauthCredentials: true,
				HasScimToken:        true,
			},
		}
		require.Equal(t, expectedOktaPluginSettings, oktaPlugin.Spec.GetOkta())

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			accessLists, err := sut.Teleport.Process.GetAuthServer().GetAccessLists(ctx)
			assert.NoError(t, err)
			assert.Len(t, accessLists, len(oktaInfra.Groups)+1 /* +1 for the SAML app being assigned to all users */)
		}, time.Second*2, time.Millisecond*100)
	})

	t.Run("update integration setting and enable user sync and app groups sync", func(t *testing.T) {
		_, err := oktaClient.UpdateIntegration(ctx, &oktav1.UpdateIntegrationRequest{
			EnableUserSync:          true,
			EnableAppGroupSync:      true,
			EnableAccessListSync:    true,
			EnableBidirectionalSync: true,
			AccessListSettings: &oktav1.AccessListSettings{
				DefaultOwner: []string{"alice-admin"},
				GroupFilters: []string{oktaInfra.Groups[0].Profile.Name},
				AppFilters:   []string{"__none__"},
			},
		})
		require.NoError(t, err)

		oktaPlugin, err := pluginClient.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
			Name: types.PluginTypeOkta,
		})
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: "https://trial-1234567.okta.com",
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           "okta-pre-created-test",
				SyncUsers:                true,
				UserSyncSource:           "saml_app",
				DisableSyncAppGroups:     false,
				DisableBidirectionalSync: false,
				SyncAccessLists:          true,
				GroupFilters:             []string{oktaInfra.Groups[0].Profile.Name},
				AppFilters:               []string{"__none__"},
				DefaultOwners:            []string{"alice-admin"},
				AppId:                    samlApp.Id,
			},
			CredentialsInfo: &types.PluginOktaCredentialsInfo{
				HasOauthCredentials: true,
				HasScimToken:        true,
			},
		}
		require.Equal(t, expectedOktaPluginSettings, oktaPlugin.Spec.GetOkta())

		mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent, withTimeout(time.Second*5), withTimePoint(time.Now()))
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			accessLists, err := sut.Teleport.Process.GetAuthServer().GetAccessLists(ctx)
			assert.NoError(t, err)
			assert.Len(t, accessLists, 1)
		}, time.Second*2, time.Millisecond*100)
	})
}

func TestPluginEnrollmentErrors(t *testing.T) {
	var scimToken = uuid.NewString()
	ctx := context.Background()
	oktaApiClientMock := newMockOktaAPIClient("https://trial-1234567.okta.com")
	oktaInfra := createOktaSetup(t, ctx, oktaApiClientMock, withAppsGroupsUsersCount(3, 3, 3))
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[0].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[1].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[2].Id)

	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	mustCreateOktaEveryoneGroupAndAssignOktaUsers(t, oktaInfra.client, oktaInfra.Users...)

	t.Run("try to configure scim integration without any okta connector", func(t *testing.T) {
		_, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
			OktaOrganizationUrl:     "https://trial-1234567.okta.com",
			ScimToken:               scimToken,
			EnableAccessListSync:    false,
			EnableAppGroupSync:      false,
			EnableUserSync:          false,
			EnableBidirectionalSync: false,
		})
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("okta client is missing permission to create okta SAML application", func(t *testing.T) {
		oktaInfra.client.scopes = []string{"okta.apps.read", "okta.groups.read", "okta.users.read"}
		_, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
			ApiCredentials:          &oktav1.OktaAPICredentials{Auth: &oktav1.OktaAPICredentials_OauthId{OauthId: "12345"}},
			OktaOrganizationUrl:     "https://trial-1234567.okta.com",
			ScimToken:               scimToken,
			EnableAccessListSync:    false,
			EnableAppGroupSync:      false,
			EnableUserSync:          false,
			EnableBidirectionalSync: false,
		})
		require.Error(t, err)
	})
}

func mustFilterGroups(t *testing.T, oktaClient oktav1.OktaServiceClient, filters []string) *oktav1.GetGroupsResponse {
	resp, err := oktaClient.GetGroups(context.Background(), &oktav1.GetGroupsRequest{
		OktaOrganizationUrl: "https://trial-1234567.okta.com",
		ApiCredentials:      apiCredentials,
		Filters:             filters,
	})
	require.NoError(t, err)
	return resp
}

type RoundTripperFunc func(*http.Request) (*http.Response, error)

func (f RoundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func mustCreateOktaEveryoneGroupAndAssignOktaUsers(t *testing.T, oktaClient *mockOktaAPIClient, users ...*oktaUserType) {
	g := okta.Group{
		Type: "BUILT_IN",
		Profile: &okta.GroupProfile{
			Name: "Everyone",
		},
		Links: map[string]string{
			"href": "https://12345.okta.com/api/v1/apps/0oailpy80iMX0bjlT697/sso/saml/metadata",
			"type": "application/xml",
		},
	}
	everyoneGroup, _, err := oktaClient.CreateGroup(context.Background(), g)
	require.NoError(t, err)
	for _, user := range users {
		_, err := oktaClient.AddUserToGroup(context.Background(), everyoneGroup.Id, user.Id)
		require.NoError(t, err)
	}
}

func TestEnrolmentPartialStepsFromLegacyConnector(t *testing.T) {
	const (
		legacyConnectorName = "connector1"
	)

	ctx := context.Background()
	oktaApiClientMock := newMockOktaAPIClient("https://trial-1234567.okta.com")
	samlAPP := createOktaSAMLAPP(t, ctx, oktaApiClientMock, "trial-1234567_teleportsamlconnectorapp_1")

	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	pluginClient := pluginsv1.NewPluginServiceClient(sut.GetAuthServiceGRPCConn(t, "alice-admin"))

	// Create a legacy SAML connector that doesn't have any label
	// The Legacy SAML connector is created manually by a user following
	// the OKTA SSO integration guide.
	connector := mustUnmarshalSAMLConnector(t, idp.SAMLConnector)

	meta := connector.GetMetadata()
	meta.Labels = map[string]string{}
	meta.Name = legacyConnectorName
	connector.SetMetadata(meta)

	_, err := sut.Teleport.Process.GetAuthServer().CreateSAMLConnector(ctx, connector)
	require.NoError(t, err)

	t.Run("enroll okta integration with user sync and legacy credentials", func(t *testing.T) {
		resp, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
			ApiCredentials: &oktav1.OktaAPICredentials{Auth: &oktav1.OktaAPICredentials_SswsBearerToken{SswsBearerToken: "token"}},
			EnableUserSync: true,
			ReuseConnector: legacyConnectorName,
		})
		require.NoError(t, err)
		require.Equal(t, resp.ConnectorInfo.OktaAppId, samlAPP.Id)

		oktaPlugin, err := pluginClient.GetPlugin(ctx, &pluginsv1.GetPluginRequest{Name: types.PluginTypeOkta})
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: "https://trial-1234567.okta.com",
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           legacyConnectorName,
				AppId:                    samlAPP.Id,
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
		_, err := oktaClient.UpdateIntegration(ctx, &oktav1.UpdateIntegrationRequest{
			ApiCredentials: &oktav1.OktaAPICredentials{Auth: &oktav1.OktaAPICredentials_OauthId{OauthId: "test_client_id_vSHak23"}},
			EnableUserSync: true,
		})
		require.NoError(t, err)

		oktaPlugin, err := pluginClient.GetPlugin(ctx, &pluginsv1.GetPluginRequest{Name: types.PluginTypeOkta})
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: "https://trial-1234567.okta.com",
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           legacyConnectorName,
				AppId:                    samlAPP.Id,
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
		oktaPlugin, err := pluginClient.GetPlugin(ctx, &pluginsv1.GetPluginRequest{Name: types.PluginTypeOkta})
		require.NoError(t, err)
		oktaPlugin.Spec.GetOkta().GetSyncSettings().UserSyncSource = ""
		oktaPlugin.Spec.GetOkta().GetSyncSettings().AppId = ""
		oktaPlugin, err = pluginClient.UpdatePlugin(ctx, &pluginsv1.UpdatePluginRequest{Plugin: oktaPlugin})
		require.NoError(t, err)
		require.Equal(t, "unknown", oktaPlugin.Spec.GetOkta().GetSyncSettings().UserSyncSource)
		require.Equal(t, "", oktaPlugin.Spec.GetOkta().GetSyncSettings().AppId)

		// Now updating integration should populate back the app ID and set user sync source to
		// "org" (because app ID was not set).

		_, err = oktaClient.UpdateIntegration(ctx, &oktav1.UpdateIntegrationRequest{
			EnableUserSync: true,
		})
		require.NoError(t, err)

		oktaPlugin, err = pluginClient.GetPlugin(ctx, &pluginsv1.GetPluginRequest{Name: types.PluginTypeOkta})
		require.NoError(t, err)
		expectedOktaPluginSettings := &types.PluginOktaSettings{
			OrgUrl: "https://trial-1234567.okta.com",
			SyncSettings: &types.PluginOktaSyncSettings{
				SsoConnectorId:           legacyConnectorName,
				AppId:                    samlAPP.Id,
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
func Test_PluginEnrolment_OAuthScopes(t *testing.T) {
	ctx := context.Background()

	oktaApiClient := newMockOktaAPIClient("https://trial-1234567.okta.com")

	_ = createOktaSAMLAPP(t, ctx, oktaApiClient, "trial-1234567_teleportsamlconnectorapp_1")
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")

	// Try (and fail) creating an integration with insufficient OAuth scopes

	oktaApiClient.scopes = []string{
		oktaapi.ScopeUserRead,
	}

	_, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		ReuseConnector:      "okta-pre-created-test",
		OktaOrganizationUrl: oktaApiClient.GetOrgUrl(),
		ApiCredentials:      apiCredentials,
		EnableUserSync:      true,
		EnableAppGroupSync:  true,
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, `Okta OAuth scopes verification failed: scope "okta.apps.read" missing, scope "okta.groups.read" missing`)

	// Create the integration (to proceed with other tests)

	oktaApiClient.scopes = []string{
		oktaapi.ScopeUserRead,
		oktaapi.ScopeAppsRead,
		oktaapi.ScopeGroupsRead,
	}

	_, err = oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		ReuseConnector:      "okta-pre-created-test",
		OktaOrganizationUrl: oktaApiClient.GetOrgUrl(),
		ApiCredentials:      apiCredentials,
		EnableUserSync:      true,
		EnableAppGroupSync:  true,
	})
	require.NoError(t, err)

	// Try (and fail) updating the integration with insufficient OAuth scopes

	oktaApiClient.scopes = []string{
		oktaapi.ScopeUserRead,
		oktaapi.ScopeAppsRead,
	}

	_, err = oktaClient.UpdateIntegration(ctx, &oktav1.UpdateIntegrationRequest{
		EnableUserSync:       true,
		EnableAppGroupSync:   true,
		EnableAccessListSync: true,
		AccessListSettings: &oktav1.AccessListSettings{
			DefaultOwner: []string{"alice-admin"},
		},
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, `Okta OAuth scopes verification failed: scope "okta.groups.read" missing`)

	// Try (and fail) updating the integration with even more insufficient OAuth scopes
	// (because of bidirectional sync)

	oktaApiClient.scopes = []string{
		oktaapi.ScopeUserRead,
		oktaapi.ScopeAppsRead,
		oktaapi.ScopeGroupsRead,
	}

	_, err = oktaClient.UpdateIntegration(ctx, &oktav1.UpdateIntegrationRequest{
		EnableUserSync:       true,
		EnableAppGroupSync:   true,
		EnableAccessListSync: true,
		AccessListSettings: &oktav1.AccessListSettings{
			DefaultOwner: []string{"alice-admin"},
		},
		EnableBidirectionalSync: true,
	})
	require.Error(t, err)
	require.True(t, trace.IsBadParameter(err))
	require.ErrorContains(t, err, "Okta OAuth scopes verification failed: scope \"okta.apps.manage\" missing, scope \"okta.groups.manage\" missing")
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
