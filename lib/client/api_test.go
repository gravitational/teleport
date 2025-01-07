/*
 * Teleport
 * Copyright (C) 2023  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package client

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"testing"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/google/go-cmp/cmp"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/utils"
)

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	modules.SetInsecureTestMode(true)
	cryptosuites.PrecomputeRSATestKeys(m)
	os.Exit(m.Run())
}

var parseProxyHostTestCases = []struct {
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

func TestParseProxyHostString(t *testing.T) {
	t.Parallel()

	for _, testCase := range parseProxyHostTestCases {
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

func TestNew(t *testing.T) {
	conf := Config{
		Host:      "localhost",
		HostLogin: "vincent",
		HostPort:  22,
		KeysDir:   t.TempDir(),
		Username:  "localuser",
		SiteName:  "site",
		Tracer:    tracing.NoopProvider().Tracer("test"),
	}
	err := conf.ParseProxyHost("proxy")
	require.NoError(t, err)

	tc, err := NewClient(&conf)
	require.NoError(t, err)
	require.NotNil(t, tc)

	la := tc.LocalAgent()
	require.NotNil(t, la)
}

func TestParseLabels(t *testing.T) {
	// simplest case:
	m, err := ParseLabelSpec("key=value")
	require.NotNil(t, m)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(m, map[string]string{
		"key": "value",
	}))

	// multiple values:
	m, err = ParseLabelSpec(`type="database";" role"=master,ver="mongoDB v1,2"`)
	require.NotNil(t, m)
	require.NoError(t, err)
	require.Len(t, m, 3)
	require.Equal(t, "master", m["role"])
	require.Equal(t, "database", m["type"])
	require.Equal(t, "mongoDB v1,2", m["ver"])

	// multiple and unicode:
	m, err = ParseLabelSpec(`服务器环境=测试,操作系统类别=Linux,机房=华北`)
	require.NoError(t, err)
	require.NotNil(t, m)
	require.Len(t, m, 3)
	require.Equal(t, "测试", m["服务器环境"])
	require.Equal(t, "Linux", m["操作系统类别"])
	require.Equal(t, "华北", m["机房"])

	// invalid specs
	m, err = ParseLabelSpec(`type="database,"role"=master,ver="mongoDB v1,2"`)
	require.Nil(t, m)
	require.Error(t, err)
	m, err = ParseLabelSpec(`type="database",role,master`)
	require.Nil(t, m)
	require.Error(t, err)
}

func TestPortsParsing(t *testing.T) {
	// empty:
	ports, err := ParsePortForwardSpec(nil)
	require.Nil(t, ports)
	require.NoError(t, err)
	ports, err = ParsePortForwardSpec([]string{})
	require.Nil(t, ports)
	require.NoError(t, err)
	// not empty (but valid)
	spec := []string{
		"80:remote.host:180",
		"10.0.10.1:443:deep.host:1443",
	}
	ports, err = ParsePortForwardSpec(spec)
	require.NoError(t, err)
	require.Len(t, ports, 2)
	require.Empty(t, cmp.Diff(ports, ForwardedPorts{
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
	}))

	// back to strings:
	clone := ports.String()
	require.Equal(t, spec[0], clone[0])
	require.Equal(t, spec[1], clone[1])

	// parse invalid spec:
	spec = []string{"foo", "bar"}
	ports, err = ParsePortForwardSpec(spec)
	require.Empty(t, ports)
	require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
}

var dynamicPortForwardParsingTestCases = []struct {
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

func TestDynamicPortsParsing(t *testing.T) {
	for _, tt := range dynamicPortForwardParsingTestCases {
		specs, err := ParseDynamicPortForwardSpec(tt.spec)
		if tt.isError {
			require.Error(t, err)
			continue
		} else {
			require.NoError(t, err)
		}

		require.Empty(t, cmp.Diff(specs, tt.output))
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

func TestGetKubeTLSServerName(t *testing.T) {
	tests := []struct {
		name          string
		kubeProxyAddr string
		want          string
	}{
		{
			name:          "ipv4 format, API domain should be used",
			kubeProxyAddr: "127.0.0.1",
			want:          "kube-teleport-proxy-alpn.teleport.cluster.local",
		},
		{
			name:          "empty host, API domain should be used",
			kubeProxyAddr: "",
			want:          "kube-teleport-proxy-alpn.teleport.cluster.local",
		},
		{
			name:          "ipv4 unspecified, API domain should be used ",
			kubeProxyAddr: "0.0.0.0",
			want:          "kube-teleport-proxy-alpn.teleport.cluster.local",
		},
		{
			name:          "localhost, API domain should be used ",
			kubeProxyAddr: "localhost",
			want:          "kube-teleport-proxy-alpn.teleport.cluster.local",
		},
		{
			name:          "valid hostname",
			kubeProxyAddr: "example.com",
			want:          "kube-teleport-proxy-alpn.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetKubeTLSServerName(tt.kubeProxyAddr)
			require.Equal(t, tt.want, got)
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

func TestApplyAuthSettings(t *testing.T) {
	tests := []struct {
		desc        string
		settingsIn  webclient.AuthenticationSettings
		tcConfigIn  Config
		tcConfigOut Config
	}{
		{
			desc: "PIV slot set by server",
			settingsIn: webclient.AuthenticationSettings{
				PIVSlot: "9c",
			},
			tcConfigOut: Config{
				PIVSlot: "9c",
			},
		}, {
			desc: "PIV slot set by client",
			tcConfigIn: Config{
				PIVSlot: "9a",
			},
			tcConfigOut: Config{
				PIVSlot: "9a",
			},
		}, {
			desc: "PIV slot set on server and client, client takes precedence",
			settingsIn: webclient.AuthenticationSettings{
				PIVSlot: "9c",
			},
			tcConfigIn: Config{
				PIVSlot: "9a",
			},
			tcConfigOut: Config{
				PIVSlot: "9a",
			},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			tc := &TeleportClient{Config: test.tcConfigIn}
			tc.applyAuthSettings(test.settingsIn)
			require.EqualValues(t, test.tcConfigOut, tc.Config)
		})
	}
}

type mockAgent struct {
	// Agent is embedded to avoid redeclaring all interface methods.
	// Only the Signers method is implemented by testAgent.
	agent.ExtendedAgent
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
	return nil, fmt.Errorf("mockSigner does not implement Sign")
}

// Signers implements agent.Agent.Signers.
func (m *mockAgent) Signers() ([]ssh.Signer, error) {
	return []ssh.Signer{&mockSigner{ValidPrincipals: m.ValidPrincipals}}, nil
}

func TestNewClient_getProxySSHPrincipal(t *testing.T) {
	for _, tc := range []struct {
		name            string
		cfg             *Config
		expectPrincipal string
	}{
		{
			name: "ProxySSHPrincipal override",
			cfg: &Config{
				Username:          "teleport_user",
				HostLogin:         "host_login",
				WebProxyAddr:      "localhost",
				ProxySSHPrincipal: "proxy_ssh_principal_override",
				Agent:             &mockAgent{ValidPrincipals: []string{"key_principal"}},
				AuthMethods:       []ssh.AuthMethod{ssh.Password("xyz") /* placeholder authmethod */},
				Tracer:            tracing.NoopProvider().Tracer("test"),
			},
			expectPrincipal: "proxy_ssh_principal_override",
		}, {
			name: "Key principal",
			cfg: &Config{
				Username:     "teleport_user",
				HostLogin:    "host_login",
				WebProxyAddr: "localhost",
				Agent:        &mockAgent{ValidPrincipals: []string{"key_principal"}},
				AuthMethods:  []ssh.AuthMethod{ssh.Password("xyz") /* placeholder authmethod */},
				Tracer:       tracing.NoopProvider().Tracer("test"),
			},
			expectPrincipal: "key_principal",
		}, {
			name: "Host login default",
			cfg: &Config{
				Username:     "teleport_user",
				HostLogin:    "host_login",
				WebProxyAddr: "localhost",
				Agent:        &mockAgent{ /* no agent key principals */ },
				AuthMethods:  []ssh.AuthMethod{ssh.Password("xyz") /* placeholder authmethod */},
				Tracer:       tracing.NoopProvider().Tracer("test"),
			},
			expectPrincipal: "host_login",
		}, {
			name: "Jump host",
			cfg: &Config{
				Username:     "teleport_user",
				HostLogin:    "host_login",
				WebProxyAddr: "localhost",
				JumpHosts: []utils.JumpHost{
					{
						Username: "jumphost_user",
					},
				},
				Agent:       &mockAgent{ /* no agent key principals */ },
				AuthMethods: []ssh.AuthMethod{ssh.Password("xyz") /* placeholder authmethod */},
				Tracer:      tracing.NoopProvider().Tracer("test"),
			},
			expectPrincipal: "jumphost_user",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			client, err := NewClient(tc.cfg)
			require.NoError(t, err)
			require.Equal(t, tc.expectPrincipal, client.getProxySSHPrincipal(), "ProxySSHPrincipal mismatch")
		})
	}
}

