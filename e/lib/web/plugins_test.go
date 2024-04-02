package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/header"
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

	s := newWebSuite(t)
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

	ctx := context.Background()
	_, err = s.testAuthServer.AuthServer.AuthServer.UpsertAccessList(ctx, newAccessList(t, "okta-access-list", types.OriginOkta))
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
