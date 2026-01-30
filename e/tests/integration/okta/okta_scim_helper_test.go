package okta

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
)

func scimBaseURL(sut *common.SUT) string {
	u := url.URL{
		Scheme: "https",
		Path:   "/v1/webapi/scim/okta",
		Host:   sut.ProxyAddr,
	}
	return u.String()
}

func newInsecureHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
}

func createSCIMClient(t *testing.T, sut *common.SUT, scimToken string) scimsdk.Client {
	t.Helper()
	scimClient, err := scimsdk.New(&scimsdk.Config{
		Endpoint:        scimBaseURL(sut),
		Token:           scimToken,
		IntegrationType: "okta",
		HTTPClient:      newInsecureHTTPClient(),
	})
	require.NoError(t, err)
	return scimClient
}

type scimIntegrationOptions struct {
	ApiCredentials     *oktav1.OktaAPICredentials
	AccessListSettings *oktav1.AccessListSettings
	EnableFullSync     bool
}

type oktaIntegrationOption func(*scimIntegrationOptions)

func withAccessListDisabled() oktaIntegrationOption {
	return func(opts *scimIntegrationOptions) {
		opts.AccessListSettings = nil
	}
}

func witAccessListSettings(config *oktav1.AccessListSettings) oktaIntegrationOption {
	return func(opts *scimIntegrationOptions) {
		opts.AccessListSettings = config
	}
}
func withEnableFullSync() oktaIntegrationOption {
	return func(opts *scimIntegrationOptions) {
		opts.EnableFullSync = true
	}
}

// createAndWaitForOktaIntegration creates Okta integration and waits for the plugin to be running.
func createAndWaitForOktaIntegration(t *testing.T, sut *common.SUT, fakeOkta *fakeOktaServer, options ...oktaIntegrationOption) string {
	t.Helper()
	opts := scimIntegrationOptions{
		ApiCredentials:     &oktav1.OktaAPICredentials{Auth: &oktav1.OktaAPICredentials_SswsBearerToken{SswsBearerToken: "12345"}},
		AccessListSettings: &oktav1.AccessListSettings{DefaultOwner: []string{"alice"}},
	}
	for _, v := range options {
		v(&opts)
	}
	scimToken := uuid.NewString()
	oktaClient := sut.GetOktaAuthClient(t, "alice-admin")

	for _, u := range fakeOkta.ListUsers() {
		require.NoError(t, fakeOkta.AssignUserToApplication(fakeOkta.provisionedSAMLApp.Id, u.Id))
	}

	req := &oktav1.CreateIntegrationRequest{
		TimeBetweenImports:  durationpb.New(1 * time.Second),
		OktaOrganizationUrl: fakeOkta.URL(),
		ScimToken:           scimToken,
		ApiCredentials:      opts.ApiCredentials,
		AccessListSettings:  opts.AccessListSettings,
		ReuseConnector:      "okta-pre-created-test",
	}
	if opts.EnableFullSync {
		req.EnableBidirectionalSync = true
		req.EnableAppGroupSync = true
		req.EnableUserSync = true
		req.DisableAssignDefaultRoles = false
		req.EnableAccessListSync = true
	}

	_, err := oktaClient.CreateIntegration(t.Context(), req)
	require.NoError(t, err)
	pluginClient := pluginsv1.NewPluginServiceClient(sut.GetAuthServiceGRPCConn(t, "alice-admin"))
	ctx := t.Context()
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		oktaPlugin, err := pluginClient.GetPlugin(ctx, &pluginsv1.GetPluginRequest{
			Name: types.PluginTypeOkta,
		})
		require.NoError(t, err)
		require.Equal(t, types.PluginStatusCode_RUNNING, oktaPlugin.GetStatus().GetCode())
	}, time.Second*2, time.Millisecond*50)
	return scimToken
}

func provisionSCIMUsers(t *testing.T, scimClient scimsdk.Client, ids ...string) []*scimsdk.User {
	t.Helper()
	ctx := context.Background()
	var users []*scimsdk.User
	for _, userID := range ids {
		userName := fmt.Sprintf("test-user-%s@example.com", userID)
		u, err := scimClient.CreateUser(ctx, &scimsdk.User{ExternalID: userID, ID: userName, UserName: userName, Active: true})
		require.NoError(t, err)
		users = append(users, u)
	}
	return users
}

func provisionSCIMGroups(t *testing.T, scimClient scimsdk.Client, groups []*scimsdk.Group) []*scimsdk.Group {
	t.Helper()
	ctx := context.Background()
	var out []*scimsdk.Group
	for _, v := range groups {
		group, err := scimClient.CreateGroup(ctx, v)
		require.NoError(t, err)
		out = append(out, group)
	}
	return out
}

func assertSCIMUserSchema(t *testing.T, user *scimsdk.User) {
	t.Helper()
	require.NotEmpty(t, user.ID)
	require.NotEmpty(t, user.ExternalID)
	require.NotEmpty(t, user.UserName)
	require.NotEmpty(t, user.Meta.Created)
	require.NotEmpty(t, user.Meta.Location)
	require.Len(t, user.Schemas, 1)
	require.Equal(t, "urn:ietf:params:scim:schemas:core:2.0:User", user.Schemas[0])
}

func assertSCIMGroupSchema(t *testing.T, group *scimsdk.Group) {
	t.Helper()
	require.NotEmpty(t, group.ID)
	require.NotEmpty(t, group.DisplayName)
	require.NotEmpty(t, group.Meta.Version)
	require.NotEmpty(t, group.Meta.Location)
	require.Len(t, group.Schemas, 1)
	require.Equal(t, "urn:ietf:params:scim:schemas:core:2.0:Group", group.Schemas[0])
}