var parseSearchKeywordsTestCases = []struct {
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

func TestParseSearchKeywords(t *testing.T) {
	t.Parallel()

	for _, tc := range parseSearchKeywordsTestCases {
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
			name:   "database client ca",
			kind:   VirtualPathCA,
			params: VirtualPathCAParams(types.DatabaseClientCA),
			expected: []string{
				"TSH_VIRTUAL_PATH_CA_DB_CLIENT",
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
			params: VirtualPathDatabaseCertParams("foo"),
			expected: []string{
				"TSH_VIRTUAL_PATH_DB_FOO",
				"TSH_VIRTUAL_PATH_DB",
			},
		},
		{
			name:   "database key",
			kind:   VirtualPathKey,
			params: VirtualPathDatabaseKeyParams("foo"),
			expected: []string{
				"TSH_VIRTUAL_PATH_KEY_DB_FOO",
				"TSH_VIRTUAL_PATH_KEY_DB",
				"TSH_VIRTUAL_PATH_KEY",
			},
		},
		{
			name:   "app",
			kind:   VirtualPathAppCert,
			params: VirtualPathAppCertParams("foo"),
			expected: []string{
				"TSH_VIRTUAL_PATH_APP_FOO",
				"TSH_VIRTUAL_PATH_APP",
			},
		},
		{
			name:   "app key",
			kind:   VirtualPathKey,
			params: VirtualPathAppKeyParams("foo"),
			expected: []string{
				"TSH_VIRTUAL_PATH_KEY_APP_FOO",
				"TSH_VIRTUAL_PATH_KEY_APP",
				"TSH_VIRTUAL_PATH_KEY",
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

func TestFormatConnectToProxyErr(t *testing.T) {
	tests := []struct {
		name string
		err  error

		wantError       string
		wantUserMessage string
	}{
		{
			name: "nil error passes through",
			err:  nil,
		},
		{
			name:      "unrelated error passes through",
			err:       fmt.Errorf("flux capacitor undercharged"),
			wantError: "flux capacitor undercharged",
		},
		{
			name:            "principals mismatch user message injected",
			err:             trace.Wrap(fmt.Errorf(`ssh: handshake failed: ssh: principal "" not in the set of valid principals for given certificate`)),
			wantError:       `ssh: handshake failed: ssh: principal "" not in the set of valid principals for given certificate`,
			wantUserMessage: unconfiguredPublicAddrMsg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := formatConnectToProxyErr(tt.err)
			if tt.wantError == "" {
				require.NoError(t, err)
				return
			}
			var traceErr *trace.TraceErr
			if errors.As(err, &traceErr) {
				require.EqualError(t, traceErr.OrigError(), tt.wantError)
			} else {
				require.EqualError(t, err, tt.wantError)
			}

			if tt.wantUserMessage != "" {
				require.Error(t, traceErr)
				require.Contains(t, traceErr.Messages, tt.wantUserMessage)
			}
		})
	}
}

type mockRoleGetter func(ctx context.Context) ([]types.Role, error)

func (m mockRoleGetter) GetRoles(ctx context.Context) ([]types.Role, error) {
	return m(ctx)
}

func TestCommandLimit(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		mfaRequired bool
		getter      roleGetter
		expected    int
	}{
		{
			name:        "mfa required",
			mfaRequired: true,
			expected:    1,
			getter: mockRoleGetter(func(ctx context.Context) ([]types.Role, error) {
				role, err := types.NewRole("test", types.RoleSpecV6{
					Options: types.RoleOptions{MaxConnections: 500},
				})
				require.NoError(t, err)

				return []types.Role{role}, nil
			}),
		},
		{
			name:     "failure getting roles",
			expected: 1,
			getter: mockRoleGetter(func(ctx context.Context) ([]types.Role, error) {
				return nil, errors.New("fail")
			}),
		},
		{
			name:     "no roles",
			expected: -1,
			getter: mockRoleGetter(func(ctx context.Context) ([]types.Role, error) {
				return nil, nil
			}),
		},
		{
			name:     "max_connections=1",
			expected: 1,
			getter: mockRoleGetter(func(ctx context.Context) ([]types.Role, error) {
				role, err := types.NewRole("test", types.RoleSpecV6{
					Options: types.RoleOptions{MaxConnections: 1},
				})
				require.NoError(t, err)

				return []types.Role{role}, nil
			}),
		},
		{
			name:     "max_connections=2",
			expected: 1,
			getter: mockRoleGetter(func(ctx context.Context) ([]types.Role, error) {
				role, err := types.NewRole("test", types.RoleSpecV6{
					Options: types.RoleOptions{MaxConnections: 2},
				})
				require.NoError(t, err)

				return []types.Role{role}, nil
			}),
		},
		{
			name:     "max_connections=500",
			expected: 250,
			getter: mockRoleGetter(func(ctx context.Context) ([]types.Role, error) {
				role, err := types.NewRole("test", types.RoleSpecV6{
					Options: types.RoleOptions{MaxConnections: 500},
				})
				require.NoError(t, err)

				return []types.Role{role}, nil
			}),
		},
		{
			name:     "max_connections=max",
			expected: math.MaxInt64 / 2,
			getter: mockRoleGetter(func(ctx context.Context) ([]types.Role, error) {
				role, err := types.NewRole("test", types.RoleSpecV6{
					Options: types.RoleOptions{MaxConnections: math.MaxInt64},
				})
				require.NoError(t, err)

				return []types.Role{role}, nil
			}),
		},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.expected, commandLimit(context.Background(), tt.getter, tt.mfaRequired))
		})
	}
}

