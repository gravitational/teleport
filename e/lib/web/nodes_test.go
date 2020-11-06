package web

import (
	"context"
	"testing"
	"time"

	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestCreateNodeJoinToken(t *testing.T) {
	m := &mockedNodeAPIGetter{}
	m.mockGenerateToken = func(ctx context.Context, req auth.GenerateTokenRequest) (string, error) {
		return "some-token-id", nil
	}

	token, err := createScriptJoinToken(context.Background(), m)
	require.Nil(t, err)

	require.Equal(t, defaults.NodeJoinTokenTTL, token.Expiry.Sub(time.Now().UTC()).Round(time.Second))
	require.Equal(t, "some-token-id", token.ID)
}

func TestGetNodeJoinScript(t *testing.T) {
	m := &mockedNodeAPIGetter{}
	m.mockGetProxyServers = func() ([]services.Server, error) {
		var s services.ServerV2
		s.SetPublicAddr("test-host:12345678")

		return []services.Server{&s}, nil
	}
	m.mockGetClusterCACert = func() (*auth.LocalCAResponse, error) {
		fakeBytes := []byte(fixtures.SigningCertPEM)
		return &auth.LocalCAResponse{TLSCA: fakeBytes}, nil
	}

	nilTokenLength := scriptSettings{
		token: "",
	}

	shortTokenLength := scriptSettings{
		token: "f18da1c9f6630a51e8daf121e7451d",
	}

	testTokenID := "f18da1c9f6630a51e8daf121e7451daa"
	validTokenLength := scriptSettings{
		token: testTokenID,
	}

	// Test zero-value initialization.
	script, err := getJoinScript(scriptSettings{}, m)
	require.Empty(t, script)
	require.True(t, trace.IsBadParameter(err))

	// Test bad token lengths.
	script, err = getJoinScript(nilTokenLength, m)
	require.Empty(t, script)
	require.True(t, trace.IsBadParameter(err))

	script, err = getJoinScript(shortTokenLength, m)
	require.Empty(t, script)
	require.True(t, trace.IsBadParameter(err))

	// Test valid token format.
	script, err = getJoinScript(validTokenLength, m)
	require.Nil(t, err)

	require.Contains(t, script, testTokenID)
	require.Contains(t, script, "test-host")
	require.Contains(t, script, "12345678")
	require.Contains(t, script, "sha256:")
}

func TestURLEscaping(t *testing.T) {
	tests := []struct {
		desc        string
		input       string
		output      string
		shouldError bool
	}{
		{
			desc:   "regular HTTPS URL",
			input:  "https%3A%2F%2Fnews.ycombinator.com",
			output: "https://news.ycombinator.com",
		},
		{
			desc:   "URL with multiple parameters",
			input:  "http%3A%2F%2Fexample.com%2Ftest%2Furl%3Fwith%3D1%26extra%3D1%26parameters%3D1",
			output: "http://example.com/test/url?with=1&extra=1&parameters=1",
		},
		{
			desc:   "URL with IP address",
			input:  "http%3A%2F%2F192.168.1.1%2Fadmin",
			output: "http://192.168.1.1/admin",
		},
		{
			desc:   "URL with username/password",
			input:  "https%3A%2F%2Fuser%3Apassword%40www.example.com",
			output: "https://user:password@www.example.com",
		},
		{
			desc:   "URL with tilde",
			input:  "https%3A%2F%2Fexample.com%2F~testcase",
			output: "https://example.com/~testcase",
		},
		{
			desc:   "URL parameter with spaces",
			input:  "http%3A%2F%2Fexample.com%2F%3Fq%3Dthis%20is%20a%20parameter%20with%20spaces",
			output: "http://example.com/?q=this is a parameter with spaces",
		},
		{
			// output values with double quotes are 'double escaped' (i.e. \\\" rather than \")
			// so that when Go's escaping layer is removed, the values remain escaped.
			desc:   "URL parameter with double quotes and spaces",
			input:  "http%3A%2F%2Fexample.com%2F%3Fq%3D%22this%20is%20a%20parameter%20with%20quotes%20and%20spaces%22",
			output: "http://example.com/?q=\\\"this is a parameter with quotes and spaces\\\"",
		},
		{
			desc:   "URL parameters with multiple quotes",
			input:  "http%3A%2F%2Fexample.com%2F%3Fquery1%3D%22firstquery%22%26query2%3D%22secondquery%22",
			output: "http://example.com/?query1=\\\"firstquery\\\"&query2=\\\"secondquery\\\"",
		},
		{
			desc:   "URL with non-escaped parameter value",
			input:  "http%3A%2F%2Fexample.com?parameter=100",
			output: "http://example.com?parameter=100",
		},
		{
			desc:        "URL with non-escaped parameter value and erroneous %",
			input:       "http%3A%2F%2Fexample.com?parameter=100%",
			shouldError: true,
		},
		{
			desc:        "URL with non-escaped value and erroneous % in parameter",
			input:       "http%3A%2F%2Fexample.com%2F%3Fparameter%3D100%%25",
			shouldError: true,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			output, err := unescapeAndStripParameter(tc.input)
			if tc.shouldError {
				require.NotNil(t, err)
				require.Equal(t, output, "")
			} else {
				require.Nil(t, err)
				require.Contains(t, output, tc.output)
			}
		})
	}
}

