/*
Copyright 2016 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"io"
	"os"
	"testing"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/stretchr/testify/require"
	"gopkg.in/check.v1"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	os.Exit(m.Run())
}

// register test suite
type APITestSuite struct{}

// bootstrap check
func TestClientAPI(t *testing.T) { check.TestingT(t) }

var _ = check.Suite(&APITestSuite{})

func TestParseProxyHostString(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		input     string
		expectErr bool
		expect    ParsedProxyHost
	}{
		{
			name:      "Empty port string",
			input:     "example.org",
			expectErr: false,
			expect: ParsedProxyHost{
				Host:                     "example.org",
				UsingDefaultWebProxyPort: true,
				WebProxyAddr:             "example.org:3080",
				SSHProxyAddr:             "example.org:3023",
			},
		}, {
			name:      "Web proxy port only",
			input:     "example.org:1234",
			expectErr: false,
			expect: ParsedProxyHost{
				Host:                     "example.org",
				UsingDefaultWebProxyPort: false,
				WebProxyAddr:             "example.org:1234",
				SSHProxyAddr:             "example.org:3023",
			},
		}, {
			name:      "Web proxy port with whitespace",
			input:     "example.org: 1234",
			expectErr: false,
			expect: ParsedProxyHost{
				Host:                     "example.org",
				UsingDefaultWebProxyPort: false,
				WebProxyAddr:             "example.org:1234",
				SSHProxyAddr:             "example.org:3023",
			},
		}, {
			name:      "Web proxy port empty with whitespace",
			input:     "example.org:  ,200",
			expectErr: false,
			expect: ParsedProxyHost{
				Host:                     "example.org",
				UsingDefaultWebProxyPort: true,
				WebProxyAddr:             "example.org:3080",
				SSHProxyAddr:             "example.org:200",
			},
		}, {
			name:      "SSH port only",
			input:     "example.org:,200",
			expectErr: false,
			expect: ParsedProxyHost{
				Host:                     "example.org",
				UsingDefaultWebProxyPort: true,
				WebProxyAddr:             "example.org:3080",
				SSHProxyAddr:             "example.org:200",
			},
		}, {
			name:      "SSH port empty",
			input:     "example.org:100,",
			expectErr: false,
			expect: ParsedProxyHost{
				Host:                     "example.org",
				UsingDefaultWebProxyPort: false,
				WebProxyAddr:             "example.org:100",
				SSHProxyAddr:             "example.org:3023",
			},
		}, {
			name:      "SSH port with whitespace",
			input:     "example.org:100, 200 ",
			expectErr: false,
			expect: ParsedProxyHost{
				Host:                     "example.org",
				UsingDefaultWebProxyPort: false,
				WebProxyAddr:             "example.org:100",
				SSHProxyAddr:             "example.org:200",
			},
		}, {
			name:      "SSH port empty with whitespace",
			input:     "example.org:100,  ",
			expectErr: false,
			expect: ParsedProxyHost{
				Host:                     "example.org",
				UsingDefaultWebProxyPort: false,
				WebProxyAddr:             "example.org:100",
				SSHProxyAddr:             "example.org:3023",
			},
		}, {
			name:      "Both ports specified",
			input:     "example.org:100,200",
			expectErr: false,
			expect: ParsedProxyHost{
				Host:                     "example.org",
				UsingDefaultWebProxyPort: false,
				WebProxyAddr:             "example.org:100",
				SSHProxyAddr:             "example.org:200",
			},
		}, {
			name:      "Both ports empty with whitespace",
			input:     "example.org: , ",
			expectErr: false,
			expect: ParsedProxyHost{
				Host:                     "example.org",
				UsingDefaultWebProxyPort: true,
				WebProxyAddr:             "example.org:3080",
				SSHProxyAddr:             "example.org:3023",
			},
		}, {
			name:      "Too many parts",
			input:     "example.org:100,200,300,400",
			expectErr: true,
			expect:    ParsedProxyHost{},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			expected := testCase.expect
			actual, err := ParseProxyHost(testCase.input)

			if testCase.expectErr {
				require.Error(t, err)
				require.Nil(t, actual)
				return
			}

			require.NoError(t, err)
			require.Equal(t, expected.Host, actual.Host)
			require.Equal(t, expected.UsingDefaultWebProxyPort, actual.UsingDefaultWebProxyPort)
			require.Equal(t, expected.WebProxyAddr, actual.WebProxyAddr)
			require.Equal(t, expected.SSHProxyAddr, actual.SSHProxyAddr)
		})
	}
}

func (s *APITestSuite) TestNew(c *check.C) {
	conf := Config{
		Host:      "localhost",
		HostLogin: "vincent",
		HostPort:  22,
		KeysDir:   "/tmp",
		Username:  "localuser",
		SiteName:  "site",
		Tracer:    tracing.NoopProvider().Tracer("test"),
	}
	err := conf.ParseProxyHost("proxy")
	c.Assert(err, check.IsNil)

	tc, err := NewClient(&conf)
	c.Assert(err, check.IsNil)
	c.Assert(tc, check.NotNil)

	la := tc.LocalAgent()
	c.Assert(la, check.NotNil)
}

func (s *APITestSuite) TestParseLabels(c *check.C) {
	// simplest case:
	m, err := ParseLabelSpec("key=value")
	c.Assert(m, check.NotNil)
	c.Assert(err, check.IsNil)
	c.Assert(m, check.DeepEquals, map[string]string{
		"key": "value",
	})
	// multiple values:
	m, err = ParseLabelSpec(`type="database";" role"=master,ver="mongoDB v1,2"`)
	c.Assert(m, check.NotNil)
	c.Assert(err, check.IsNil)
	c.Assert(m, check.HasLen, 3)
	c.Assert(m["role"], check.Equals, "master")
	c.Assert(m["type"], check.Equals, "database")
	c.Assert(m["ver"], check.Equals, "mongoDB v1,2")

	// multiple and unicode:
	m, err = ParseLabelSpec(`服务器环境=测试,操作系统类别=Linux,机房=华北`)
	c.Assert(err, check.IsNil)
	c.Assert(m, check.NotNil)
	c.Assert(m, check.HasLen, 3)
	c.Assert(m["服务器环境"], check.Equals, "测试")
	c.Assert(m["操作系统类别"], check.Equals, "Linux")
	c.Assert(m["机房"], check.Equals, "华北")

	// invalid specs
	m, err = ParseLabelSpec(`type="database,"role"=master,ver="mongoDB v1,2"`)
	c.Assert(m, check.IsNil)
	c.Assert(err, check.NotNil)
	m, err = ParseLabelSpec(`type="database",role,master`)
	c.Assert(m, check.IsNil)
	c.Assert(err, check.NotNil)
}

func (s *APITestSuite) TestPortsParsing(c *check.C) {
	// empty:
	ports, err := ParsePortForwardSpec(nil)
	c.Assert(ports, check.IsNil)
	c.Assert(err, check.IsNil)
	ports, err = ParsePortForwardSpec([]string{})
	c.Assert(ports, check.IsNil)
	c.Assert(err, check.IsNil)
	// not empty (but valid)
	spec := []string{
		"80:remote.host:180",
		"10.0.10.1:443:deep.host:1443",
	}
	ports, err = ParsePortForwardSpec(spec)
	c.Assert(err, check.IsNil)
	c.Assert(ports, check.HasLen, 2)
	c.Assert(ports, check.DeepEquals, ForwardedPorts{
		{
			SrcIP:    "127.0.0.1",
			SrcPort:  80,
			DestHost: "remote.host",
			DestPort: 180,
		},
		{
			SrcIP:    "10.0.10.1",
			SrcPort:  443,
			DestHost: "deep.host",
			DestPort: 1443,
		},
	})
	// back to strings:
	clone := ports.String()
	c.Assert(spec[0], check.Equals, clone[0])
	c.Assert(spec[1], check.Equals, clone[1])

	// parse invalid spec:
	spec = []string{"foo", "bar"}
	ports, err = ParsePortForwardSpec(spec)
	c.Assert(ports, check.IsNil)
	c.Assert(err, check.ErrorMatches, "^Invalid port forwarding spec: .foo.*")
}

func (s *APITestSuite) TestDynamicPortsParsing(c *check.C) {
	tests := []struct {
		spec    []string
		isError bool
		output  DynamicForwardedPorts
	}{
		{
			spec:    nil,
			isError: false,
			output:  DynamicForwardedPorts{},
		},
		{
			spec:    []string{},
			isError: false,
			output:  DynamicForwardedPorts{},
		},
		{
			spec:    []string{"localhost"},
			isError: true,
			output:  DynamicForwardedPorts{},
		},
		{
			spec:    []string{"localhost:123:456"},
			isError: true,
			output:  DynamicForwardedPorts{},
		},
		{
			spec:    []string{"8080"},
			isError: false,
			output: DynamicForwardedPorts{
				DynamicForwardedPort{
					SrcIP:   "127.0.0.1",
					SrcPort: 8080,
				},
			},
		},
		{
			spec:    []string{":8080"},
			isError: false,
			output: DynamicForwardedPorts{
				DynamicForwardedPort{
					SrcIP:   "127.0.0.1",
					SrcPort: 8080,
				},
			},
		},
		{
			spec:    []string{":8080:8081"},
			isError: true,
			output:  DynamicForwardedPorts{},
		},
		{
			spec:    []string{"[::1]:8080"},
			isError: false,
			output: DynamicForwardedPorts{
				DynamicForwardedPort{
					SrcIP:   "::1",
					SrcPort: 8080,
				},
			},
		},
		{
			spec:    []string{"10.0.0.1:8080"},
			isError: false,
			output: DynamicForwardedPorts{
				DynamicForwardedPort{
					SrcIP:   "10.0.0.1",
					SrcPort: 8080,
				},
			},
		},
		{
			spec:    []string{":8080", "10.0.0.1:8080"},
			isError: false,
			output: DynamicForwardedPorts{
				DynamicForwardedPort{
					SrcIP:   "127.0.0.1",
					SrcPort: 8080,
				},
				DynamicForwardedPort{
					SrcIP:   "10.0.0.1",
					SrcPort: 8080,
				},
			},
		},
	}

	for _, tt := range tests {
		specs, err := ParseDynamicPortForwardSpec(tt.spec)
		if tt.isError {
			c.Assert(err, check.NotNil)
			continue
		} else {
			c.Assert(err, check.IsNil)
		}

		c.Assert(specs, check.DeepEquals, tt.output)
	}
}

func TestWebProxyHostPort(t *testing.T) {
	t.Parallel()

	tests := []struct {
		desc         string
		webProxyAddr string
		wantHost     string
		wantPort     int
	}{
		{
			desc:         "valid WebProxyAddr",
			webProxyAddr: "example.com:12345",
			wantHost:     "example.com",
			wantPort:     12345,
		},
		{
			desc:         "WebProxyAddr without port",
			webProxyAddr: "example.com",
			wantHost:     "example.com",
			wantPort:     defaults.HTTPListenPort,
		},
		{
			desc:         "invalid WebProxyAddr",
			webProxyAddr: "not a valid addr",
			wantHost:     "unknown",
			wantPort:     defaults.HTTPListenPort,
		},
		{
			desc:         "empty WebProxyAddr",
			webProxyAddr: "",
			wantHost:     "unknown",
			wantPort:     defaults.HTTPListenPort,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			c := &Config{WebProxyAddr: tt.webProxyAddr}
			gotHost, gotPort := c.WebProxyHostPort()
			require.Equal(t, tt.wantHost, gotHost)
			require.Equal(t, tt.wantPort, gotPort)
		})
	}
}

// TestApplyProxySettings validates that settings received from the proxy's
// ping endpoint are correctly applied to Teleport client.
func TestApplyProxySettings(t *testing.T) {
	tests := []struct {
		desc        string
		settingsIn  webclient.ProxySettings
		tcConfigIn  Config
		tcConfigOut Config
	}{
		{
			desc:       "Postgres public address unspecified, defaults to web proxy address",
			settingsIn: webclient.ProxySettings{},
			tcConfigIn: Config{
				WebProxyAddr: "web.example.com:443",
			},
			tcConfigOut: Config{
				WebProxyAddr:      "web.example.com:443",
				PostgresProxyAddr: "web.example.com:443",
			},
		},
		{
			desc: "MySQL enabled without public address, defaults to web proxy host and MySQL default port",
			settingsIn: webclient.ProxySettings{
				DB: webclient.DBProxySettings{
					MySQLListenAddr: "0.0.0.0:3036",
				},
			},
			tcConfigIn: Config{
				WebProxyAddr: "web.example.com:443",
			},
			tcConfigOut: Config{
				WebProxyAddr:      "web.example.com:443",
				PostgresProxyAddr: "web.example.com:443",
				MySQLProxyAddr:    "web.example.com:3036",
			},
		},
		{
			desc: "both Postgres and MySQL custom public addresses are specified",
			settingsIn: webclient.ProxySettings{
				DB: webclient.DBProxySettings{
					PostgresPublicAddr: "postgres.example.com:5432",
					MySQLListenAddr:    "0.0.0.0:3036",
					MySQLPublicAddr:    "mysql.example.com:3306",
				},
			},
			tcConfigIn: Config{
				WebProxyAddr: "web.example.com:443",
			},
			tcConfigOut: Config{
				WebProxyAddr:      "web.example.com:443",
				PostgresProxyAddr: "postgres.example.com:5432",
				MySQLProxyAddr:    "mysql.example.com:3306",
			},
		},
		{
			desc: "Postgres public address port unspecified, defaults to web proxy address port",
			settingsIn: webclient.ProxySettings{
				DB: webclient.DBProxySettings{
					PostgresPublicAddr: "postgres.example.com",
				},
			},
			tcConfigIn: Config{
				WebProxyAddr: "web.example.com:443",
			},
			tcConfigOut: Config{
				WebProxyAddr:      "web.example.com:443",
				PostgresProxyAddr: "postgres.example.com:443",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			tc := &TeleportClient{Config: test.tcConfigIn}
			err := tc.applyProxySettings(test.settingsIn)
			require.NoError(t, err)
			require.EqualValues(t, test.tcConfigOut, tc.Config)
		})
	}
}

type mockAgent struct {
	// Agent is embedded to avoid redeclaring all interface methods.
	// Only the Signers method is implemented by testAgent.
	agent.Agent
	ValidPrincipals []string
}

type mockSigner struct {
	ValidPrincipals []string
}

func (s *mockSigner) PublicKey() ssh.PublicKey {
	return &ssh.Certificate{
		ValidPrincipals: s.ValidPrincipals,
	}
}

func (s *mockSigner) Sign(rand io.Reader, b []byte) (*ssh.Signature, error) {
	return nil, trace.Errorf("mockSigner does not implement Sign")
}

// Signers implements agent.Agent.Signers.
func (m *mockAgent) Signers() ([]ssh.Signer, error) {
	return []ssh.Signer{&mockSigner{ValidPrincipals: m.ValidPrincipals}}, nil
}

func TestNewClient_UseKeyPrincipals(t *testing.T) {
	cfg := &Config{
		Username:         "xyz",
		HostLogin:        "xyz",
		WebProxyAddr:     "localhost",
		SkipLocalAuth:    true,
		UseKeyPrincipals: true, // causes VALID to be returned, as key was used
		Agent:            &mockAgent{ValidPrincipals: []string{"VALID"}},
		AuthMethods:      []ssh.AuthMethod{ssh.Password("xyz") /* placeholder authmethod */},
		Tracer:           tracing.NoopProvider().Tracer("test"),
	}
	client, err := NewClient(cfg)
	require.NoError(t, err)
	require.Equal(t, "VALID", client.getProxySSHPrincipal(), "ProxySSHPrincipal mismatch")

	cfg.UseKeyPrincipals = false // causes xyz to be returned as key was not used

	client, err = NewClient(cfg)
	require.NoError(t, err)
	require.Equal(t, "xyz", client.getProxySSHPrincipal(), "ProxySSHPrincipal mismatch")
}