func TestRootClusterName(t *testing.T) {
	ctx := context.Background()
	ca := newTestAuthority(t)

	rootCluster := ca.trustedCerts.ClusterName
	leafCluster := "leaf-cluster"
	keyRing := ca.makeSignedKeyRing(t, KeyRingIndex{
		ProxyHost:   "proxy.example.com",
		ClusterName: leafCluster,
		Username:    "teleport-user",
	}, false)

	for _, tc := range []struct {
		name      string
		modifyCfg func(t *Config)
	}{
		{
			name: "static TLS",
			modifyCfg: func(c *Config) {
				tlsConfig, err := keyRing.TeleportClientTLSConfig(nil, []string{leafCluster, rootCluster})
				require.NoError(t, err)
				c.TLS = tlsConfig
			},
		}, {
			name: "key store",
			modifyCfg: func(c *Config) {
				c.ClientStore = NewMemClientStore()
				err := c.ClientStore.AddKeyRing(keyRing)
				require.NoError(t, err)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				WebProxyAddr: "proxy.example.com",
				Username:     "teleport-user",
				SiteName:     leafCluster,
			}
			tc.modifyCfg(cfg)

			tc, err := NewClient(cfg)
			require.NoError(t, err)

			clusterName, err := tc.RootClusterName(ctx)
			require.NoError(t, err)
			require.Equal(t, rootCluster, clusterName)
		})
	}
}

