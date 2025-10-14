package okta

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/okta/okta-sdk-golang/v2/okta"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/durationpb"

	oktav1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/okta/v1"
	pluginsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/plugins/v1"
	"github.com/gravitational/teleport/api/types"
	scimsdk "github.com/gravitational/teleport/e/lib/scim/sdk"
	"github.com/gravitational/teleport/e/tests/common"
)

// muxToTransportWrapper allows to forward http.ServeMux as http.Transport.
type muxToTransportWrapper struct {
	handler http.Handler
}

func (rt *muxToTransportWrapper) RoundTrip(req *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()
	rt.handler.ServeHTTP(rec, req)
	return rec.Result(), nil
}

// TODO(smallinsky): ideally the okta infra and SCIM provisioning behavior should be consistent and the SCIM push should be
// automatically called during the CRUD operation on the oktaInfra object instead of
// having to call it manually via SCIM client
func setupOktaAPIServerForSCIMFlow(t *testing.T, mockClient *mockOktaAPIClient) *http.ServeMux {
	t.Helper()
	r := http.NewServeMux()
	r.HandleFunc("GET /api/v1/groups", func(writer http.ResponseWriter, request *http.Request) {
		groups, _, err := mockClient.ListGroups(request.Context(), nil)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(t, writer, groups)
	})
	r.HandleFunc("GET /api/v1/users/{id}/groups", func(writer http.ResponseWriter, request *http.Request) {
		groups, _, err := mockClient.ListUserGroups(request.Context(), request.PathValue("id"))
		if err != nil {
			http.Error(writer, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(t, writer, groups)
	})
	return r
}

func jsonResponse(t *testing.T, writer http.ResponseWriter, data any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(writer).Encode(data)
	assert.NoError(t, err)
}

func createSCIMClient(t *testing.T, sut *common.SUT, scimToken string) scimsdk.Client {
	t.Helper()
	u := url.URL{
		Scheme: "https",
		Path:   "/v1/webapi/scim/okta",
		Host:   sut.ProxyAddr,
	}
	scimClient, err := scimsdk.New(&scimsdk.Config{
		Endpoint:        u.String(),
		Token:           scimToken,
		IntegrationType: "okta",
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
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
func createAndWaitForOktaIntegration(t *testing.T, sut *common.SUT, mockClient *mockOktaAPIClient, options ...oktaIntegrationOption) string {
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

	samlApp := createOktaSAMLAPP(t, t.Context(), mockClient, "trial-1234567_teleportsamlconnectorapp_1")
	users, _, err := mockClient.ListUsers(t.Context(), nil)
	require.NoError(t, err)
	for _, u := range users {
		_, _, err := mockClient.AssignUserToApplication(t.Context(), samlApp.Id, okta.AppUser{Id: u.Id})
		require.NoError(t, err)
	}

	req := &oktav1.CreateIntegrationRequest{
		TimeBetweenImports:  durationpb.New(1 * time.Second),
		OktaOrganizationUrl: "https://trial-1234567.okta.com",
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

	_, err = oktaClient.CreateIntegration(t.Context(), req)
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