func TestParseSearchKeywords(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		spec     string
		expected []string
	}{
		{
			name: "empty input",
			spec: "",
		},
		{
			name:     "simple input",
			spec:     "foo",
			expected: []string{"foo"},
		},
		{
			name:     "complex input",
			spec:     `"foo,bar","some phrase's",baz=qux's ,"some other  phrase"," another one  "`,
			expected: []string{"foo,bar", "some phrase's", "baz=qux's", "some other  phrase", "another one"},
		},
		{
			name:     "unicode input",
			spec:     `"服务器环境=测试,操作系统类别", Linux , 机房=华北 `,
			expected: []string{"服务器环境=测试,操作系统类别", "Linux", "机房=华北"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := ParseSearchKeywords(tc.spec, ',')
			require.Equal(t, tc.expected, m)
		})
	}

	// Test default delimiter (which is a comma)
	m := ParseSearchKeywords("foo,bar", rune(0))
	require.Equal(t, []string{"foo", "bar"}, m)
}

func TestParseSearchKeywords_SpaceDelimiter(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		spec     string
		expected []string
	}{
		{
			name:     "simple input",
			spec:     "foo",
			expected: []string{"foo"},
		},
		{
			name:     "complex input",
			spec:     `foo,bar "some phrase's" baz=qux's "some other  phrase" " another one  "`,
			expected: []string{"foo,bar", "some phrase's", "baz=qux's", "some other  phrase", "another one"},
		},
		{
			name:     "unicode input",
			spec:     `服务器环境=测试,操作系统类别 Linux  机房=华北 `,
			expected: []string{"服务器环境=测试,操作系统类别", "Linux", "机房=华北"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := ParseSearchKeywords(tc.spec, ' ')
			require.Equal(t, tc.expected, m)
		})
	}
}