func TestLoadTLSConfigForClusters(t *testing.T) {
	rootCA := newTestAuthority(t)

	rootCluster := rootCA.trustedCerts.ClusterName
	keyRing := rootCA.makeSignedKeyRing(t, KeyRingIndex{
		ProxyHost:   "proxy.example.com",
		ClusterName: rootCluster,
		Username:    "teleport-user",
	}, false)

	tlsCertPoolNoCA, err := keyRing.clientCertPool()
	require.NoError(t, err)
	tlsCertPoolRootCA, err := keyRing.clientCertPool(rootCluster)
	require.NoError(t, err)

	tlsConfig, err := keyRing.TeleportClientTLSConfig(nil, []string{rootCluster})
	require.NoError(t, err)

	for _, tt := range []struct {
		name      string
		clusters  []string
		modifyCfg func(t *Config)
		expectCAs *x509.CertPool
	}{
		{
			name:     "static TLS",
			clusters: []string{rootCluster},
			modifyCfg: func(c *Config) {
				c.TLS = tlsConfig.Clone()
			},
			expectCAs: tlsCertPoolRootCA,
		}, {
			name:     "key store no clusters",
			clusters: []string{},
			modifyCfg: func(c *Config) {
				c.ClientStore = NewMemClientStore()
				err := c.ClientStore.AddKeyRing(keyRing)
				require.NoError(t, err)
			},
			expectCAs: tlsCertPoolNoCA,
		}, {
			name:     "key store root cluster",
			clusters: []string{rootCluster},
			modifyCfg: func(c *Config) {
				c.ClientStore = NewMemClientStore()
				err := c.ClientStore.AddKeyRing(keyRing)
				require.NoError(t, err)
			},
			expectCAs: tlsCertPoolRootCA,
		}, {
			name:     "key store unknown clusters",
			clusters: []string{"leaf-1", "leaf-2"},
			modifyCfg: func(c *Config) {
				c.ClientStore = NewMemClientStore()
				err := c.ClientStore.AddKeyRing(keyRing)
				require.NoError(t, err)
			},
			expectCAs: tlsCertPoolNoCA,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				WebProxyAddr: "proxy.example.com",
				Username:     "teleport-user",
				SiteName:     rootCluster,
			}
			tt.modifyCfg(cfg)

			tc, err := NewClient(cfg)
			require.NoError(t, err)

			tlsConfig, err := tc.LoadTLSConfigForClusters(tt.clusters)
			require.NoError(t, err)
			require.True(t, tlsConfig.RootCAs.Equal(tt.expectCAs))
		})
	}
}

