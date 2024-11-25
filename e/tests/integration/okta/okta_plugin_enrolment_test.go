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

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	common "github.com/gravitational/teleport/e/tests/common"
	"github.com/gravitational/teleport/e/tests/common/idp"
	"github.com/gravitational/teleport/lib/events"
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
	oktaApiClientMock := newMockOktaAPIClient()
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

	t.Run("filter apps", func(t *testing.T) {
		var resp, err = oktaClient.GetApps(ctx, &oktav1.GetAppsRequest{
			OktaOrganizationUrl: "https://trial-1234567.okta.com",
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
		mustCreateOktaEveryoneGroupAndAssignOktaUsers(t, oktaInfra)

		resp, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
			OktaOrganizationUrl:  "https://trial-1234567.okta.com",
			ApiCredentials:       apiCredentials,
			EnableAccessListSync: true,
			EnableAppGroupSync:   true,
			EnableUserSync:       true,
			AccessListSettings: &oktav1.AccessListSettings{
				DefaultOwner: []string{"alice-admin"},
			},
		})
		require.NoError(t, err)

		_, _, err = oktaInfra.client.GetApplication(ctx, resp.GetConnectorInfo().GetOktaAppId(), &okta.SamlApplication{}, nil)
		require.NoError(t, err)
		_, err = sut.Teleport.Process.GetAuthServer().GetSAMLConnector(ctx, "okta-integration", false)
		require.NoError(t, err)
	})
}

// TestPluginEnrolmentSSOMetadataURL tests the enrolment of the Okta plugin where the SSO metadata URL is provided
// and the SAML connector is created based on the provided metadata URL.
func TestPluginEnrolmentSSOMetadataURL(t *testing.T) {
	ctx := context.Background()

	oktaInfra := createOktaSetup(t, ctx, newMockOktaAPIClient(), withAppsGroupsUsersCount(1, 1, 1))
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
	oktaInfra.client.setRoundTripper(httpMock)
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
		common.WithHTTPClient(httpMock),
	)

	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	_, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
		OktaOrganizationUrl:  "https://trial-1234567.okta.com",
		ApiCredentials:       apiCredentials,
		EnableAccessListSync: true,
		EnableAppGroupSync:   true,
		EnableUserSync:       true,
		AccessListSettings: &oktav1.AccessListSettings{
			DefaultOwner: []string{"alice-admin"},
		},
		SsoMetadataUrl: "https://trial-1234567.okta.com/app/123487988/sso/saml/metadata",
	})
	require.NoError(t, err)
	resp, err := sut.Teleport.Process.GetAuthServer().GetSAMLConnector(ctx, "okta-integration", false)
	require.NoError(t, err)
	require.Equal(t, "https://trial-1234567.okta.com", resp.GetMetadata().Labels[types.OktaOrgURLLabel])
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
	resp, err := sut.Teleport.Process.GetAuthServer().GetSAMLConnector(ctx, "okta-integration", false)
	require.NoError(t, err)
	require.Equal(t, "https://trial-7284229.okta.com", resp.GetMetadata().Labels[types.OktaOrgURLLabel])
}