func TestVirtualPathNames(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		kind     VirtualPathKind
		params   VirtualPathParams
		expected []string
	}{
		{
			name:   "dummy",
			kind:   VirtualPathKind("foo"),
			params: VirtualPathParams{"a", "b", "c"},
			expected: []string{
				"TSH_VIRTUAL_PATH_FOO_A_B_C",
				"TSH_VIRTUAL_PATH_FOO_A_B",
				"TSH_VIRTUAL_PATH_FOO_A",
				"TSH_VIRTUAL_PATH_FOO",
			},
		},
		{
			name:     "key",
			kind:     VirtualPathKey,
			params:   nil,
			expected: []string{"TSH_VIRTUAL_PATH_KEY"},
		},
		{
			name:   "database ca",
			kind:   VirtualPathCA,
			params: VirtualPathCAParams(types.DatabaseCA),
			expected: []string{
				"TSH_VIRTUAL_PATH_CA_DB",
				"TSH_VIRTUAL_PATH_CA",
			},
		},
		{
			name:   "host ca",
			kind:   VirtualPathCA,
			params: VirtualPathCAParams(types.HostCA),
			expected: []string{
				"TSH_VIRTUAL_PATH_CA_HOST",
				"TSH_VIRTUAL_PATH_CA",
			},
		},
		{
			name:   "database",
			kind:   VirtualPathDatabase,
			params: VirtualPathDatabaseParams("foo"),
			expected: []string{
				"TSH_VIRTUAL_PATH_DB_FOO",
				"TSH_VIRTUAL_PATH_DB",
			},
		},
		{
			name:   "app",
			kind:   VirtualPathApp,
			params: VirtualPathAppParams("foo"),
			expected: []string{
				"TSH_VIRTUAL_PATH_APP_FOO",
				"TSH_VIRTUAL_PATH_APP",
			},
		},
		{
			name:   "kube",
			kind:   VirtualPathKubernetes,
			params: VirtualPathKubernetesParams("foo"),
			expected: []string{
				"TSH_VIRTUAL_PATH_KUBE_FOO",
				"TSH_VIRTUAL_PATH_KUBE",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			names := VirtualPathEnvNames(tc.kind, tc.params)
			require.Equal(t, tc.expected, names)
		})
	}
}