func TestConnectToProxyCancelledContext(t *testing.T) {
	cfg := MakeDefaultConfig()

	cfg.Agent = &mockAgent{}
	cfg.AuthMethods = []ssh.AuthMethod{ssh.Password("xyz")}
	cfg.AddKeysToAgent = AddKeysToAgentNo
	cfg.WebProxyAddr = "dummy"
	cfg.KeysDir = t.TempDir()
	cfg.TLSRoutingEnabled = true

	clt, err := NewClient(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	clusterClient, err := clt.ConnectToCluster(ctx)
	require.Nil(t, clusterClient)
	require.Error(t, err)
}

func TestIsErrorResolvableWithRelogin(t *testing.T) {
	for _, tt := range []struct {
		name             string
		err              error
		expectResolvable bool
	}{
		{
			name:             "private key policy error should be resolvable",
			err:              keys.NewPrivateKeyPolicyError(keys.PrivateKeyPolicyHardwareKey),
			expectResolvable: true,
		}, {
			name: "wrapped private key policy error should be resolvable",
			err: &interceptors.RemoteError{
				Err: keys.NewPrivateKeyPolicyError(keys.PrivateKeyPolicyHardwareKey),
			},
			expectResolvable: true,
		},
		{
			name:             "trace.BadParameter should be resolvable",
			err:              trace.BadParameter("bad"),
			expectResolvable: true,
		},
		{
			name: "nonRetryableError should not be resolvable",
			err: trace.Wrap(&NonRetryableError{
				Err: trace.BadParameter("bad"),
			}),
			expectResolvable: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			resolvable := IsErrorResolvableWithRelogin(tt.err)
			if tt.expectResolvable {
				require.True(t, resolvable, "Expected error to be resolvable with relogin")
			} else {
				require.False(t, resolvable, "Expected error to be unresolvable with relogin")
			}
		})
	}
}

type fakeResourceClient struct {
	apiclient.GetResourcesClient

	nodes []*types.ServerV2
}

func (f fakeResourceClient) GetResources(ctx context.Context, req *proto.ListResourcesRequest) (*proto.ListResourcesResponse, error) {
	out := make([]*proto.PaginatedResource, 0, len(f.nodes))
	for _, n := range f.nodes {
		out = append(out, &proto.PaginatedResource{Resource: &proto.PaginatedResource_Node{Node: n}})
	}

	return &proto.ListResourcesResponse{Resources: out}, nil
}

func (f fakeResourceClient) ListUnifiedResources(ctx context.Context, req *proto.ListUnifiedResourcesRequest) (*proto.ListUnifiedResourcesResponse, error) {
	out := make([]*proto.PaginatedResource, 0, len(f.nodes))
	for _, n := range f.nodes {
		out = append(out, &proto.PaginatedResource{Resource: &proto.PaginatedResource_Node{Node: n}})
	}

	return &proto.ListUnifiedResourcesResponse{Resources: out}, nil
}