func TestPluginEnrolmentPartialSteps(t *testing.T) {
	ctx := context.Background()
	scimToken := uuid.NewString()

	oktaInfra := createOktaSetupTreeAppGroupUserAndBasicUserGroupAssigment(t, ctx)
	sut := common.InitSUT(t,
		common.WithSAMLConnector(idp.SAMLConnector),
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	mustCreateOktaEveryoneGroupAndAssignOktaUsers(t, oktaInfra)

	t.Run("enroll okta integration with SCM only", func(t *testing.T) {
		_, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
			OktaOrganizationUrl:  "https://trial-1234567.okta.com",
			ScimToken:            scimToken,
			EnableAccessListSync: false,
			EnableAppGroupSync:   false,
			EnableUserSync:       false,
			ReuseConnector:       "okta",
		})
		require.NoError(t, err)
		pluginClient := pluginsv1.NewPluginServiceClient(sut.GetAuthServiceGRPCConn(t, "alice-admin"))
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
		mustWaitForEvent(t, sut, events.OktaUserSyncEvent, withTimeout(time.Second*3), withTimePoint(from))
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
			ApiCredentials:     apiCredentials,
			EnableUserSync:     true,
			EnableAppGroupSync: true,
		})
		require.NoError(t, err)

		mustWaitForEvent(t, sut, events.OktaGroupsUpdateEvent, withTimeout(time.Second*3), withTimePoint(from))
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
				EnableUserSync:       true,
				EnableAppGroupSync:   true,
				EnableAccessListSync: true,
				AccessListSettings: &oktav1.AccessListSettings{
					DefaultOwner: []string{"alice-admin"},
				},
			})
			require.NoError(t, err)
		}, time.Second, 200*time.Millisecond)
		mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent, withTimeout(time.Second*3), withTimePoint(from))

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			accessLists, err := sut.Teleport.Process.GetAuthServer().GetAccessLists(ctx)
			assert.NoError(t, err)
			assert.Len(t, accessLists, len(oktaInfra.Groups))
		}, time.Second*2, time.Millisecond*100)
	})

	t.Run("update integration setting and enable user sync and app groups sync", func(t *testing.T) {
		_, err := oktaClient.UpdateIntegration(ctx, &oktav1.UpdateIntegrationRequest{
			EnableUserSync:       true,
			EnableAppGroupSync:   true,
			EnableAccessListSync: true,
			AccessListSettings: &oktav1.AccessListSettings{
				DefaultOwner: []string{"alice-admin"},
				GroupFilters: []string{oktaInfra.Groups[0].Profile.Name},
			},
		})
		require.NoError(t, err)
		mustWaitForEvent(t, sut, events.OktaAccessListSyncEvent, withTimeout(time.Second*3), withTimePoint(time.Now()))

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
	oktaInfra := createOktaSetup(t, ctx, newMockOktaAPIClient(), withAppsGroupsUsersCount(3, 3, 3))
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[0].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[1].Id)
	oktaInfra.addUserToGroup(t, oktaInfra.Groups[0].Id, oktaInfra.Users[2].Id)

	sut := common.InitSUT(t,
		common.WithLicense("../../../fixtures/license-eub.pem"),
		common.WithUser(t, "alice-admin", "editor"),
	)
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")
	mustCreateOktaEveryoneGroupAndAssignOktaUsers(t, oktaInfra)

	t.Run("try to configure scim integration without any okta connector", func(t *testing.T) {
		_, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
			OktaOrganizationUrl:  "https://trial-1234567.okta.com",
			ScimToken:            scimToken,
			EnableAccessListSync: false,
			EnableAppGroupSync:   false,
			EnableUserSync:       false,
		})
		require.True(t, trace.IsBadParameter(err))
	})

	t.Run("okta client is missing permission to create okta SAML application", func(t *testing.T) {
		oktaInfra.client.scopes = []string{"okta.apps.read", "okta.groups.read", "okta.users.read"}
		_, err := oktaClient.CreateIntegration(ctx, &oktav1.CreateIntegrationRequest{
			ApiCredentials:       &oktav1.OktaAPICredentials{Auth: &oktav1.OktaAPICredentials_OauthId{OauthId: "12345"}},
			OktaOrganizationUrl:  "https://trial-1234567.okta.com",
			ScimToken:            scimToken,
			EnableAccessListSync: false,
			EnableAppGroupSync:   false,
			EnableUserSync:       false,
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

func mustCreateOktaEveryoneGroupAndAssignOktaUsers(t *testing.T, oktaInfra *oktaInfraSetup) {
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
	everyoneGroup, _, err := oktaInfra.client.CreateGroup(context.Background(), g)
	require.NoError(t, err)
	for _, user := range oktaInfra.Users {
		_, err := oktaInfra.client.AddUserToGroup(context.Background(), everyoneGroup.Id, user.Id)
		require.NoError(t, err)
	}
}
