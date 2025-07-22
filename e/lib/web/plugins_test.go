package web

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/roundtrip"
	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/e/lib/idp/saml/testenv"
	"github.com/gravitational/teleport/e/lib/web/ui"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
)

// TODO(kimlisa): DELETE IN v19.0 (csrf)
func TestCreatePlugin_Deprecated(t *testing.T) {
	t.Parallel()

	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")

	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {}))
	defer func() { testServer.Close() }()

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
				"csrf_token":       {webPack.csrfToken},
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
				"csrf_token":  {webPack.csrfToken},
			},
			expectedResp: "unknown plugin type",
		},
		{
			name:     "AWS IC plugin",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				awsICPluginNameField:                        {types.PluginTypeAWSIdentityCenter},
				"type":                                      {types.PluginTypeAWSIdentityCenter},
				awsICPluginICRegionField:                    {"ca-central-1"},
				awsICPluginICARNField:                       {"arn:aws:sso:::instance/ssoins-8893885e0d4lllka"},
				awsICPluginOIDCIntegrationNameField:         {icOIDCIntegrationName},
				awsICPluginAccessListDefaultOwnersField:     {`["user1", "user2"]`},
				awsICPluginSAMLServiceProviderNameField:     {newServcieProviderName},
				awsICPluginSAMLServiceProviderMetadataField: {newEntityDescriptor("https://example.com", "https://example.com/acs")},
				awsICPluginSCIMBaseURLField:                 {"https://scim.ca-central-1.amazonaws.com/random-id/scim/v2"},
				awsICPluginSCIMAccessTokenField:             {"abc123example"},
				"csrf_token":                                {webPack.csrfToken},
			},
			expectedResp: "",
		},
		{
			name:     "Opsgenie plugin",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":         {"opsgenie"},
				"apiEndpoint":  {testServer.URL},
				"apiKey":       {"some-api-key"},
				"scheduleName": {"some-schedule-name"},
				"csrf_token":   {webPack.csrfToken},
			},
			expectedResp: "some-schedule-name",
		},
		{
			name:     "Servicenow plugin",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":        {"servicenow"},
				"apiEndpoint": {testServer.URL},
				"username":    {"some-username"},
				"password":    {"some-password"},
				"closeCode":   {"some-close-code"},
				"csrf_token":  {webPack.csrfToken},
			},
			expectedResp: "Incidents will be created at",
		},
		{
			name:     "PagerDuty plugin",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":        {"pagerduty"},
				"apiEndPoint": {"https://www.some-apiendoint.com"},
				"apiKey":      {"some-api-key"},
				"email":       {"root@example.com"},
				"csrf_token":  {webPack.csrfToken},
			},
			expectedResp: "root@example.com",
		},
		{
			name:     "Mattermost plugin with only team/channel defined",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":       {"mattermost"},
				"url":        {"https://www.some-apiendoint.com"},
				"token":      {"some-token"},
				"channel":    {"some-channel"},
				"team":       {"some-team"},
				"csrf_token": {webPack.csrfToken},
			},
			expectedResp: `and to the \"some-channel\" channel from team \"some-team\"`,
			delete:       true,
		},
		{
			name:     "Mattermost plugin with only email defined",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":       {"mattermost"},
				"url":        {"https://www.some-apiendoint.com"},
				"token":      {"some-token"},
				"email":      {"some-email"},
				"csrf_token": {webPack.csrfToken},
			},
			expectedResp: `and to Mattermost user \"some-email\"`,
			delete:       true,
		},
		{
			name:     "Mattermost plugin with both team/channel and email defined",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":       {"mattermost"},
				"url":        {"https://www.some-apiendoint.com"},
				"token":      {"some-token"},
				"email":      {"some-email"},
				"channel":    {"some-channel"},
				"team":       {"some-team"},
				"csrf_token": {webPack.csrfToken},
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
				"csrf_token":        {webPack.csrfToken},
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
				"csrf_token":        {webPack.csrfToken},
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
				"csrf_token":        {webPack.csrfToken},
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

func TestCreateStaticAuthPluginHandle(t *testing.T) {
	t.Parallel()

	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")

	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {}))
	defer func() { testServer.Close() }()

	var testCases = []struct {
		name         string
		request      url.Values
		expectedResp string
		delete       bool
		// TODO(kimlisa): DELETE IN v19.0 (csrf)
		isOAuth bool
	}{
		// TODO(kimlisa): DELETE IN v19.0 (csrf):
		// Replace test with returning an error for oauth required plugins.
		{
			name: "Slack want redirect",
			request: url.Values{
				"type":             {"slack"},
				"name":             {"test"},
				"fallback_channel": {"test_fallback_channel"},
			},
			isOAuth:      true,
			expectedResp: "Teleport Redirection Service",
		},
		{
			name: "incorrect plugin subType",
			request: url.Values{
				"type":        {"unknown"},
				"apiEndpoint": {"https://testserver.com"},
			},
			expectedResp: "unknown plugin type",
		},
		{
			name: "AWS IC plugin",
			request: url.Values{
				awsICPluginNameField:                        {types.PluginTypeAWSIdentityCenter},
				"type":                                      {types.PluginTypeAWSIdentityCenter},
				awsICPluginICRegionField:                    {"ca-central-1"},
				awsICPluginICARNField:                       {"arn:aws:sso:::instance/ssoins-8893885e0d4lllka"},
				awsICPluginOIDCIntegrationNameField:         {icOIDCIntegrationName},
				awsICPluginAccessListDefaultOwnersField:     {`["user1", "user2"]`},
				awsICPluginSAMLServiceProviderNameField:     {newServcieProviderName},
				awsICPluginSAMLServiceProviderMetadataField: {newEntityDescriptor("https://example.com", "https://example.com/acs")},
				awsICPluginSCIMBaseURLField:                 {"https://scim.ca-central-1.amazonaws.com/random-id/scim/v2"},
				awsICPluginSCIMAccessTokenField:             {"abc123example"},
			},
			expectedResp: "",
		},
		{
			name: "Opsgenie plugin",
			request: url.Values{
				"type":         {"opsgenie"},
				"apiEndpoint":  {testServer.URL},
				"apiKey":       {"some-api-key"},
				"scheduleName": {"some-schedule-name"},
			},
			expectedResp: "some-schedule-name",
		},
		{
			name: "Servicenow plugin",
			request: url.Values{
				"type":        {"servicenow"},
				"apiEndpoint": {testServer.URL},
				"username":    {"some-username"},
				"password":    {"some-password"},
				"closeCode":   {"some-close-code"},
			},
			expectedResp: "Incidents will be created at",
		},
		{
			name: "PagerDuty plugin",
			request: url.Values{
				"type":        {"pagerduty"},
				"apiEndPoint": {"https://www.some-apiendoint.com"},
				"apiKey":      {"some-api-key"},
				"email":       {"root@example.com"},
			},
			expectedResp: "root@example.com",
		},
		{
			name: "Mattermost plugin with only team/channel defined",
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
			name: "Mattermost plugin with only email defined",
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
			name: "Mattermost plugin with both team/channel and email defined",
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
			name: "Datadog plugin",
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
			name: "Email (mailgun) plugin",
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
			name: "Email (smtp) plugin",
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
			resp, err := webPack.clt.PostForm(s.ctx, webPack.clt.Endpoint("enterprise", "plugins", "staticauth"), tc.request)
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

func TestOAuthPluginStart(t *testing.T) {
	t.Parallel()

	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")

	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {}))
	defer func() { testServer.Close() }()

	var testCases = []struct {
		name    string
		request url.Values
		wantErr bool
		assert  func(t *testing.T, re *roundtrip.Response, err error)
	}{
		{
			name: "Slack",
			request: url.Values{
				"type":             {"slack"},
				"name":             {"test"},
				"fallback_channel": {"test_fallback_channel"},
			},
			assert: func(t *testing.T, re *roundtrip.Response, err error) {
				require.NoError(t, err)
				require.True(t, true, cookieExist(re.Cookies(), "__Host-plugin-params"))
				resp := ui.OAuthPluginStartResponse{}
				require.NoError(t, json.Unmarshal(re.Bytes(), &resp))
				require.True(t, strings.HasPrefix(resp.RedirectURL, "https://slack.com/oauth/v2/authorize"))
			},
		},
		{
			name: "incorrect plugin subType",
			request: url.Values{
				"type":        {"unknown"},
				"apiEndpoint": {"https://testserver.com"},
			},
			assert: func(t *testing.T, re *roundtrip.Response, err error) {
				require.True(t, trace.IsBadParameter(err))
				require.False(t, false, cookieExist(re.Cookies(), "__Host-plugin-params"))
				require.Contains(t, string(re.Bytes()), "unknown plugin type")
			},
		},
		{
			name: "non oauth plugin type",
			request: url.Values{
				"type":         {"opsgenie"},
				"apiEndpoint":  {testServer.URL},
				"apiKey":       {"some-api-key"},
				"scheduleName": {"some-schedule-name"},
			},
			assert: func(t *testing.T, re *roundtrip.Response, err error) {
				require.False(t, false, cookieExist(re.Cookies(), "__Host-plugin-params"))
				require.True(t, trace.IsNotImplemented(err))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			re, err := webPack.clt.PostWithFormData(s.ctx, webPack.clt.Endpoint("enterprise", "plugins", "oauth", "start"), tc.request)
			tc.assert(t, re, err)
		})
	}
}