func TestGetAppJoinScript(t *testing.T) {
	m := &mockedNodeAPIGetter{}
	m.mockGetProxyServers = func() ([]services.Server, error) {
		var s services.ServerV2
		s.SetPublicAddr("test-host:12345678")

		return []services.Server{&s}, nil
	}
	m.mockGetClusterCACert = func() (*auth.LocalCAResponse, error) {
		fakeBytes := []byte(fixtures.SigningCertPEM)
		return &auth.LocalCAResponse{TLSCA: fakeBytes}, nil
	}

	testTokenID := "f18da1c9f6630a51e8daf121e7451daa"
	badAppName := scriptSettings{
		token:          testTokenID,
		appInstallMode: true,
		appName:        "",
		appURI:         "127.0.0.1:0",
	}

	badAppURI := scriptSettings{
		token:          testTokenID,
		appInstallMode: true,
		appName:        "test-app",
		appURI:         "",
	}

	// Test invalid app data.
	script, err := getJoinScript(badAppName, m)
	require.Empty(t, script)
	require.True(t, trace.IsBadParameter(err))

	script, err = getJoinScript(badAppURI, m)
	require.Empty(t, script)
	require.True(t, trace.IsBadParameter(err))

	// Test various 'good' cases.
	expectedOutputs := []string{
		testTokenID,
		"test-host",
		"12345678",
		"sha256:",
	}

	tests := []struct {
		desc     string
		settings scriptSettings
		outputs  []string
	}{
		{
			desc: "node only join mode with other values not provided",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: false,
			},
			outputs: expectedOutputs,
		},
		{
			desc: "node only join mode with values set to blank",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: false,
				appName:        "",
				appURI:         "",
			},
			outputs: expectedOutputs,
		},
		{
			desc: "all settings set correctly",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: true,
				appName:        "test-app",
				appURI:         "http://localhost:12345",
			},
			outputs: append(
				expectedOutputs,
				"test-app",
				"http://localhost:12345",
			),
		},
		{
			desc: "all settings set correctly with a longer app name",
			settings: scriptSettings{
				token:          testTokenID,
				appInstallMode: true,
				appName:        "this-is-a-much-longer-app-name-being-used-for-testing",
				appURI:         "https://1.2.3.4:54321",
			},
			outputs: append(
				expectedOutputs,
				"this-is-a-much-longer-app-name-being-used-for-testing",
				"https://1.2.3.4:54321",
			),
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.desc, func(t *testing.T) {
			script, err = getJoinScript(tc.settings, m)
			require.Nil(t, err)
			for _, output := range tc.outputs {
				require.Contains(t, script, output)
			}
		})
	}
}

type mockedNodeAPIGetter struct {
	mockGenerateToken    func(ctx context.Context, req auth.GenerateTokenRequest) (string, error)
	mockGetProxyServers  func() ([]services.Server, error)
	mockGetClusterCACert func() (*auth.LocalCAResponse, error)
}

func (m *mockedNodeAPIGetter) GenerateToken(ctx context.Context, req auth.GenerateTokenRequest) (string, error) {
	if m.mockGenerateToken != nil {
		return m.mockGenerateToken(ctx, req)
	}

	return "", trace.NotImplemented("mockGenerateToken not implemented")
}

func (m *mockedNodeAPIGetter) GetProxies() ([]services.Server, error) {
	if m.mockGetProxyServers != nil {
		return m.mockGetProxyServers()
	}

	return nil, trace.NotImplemented("mockGetProxyServers not implemented")
}

func (m *mockedNodeAPIGetter) GetClusterCACert() (*auth.LocalCAResponse, error) {
	if m.mockGetClusterCACert != nil {
		return m.mockGetClusterCACert()
	}

	return nil, trace.NotImplemented("mockGetClusterCACert not implemented")
}
