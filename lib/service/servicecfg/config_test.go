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

package servicecfg

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/srv/app/common"
	"github.com/gravitational/teleport/lib/utils"
)

func TestDefaultConfig(t *testing.T) {
	config := MakeDefaultConfig()
	require.NotNil(t, config)

	// all 3 services should be enabled by default
	require.True(t, config.Auth.Enabled)
	require.True(t, config.SSH.Enabled)
	require.True(t, config.Proxy.Enabled)

	localAuthAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: "0.0.0.0:3025"}

	// data dir, hostname and auth server
	require.Equal(t, config.DataDir, defaults.DataDir)
	if len(config.Hostname) < 2 {
		t.Fatal("default hostname wasn't properly set")
	}

	// crypto settings
	require.Equal(t, config.CipherSuites, utils.DefaultCipherSuites())
	// Unfortunately, the below algos don't have exported constants in
	// golang.org/x/crypto/ssh for us to use.
	require.ElementsMatch(t, config.Ciphers, []string{
		"aes128-gcm@openssh.com",
		"aes256-gcm@openssh.com",
		"chacha20-poly1305@openssh.com",
		"aes128-ctr",
		"aes192-ctr",
		"aes256-ctr",
	})
	require.ElementsMatch(t, config.KEXAlgorithms, []string{
		"curve25519-sha256",
		"curve25519-sha256@libssh.org",
		"ecdh-sha2-nistp256",
		"ecdh-sha2-nistp384",
		"ecdh-sha2-nistp521",
		"diffie-hellman-group14-sha256",
	})
	require.ElementsMatch(t, config.MACAlgorithms, []string{
		"hmac-sha2-256-etm@openssh.com",
		"hmac-sha2-512-etm@openssh.com",
		"hmac-sha2-256",
		"hmac-sha2-512",
	})

	// auth section
	auth := config.Auth
	require.Equal(t, localAuthAddr, auth.ListenAddr)
	require.Equal(t, int64(defaults.LimiterMaxConnections), auth.Limiter.MaxConnections)
	require.Equal(t, defaults.LimiterMaxConcurrentUsers, auth.Limiter.MaxNumberOfUsers)
	require.Equal(t, lite.GetName(), config.Auth.StorageConfig.Type)
	require.Equal(t, filepath.Join(config.DataDir, defaults.BackendDir), auth.StorageConfig.Params[defaults.BackendPath])

	// SSH section
	ssh := config.SSH
	require.Equal(t, int64(defaults.LimiterMaxConnections), ssh.Limiter.MaxConnections)
	require.Equal(t, defaults.LimiterMaxConcurrentUsers, ssh.Limiter.MaxNumberOfUsers)
	require.True(t, ssh.AllowTCPForwarding)

	// proxy section
	proxy := config.Proxy
	require.Equal(t, int64(defaults.LimiterMaxConnections), proxy.Limiter.MaxConnections)
	require.Equal(t, defaults.LimiterMaxConcurrentUsers, proxy.Limiter.MaxNumberOfUsers)

	// Misc levers and dials
	require.Equal(t, defaults.HighResPollingPeriod, config.RotationConnectionInterval)
}

