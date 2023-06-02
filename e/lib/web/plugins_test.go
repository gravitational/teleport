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
			name:     "detect duplicate plugin request",
			endpoint: webPack.clt.Endpoint("enterprise", "plugin"),
			request: url.Values{
				"type":        {"jamf"},
				"apiEndpoint": {"https://testserver.com"},
				"username":    {"uname"},
				"password":    {"pass"},
				"csrf_token":  {webPack.csrfToken},
			},
			expectedResp: "Devices will be synced from Jamf to Teleport device inventory",
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
