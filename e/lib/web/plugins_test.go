package web

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
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
			name:     "Okta plugin",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":       {"okta"},
				"orgURL":     {"https://www.okta.com"},
				"apiToken":   {"some-api-token"},
				"csrf_token": {webPack.csrfToken},
			},
			expectedResp: "Okta applications and groups will be synced to Teleport",
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
			name:     "Mattermost plugin",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":       {"mattermost"},
				"url":        {"https://www.some-apiendoint.com"},
				"token":      {"some-token"},
				"channel":    {"some-channel"},
				"team":       {"some-team"},
				"csrf_token": {webPack.csrfToken},
			},
			expectedResp: `to the \"some-channel\" channel from team \"some-team\"`,
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