func TestGetTargetNodes(t *testing.T) {
	tests := []struct {
		name      string
		options   SSHOptions
		labels    map[string]string
		search    []string
		predicate string
		host      string
		port      int
		clt       fakeResourceClient
		expected  []TargetNode
	}{
		{
			name: "options override",
			options: SSHOptions{
				HostAddress: "test:1234",
			},
			expected: []TargetNode{{Hostname: "test:1234", Addr: "test:1234"}},
		},
		{
			name:     "explicit target",
			host:     "test",
			port:     1234,
			expected: []TargetNode{{Hostname: "test", Addr: "test:1234"}},
		},
		{
			name:     "labels",
			labels:   map[string]string{"foo": "bar"},
			expected: []TargetNode{{Hostname: "labels", Addr: "abcd:0"}},
			clt:      fakeResourceClient{nodes: []*types.ServerV2{{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "labels"}}}},
		},
		{
			name:     "search",
			search:   []string{"foo", "bar"},
			expected: []TargetNode{{Hostname: "search", Addr: "abcd:0"}},
			clt:      fakeResourceClient{nodes: []*types.ServerV2{{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "search"}}}},
		},
		{
			name:      "predicate",
			predicate: `resource.spec.hostname == "test"`,
			expected:  []TargetNode{{Hostname: "predicate", Addr: "abcd:0"}},
			clt:       fakeResourceClient{nodes: []*types.ServerV2{{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "predicate"}}}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clt := TeleportClient{
				Config: Config{
					Tracer:              tracing.NoopTracer(""),
					Labels:              test.labels,
					SearchKeywords:      test.search,
					PredicateExpression: test.predicate,
					Host:                test.host,
					HostPort:            test.port,
				},
			}

			match, err := clt.GetTargetNodes(context.Background(), test.clt, test.options)
			require.NoError(t, err)
			require.EqualValues(t, test.expected, match)
		})
	}
}

type fakeGetTargetNodeClient struct {
	authclient.ClientI

	nodes             []*types.ServerV2
	resolved          *types.ServerV2
	resolveErr        error
	routeToMostRecent bool
}

func (f fakeGetTargetNodeClient) ListUnifiedResources(ctx context.Context, req *proto.ListUnifiedResourcesRequest) (*proto.ListUnifiedResourcesResponse, error) {
	out := make([]*proto.PaginatedResource, 0, len(f.nodes))
	for _, n := range f.nodes {
		out = append(out, &proto.PaginatedResource{Resource: &proto.PaginatedResource_Node{Node: n}})
	}

	return &proto.ListUnifiedResourcesResponse{Resources: out}, nil
}

func (f fakeGetTargetNodeClient) ResolveSSHTarget(ctx context.Context, req *proto.ResolveSSHTargetRequest) (*proto.ResolveSSHTargetResponse, error) {
	if f.resolveErr != nil {
		return nil, f.resolveErr
	}

	return &proto.ResolveSSHTargetResponse{Server: f.resolved}, nil
}

func (f fakeGetTargetNodeClient) GetClusterNetworkingConfig(ctx context.Context) (types.ClusterNetworkingConfig, error) {
	cfg := types.DefaultClusterNetworkingConfig()
	if f.routeToMostRecent {
		cfg.SetRoutingStrategy(types.RoutingStrategy_MOST_RECENT)
	}

	return cfg, nil
}

