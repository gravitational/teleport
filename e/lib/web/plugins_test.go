package web

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/idp/saml/testenv"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
)

func TestCreatePluginHandle(t *testing.T) {
	t.Parallel()

	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")

	var testCases = []struct {
		name         string
		endpoint     string
		request      url.Values
		expectedResp string
		isOAuth      bool
		delete       bool
	}{
		{
			name:     "Slack want redirect",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":             {"slack"},
				"name":             {"test"},
				"fallback_channel": {"test_fallback_channel"},
			},
			isOAuth:      true,
			expectedResp: "Teleport Redirection Service",
		},
		{
			name:     "incorrect plugin subType",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":        {"unknown"},
				"apiEndpoint": {"https://testserver.com"},
			},
			expectedResp: "unknown plugin type",
		},
		{
			name:     "Opsgenie plugin",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":         {"opsgenie"},
				"apiEndpoint":  {"https://www.some-apiendoint.com"},
				"apiKey":       {"some-api-key"},
				"scheduleName": {"some-schedule-name"},
				"csrf_token":   {webPack.csrfToken},
			},
			expectedResp: "some-schedule-name",
		},
		{
			name:     "PagerDuty plugin",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":        {"pagerduty"},
				"apiEndPoint": {"https://www.some-apiendoint.com"},
				"apiKey":      {"some-api-key"},
				"email":       {"root@example.com"},
			},
			expectedResp: "root@example.com",
		},
		{
			name:     "Mattermost plugin with only team/channel defined",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":    {"mattermost"},
				"url":     {"https://www.some-apiendoint.com"},
				"token":   {"some-token"},
				"channel": {"some-channel"},
				"team":    {"some-team"},
			},
			expectedResp: `and to the \"some-channel\" channel from team \"some-team\"`,
			delete:       true,
		},
		{
			name:     "Mattermost plugin with only email defined",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":  {"mattermost"},
				"url":   {"https://www.some-apiendoint.com"},
				"token": {"some-token"},
				"email": {"some-email"},
			},
			expectedResp: `and to Mattermost user \"some-email\"`,
			delete:       true,
		},
		{
			name:     "Mattermost plugin with both team/channel and email defined",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":    {"mattermost"},
				"url":     {"https://www.some-apiendoint.com"},
				"token":   {"some-token"},
				"email":   {"some-email"},
				"channel": {"some-channel"},
				"team":    {"some-team"},
			},
			expectedResp: `, to Mattermost user \"some-email\", and to the \"some-channel\" channel from team \"some-team\"`,
		},
		{
			name:     "Datadog plugin",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":              {"datadog"},
				"apiEndpoint":       {"https://www.some-apiendpoint.com"},
				"fallbackRecipient": {"root@example.com"},
				"apiKey":            {"some-api-key"},
				"applicationKey":    {"some-application-key"},
			},
			expectedResp: `Incidents will be created at \"https://www.some-apiendpoint.com\" and notify \"root@example.com\" recipient`,
		},
		{
			name:     "Email (mailgun) plugin",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":              {"email"},
				"service":           {"mailgun"},
				"sender":            {"sender@example.com"},
				"fallbackRecipient": {"root@example.com"},
				"domain":            {"sandbox.mailgun.org"},
				"privateKey":        {"some-private-key"},
			},
			expectedResp: `Emails will be sent by \"sender@example.com\" to \"root@example.com\"`,
			delete:       true,
		},
		{
			name:     "Email (smtp) plugin",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":              {"email"},
				"service":           {"smtp"},
				"sender":            {"sender@example.com"},
				"fallbackRecipient": {"root@example.com"},
				"host":              {"smtp.example.com"},
				"port":              {"587"},
				"startTLSPolicy":    {"mandatory"},
				"username":          {"user@example.com"},
				"password":          {"example-password"},
			},
			expectedResp: `Emails will be sent by \"sender@example.com\" to \"root@example.com\"`,
			delete:       true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := webPack.clt.PostForm(s.ctx, tc.endpoint, tc.request)
			require.NoError(t, err)
			if tc.isOAuth {
				require.True(t, true, cookieExist(resp.Cookies(), "__Host-plugin-params"))
			}

			if tc.expectedResp != "" {
				require.Contains(t, string(resp.Bytes()), tc.expectedResp)
			}
			if tc.delete {
				endpoint := webPack.clt.Endpoint("enterprise", "plugin", tc.request["type"][0])
				_, err := webPack.clt.Delete(s.ctx, endpoint)
				require.NoError(t, err)
			}
		})
	}
}