// TestCheckApp validates application configuration.
func TestCheckApp(t *testing.T) {
	type tc struct {
		desc  string
		inApp App
		err   string
	}
	tests := []tc{
		{
			desc: "valid subdomain",
			inApp: App{
				Name: "foo",
				URI:  "http://localhost",
			},
		},
		{
			desc: "subdomain cannot start with a dash",
			inApp: App{
				Name: "-foo",
				URI:  "http://localhost",
			},
			err: "must be a valid DNS subdomain",
		},
		{
			desc: `subdomain cannot contain the exclamation mark character "!"`,
			inApp: App{
				Name: "foo!bar",
				URI:  "http://localhost",
			},
			err: "must be a valid DNS subdomain",
		},
		{
			desc: "subdomain of length 63 characters is valid (maximum length)",
			inApp: App{
				Name: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				URI:  "http://localhost",
			},
		},
		{
			desc: "subdomain of length 64 characters is invalid",
			inApp: App{
				Name: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				URI:  "http://localhost",
			},
			err: "must be a valid DNS subdomain",
		},
	}
	for _, h := range common.ReservedHeaders {
		tests = append(tests, tc{
			desc: fmt.Sprintf("reserved header rewrite %v", h),
			inApp: App{
				Name: "foo",
				URI:  "http://localhost",
				Rewrite: &Rewrite{
					Headers: []Header{
						{
							Name:  h,
							Value: "rewritten",
						},
					},
				},
			},
			err: `invalid application "foo" header rewrite configuration`,
		})
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			err := tt.inApp.CheckAndSetDefaults()
			if tt.err != "" {
				require.Contains(t, err.Error(), tt.err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestCheckDatabase(t *testing.T) {
	tests := []struct {
		desc       string
		inDatabase Database
		outErr     bool
	}{
		{
			desc: "ok",
			inDatabase: Database{
				Name:     "example",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			},
			outErr: false,
		},
		{
			desc: "empty database name",
			inDatabase: Database{
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			},
			outErr: true,
		},
		{
			desc: "fails services.ValidateDatabase",
			inDatabase: Database{
				Name:     "??--++",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
			},
			outErr: true,
		},
		{
			desc: "GCP valid configuration",
			inDatabase: Database{
				Name:     "example",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				GCP: DatabaseGCP{
					ProjectID:  "project-1",
					InstanceID: "instance-1",
				},
				TLS: DatabaseTLS{
					CACert: fixtures.LocalhostCert,
				},
			},
			outErr: false,
		},
		{
			desc: "GCP project ID specified without instance ID",
			inDatabase: Database{
				Name:     "example",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				GCP: DatabaseGCP{
					ProjectID: "project-1",
				},
				TLS: DatabaseTLS{
					CACert: fixtures.LocalhostCert,
				},
			},
			outErr: true,
		},
		{
			desc: "GCP instance ID specified without project ID",
			inDatabase: Database{
				Name:     "example",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				GCP: DatabaseGCP{
					InstanceID: "instance-1",
				},
				TLS: DatabaseTLS{
					CACert: fixtures.LocalhostCert,
				},
			},
			outErr: true,
		},
		{
			desc: "SQL Server correct configuration",
			inDatabase: Database{
				Name:     "sqlserver",
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver.example.com:1433",
				AD: DatabaseAD{
					KeytabFile: "/etc/keytab",
					Domain:     "test-domain",
					SPN:        "test-spn",
				},
			},
			outErr: false,
		},
		{
			desc: "SQL Server missing keytab",
			inDatabase: Database{
				Name:     "sqlserver",
				Protocol: defaults.ProtocolSQLServer,
				URI:      "localhost:1433",
				AD: DatabaseAD{
					Domain: "test-domain",
					SPN:    "test-spn",
				},
			},
			outErr: true,
		},
		{
			desc: "SQL Server missing AD domain",
			inDatabase: Database{
				Name:     "sqlserver",
				Protocol: defaults.ProtocolSQLServer,
				URI:      "localhost:1433",
				AD: DatabaseAD{
					KeytabFile: "/etc/keytab",
					SPN:        "test-spn",
				},
			},
			outErr: true,
		},
		{
			desc: "SQL Server missing SPN",
			inDatabase: Database{
				Name:     "sqlserver",
				Protocol: defaults.ProtocolSQLServer,
				URI:      "localhost:1433",
				AD: DatabaseAD{
					KeytabFile: "/etc/keytab",
					Domain:     "test-domain",
				},
			},
			outErr: true,
		},
		{
			desc: "SQL Server missing LDAP Cert",
			inDatabase: Database{
				Name:     "sqlserver",
				Protocol: defaults.ProtocolSQLServer,
				URI:      "localhost:1433",
				AD: DatabaseAD{
					Domain:      "test-domain",
					SPN:         "test-spn",
					KDCHostName: "test-domain",
				},
			},
			outErr: true,
		},
		{
			desc: "SQL Server missing KDC Hostname",
			inDatabase: Database{
				Name:     "sqlserver",
				Protocol: defaults.ProtocolSQLServer,
				URI:      "localhost:1433",
				AD: DatabaseAD{
					Domain:   "test-domain",
					SPN:      "test-spn",
					LDAPCert: "random-content",
				},
			},
			outErr: true,
		},
		{
			desc: "MySQL with server version",
			inDatabase: Database{
				Name:     "mysql-foo",
				Protocol: defaults.ProtocolMySQL,
				URI:      "localhost:3306",
				MySQL: MySQLOptions{
					ServerVersion: "8.0.31",
				},
			},
			outErr: false,
		},
	}
	for _, test := range tests {
		test := test
		t.Run(test.desc, func(t *testing.T) {
			t.Parallel()

			err := test.inDatabase.CheckAndSetDefaults()
			if test.outErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestParseHeaders validates parsing of strings into http header objects.
func TestParseHeaders(t *testing.T) {
	tests := []struct {
		desc string
		in   []string
		out  []Header
		err  string
	}{
		{
			desc: "parse multiple headers",
			in: []string{
				"Host: example.com    ",
				"X-Teleport-Logins: root, {{internal.logins}}",
				"X-Env  : {{external.env}}",
				"X-Env: env:prod",
				"X-Empty:",
			},
			out: []Header{
				{Name: "Host", Value: "example.com"},
				{Name: "X-Teleport-Logins", Value: "root, {{internal.logins}}"},
				{Name: "X-Env", Value: "{{external.env}}"},
				{Name: "X-Env", Value: "env:prod"},
				{Name: "X-Empty", Value: ""},
			},
		},
		{
			desc: "invalid header format (missing value)",
			in:   []string{"X-Header"},
			err:  `failed to parse "X-Header" as http header`,
		},
		{
			desc: "invalid header name (empty)",
			in:   []string{": missing"},
			err:  `invalid http header name: ": missing"`,
		},
		{
			desc: "invalid header name (space)",
			in:   []string{"X Space: space"},
			err:  `invalid http header name: "X Space: space"`,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			out, err := ParseHeaders(test.in)
			if test.err != "" {
				require.EqualError(t, err, test.err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.out, out)
			}
		})
	}
}

// TestHostLabelMatching tests regex-based host matching.
func TestHostLabelMatching(t *testing.T) {
	matchAllRule := regexp.MustCompile(`^.+`)

	for _, test := range []struct {
		desc      string
		hostnames []string
		rules     HostLabelRules
		expected  map[string]string
	}{
		{
			desc:      "single rule matches all",
			hostnames: []string{"foo", "foo.bar", "127.0.0.1", "test.example.com"},
			rules:     NewHostLabelRules(HostLabelRule{Regexp: matchAllRule, Labels: map[string]string{"foo": "bar"}}),
			expected:  map[string]string{"foo": "bar"},
		},
		{
			desc:      "only one rule matches",
			hostnames: []string{"db.example.com"},
			rules: NewHostLabelRules(
				HostLabelRule{Regexp: regexp.MustCompile(`^db\.example\.com$`), Labels: map[string]string{"role": "db"}},
				HostLabelRule{Regexp: regexp.MustCompile(`^app\.example\.com$`), Labels: map[string]string{"role": "app"}},
			),
			expected: map[string]string{"role": "db"},
		},
		{
			desc:      "all rules match",
			hostnames: []string{"test.example.com"},
			rules: NewHostLabelRules(
				HostLabelRule{Regexp: regexp.MustCompile(`\.example\.com$`), Labels: map[string]string{"foo": "bar"}},
				HostLabelRule{Regexp: regexp.MustCompile(`\.example\.com$`), Labels: map[string]string{"baz": "quux"}},
			),
			expected: map[string]string{"foo": "bar", "baz": "quux"},
		},
		{
			desc:      "no rules match",
			hostnames: []string{"test.example.com"},
			rules: NewHostLabelRules(
				HostLabelRule{Regexp: regexp.MustCompile(`\.xyz$`), Labels: map[string]string{"foo": "bar"}},
				HostLabelRule{Regexp: regexp.MustCompile(`\.xyz$`), Labels: map[string]string{"baz": "quux"}},
			),
			expected: map[string]string{},
		},
		{
			desc:      "conflicting rules, last one wins",
			hostnames: []string{"test.example.com"},
			rules: NewHostLabelRules(
				HostLabelRule{Regexp: regexp.MustCompile(`\.example\.com$`), Labels: map[string]string{"test": "one"}},
				HostLabelRule{Regexp: regexp.MustCompile(`^test\.`), Labels: map[string]string{"test": "two"}},
			),
			expected: map[string]string{"test": "two"},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			for _, host := range test.hostnames {
				require.Equal(t, test.expected, test.rules.LabelsForHost(host))
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		desc    string
		config  *Config
		wantErr string
	}{
		{
			desc: "invalid version",
			config: &Config{
				Version: "v1.1",
			},
			wantErr: fmt.Sprintf("version must be one of %s", strings.Join(defaults.TeleportConfigVersions, ", ")),
		},
		{
			desc: "no service enabled",
			config: &Config{
				Version: defaults.TeleportConfigVersionV2,
			},
			wantErr: "config: enable at least one of auth_service, ssh_service, proxy_service, app_service, database_service, kubernetes_service, windows_desktop_service, discovery_service, okta_service ",
		},
		{
			desc: "no auth_servers or proxy_server specified",
			config: &Config{
				Version: defaults.TeleportConfigVersionV3,
				Auth: AuthConfig{
					Enabled: true,
				},
			},
			wantErr: "config: auth_server or proxy_server is required",
		},
		{
			desc: "no auth_servers specified",
			config: &Config{
				Version: defaults.TeleportConfigVersionV2,
				Auth: AuthConfig{
					Enabled: true,
				},
			},
			wantErr: "config: auth_servers is required",
		},
		{
			desc: "specifying proxy_server with the wrong config version",
			config: &Config{
				Version: defaults.TeleportConfigVersionV2,
				Auth: AuthConfig{
					Enabled: true,
				},
				ProxyServer: *utils.MustParseAddr("0.0.0.0"),
			},
			wantErr: "config: proxy_server is supported from config version v3 onwards",
		},
		{
			desc: "specifying auth_server when app_service is enabled",
			config: &Config{
				Version: defaults.TeleportConfigVersionV3,
				Apps: AppsConfig{
					Enabled: true,
				},
				DataDir:     "/",
				authServers: []utils.NetAddr{*utils.MustParseAddr("0.0.0.0")},
			},
			wantErr: "config: when app_service is enabled, proxy_server must be specified instead of auth_server",
		},
		{
			desc: "specifying auth_server when db_service is enabled",
			config: &Config{
				Version: defaults.TeleportConfigVersionV3,
				Databases: DatabasesConfig{
					Enabled: true,
				},
				DataDir:     "/",
				authServers: []utils.NetAddr{*utils.MustParseAddr("0.0.0.0")},
			},
			wantErr: "config: when db_service is enabled, proxy_server must be specified instead of auth_server",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			err := ValidateConfig(test.config)
			if test.wantErr == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.wantErr)
			}
		})
	}
}

func TestVerifyEnabledService(t *testing.T) {
	tests := []struct {
		desc             string
		config           *Config
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			desc:             "auth enabled",
			config:           &Config{Auth: AuthConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "ssh enabled",
			config:           &Config{SSH: SSHConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "proxy enabled",
			config:           &Config{Proxy: ProxyConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "kube enabled",
			config:           &Config{Kube: KubeConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "apps enabled",
			config:           &Config{Apps: AppsConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "databases enabled",
			config:           &Config{Databases: DatabasesConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "windows desktop enabled",
			config:           &Config{WindowsDesktop: WindowsDesktopConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "discovery enabled",
			config:           &Config{Discovery: DiscoveryConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc:             "okta enabled",
			config:           &Config{Okta: OktaConfig{Enabled: true}},
			errAssertionFunc: require.NoError,
		},
		{
			desc: "jamf enabled",
			config: &Config{
				Jamf: JamfConfig{
					Spec: &types.JamfSpecV1{
						Enabled:     true,
						ApiEndpoint: "https://example.jamfcloud.com",
						Username:    "llama",
						Password:    "supersecret!!1!ONE",
					},
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			desc:   "nothing enabled",
			config: &Config{},
			errAssertionFunc: func(t require.TestingT, err error, _ ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "err is not a BadParameter error: %T", err)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			test.errAssertionFunc(t, verifyEnabledService(test.config))
		})
	}
}

func TestWebPublicAddr(t *testing.T) {
	tests := []struct {
		name     string
		config   ProxyConfig
		expected string
	}{
		{
			name:     "no public address specified",
			expected: "https://<proxyhost>:3080",
		},
		{
			name: "default port",
			config: ProxyConfig{
				PublicAddrs: []utils.NetAddr{
					{Addr: "0.0.0.0", AddrNetwork: "tcp"},
				},
			},
			expected: "https://0.0.0.0:3080",
		},
		{
			name: "non-default port",
			config: ProxyConfig{
				PublicAddrs: []utils.NetAddr{
					{Addr: "0.0.0.0:443", AddrNetwork: "tcp"},
				},
			},
			expected: "https://0.0.0.0:443",
		},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			out, err := test.config.WebPublicAddr()
			require.NoError(t, err)

			require.Equal(t, test.expected, out)
		})
	}
}