func TestGetTargetNode(t *testing.T) {
	now := time.Now()
	then := now.Add(-5 * time.Hour)

	tests := []struct {
		name         string
		options      *SSHOptions
		labels       map[string]string
		search       []string
		predicate    string
		host         string
		port         int
		clt          fakeGetTargetNodeClient
		errAssertion require.ErrorAssertionFunc
		expected     TargetNode
	}{
		{
			name: "options override",
			options: &SSHOptions{
				HostAddress: "test:1234",
			},
			host:         "llama",
			port:         56789,
			errAssertion: require.NoError,
			expected:     TargetNode{Hostname: "test:1234", Addr: "test:1234"},
		},
		{
			name:         "explicit target",
			host:         "test",
			port:         1234,
			errAssertion: require.NoError,
			expected:     TargetNode{Hostname: "test", Addr: "test:1234"},
		},
		{
			name:         "resolved labels",
			labels:       map[string]string{"foo": "bar"},
			errAssertion: require.NoError,
			expected:     TargetNode{Hostname: "resolved-labels", Addr: "abcd:0"},
			clt: fakeGetTargetNodeClient{
				nodes:    []*types.ServerV2{{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "labels"}}},
				resolved: &types.ServerV2{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "resolved-labels"}},
			},
		},
		{
			name:         "fallback labels",
			labels:       map[string]string{"foo": "bar"},
			errAssertion: require.NoError,
			expected:     TargetNode{Hostname: "labels", Addr: "abcd:0"},
			clt: fakeGetTargetNodeClient{
				nodes:      []*types.ServerV2{{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "labels"}}},
				resolved:   &types.ServerV2{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "resolved-labels"}},
				resolveErr: trace.NotImplemented(""),
			},
		},
		{
			name:         "resolved search",
			search:       []string{"foo", "bar"},
			errAssertion: require.NoError,
			expected:     TargetNode{Hostname: "resolved-search", Addr: "abcd:0"},
			clt: fakeGetTargetNodeClient{
				nodes:    []*types.ServerV2{{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "search"}}},
				resolved: &types.ServerV2{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "resolved-search"}},
			},
		},

		{
			name:         "fallback search",
			search:       []string{"foo", "bar"},
			errAssertion: require.NoError,
			expected:     TargetNode{Hostname: "search", Addr: "abcd:0"},
			clt: fakeGetTargetNodeClient{
				nodes:      []*types.ServerV2{{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "search"}}},
				resolveErr: trace.NotImplemented(""),
				resolved:   &types.ServerV2{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "resolved-search"}},
			},
		},
		{
			name:         "resolved predicate",
			predicate:    `resource.spec.hostname == "test"`,
			errAssertion: require.NoError,
			expected:     TargetNode{Hostname: "resolved-predicate", Addr: "abcd:0"},
			clt: fakeGetTargetNodeClient{
				nodes:    []*types.ServerV2{{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "predicate"}}},
				resolved: &types.ServerV2{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "resolved-predicate"}},
			},
		},
		{
			name:         "fallback predicate",
			predicate:    `resource.spec.hostname == "test"`,
			errAssertion: require.NoError,
			expected:     TargetNode{Hostname: "predicate", Addr: "abcd:0"},
			clt: fakeGetTargetNodeClient{
				nodes:      []*types.ServerV2{{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "predicate"}}},
				resolveErr: trace.NotImplemented(""),
				resolved:   &types.ServerV2{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "resolved-predicate"}},
			},
		},
		{
			name:         "fallback ambiguous hosts",
			predicate:    `resource.spec.hostname == "test"`,
			errAssertion: require.Error,
			clt: fakeGetTargetNodeClient{
				nodes: []*types.ServerV2{
					{Metadata: types.Metadata{Name: "abcd-1"}, Spec: types.ServerSpecV2{Hostname: "predicate"}},
					{Metadata: types.Metadata{Name: "abcd-2"}, Spec: types.ServerSpecV2{Hostname: "predicate"}},
				},
				resolveErr: trace.NotImplemented(""),
				resolved:   &types.ServerV2{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "resolved-predicate"}},
			},
		},
		{
			name:         "fallback and route to recent",
			predicate:    `resource.spec.hostname == "test"`,
			errAssertion: require.NoError,
			expected:     TargetNode{Hostname: "predicate-now", Addr: "abcd-1:0"},
			clt: fakeGetTargetNodeClient{
				nodes: []*types.ServerV2{
					{Metadata: types.Metadata{Name: "abcd-0", Expires: &then}, Spec: types.ServerSpecV2{Hostname: "predicate-then"}},
					{Metadata: types.Metadata{Name: "abcd-1", Expires: &now}, Spec: types.ServerSpecV2{Hostname: "predicate-now"}},
					{Metadata: types.Metadata{Name: "abcd-2", Expires: &then}, Spec: types.ServerSpecV2{Hostname: "predicate-then-again"}},
				},
				resolveErr:        trace.NotImplemented(""),
				routeToMostRecent: true,
				resolved:          &types.ServerV2{Metadata: types.Metadata{Name: "abcd"}, Spec: types.ServerSpecV2{Hostname: "resolved-predicate"}},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			clt := TeleportClient{
				Config: Config{
					Tracer:              tracing.NoopTracer(""),
					Labels:              test.labels,
					SearchKeywords:      test.search,
					PredicateExpression: test.predicate,
					Host:                test.host,
					HostPort:            test.port,
				},
			}

			match, err := clt.GetTargetNode(context.Background(), test.clt, test.options)
			test.errAssertion(t, err)
			if match == nil {
				match = &TargetNode{}
			}
			require.EqualValues(t, test.expected, *match)
		})
	}
}