func TestPluginCleanup(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
	})

	// We define a real clock here, so we don't run into cases of
	// `backend.RunWhileLocked()` never retrying lock acquisition.
	clock := clockwork.NewRealClock()
	s := newWebSuite(t, withClock(clock), withRunWhileLockedRetryInterval(100*time.Millisecond))
	webPack := s.newAuthWebPack(t, "foo")

	_, err := s.testAuthServer.AuthServer.AuthServer.UpsertRole(s.ctx, services.NewSystemOktaAccessRole())
	require.NoError(t, err)
	_, err = s.testAuthServer.AuthServer.AuthServer.UpsertRole(s.ctx, services.NewSystemOktaRequesterRole())
	require.NoError(t, err)

	endpoint := webPack.clt.Endpoint("enterprise", "plugins", "needscleanup", types.PluginTypeOkta)
	resp, err := webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())
	needsCleanup := ui.PluginNeedsCleanup{}
	require.NoError(t, json.Unmarshal(resp.Bytes(), &needsCleanup))
	require.False(t, needsCleanup.NeedsCleanup)

	_, err = s.testAuthServer.AuthServer.AuthServer.UpsertAccessList(s.ctx, newAccessList(t, "okta-access-list", types.OriginOkta))
	require.NoError(t, err)

	resp, err = webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())
	require.NoError(t, json.Unmarshal(resp.Bytes(), &needsCleanup))
	require.True(t, needsCleanup.NeedsCleanup)

	endpoint = webPack.clt.Endpoint("enterprise", "plugins", "cleanup", types.PluginTypeOkta)
	resp, err = webPack.clt.PutForm(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())
	require.Contains(t, string(resp.Bytes()), "ok")

	endpoint = webPack.clt.Endpoint("enterprise", "plugins", "needscleanup", types.PluginTypeOkta)
	resp, err = webPack.clt.Get(s.ctx, endpoint, url.Values{})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code())
	require.NoError(t, json.Unmarshal(resp.Bytes(), &needsCleanup))
	require.False(t, needsCleanup.NeedsCleanup)
}

func TestValidatePluginEntraID(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
	})

	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	authServer := s.testAuthServer.AuthServer.AuthServer

	// Create existing objects
	_, err := s.testAuthServer.AuthServer.AuthServer.UpsertRole(s.ctx, services.NewPresetRequesterRole())
	require.NoError(t, err)

	const existingIntegrationName = "existingintegration"
	integration, err := types.NewIntegrationAzureOIDC(types.Metadata{Name: existingIntegrationName}, &types.AzureOIDCIntegrationSpecV1{
		TenantID: "foo",
		ClientID: "bar",
	})
	require.NoError(t, err)
	_, err = authServer.CreateIntegration(s.ctx, integration)
	require.NoError(t, err)

	const existingAuthConnectorName = "existingconnector"
	connector, err := types.NewSAMLConnector(existingAuthConnectorName, types.SAMLConnectorSpecV2{
		AssertionConsumerService: "https://teleport.local/webapi/v1/saml/acs/existingconnector",
		SSO:                      "https://example.org",
		EntityDescriptor:         testenv.NewTestEntityDescriptor("foo", "https://entraid.com/acs"),
		AttributesToRoles: []types.AttributeMapping{
			{
				Name:  "foo",
				Value: "bar",
				Roles: []string{"requester"},
			},
		},
	})
	require.NoError(t, err)
	_, err = authServer.CreateSAMLConnector(s.ctx, connector)
	require.NoError(t, err)

	valid := url.Values{
		"type":              {"entra-id"},
		"name":              {"myintegration"},
		"authConnectorName": {"myconnector"},
		"defaultOwners":     {`["alice", "bob"]`},
	}
	endpoint := webPack.clt.Endpoint("enterprise", "plugins", "validate")
	resp, err := webPack.clt.PostForm(s.ctx, endpoint, valid)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code(), string(resp.Bytes()))

	testCases := []struct {
		form      func() url.Values
		expectErr string
	}{
		{
			form: func() url.Values {
				return valid
			},
		},
		{
			form: func() url.Values {
				f := maps.Clone(valid)
				delete(f, "name")
				return f
			},
			expectErr: "integration name must be specified",
		},
		{
			form: func() url.Values {
				f := maps.Clone(valid)
				delete(f, "authConnectorName")
				return f
			},
			expectErr: "auth connector name must be specified",
		},
		{
			form: func() url.Values {
				f := maps.Clone(valid)
				delete(f, "defaultOwners")
				return f
			},
			expectErr: "default owners must be specified",
		},
		{
			form: func() url.Values {
				f := maps.Clone(valid)
				f["name"] = []string{existingIntegrationName}
				return f
			},
			expectErr: `integration named \"existingintegration\" already exists`,
		},
		{
			form: func() url.Values {
				f := maps.Clone(valid)
				f["authConnectorName"] = []string{existingAuthConnectorName}
				return f
			},
			expectErr: `auth connector named \"existingconnector\" already exists`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.expectErr, func(t *testing.T) {
			endpoint := webPack.clt.Endpoint("enterprise", "plugins", "validate")
			resp, err := webPack.clt.PostForm(s.ctx, endpoint, tc.form())
			require.NoError(t, err)
			payload := string(resp.Bytes())
			if tc.expectErr == "" {
				require.Equal(t, http.StatusOK, resp.Code(), payload)
			} else {
				require.Contains(t, payload, tc.expectErr)
			}
		})
	}
}

func cookieExist(cookies []*http.Cookie, name string) bool {
	for i := range cookies {
		if cookies[i].Name == name {
			return true
		}
	}
	return false
}

func newAccessList(t *testing.T, name, origin string) *accesslist.AccessList {
	t.Helper()

	var labels map[string]string
	if origin != "" {
		labels = map[string]string{}
		labels[types.OriginLabel] = origin
	}

	accessList, err := accesslist.NewAccessList(header.Metadata{
		Name:   name,
		Labels: labels,
	}, accesslist.Spec{
		Title: "some title",
		OwnerGrants: accesslist.Grants{
			Roles: []string{"grant-role"},
		},
		Grants: accesslist.Grants{
			Roles: []string{"role"},
		},
		Owners: []accesslist.Owner{
			{

				Name: "some-owner",
			},
		},
	})
	require.NoError(t, err)

	return accessList
}