func TestPluginUpdate(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{
		TestBuildType: modules.BuildEnterprise,
	})
	s := newWebSuite(t)
	webPack := s.newAuthWebPack(t, "foo")
	endpoint := webPack.clt.Endpoint("enterprise", "plugin")
	cases := []struct {
		name    string
		payload ui.PluginUpdateRequest
		assert  func(t *testing.T, r *roundtrip.Response, err error)
	}{
		{
			name: "when no existing plugin",
			payload: ui.PluginUpdateRequest{
				Plugin: "okta",
				Okta: &ui.OktaPluginUpdate{
					SCIMToken: "abcdefghijklmnop",
				},
			},
			assert: func(t *testing.T, r *roundtrip.Response, err error) {
				require.Equal(t, http.StatusNotFound, r.Code())
				require.ErrorContains(t, err, "plugin \"okta\" doesn't exist")
			},
		},
		{
			name: "when updates not supported",
			payload: ui.PluginUpdateRequest{
				Plugin: "slack",
			},
			assert: func(t *testing.T, r *roundtrip.Response, err error) {
				require.Equal(t, http.StatusBadRequest, r.Code())
				require.ErrorContains(t, err, "plugin type \"slack\" does not support updates")
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := webPack.clt.PutJSON(s.ctx, endpoint, tc.payload)
			tc.assert(t, resp, err)
		})
	}
}

func TestPluginCleanup(t *testing.T) {
	modulestest.SetTestModules(t, modulestest.Modules{
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
	modulestest.SetTestModules(t, modulestest.Modules{
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