func TestNonRetryableError(t *testing.T) {
	orgError := trace.AccessDenied("do not enter")
	err := &NonRetryableError{
		Err: orgError,
	}
	require.Error(t, err)
	assert.Equal(t, "do not enter", err.Error())
	assert.True(t, IsNonRetryableError(err))
	assert.True(t, trace.IsAccessDenied(err))
	assert.Equal(t, orgError, err.Unwrap())
}

func TestWarningAboutIncompatibleClientVersion(t *testing.T) {
	tests := []struct {
		name            string
		clientVersion   string
		serverVersion   string
		expectedWarning string
	}{
		{
			name:          "client on a higher major version than server triggers a warning",
			clientVersion: "17.0.0",
			serverVersion: "16.0.0",
			expectedWarning: `
WARNING
Detected potentially incompatible client and server versions.
Maximum client version supported by the server is 16.x.x but you are using 17.0.0.
Please downgrade tsh to 16.x.x or use the --skip-version-check flag to bypass this check.
Future versions of tsh will fail when incompatible versions are detected.

`,
		},
		{
			name:          "client on a too low major version compared to server triggers a warning",
			clientVersion: "16.4.0",
			serverVersion: "18.0.0",
			expectedWarning: `
WARNING
Detected potentially incompatible client and server versions.
Minimum client version supported by the server is 17.0.0 but you are using 16.4.0.
Please upgrade tsh to 17.0.0 or newer or use the --skip-version-check flag to bypass this check.
Future versions of tsh will fail when incompatible versions are detected.

`,
		},
		{
			name:            "client on a higher minor version than server does not trigger a warning",
			clientVersion:   "17.1.0",
			serverVersion:   "17.0.0",
			expectedWarning: "",
		},
		{
			name:            "client on a lower major version than server does not trigger a warning",
			clientVersion:   "17.0.0",
			serverVersion:   "18.0.0",
			expectedWarning: "",
		},
		{
			name:            "client and server on the same version do not trigger a warning",
			clientVersion:   "18.0.0",
			serverVersion:   "18.0.0",
			expectedWarning: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			minClientVersion, err := semver.NewVersion(test.serverVersion)
			require.NoError(t, err)
			minClientVersion.Major = minClientVersion.Major - 1
			warning, err := getClientIncompatibilityWarning(versions{
				MinClient: minClientVersion.String(),
				Client:    test.clientVersion,
				Server:    test.serverVersion,
			})
			require.NoError(t, err)
			require.Equal(t, test.expectedWarning, warning)
		})
	}
}

func TestParsePortMapping(t *testing.T) {
	tests := []struct {
		in      string
		want    PortMapping
		wantErr bool
	}{
		{
			in:   "",
			want: PortMapping{},
		},
		{
			in:   "1337",
			want: PortMapping{LocalPort: 1337},
		},
		{
			in:   "1337:42",
			want: PortMapping{LocalPort: 1337, TargetPort: 42},
		},
		{
			in:   "0:0",
			want: PortMapping{},
		},
		{
			in:   "0:42",
			want: PortMapping{TargetPort: 42},
		},
		{
			in:      " ",
			wantErr: true,
		},
		{
			in:      "1337:",
			wantErr: true,
		},
		{
			in:      ":42",
			wantErr: true,
		},
		{
			in:      "13371337",
			wantErr: true,
		},
		{
			in:      "42:73317331",
			wantErr: true,
		},
		{
			in:      "1337:42:42",
			wantErr: true,
		},
		{
			in:      "1337:42:",
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			out, err := ParsePortMapping(test.in)
			if test.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.want, out)
			}
		})
	}
}
