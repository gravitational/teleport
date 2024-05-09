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

package config

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/installers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

type testConfigFiles struct {
	tempDir              string
	configFile           string // good
	configFileNoContent  string // empty file
	configFileBadContent string // garbage inside
	configFileStatic     string // file from a static YAML fixture
}

var testConfigs testConfigFiles

func writeTestConfigs() error {
	var err error

	testConfigs.tempDir, err = os.MkdirTemp("", "teleport-config")
	if err != nil {
		return err
	}
	// create a good config file fixture
	testConfigs.configFile = filepath.Join(testConfigs.tempDir, "good-config.yaml")
	if err = os.WriteFile(testConfigs.configFile, []byte(makeConfigFixture()), 0o660); err != nil {
		return err
	}
	// create a static config file fixture
	testConfigs.configFileStatic = filepath.Join(testConfigs.tempDir, "static-config.yaml")
	if err = os.WriteFile(testConfigs.configFileStatic, []byte(StaticConfigString), 0o660); err != nil {
		return err
	}
	// create an empty config file
	testConfigs.configFileNoContent = filepath.Join(testConfigs.tempDir, "empty-config.yaml")
	if err = os.WriteFile(testConfigs.configFileNoContent, []byte(""), 0o660); err != nil {
		return err
	}
	// create a bad config file fixture
	testConfigs.configFileBadContent = filepath.Join(testConfigs.tempDir, "bad-config.yaml")
	return os.WriteFile(testConfigs.configFileBadContent, []byte("bad-data!"), 0o660)
}

func (tc testConfigFiles) cleanup() {
	if tc.tempDir != "" {
		os.RemoveAll(tc.tempDir)
	}
}

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()

	if err := writeTestConfigs(); err != nil {
		testConfigs.cleanup()
		fmt.Println("failed writing test configs:", err)
		os.Exit(1)
	}
	res := m.Run()
	testConfigs.cleanup()
	os.Exit(res)
}

func TestSampleConfig(t *testing.T) {
	testCases := []struct {
		name                   string
		input                  SampleFlags
		expectError            bool
		expectClusterName      ClusterName
		expectLicenseFile      string
		expectProxyPublicAddrs apiutils.Strings
		expectProxyWebAddr     string
		expectProxyKeyPairs    []KeyPair
	}{
		{
			name: "ACMEEnabled",
			input: SampleFlags{
				ClusterName: "cookie.localhost",
				ACMEEnabled: true,
				ACMEEmail:   "alice@example.com",
				LicensePath: "/tmp/license.pem",
			},
			expectClusterName:      ClusterName("cookie.localhost"),
			expectLicenseFile:      "/tmp/license.pem",
			expectProxyPublicAddrs: apiutils.Strings{"cookie.localhost:443"},
			expectProxyWebAddr:     "0.0.0.0:443",
		},
		{
			name: "public and web addr",
			input: SampleFlags{
				PublicAddr: "tele.example.com:4422",
			},
			expectProxyPublicAddrs: apiutils.Strings{"tele.example.com:4422"},
			expectProxyWebAddr:     "0.0.0.0:4422",
		},
		{
			name: "public and web addr with default port",
			input: SampleFlags{
				PublicAddr: "tele.example.com",
			},
			expectProxyPublicAddrs: apiutils.Strings{"tele.example.com:443"},
			expectProxyWebAddr:     "0.0.0.0:443",
		},
		{
			name: "key file missing",
			input: SampleFlags{
				CertFile: "/var/lib/teleport/fullchain.pem",
			},
			expectError: true,
		},
		{
			name: "load x509 key pair failed",
			input: SampleFlags{
				KeyFile:  "/var/lib/teleport/key.pem",
				CertFile: "/var/lib/teleport/fullchain.pem",
			},
			expectError: true,
		},
		{
			name: "cluster name missing",
			input: SampleFlags{
				ACMEEnabled: true,
			},
			expectError: true,
		},
		{
			name: "ACMEEnabled conflict with key file",
			input: SampleFlags{
				ClusterName: "cookie.localhost",
				ACMEEnabled: true,
				KeyFile:     "/var/lib/teleport/privkey.pem",
				CertFile:    "/var/lib/teleport/fullchain.pem",
			},
			expectError: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			sfc, err := MakeSampleFileConfig(testCase.input)

			if testCase.expectError {
				require.Error(t, err)
				require.Nil(t, sfc)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, sfc)

			fn := filepath.Join(t.TempDir(), "default-config.yaml")
			err = os.WriteFile(fn, []byte(sfc.DebugDumpToYAML()), 0o660)
			require.NoError(t, err)

			// make sure it could be parsed:
			fc, err := ReadFromFile(fn)
			require.NoError(t, err)

			// validate a couple of values:
			require.Equal(t, defaults.DataDir, fc.Global.DataDir)
			require.Equal(t, "INFO", fc.Logger.Severity)
			require.Equal(t, testCase.expectClusterName, fc.Auth.ClusterName)
			require.Equal(t, testCase.expectLicenseFile, fc.Auth.LicenseFile)
			require.Equal(t, testCase.expectProxyWebAddr, fc.Proxy.WebAddr)
			require.ElementsMatch(t, testCase.expectProxyPublicAddrs, fc.Proxy.PublicAddr)
			require.ElementsMatch(t, testCase.expectProxyKeyPairs, fc.Proxy.KeyPairs)

			require.False(t, lib.IsInsecureDevMode())
		})
	}
}

// TestBooleanParsing tests that types.Bool and *types.BoolOption are parsed
// properly
func TestBooleanParsing(t *testing.T) {
	testCases := []struct {
		s string
		b bool
	}{
		{s: "true", b: true},
		{s: "'true'", b: true},
		{s: "yes", b: true},
		{s: "'yes'", b: true},
		{s: "'1'", b: true},
		{s: "1", b: true},
		{s: "no", b: false},
		{s: "0", b: false},
	}
	for i, tc := range testCases {
		msg := fmt.Sprintf("test case %v", i)
		conf, err := ReadFromString(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`
teleport:
  advertise_ip: 10.10.10.1
proxy_service:
  enabled: yes
  trust_x_forwarded_for: %v
auth_service:
  enabled: yes
  disconnect_expired_cert: %v
`, tc.s, tc.s))))
		require.NoError(t, err, msg)
		require.Equal(t, tc.b, conf.Proxy.TrustXForwardedFor.Value(), msg)
		require.Equal(t, tc.b, conf.Auth.DisconnectExpiredCert.Value, msg)
	}
}

// TestDurationParsing tests that duration options
// are parsed properly
func TestDuration(t *testing.T) {
	testCases := []struct {
		s string
		d time.Duration
	}{
		{s: "1s", d: time.Second},
		{s: "never", d: 0},
		{s: "'1m'", d: time.Minute},
	}
	for i, tc := range testCases {
		comment := fmt.Sprintf("test case %v", i)
		conf, err := ReadFromString(base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(`
teleport:
  advertise_ip: 10.10.10.1
auth_service:
  enabled: yes
  client_idle_timeout: %v
`, tc.s))))
		require.NoError(t, err, comment)
		require.Equal(t, tc.d, conf.Auth.ClientIdleTimeout.Value(), comment)
	}
}

func TestConfigReading(t *testing.T) {
	// non-existing file:
	conf, err := ReadFromFile("/heaven/trees/apple.ymL")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to open file")
	require.Nil(t, conf)
	// bad content:
	_, err = ReadFromFile(testConfigs.configFileBadContent)
	require.Error(t, err)
	// empty config (must not fail)
	conf, err = ReadFromFile(testConfigs.configFileNoContent)
	require.NoError(t, err)
	require.NotNil(t, conf)
	require.True(t, conf.Auth.Enabled())
	require.True(t, conf.Proxy.Enabled())
	require.True(t, conf.SSH.Enabled())
	require.False(t, conf.Kube.Enabled())
	require.False(t, conf.Discovery.Enabled())

	// good config
	conf, err = ReadFromFile(testConfigs.configFile)
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(conf, &FileConfig{
		Version: defaults.TeleportConfigVersionV1,
		Global: Global{
			NodeName:    NodeName,
			AuthServers: []string{"auth0.server.example.org:3024", "auth1.server.example.org:3024"},
			Limits: ConnectionLimits{
				MaxConnections: 100,
				MaxUsers:       5,
				Rates:          ConnectionRates,
			},
			Logger: Log{
				Output:   "stderr",
				Severity: "INFO",
				Format: LogFormat{
					Output: "text",
				},
			},
			Storage: backend.Config{
				Type: "bolt",
			},
			DataDir: "/path/to/data",
			CAPin:   apiutils.Strings([]string{"rsa256:123", "rsa256:456"}),
		},
		Auth: Auth{
			Service: Service{
				defaultEnabled: true,
				EnabledFlag:    "Yeah",
				ListenAddress:  "tcp://auth",
			},
			LicenseFile:           "lic.pem",
			DisconnectExpiredCert: types.NewBoolOption(true),
			ClientIdleTimeout:     types.Duration(17 * time.Second),
			WebIdleTimeout:        types.Duration(19 * time.Second),
			RoutingStrategy:       types.RoutingStrategy_MOST_RECENT,
			ProxyPingInterval:     types.Duration(10 * time.Second),
		},
		SSH: SSH{
			Service: Service{
				defaultEnabled: true,
				EnabledFlag:    "true",
				ListenAddress:  "tcp://ssh",
			},
			Labels:   Labels,
			Commands: CommandLabels,
		},
		Discovery: Discovery{
			Service: Service{
				defaultEnabled: false,
				EnabledFlag:    "true",
				ListenAddress:  "",
			},
			AWSMatchers: []AWSMatcher{
				{
					Types:   []string{"ec2"},
					Regions: []string{"us-west-1", "us-east-1"},
					Tags: map[string]apiutils.Strings{
						"a": {"b"},
					},
					AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
					ExternalID:    "externalID123",
				},
				{
					Types:   []string{"eks"},
					Regions: []string{"us-west-1", "us-east-1"},
					Tags: map[string]apiutils.Strings{
						"a": {"b"},
					},
					Integration:      "integration1",
					KubeAppDiscovery: true,
				},
			},
			AzureMatchers: []AzureMatcher{
				{
					Types:   []string{"aks"},
					Regions: []string{"uswest1"},
					ResourceTags: map[string]apiutils.Strings{
						"a": {"b"},
					},
					ResourceGroups: []string{"group1"},
					Subscriptions:  []string{"sub1"},
				},
			},
			GCPMatchers: []GCPMatcher{
				{
					Types:     []string{"gke"},
					Locations: []string{"uswest1"},
					Labels: map[string]apiutils.Strings{
						"a": {"b"},
					},
					ProjectIDs: []string{"p1", "p2"},
				},
			},
		},
		Proxy: Proxy{
			Service: Service{
				defaultEnabled: true,
				EnabledFlag:    "yes",
				ListenAddress:  "tcp://proxy_ssh_addr",
			},
			KeyFile:  "/etc/teleport/proxy.key",
			CertFile: "/etc/teleport/proxy.crt",
			KeyPairs: []KeyPair{
				{
					PrivateKey:  "/etc/teleport/proxy.key",
					Certificate: "/etc/teleport/proxy.crt",
				},
			},
			WebAddr: "tcp://web_addr",
			TunAddr: "reverse_tunnel_address:3311",
			IdP: IdP{
				SAMLIdP: SAMLIdP{
					EnabledFlag: "true",
					BaseURL:     "https://test-url.com",
				},
			},
		},
		Kube: Kube{
			Service: Service{
				EnabledFlag:   "yes",
				ListenAddress: "tcp://kube",
			},
			KubeClusterName: "kube-cluster",
			PublicAddr:      apiutils.Strings([]string{"kube-host:1234"}),
			ResourceMatchers: []ResourceMatcher{
				{
					Labels: map[string]apiutils.Strings{"*": {"*"}},
				},
			},
		},
		Apps: Apps{
			Service: Service{
				EnabledFlag: "yes",
			},
			Apps: []*App{
				{
					Name:          "foo",
					URI:           "http://127.0.0.1:8080",
					PublicAddr:    "foo.example.com",
					StaticLabels:  Labels,
					DynamicLabels: CommandLabels,
				},
			},
			ResourceMatchers: []ResourceMatcher{
				{
					Labels: map[string]apiutils.Strings{
						"*": {"*"},
					},
				},
			},
		},
		Databases: Databases{
			Service: Service{
				EnabledFlag: "yes",
			},
			Databases: []*Database{
				{
					Name:          "postgres",
					Protocol:      defaults.ProtocolPostgres,
					URI:           "localhost:5432",
					StaticLabels:  Labels,
					DynamicLabels: CommandLabels,
				},
			},
			ResourceMatchers: []ResourceMatcher{
				{
					Labels: map[string]apiutils.Strings{
						"a": {"b"},
					},
				},
				{
					Labels: map[string]apiutils.Strings{
						"c": {"d"},
					},
					AWS: ResourceMatcherAWS{
						AssumeRoleARN: "arn:aws:iam::123456789012:role/DBAccess",
						ExternalID:    "externalID123",
					},
				},
			},
			AWSMatchers: []AWSMatcher{
				{
					Types:   []string{"rds"},
					Regions: []string{"us-west-1", "us-east-1"},
					Tags: map[string]apiutils.Strings{
						"a": {"b"},
					},
					AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
					ExternalID:    "externalID123",
				},
				{
					Types:   []string{"rds"},
					Regions: []string{"us-central-1"},
					Tags: map[string]apiutils.Strings{
						"c": {"d"},
					},
					AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				},
			},
			AzureMatchers: []AzureMatcher{
				{
					Subscriptions:  []string{"sub1", "sub2"},
					ResourceGroups: []string{"rg1", "rg2"},
					Types:          []string{"mysql"},
					Regions:        []string{"eastus", "westus"},
					ResourceTags: map[string]apiutils.Strings{
						"a": {"b"},
					},
				},
				{
					Subscriptions:  []string{"sub3", "sub4"},
					ResourceGroups: []string{"rg3", "rg4"},
					Types:          []string{"postgres"},
					Regions:        []string{"centralus"},
					ResourceTags: map[string]apiutils.Strings{
						"c": {"d"},
					},
				},
				{
					Subscriptions:  nil,
					ResourceGroups: nil,
					Types:          []string{"mysql", "postgres"},
					Regions:        []string{"centralus"},
					ResourceTags: map[string]apiutils.Strings{
						"e": {"f"},
					},
				},
			},
		},
		Metrics: Metrics{
			Service: Service{
				ListenAddress: "tcp://metrics",
				EnabledFlag:   "yes",
			},
			KeyPairs: []KeyPair{
				{
					PrivateKey:  "/etc/teleport/proxy.key",
					Certificate: "/etc/teleport/proxy.crt",
				},
			},
			CACerts:           []string{"/etc/teleport/ca.crt"},
			GRPCServerLatency: true,
			GRPCClientLatency: true,
		},
		Debug: DebugService{
			Service: Service{
				defaultEnabled: true,
				EnabledFlag:    "yes",
			},
		},
		WindowsDesktop: WindowsDesktopService{
			Service: Service{
				EnabledFlag:   "yes",
				ListenAddress: "tcp://windows_desktop",
			},
			PublicAddr: apiutils.Strings([]string{"winsrv.example.com:3028", "no-port.winsrv.example.com"}),
			ADHosts:    apiutils.Strings([]string{"win.example.com:3389", "no-port.win.example.com"}),
		},
		Tracing: TracingService{
			EnabledFlag: "yes",
			ExporterURL: "https://localhost:4318",
			KeyPairs: []KeyPair{
				{
					PrivateKey:  "/etc/teleport/exporter.key",
					Certificate: "/etc/teleport/exporter.crt",
				},
			},
			CACerts:                []string{"/etc/teleport/exporter.crt"},
			SamplingRatePerMillion: 10,
		},
	}, cmp.AllowUnexported(Service{})))
	require.True(t, conf.Auth.Configured())
	require.True(t, conf.Auth.Enabled())
	require.True(t, conf.Proxy.Configured())
	require.True(t, conf.Proxy.Enabled())
	require.True(t, conf.SSH.Configured())
	require.True(t, conf.SSH.Enabled())
	require.True(t, conf.Kube.Configured())
	require.True(t, conf.Kube.Enabled())
	require.True(t, conf.Apps.Configured())
	require.True(t, conf.Apps.Enabled())
	require.True(t, conf.Databases.Configured())
	require.True(t, conf.Databases.Enabled())
	require.True(t, conf.Metrics.Configured())
	require.True(t, conf.Metrics.Enabled())
	require.True(t, conf.Debug.Configured())
	require.True(t, conf.Debug.Enabled())
	require.True(t, conf.WindowsDesktop.Configured())
	require.True(t, conf.WindowsDesktop.Enabled())
	require.True(t, conf.Tracing.Enabled())

	// good config from file
	conf, err = ReadFromFile(testConfigs.configFileStatic)
	require.NoError(t, err)
	require.NotNil(t, conf)
	checkStaticConfig(t, conf)

	// good config from base64 encoded string
	conf, err = ReadFromString(base64.StdEncoding.EncodeToString([]byte(StaticConfigString)))
	require.NoError(t, err)
	require.NotNil(t, conf)
	checkStaticConfig(t, conf)
}

func TestLabelParsing(t *testing.T) {
	var conf servicecfg.SSHConfig
	var err error
	// empty spec. no errors, no labels
	err = parseLabelsApply("", &conf)
	require.NoError(t, err)
	require.Nil(t, conf.CmdLabels)
	require.Nil(t, conf.Labels)

	// simple static labels
	err = parseLabelsApply(`key=value,more="much better"`, &conf)
	require.NoError(t, err)
	require.NotNil(t, conf.CmdLabels)
	require.Empty(t, conf.CmdLabels)
	require.Equal(t, map[string]string{
		"key":  "value",
		"more": "much better",
	}, conf.Labels)

	// static labels + command labels
	err = parseLabelsApply(`key=value,more="much better",arch=[5m2s:/bin/uname -m "p1 p2"]`, &conf)
	require.NoError(t, err)
	require.Equal(t, map[string]string{
		"key":  "value",
		"more": "much better",
	}, conf.Labels)
	require.Equal(t, services.CommandLabels{
		"arch": &types.CommandLabelV2{
			Period:  types.NewDuration(time.Minute*5 + time.Second*2),
			Command: []string{"/bin/uname", "-m", `"p1 p2"`},
		},
	}, conf.CmdLabels)
}

// TestFileConfigCheck makes sure we don't start with invalid settings.
func TestFileConfigCheck(t *testing.T) {
	tests := []struct {
		desc     string
		inConfig string
		outError bool
	}{
		{
			desc: "all defaults, valid",
			inConfig: `
teleport:
`,
		},
		{
			desc: "invalid cipher, not valid",
			inConfig: `
teleport:
  ciphers:
    - aes256-ctr
    - fake-cipher
  kex_algos:
    - kexAlgoCurve25519SHA256
  mac_algos:
    - hmac-sha2-256-etm@openssh.com
`,
			outError: true,
		},
		{
			desc: "proxy-peering, valid",
			inConfig: `
proxy_service:
  peer_listen_addr: peerhost:1234
  peer_public_addr: peer.example:1234
`,
		},
	}

	for _, tt := range tests {
		comment := fmt.Sprintf(tt.desc)

		_, err := ReadConfig(bytes.NewBufferString(tt.inConfig))
		if tt.outError {
			require.Error(t, err, comment)
		} else {
			require.NoError(t, err, comment)
		}
	}
}

func TestApplyConfig(t *testing.T) {
	tempDir := t.TempDir()
	authTokenPath := filepath.Join(tempDir, "small-config-token")
	err := os.WriteFile(authTokenPath, []byte("join-token"), 0o644)
	require.NoError(t, err)

	caPinPath := filepath.Join(tempDir, "small-config-ca-pin")
	err = os.WriteFile(caPinPath, []byte("ca-pin-from-file1\nca-pin-from-file2"), 0o644)
	require.NoError(t, err)

	staticTokenPath := filepath.Join(tempDir, "small-config-static-tokens")
	err = os.WriteFile(staticTokenPath, []byte("token-from-file1\ntoken-from-file2"), 0o644)
	require.NoError(t, err)

	pkcs11LibPath := filepath.Join(tempDir, "fake-pkcs11-lib.so")
	err = os.WriteFile(pkcs11LibPath, []byte("fake-pkcs11-lib"), 0o644)
	require.NoError(t, err)

	oktaAPITokenPath := filepath.Join(tempDir, "okta-api-token")
	err = os.WriteFile(oktaAPITokenPath, []byte("okta-api-token"), 0o644)
	require.NoError(t, err)

	conf, err := ReadConfig(bytes.NewBufferString(fmt.Sprintf(
		SmallConfigString,
		authTokenPath,
		caPinPath,
		staticTokenPath,
		pkcs11LibPath,
		oktaAPITokenPath,
	)))
	require.NoError(t, err)
	require.NotNil(t, conf)
	require.Equal(t, apiutils.Strings{"web3:443"}, conf.Proxy.PublicAddr)

	cfg := servicecfg.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.NoError(t, err)

	token, err := cfg.Token()
	require.NoError(t, err)

	require.Equal(t, "join-token", token)
	require.Equal(t, types.ProvisionTokensFromV1([]types.ProvisionTokenV1{
		{
			Token:   "xxx",
			Roles:   types.SystemRoles([]types.SystemRole{"Proxy", "Node"}),
			Expires: time.Unix(0, 0).UTC(),
		},
		{
			Token:   "token-from-file1",
			Roles:   types.SystemRoles([]types.SystemRole{"Node"}),
			Expires: time.Unix(0, 0).UTC(),
		},
		{
			Token:   "token-from-file2",
			Roles:   types.SystemRoles([]types.SystemRole{"Node"}),
			Expires: time.Unix(0, 0).UTC(),
		},
		{
			Token:   "yyy",
			Roles:   types.SystemRoles([]types.SystemRole{"Auth"}),
			Expires: time.Unix(0, 0).UTC(),
		},
	}), cfg.Auth.StaticTokens.GetStaticTokens())
	require.Equal(t, "magadan", cfg.Auth.ClusterName.GetClusterName())
	require.True(t, cfg.Auth.Preference.GetAllowLocalAuth())
	require.Equal(t, "10.10.10.1", cfg.AdvertiseIP)
	tunnelStrategyType, err := cfg.Auth.NetworkingConfig.GetTunnelStrategyType()
	require.NoError(t, err)
	require.Equal(t, types.AgentMesh, tunnelStrategyType)
	require.Equal(t, types.DefaultAgentMeshTunnelStrategy(), cfg.Auth.NetworkingConfig.GetAgentMeshTunnelStrategy())
	require.Equal(t, types.OriginConfigFile, cfg.Auth.Preference.Origin())
	require.Equal(t, types.OriginDefaults, cfg.Auth.NetworkingConfig.Origin())
	require.Equal(t, types.OriginDefaults, cfg.Auth.SessionRecordingConfig.Origin())

	require.True(t, cfg.Proxy.Enabled)
	require.Equal(t, "tcp://webhost:3080", cfg.Proxy.WebAddr.FullAddress())
	require.Equal(t, "tcp://tunnelhost:1001", cfg.Proxy.ReverseTunnelListenAddr.FullAddress())
	require.Equal(t, "tcp://webhost:3336", cfg.Proxy.MySQLAddr.FullAddress())
	require.Equal(t, "tcp://webhost:27017", cfg.Proxy.MongoAddr.FullAddress())
	require.Len(t, cfg.Proxy.PostgresPublicAddrs, 1)
	require.Equal(t, "tcp://postgres.example:5432", cfg.Proxy.PostgresPublicAddrs[0].FullAddress())
	require.Len(t, cfg.Proxy.MySQLPublicAddrs, 1)
	require.Equal(t, "tcp://mysql.example:3306", cfg.Proxy.MySQLPublicAddrs[0].FullAddress())
	require.Len(t, cfg.Proxy.MongoPublicAddrs, 1)
	require.Equal(t, "tcp://mongo.example:27017", cfg.Proxy.MongoPublicAddrs[0].FullAddress())
	require.Equal(t, "tcp://peerhost:1234", cfg.Proxy.PeerAddress.FullAddress())
	require.Equal(t, "tcp://peer.example:1234", cfg.Proxy.PeerPublicAddr.FullAddress())
	require.True(t, cfg.Proxy.IdP.SAMLIdP.Enabled)
	require.Equal(t, "", cfg.Proxy.IdP.SAMLIdP.BaseURL)

	require.Equal(t, "tcp://127.0.0.1:3000", cfg.DiagnosticAddr.FullAddress())

	u2fCAFromFile, err := os.ReadFile("testdata/u2f_attestation_ca.pem")
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(cfg.Auth.Preference, &types.AuthPreferenceV2{
		Kind:    types.KindClusterAuthPreference,
		Version: types.V2,
		Metadata: types.Metadata{
			Name:      "cluster-auth-preference",
			Namespace: apidefaults.Namespace,
			Labels:    map[string]string{types.OriginLabel: types.OriginConfigFile},
		},
		Spec: types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.SecondFactorOptional,
			Webauthn: &types.Webauthn{
				RPID: "goteleport.com",
				AttestationAllowedCAs: []string{
					string(u2fCAFromFile),
					`-----BEGIN CERTIFICATE-----
MIIDFzCCAf+gAwIBAgIDBAZHMA0GCSqGSIb3DQEBCwUAMCsxKTAnBgNVBAMMIFl1
YmljbyBQSVYgUm9vdCBDQSBTZXJpYWwgMjYzNzUxMCAXDTE2MDMxNDAwMDAwMFoY
DzIwNTIwNDE3MDAwMDAwWjArMSkwJwYDVQQDDCBZdWJpY28gUElWIFJvb3QgQ0Eg
U2VyaWFsIDI2Mzc1MTCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEBAMN2
cMTNR6YCdcTFRxuPy31PabRn5m6pJ+nSE0HRWpoaM8fc8wHC+Tmb98jmNvhWNE2E
ilU85uYKfEFP9d6Q2GmytqBnxZsAa3KqZiCCx2LwQ4iYEOb1llgotVr/whEpdVOq
joU0P5e1j1y7OfwOvky/+AXIN/9Xp0VFlYRk2tQ9GcdYKDmqU+db9iKwpAzid4oH
BVLIhmD3pvkWaRA2H3DA9t7H/HNq5v3OiO1jyLZeKqZoMbPObrxqDg+9fOdShzgf
wCqgT3XVmTeiwvBSTctyi9mHQfYd2DwkaqxRnLbNVyK9zl+DzjSGp9IhVPiVtGet
X02dxhQnGS7K6BO0Qe8CAwEAAaNCMEAwHQYDVR0OBBYEFMpfyvLEojGc6SJf8ez0
1d8Cv4O/MA8GA1UdEwQIMAYBAf8CAQEwDgYDVR0PAQH/BAQDAgEGMA0GCSqGSIb3
DQEBCwUAA4IBAQBc7Ih8Bc1fkC+FyN1fhjWioBCMr3vjneh7MLbA6kSoyWF70N3s
XhbXvT4eRh0hvxqvMZNjPU/VlRn6gLVtoEikDLrYFXN6Hh6Wmyy1GTnspnOvMvz2
lLKuym9KYdYLDgnj3BeAvzIhVzzYSeU77/Cupofj093OuAswW0jYvXsGTyix6B3d
bW5yWvyS9zNXaqGaUmP3U9/b6DlHdDogMLu3VLpBB9bm5bjaKWWJYgWltCVgUbFq
Fqyi4+JE014cSgR57Jcu3dZiehB6UtAPgad9L5cNvua/IWRmm+ANy3O2LH++Pyl8
SREzU8onbBsjMg9QDiSf5oJLKvd/Ren+zGY7
-----END CERTIFICATE-----
`,
				},
			},
			AllowLocalAuth:        types.NewBoolOption(true),
			DisconnectExpiredCert: types.NewBoolOption(false),
			LockingMode:           constants.LockingModeBestEffort,
			AllowPasswordless:     types.NewBoolOption(true),
			AllowHeadless:         types.NewBoolOption(true),
			DefaultSessionTTL:     types.Duration(apidefaults.CertDuration),
			IDP: &types.IdPOptions{
				SAML: &types.IdPSAMLOptions{
					Enabled: types.NewBoolOption(true),
				},
			},
			Okta: &types.OktaOptions{},
		},
	}, protocmp.Transform()))

	require.Equal(t, pkcs11LibPath, cfg.Auth.KeyStore.PKCS11.Path)
	require.Equal(t, "example_token", cfg.Auth.KeyStore.PKCS11.TokenLabel)
	require.Equal(t, 1, *cfg.Auth.KeyStore.PKCS11.SlotNumber)
	require.Equal(t, "example_pin", cfg.Auth.KeyStore.PKCS11.Pin)
	require.ElementsMatch(t, []string{"ca-pin-from-string", "ca-pin-from-file1", "ca-pin-from-file2"}, cfg.CAPins)

	require.True(t, cfg.Databases.Enabled)
	require.Empty(t, cmp.Diff(cfg.Databases.ResourceMatchers,
		[]services.ResourceMatcher{
			{
				Labels: map[string]apiutils.Strings{
					"*": {"*"},
				},
				AWS: services.ResourceMatcherAWS{
					AssumeRoleARN: "arn:aws:iam::123456789012:role/DBAccess",
					ExternalID:    "externalID123",
				},
			},
		},
	))
	require.Empty(t, cmp.Diff(cfg.Databases.AzureMatchers,
		[]types.AzureMatcher{
			{
				Subscriptions:  []string{"sub1", "sub2"},
				ResourceGroups: []string{"group1", "group2"},
				Types:          []string{"postgres", "mysql"},
				Regions:        []string{"eastus", "centralus"},
				ResourceTags: map[string]apiutils.Strings{
					"a": {"b"},
				},
			},
			{
				Subscriptions:  nil,
				ResourceGroups: nil,
				Types:          []string{"postgres", "mysql"},
				Regions:        []string{"westus"},
				ResourceTags: map[string]apiutils.Strings{
					"c": {"d"},
				},
			},
		}))
	require.Empty(t, cmp.Diff(cfg.Databases.AWSMatchers,
		[]types.AWSMatcher{
			{
				Types:   []string{"rds"},
				Regions: []string{"us-west-1"},
				AssumeRole: &types.AssumeRole{
					RoleARN:    "arn:aws:iam::123456789012:role/DBDiscoverer",
					ExternalID: "externalID123",
				},
			},
		}))

	require.True(t, cfg.Kube.Enabled)
	require.Empty(t, cmp.Diff(cfg.Kube.ResourceMatchers,
		[]services.ResourceMatcher{
			{
				Labels: map[string]apiutils.Strings{
					"*": {"*"},
				},
			},
		},
	))
	require.Equal(t, "/tmp/kubeconfig", cfg.Kube.KubeconfigPath)
	require.Empty(t, cmp.Diff(cfg.Kube.StaticLabels,
		map[string]string{
			"testKey": "testValue",
		},
	))

	require.True(t, cfg.Discovery.Enabled)
	require.Equal(t, []string{"eu-central-1"}, cfg.Discovery.AWSMatchers[0].Regions)
	require.Equal(t, []string{"ec2"}, cfg.Discovery.AWSMatchers[0].Types)
	require.Equal(t, "arn:aws:iam::123456789012:role/DBDiscoverer", cfg.Discovery.AWSMatchers[0].AssumeRole.RoleARN)
	require.Equal(t, "externalID123", cfg.Discovery.AWSMatchers[0].AssumeRole.ExternalID)
	require.Equal(t, &types.InstallerParams{
		InstallTeleport: true,
		JoinMethod:      "iam",
		JoinToken:       types.IAMInviteTokenName,
		ScriptName:      "default-installer",
		SSHDConfig:      types.SSHDConfigPath,
		EnrollMode:      types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
	}, cfg.Discovery.AWSMatchers[0].Params)

	require.True(t, cfg.Okta.Enabled)
	require.Equal(t, "https://some-endpoint", cfg.Okta.APIEndpoint)
	require.Equal(t, oktaAPITokenPath, cfg.Okta.APITokenPath)
	require.Equal(t, time.Second*300, cfg.Okta.SyncPeriod)
	require.True(t, cfg.Okta.SyncSettings.SyncAccessLists)
}

// TestApplyConfigNoneEnabled makes sure that if a section is not enabled,
// it's fields are not read in.
func TestApplyConfigNoneEnabled(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(NoServicesConfigString))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.NoError(t, err)

	require.False(t, cfg.Auth.Enabled)
	require.Empty(t, cfg.Auth.PublicAddrs)
	require.Equal(t, types.OriginDefaults, cfg.Auth.Preference.Origin())
	require.Equal(t, types.OriginDefaults, cfg.Auth.NetworkingConfig.Origin())
	require.Equal(t, types.OriginDefaults, cfg.Auth.SessionRecordingConfig.Origin())
	require.False(t, cfg.Proxy.Enabled)
	require.Empty(t, cfg.Proxy.PublicAddrs)
	require.False(t, cfg.SSH.Enabled)
	require.Empty(t, cfg.SSH.PublicAddrs)
	require.False(t, cfg.Apps.Enabled)
	require.False(t, cfg.Databases.Enabled)
	require.False(t, cfg.Metrics.Enabled)
	require.True(t, cfg.DebugService.Enabled)
	require.False(t, cfg.WindowsDesktop.Enabled)
	require.Empty(t, cfg.Proxy.PostgresPublicAddrs)
	require.Empty(t, cfg.Proxy.MySQLPublicAddrs)
}

// TestApplyConfigNoneEnabled makes sure that if the auth file configuration
// does not have `cluster_auth_preference`, `cluster_networking_config` and
// `session_recording` fields, then they all have the origin label as defaults.
func TestApplyDefaultAuthResources(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(DefaultAuthResourcesConfigString))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.NoError(t, err)

	require.True(t, cfg.Auth.Enabled)
	require.Equal(t, "example.com", cfg.Auth.ClusterName.GetClusterName())
	require.Equal(t, types.OriginDefaults, cfg.Auth.Preference.Origin())
	require.Equal(t, types.OriginDefaults, cfg.Auth.NetworkingConfig.Origin())
	require.Equal(t, types.OriginDefaults, cfg.Auth.SessionRecordingConfig.Origin())
}

// TestApplyCustomAuthPreference makes sure that if the auth file configuration
// has a `cluster_auth_preference` field, then it will have the origin label as
// config-file.
func TestApplyCustomAuthPreference(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(CustomAuthPreferenceConfigString))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.NoError(t, err)

	require.True(t, cfg.Auth.Enabled)
	require.Equal(t, "example.com", cfg.Auth.ClusterName.GetClusterName())
	require.Equal(t, types.OriginConfigFile, cfg.Auth.Preference.Origin())
	require.True(t, cfg.Auth.Preference.GetDisconnectExpiredCert())
	require.Equal(t, types.OriginDefaults, cfg.Auth.NetworkingConfig.Origin())
	require.Equal(t, types.OriginDefaults, cfg.Auth.SessionRecordingConfig.Origin())
}

// TestApplyCustomAuthPreferenceWithMOTD makes sure that if the auth file configuration
// has only the `message_of_the_day` `cluster_auth_preference` field, then it will have
// the origin label as defaults (instead of config-file).
func TestApplyCustomAuthPreferenceWithMOTD(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(AuthPreferenceConfigWithMOTDString))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.NoError(t, err)

	require.True(t, cfg.Auth.Enabled)
	require.Equal(t, "example.com", cfg.Auth.ClusterName.GetClusterName())
	require.Equal(t, types.OriginDefaults, cfg.Auth.Preference.Origin())
	require.Equal(t, "welcome!", cfg.Auth.Preference.GetMessageOfTheDay())
	require.Equal(t, types.OriginDefaults, cfg.Auth.NetworkingConfig.Origin())
	require.Equal(t, types.OriginDefaults, cfg.Auth.SessionRecordingConfig.Origin())
}

// TestApplyCustomNetworkingConfig makes sure that if the auth file configuration
// has a `cluster_networking_config` field, then it will have the origin label as
// config-file.
func TestApplyCustomNetworkingConfig(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(CustomNetworkingConfigString))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.NoError(t, err)

	require.True(t, cfg.Auth.Enabled)
	require.Equal(t, "example.com", cfg.Auth.ClusterName.GetClusterName())
	require.Equal(t, types.OriginDefaults, cfg.Auth.Preference.Origin())
	require.Equal(t, types.OriginConfigFile, cfg.Auth.NetworkingConfig.Origin())
	require.Equal(t, 10*time.Second, cfg.Auth.NetworkingConfig.GetWebIdleTimeout())
	require.Equal(t, types.OriginDefaults, cfg.Auth.SessionRecordingConfig.Origin())
}

// TestApplyCustomSessionRecordingConfig makes sure that if the auth file configuration
// has a `session_recording` field, then it will have the origin label as
// config-file.
func TestApplyCustomSessionRecordingConfig(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(CustomSessionRecordingConfigString))
	require.NoError(t, err)

	cfg := servicecfg.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.NoError(t, err)

	require.True(t, cfg.Auth.Enabled)
	require.Equal(t, "example.com", cfg.Auth.ClusterName.GetClusterName())
	require.Equal(t, types.OriginDefaults, cfg.Auth.Preference.Origin())
	require.Equal(t, types.OriginDefaults, cfg.Auth.NetworkingConfig.Origin())
	require.Equal(t, types.OriginConfigFile, cfg.Auth.SessionRecordingConfig.Origin())
	require.True(t, cfg.Auth.SessionRecordingConfig.GetProxyChecksHostKeys())
}

// TestPostgresPublicAddr makes sure Postgres proxy public address default
// port logic works correctly.
func TestPostgresPublicAddr(t *testing.T) {
	tests := []struct {
		desc string
		fc   *FileConfig
		out  []string
	}{
		{
			desc: "postgres public address with port set",
			fc: &FileConfig{
				Proxy: Proxy{
					WebAddr:            "0.0.0.0:8080",
					PublicAddr:         []string{"web.example.com:443"},
					PostgresPublicAddr: []string{"postgres.example.com:5432"},
				},
			},
			out: []string{"postgres.example.com:5432"},
		},
		{
			desc: "when port not set, defaults to web proxy public port",
			fc: &FileConfig{
				Proxy: Proxy{
					WebAddr:            "0.0.0.0:8080",
					PublicAddr:         []string{"web.example.com:443"},
					PostgresPublicAddr: []string{"postgres.example.com"},
				},
			},
			out: []string{"postgres.example.com:443"},
		},
		{
			desc: "when port and public addr not set, defaults to web proxy listen port",
			fc: &FileConfig{
				Proxy: Proxy{
					WebAddr:            "0.0.0.0:8080",
					PostgresPublicAddr: []string{"postgres.example.com"},
				},
			},
			out: []string{"postgres.example.com:8080"},
		},
		{
			desc: "when port and listen/public addrs not set, defaults to web proxy default port",
			fc: &FileConfig{
				Proxy: Proxy{
					PostgresPublicAddr: []string{"postgres.example.com"},
				},
			},
			out: []string{net.JoinHostPort("postgres.example.com", strconv.Itoa(defaults.HTTPListenPort))},
		},
		{
			desc: "when PostgresAddr is provided with port, the explicitly provided port should be use",
			fc: &FileConfig{
				Proxy: Proxy{
					WebAddr:            "0.0.0.0:8080",
					PostgresAddr:       "0.0.0.0:12345",
					PostgresPublicAddr: []string{"postgres.example.com"},
				},
			},
			out: []string{"postgres.example.com:12345"},
		},
		{
			desc: "when PostgresAddr is provided without port, defaults PostgresPort should be used",
			fc: &FileConfig{
				Proxy: Proxy{
					WebAddr:            "0.0.0.0:8080",
					PostgresAddr:       "0.0.0.0",
					PostgresPublicAddr: []string{"postgres.example.com"},
				},
			},
			out: []string{net.JoinHostPort("postgres.example.com", strconv.Itoa(defaults.PostgresListenPort))},
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			cfg := servicecfg.MakeDefaultConfig()
			err := applyProxyConfig(test.fc, cfg)
			require.NoError(t, err)
			require.EqualValues(t, test.out, utils.NetAddrsToStrings(cfg.Proxy.PostgresPublicAddrs))
		})
	}
}

// TestProxyPeeringPublicAddr makes sure the public address can only be
// set if the listen addr is set.
func TestProxyPeeringPublicAddr(t *testing.T) {
	tests := []struct {
		desc    string
		fc      *FileConfig
		wantErr bool
	}{
		{
			desc: "full proxy peering config",
			fc: &FileConfig{
				Proxy: Proxy{
					PeerAddr:       "peerhost:1234",
					PeerPublicAddr: "peer.example:5432",
				},
			},
			wantErr: false,
		},
		{
			desc: "no public proxy peering addr in config",
			fc: &FileConfig{
				Proxy: Proxy{
					PeerAddr: "peerhost:1234",
				},
			},
			wantErr: false,
		},
		{
			desc: "no private proxy peering addr in config",
			fc: &FileConfig{
				Proxy: Proxy{
					PeerPublicAddr: "peer.example:1234",
				},
			},
			wantErr: true,
		},
	}
	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			cfg := servicecfg.MakeDefaultConfig()
			err := applyProxyConfig(test.fc, cfg)
			if test.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestProxyMustJoinViaAuth(t *testing.T) {
	cfg := servicecfg.MakeDefaultConfig()

	err := ApplyFileConfig(&FileConfig{
		Version: defaults.TeleportConfigVersionV3,
		Proxy:   Proxy{Service: Service{EnabledFlag: "yes"}},
		Global:  Global{ProxyServer: "proxy.example.com:3080"},
	}, cfg)
	require.True(t, trace.IsBadParameter(err), "expected bad parameter, got %v", err)
}

func TestBackendDefaults(t *testing.T) {
	read := func(val string) *servicecfg.Config {
		// Default value is lite backend.
		conf, err := ReadConfig(bytes.NewBufferString(val))
		require.NoError(t, err)
		require.NotNil(t, conf)

		cfg := servicecfg.MakeDefaultConfig()
		err = ApplyFileConfig(conf, cfg)
		require.NoError(t, err)
		return cfg
	}

	// Default value is lite backend.
	cfg := read(`teleport:
  data_dir: /var/lib/teleport
`)
	require.Equal(t, cfg.Auth.StorageConfig.Type, lite.GetName())
	require.Equal(t, cfg.Auth.StorageConfig.Params[defaults.BackendPath], filepath.Join("/var/lib/teleport", defaults.BackendDir))

	// If no path is specified, the default is picked. In addition, internally
	// dir gets converted into lite.
	cfg = read(`teleport:
     data_dir: /var/lib/teleport
     storage:
       type: dir
`)
	require.Equal(t, cfg.Auth.StorageConfig.Type, lite.GetName())
	require.Equal(t, cfg.Auth.StorageConfig.Params[defaults.BackendPath], filepath.Join("/var/lib/teleport", defaults.BackendDir))

	// Support custom paths for dir/lite backends.
	cfg = read(`teleport:
     data_dir: /var/lib/teleport
     storage:
       type: dir
       path: /var/lib/teleport/mybackend
`)
	require.Equal(t, lite.GetName(), cfg.Auth.StorageConfig.Type)
	require.Equal(t, "/var/lib/teleport/mybackend", cfg.Auth.StorageConfig.Params[defaults.BackendPath])

	// Kubernetes proxy is disabled by default.
	cfg = read(`teleport:
     data_dir: /var/lib/teleport
`)
	require.False(t, cfg.Proxy.Kube.Enabled)
}

func TestTunnelStrategy(t *testing.T) {
	tests := []struct {
		desc           string
		config         string
		readErr        require.ErrorAssertionFunc
		applyErr       require.ErrorAssertionFunc
		tunnelStrategy interface{}
	}{
		{
			desc: "Ensure default is used when no tunnel strategy is given",
			config: strings.Join([]string{
				"auth_service:",
				"  enabled: yes",
			}, "\n"),
			readErr:        require.NoError,
			applyErr:       require.NoError,
			tunnelStrategy: types.DefaultAgentMeshTunnelStrategy(),
		},
		{
			desc: "Ensure default parameters are used for proxy peering strategy",
			config: strings.Join([]string{
				"auth_service:",
				"  enabled: yes",
				"  tunnel_strategy:",
				"    type: proxy_peering",
			}, "\n"),
			readErr:        require.NoError,
			applyErr:       require.NoError,
			tunnelStrategy: types.DefaultProxyPeeringTunnelStrategy(),
		},
		{
			desc: "Ensure proxy peering strategy parameters are set",
			config: strings.Join([]string{
				"auth_service:",
				"  enabled: yes",
				"  tunnel_strategy:",
				"    type: proxy_peering",
				"    agent_connection_count: 2",
			}, "\n"),
			readErr:  require.NoError,
			applyErr: require.NoError,
			tunnelStrategy: &types.ProxyPeeringTunnelStrategy{
				AgentConnectionCount: 2,
			},
		},
		{
			desc: "Ensure tunnel strategy cannot take unknown parameters",
			config: strings.Join([]string{
				"auth_service:",
				"  enabled: yes",
				"  tunnel_strategy:",
				"    type: agent_mesh",
				"    agent_connection_count: 2",
			}, "\n"),
			readErr:        require.Error,
			applyErr:       require.NoError,
			tunnelStrategy: types.DefaultAgentMeshTunnelStrategy(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			conf, err := ReadConfig(bytes.NewBufferString(tc.config))
			tc.readErr(t, err)

			cfg := servicecfg.MakeDefaultConfig()
			err = ApplyFileConfig(conf, cfg)
			tc.applyErr(t, err)

			var actualStrategy interface{}
			if cfg.Auth.NetworkingConfig == nil {
			} else if s := cfg.Auth.NetworkingConfig.GetAgentMeshTunnelStrategy(); s != nil {
				actualStrategy = s
			} else if s := cfg.Auth.NetworkingConfig.GetProxyPeeringTunnelStrategy(); s != nil {
				actualStrategy = s
			}
			require.Equal(t, tc.tunnelStrategy, actualStrategy)
		})
	}
}

func TestParseCachePolicy(t *testing.T) {
	tcs := []struct {
		in  *CachePolicy
		out *servicecfg.CachePolicy
		err error
	}{
		{in: &CachePolicy{EnabledFlag: "yes", TTL: "never"}, out: &servicecfg.CachePolicy{Enabled: true}},
		{in: &CachePolicy{EnabledFlag: "true", TTL: "10h"}, out: &servicecfg.CachePolicy{Enabled: true}},
		{in: &CachePolicy{Type: "whatever", EnabledFlag: "false", TTL: "10h"}, out: &servicecfg.CachePolicy{Enabled: false}},
		{in: &CachePolicy{Type: "name", EnabledFlag: "yes", TTL: "never"}, out: &servicecfg.CachePolicy{Enabled: true}},
		{in: &CachePolicy{EnabledFlag: "no"}, out: &servicecfg.CachePolicy{Enabled: false}},
	}
	for i, tc := range tcs {
		comment := fmt.Sprintf("test case #%v", i)
		out, err := tc.in.Parse()
		if tc.err != nil {
			require.IsType(t, tc.err, err, comment)
		} else {
			require.NoError(t, err, comment)
			require.Equal(t, out, tc.out, comment)
		}
	}
}

func checkStaticConfig(t *testing.T, conf *FileConfig) {
	require.Equal(t, "xxxyyy", conf.AuthToken)
	require.Equal(t, "10.10.10.1:3022", conf.AdvertiseIP)
	require.Equal(t, "/var/run/teleport.pid", conf.PIDFile)

	require.Empty(t, cmp.Diff(conf.Limits, ConnectionLimits{
		MaxConnections: 90,
		MaxUsers:       91,
		Rates: []ConnectionRate{
			{Average: 70, Burst: 71, Period: time.Minute + time.Second},
			{Average: 170, Burst: 171, Period: 10*time.Minute + 10*time.Second},
		},
	}))

	// proxy_service section is missing.
	require.False(t, conf.Proxy.Configured())
	require.True(t, conf.Proxy.Enabled())
	require.False(t, conf.Proxy.Disabled()) // Missing "proxy_service" does NOT mean it's been disabled
	require.Empty(t, cmp.Diff(conf.Proxy, Proxy{
		Service: Service{defaultEnabled: true},
	}, cmp.AllowUnexported(Service{})))

	// kubernetes_service section is missing.
	require.False(t, conf.Kube.Configured())
	require.False(t, conf.Kube.Enabled())
	require.True(t, conf.Kube.Disabled())
	require.Empty(t, cmp.Diff(conf.Kube, Kube{
		Service: Service{defaultEnabled: false},
	}, cmp.AllowUnexported(Service{})))

	require.True(t, conf.SSH.Configured()) // "ssh_service" has been explicitly set to "no"
	require.False(t, conf.SSH.Enabled())
	require.True(t, conf.SSH.Disabled())
	require.Empty(t, cmp.Diff(conf.SSH, SSH{
		Service: Service{
			defaultEnabled: true,
			EnabledFlag:    "no",
			ListenAddress:  "ssh:3025",
		},
		Labels: map[string]string{
			"name": "mongoserver",
			"role": "follower",
		},
		Commands: []CommandLabel{
			{Name: "hostname", Command: []string{"/bin/hostname"}, Period: 10 * time.Millisecond},
			{Name: "date", Command: []string{"/bin/date"}, Period: 20 * time.Millisecond},
		},
		PublicAddr: apiutils.Strings{"luna3:22"},
	}, cmp.AllowUnexported(Service{})))

	require.Empty(t, cmp.Diff(conf.Discovery, Discovery{AWSMatchers: nil}, cmp.AllowUnexported(Service{})))

	require.True(t, conf.Auth.Configured())
	require.True(t, conf.Auth.Enabled())
	require.False(t, conf.Auth.Disabled())
	require.Empty(t, cmp.Diff(conf.Auth, Auth{
		Service: Service{
			defaultEnabled: true,
			EnabledFlag:    "yes",
			ListenAddress:  "auth:3025",
		},
		ReverseTunnels: []ReverseTunnel{
			{
				DomainName: "tunnel.example.com",
				Addresses:  []string{"com-1", "com-2"},
			},
			{
				DomainName: "tunnel.example.org",
				Addresses:  []string{"org-1"},
			},
		},
		StaticTokens: StaticTokens{
			"proxy,node:xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			"auth:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		},
		PublicAddr: apiutils.Strings{
			"auth.default.svc.cluster.local:3080",
		},
		ClientIdleTimeout:     types.Duration(17 * time.Second),
		DisconnectExpiredCert: types.NewBoolOption(true),
		RoutingStrategy:       types.RoutingStrategy_MOST_RECENT,
	}, cmp.AllowUnexported(Service{})))

	policy, err := conf.CachePolicy.Parse()
	require.NoError(t, err)
	require.True(t, policy.Enabled)
	require.Equal(t, time.Minute*12, policy.MaxRetryPeriod)
}

var (
	NodeName        = "edsger.example.com"
	AuthServers     = []string{"auth0.server.example.org:3024", "auth1.server.example.org:3024"}
	ConnectionRates = []ConnectionRate{
		{
			Period:  time.Minute,
			Average: 5,
			Burst:   10,
		},
		{
			Period:  time.Minute * 10,
			Average: 10,
			Burst:   100,
		},
	}
	Labels = map[string]string{
		"name": "mongoserver",
		"role": "follower",
	}
	CommandLabels = []CommandLabel{
		{
			Name:    "os",
			Command: []string{"uname", "-o"},
			Period:  time.Minute * 15,
		},
		{
			Name:    "hostname",
			Command: []string{"/bin/hostname"},
			Period:  time.Millisecond * 10,
		},
	}
)

// makeConfigFixture returns a valid content for teleport.yaml file
func makeConfigFixture() string {
	conf := FileConfig{}

	// common config:
	conf.NodeName = NodeName
	conf.DataDir = "/path/to/data"
	conf.AuthServers = AuthServers
	conf.Limits.MaxConnections = 100
	conf.Limits.MaxUsers = 5
	conf.Limits.Rates = ConnectionRates
	conf.Logger.Output = "stderr"
	conf.Logger.Severity = "INFO"
	conf.Logger.Format = LogFormat{Output: "text"}
	conf.Storage.Type = "bolt"
	conf.CAPin = []string{"rsa256:123", "rsa256:456"}

	// auth service:
	conf.Auth.EnabledFlag = "Yeah"
	conf.Auth.ListenAddress = "tcp://auth"
	conf.Auth.LicenseFile = "lic.pem"
	conf.Auth.ClientIdleTimeout = types.NewDuration(17 * time.Second)
	conf.Auth.WebIdleTimeout = types.NewDuration(19 * time.Second)
	conf.Auth.DisconnectExpiredCert = types.NewBoolOption(true)
	conf.Auth.RoutingStrategy = types.RoutingStrategy_MOST_RECENT
	conf.Auth.ProxyPingInterval = types.NewDuration(10 * time.Second)

	// ssh service:
	conf.SSH.EnabledFlag = "true"
	conf.SSH.ListenAddress = "tcp://ssh"
	conf.SSH.Labels = Labels
	conf.SSH.Commands = CommandLabels

	// discovery service
	conf.Discovery.EnabledFlag = "true"
	conf.Discovery.AWSMatchers = []AWSMatcher{
		{
			Types:         []string{"ec2"},
			Regions:       []string{"us-west-1", "us-east-1"},
			Tags:          map[string]apiutils.Strings{"a": {"b"}},
			AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
			ExternalID:    "externalID123",
		},
		{
			Types:            []string{"eks"},
			Regions:          []string{"us-west-1", "us-east-1"},
			Tags:             map[string]apiutils.Strings{"a": {"b"}},
			Integration:      "integration1",
			KubeAppDiscovery: true,
		},
	}

	conf.Discovery.AzureMatchers = []AzureMatcher{
		{
			Types:   []string{"aks"},
			Regions: []string{"uswest1"},
			ResourceTags: map[string]apiutils.Strings{
				"a": {"b"},
			},
			ResourceGroups: []string{"group1"},
			Subscriptions:  []string{"sub1"},
		},
	}

	conf.Discovery.GCPMatchers = []GCPMatcher{
		{
			Types:     []string{"gke"},
			Locations: []string{"uswest1"},
			Labels: map[string]apiutils.Strings{
				"a": {"b"},
			},
			ProjectIDs: []string{"p1", "p2"},
		},
	}

	// proxy-service:
	conf.Proxy.EnabledFlag = "yes"
	conf.Proxy.ListenAddress = "tcp://proxy"
	conf.Proxy.KeyFile = "/etc/teleport/proxy.key"
	conf.Proxy.CertFile = "/etc/teleport/proxy.crt"
	conf.Proxy.KeyPairs = []KeyPair{
		{
			PrivateKey:  "/etc/teleport/proxy.key",
			Certificate: "/etc/teleport/proxy.crt",
		},
	}
	conf.Proxy.ListenAddress = "tcp://proxy_ssh_addr"
	conf.Proxy.WebAddr = "tcp://web_addr"
	conf.Proxy.TunAddr = "reverse_tunnel_address:3311"
	conf.Proxy.IdP.SAMLIdP.EnabledFlag = "true"
	conf.Proxy.IdP.SAMLIdP.BaseURL = "https://test-url.com"

	// kubernetes service:
	conf.Kube = Kube{
		Service: Service{
			EnabledFlag:   "yes",
			ListenAddress: "tcp://kube",
		},
		KubeClusterName: "kube-cluster",
		PublicAddr:      apiutils.Strings([]string{"kube-host:1234"}),
		ResourceMatchers: []ResourceMatcher{
			{
				Labels: map[string]apiutils.Strings{"*": {"*"}},
			},
		},
	}

	// Application service.
	conf.Apps.EnabledFlag = "yes"
	conf.Apps.Apps = []*App{
		{
			Name:          "foo",
			URI:           "http://127.0.0.1:8080",
			PublicAddr:    "foo.example.com",
			StaticLabels:  Labels,
			DynamicLabels: CommandLabels,
		},
	}
	conf.Apps.ResourceMatchers = []ResourceMatcher{
		{
			Labels: map[string]apiutils.Strings{"*": {"*"}},
		},
	}

	// Database service.
	conf.Databases.EnabledFlag = "yes"
	conf.Databases.Databases = []*Database{
		{
			Name:          "postgres",
			Protocol:      defaults.ProtocolPostgres,
			URI:           "localhost:5432",
			StaticLabels:  Labels,
			DynamicLabels: CommandLabels,
		},
	}
	conf.Databases.ResourceMatchers = []ResourceMatcher{
		{
			Labels: map[string]apiutils.Strings{"a": {"b"}},
		},
		{
			Labels: map[string]apiutils.Strings{"c": {"d"}},
			AWS: ResourceMatcherAWS{
				AssumeRoleARN: "arn:aws:iam::123456789012:role/DBAccess",
				ExternalID:    "externalID123",
			},
		},
	}
	conf.Databases.AWSMatchers = []AWSMatcher{
		{
			Types:         []string{"rds"},
			Regions:       []string{"us-west-1", "us-east-1"},
			Tags:          map[string]apiutils.Strings{"a": {"b"}},
			AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
			ExternalID:    "externalID123",
		},
		{
			Types:         []string{"rds"},
			Regions:       []string{"us-central-1"},
			Tags:          map[string]apiutils.Strings{"c": {"d"}},
			AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
		},
	}
	conf.Databases.AzureMatchers = []AzureMatcher{
		{
			Subscriptions:  []string{"sub1", "sub2"},
			ResourceGroups: []string{"rg1", "rg2"},
			Types:          []string{"mysql"},
			Regions:        []string{"eastus", "westus"},
			ResourceTags: map[string]apiutils.Strings{
				"a": {"b"},
			},
		},
		{
			Subscriptions:  []string{"sub3", "sub4"},
			ResourceGroups: []string{"rg3", "rg4"},
			Types:          []string{"postgres"},
			Regions:        []string{"centralus"},
			ResourceTags: map[string]apiutils.Strings{
				"c": {"d"},
			},
		},
		{
			Types:   []string{"mysql", "postgres"},
			Regions: []string{"centralus"},
			ResourceTags: map[string]apiutils.Strings{
				"e": {"f"},
			},
		},
	}

	// Metrics service.
	conf.Metrics.EnabledFlag = "yes"
	conf.Metrics.ListenAddress = "tcp://metrics"
	conf.Metrics.GRPCServerLatency = true
	conf.Metrics.GRPCClientLatency = true
	conf.Metrics.CACerts = []string{"/etc/teleport/ca.crt"}
	conf.Metrics.KeyPairs = []KeyPair{
		{
			PrivateKey:  "/etc/teleport/proxy.key",
			Certificate: "/etc/teleport/proxy.crt",
		},
	}

	// Debug service.
	conf.Debug.EnabledFlag = "yes"

	// Windows Desktop Service
	conf.WindowsDesktop = WindowsDesktopService{
		Service: Service{
			EnabledFlag:   "yes",
			ListenAddress: "tcp://windows_desktop",
		},
		PublicAddr: apiutils.Strings([]string{"winsrv.example.com:3028", "no-port.winsrv.example.com"}),
		ADHosts:    apiutils.Strings([]string{"win.example.com:3389", "no-port.win.example.com"}),
	}

	// Tracing service.
	conf.Tracing.EnabledFlag = "yes"
	conf.Tracing.ExporterURL = "https://localhost:4318"
	conf.Tracing.SamplingRatePerMillion = 10
	conf.Tracing.CACerts = []string{"/etc/teleport/exporter.crt"}
	conf.Tracing.KeyPairs = []KeyPair{
		{
			PrivateKey:  "/etc/teleport/exporter.key",
			Certificate: "/etc/teleport/exporter.crt",
		},
	}

	return conf.DebugDumpToYAML()
}

func TestPermitUserEnvironment(t *testing.T) {
	tests := []struct {
		inConfigString           string
		inPermitUserEnvironment  bool
		outPermitUserEnvironment bool
	}{
		// 0 - set on the command line, expect PermitUserEnvironment to be true
		{
			``,
			true,
			true,
		},
		// 1 - set in config file, expect PermitUserEnvironment to be true
		{
			`
ssh_service:
  permit_user_env: true
`,
			false,
			true,
		},
		// 2 - not set anywhere, expect PermitUserEnvironment to be false
		{
			``,
			false,
			false,
		},
	}

	// run tests
	for i, tt := range tests {
		comment := fmt.Sprintf("Test %v", i)

		clf := CommandLineFlags{
			ConfigString:          base64.StdEncoding.EncodeToString([]byte(tt.inConfigString)),
			PermitUserEnvironment: tt.inPermitUserEnvironment,
		}
		cfg := servicecfg.MakeDefaultConfig()

		err := Configure(&clf, cfg, false)
		require.NoError(t, err, comment)
		require.Equal(t, tt.outPermitUserEnvironment, cfg.SSH.PermitUserEnvironment, comment)
	}
}

func TestSetDefaultListenerAddresses(t *testing.T) {
	tests := []struct {
		desc string
		fc   FileConfig
		want servicecfg.ProxyConfig
	}{
		{
			desc: "v1 config should set default proxy listen addresses",
			fc: FileConfig{
				Version: defaults.TeleportConfigVersionV1,
				Proxy: Proxy{
					Service: Service{
						defaultEnabled: true,
					},
				},
			},
			want: servicecfg.ProxyConfig{
				WebAddr:                 *utils.MustParseAddr("0.0.0.0:3080"),
				ReverseTunnelListenAddr: *utils.MustParseAddr("0.0.0.0:3024"),
				SSHAddr:                 *utils.MustParseAddr("0.0.0.0:3023"),
				Enabled:                 true,
				Kube: servicecfg.KubeProxyConfig{
					Enabled: false,
				},
				Limiter: limiter.Config{
					MaxConnections:   defaults.LimiterMaxConnections,
					MaxNumberOfUsers: 250,
				},
				IdP: servicecfg.IdP{
					SAMLIdP: servicecfg.SAMLIdP{
						Enabled: true,
					},
				},
			},
		},
		{
			desc: "v2 config should not set any default listen addresses",
			fc: FileConfig{
				Version: defaults.TeleportConfigVersionV2,
				Proxy: Proxy{
					Service: Service{
						defaultEnabled: true,
					},
					WebAddr: "0.0.0.0:9999",
				},
			},
			want: servicecfg.ProxyConfig{
				WebAddr: *utils.MustParseAddr("0.0.0.0:9999"),
				Enabled: true,
				Kube: servicecfg.KubeProxyConfig{
					Enabled: true,
				},
				Limiter: limiter.Config{
					MaxConnections:   defaults.LimiterMaxConnections,
					MaxNumberOfUsers: 250,
				},
				IdP: servicecfg.IdP{
					SAMLIdP: servicecfg.SAMLIdP{
						Enabled: true,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			cfg := servicecfg.MakeDefaultConfig()

			require.NoError(t, ApplyFileConfig(&tt.fc, cfg))
			require.NoError(t, Configure(&CommandLineFlags{}, cfg, false))

			opts := cmp.Options{
				cmpopts.EquateEmpty(),
				cmpopts.IgnoreFields(servicecfg.ProxyConfig{}, "AutomaticUpgradesChannels"),
			}
			require.Empty(t, cmp.Diff(cfg.Proxy, tt.want, opts...))
		})
	}
}

// TestDebugFlag ensures that the debug command-line flag is correctly set in the config.
func TestDebugFlag(t *testing.T) {
	clf := CommandLineFlags{
		Debug: true,
	}
	cfg := servicecfg.MakeDefaultConfig()
	require.False(t, cfg.Debug)
	err := Configure(&clf, cfg, false)
	require.NoError(t, err)
	require.True(t, cfg.Debug)
}

func TestMergingCAPinConfig(t *testing.T) {
	tests := []struct {
		desc       string
		cliPins    []string
		configPins string // this goes into the yaml in bracket syntax [val1,val2,...]
		want       []string
	}{
		{
			desc:       "pin taken from cli only",
			cliPins:    []string{"cli-pin"},
			configPins: "",
			want:       []string{"cli-pin"},
		},
		{
			desc:       "pin taken from file config only",
			cliPins:    []string{},
			configPins: "fc-pin",
			want:       []string{"fc-pin"},
		},
		{
			desc:       "non-empty pins from cli override file config",
			cliPins:    []string{"cli-pin1", "", "cli-pin2", ""},
			configPins: "fc-pin",
			want:       []string{"cli-pin1", "cli-pin2"},
		},
		{
			desc:       "all empty pins from cli do not override file config",
			cliPins:    []string{"", ""},
			configPins: "fc-pin1,fc-pin2",
			want:       []string{"fc-pin1", "fc-pin2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			clf := CommandLineFlags{
				CAPins: tt.cliPins,
				ConfigString: base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf(
					configWithCAPins,
					tt.configPins,
				))),
			}
			cfg := servicecfg.MakeDefaultConfig()
			require.Empty(t, cfg.CAPins)
			err := Configure(&clf, cfg, false)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.want, cfg.CAPins)
		})
	}
}

func TestLicenseFile(t *testing.T) {
	testCases := []struct {
		path    string
		datadir string
		result  string
	}{
		// 0 - no license, no data dir
		{
			path:    "",
			datadir: "",
			result:  filepath.Join(defaults.DataDir, defaults.LicenseFile),
		},
		// 1 - relative path, default data dir
		{
			path:    "lic.pem",
			datadir: "",
			result:  filepath.Join(defaults.DataDir, "lic.pem"),
		},
		// 2 - relative path, custom data dir
		{
			path:    "baz.pem",
			datadir: filepath.Join("foo", "bar"),
			result:  filepath.Join("foo", "bar", "baz.pem"),
		},
		// 3 - absolute path
		{
			path:   "/etc/teleport/license",
			result: "/etc/teleport/license",
		},
	}

	cfg := servicecfg.MakeDefaultConfig()

	// the license file should be empty by default, as we can only fill
	// in the default (<datadir>/license.pem) after we know what the
	// data dir is supposed to be
	require.Empty(t, cfg.Auth.LicenseFile)

	for i, tc := range testCases {
		t.Run(fmt.Sprintf("test%d", i), func(t *testing.T) {
			fc := new(FileConfig)
			require.NoError(t, fc.CheckAndSetDefaults())
			fc.Auth.LicenseFile = tc.path
			fc.DataDir = tc.datadir
			err := ApplyFileConfig(fc, cfg)
			require.NoError(t, err)
			require.Equal(t, tc.result, cfg.Auth.LicenseFile)
		})
	}
}

// TestFIPS makes sure configuration is correctly updated/enforced when in
// FedRAMP/FIPS 140-2 mode.
func TestFIPS(t *testing.T) {
	tests := []struct {
		inConfigString string
		inFIPSMode     bool
		outError       bool
	}{
		{
			inConfigString: configWithoutFIPSKex,
			inFIPSMode:     true,
			outError:       true,
		},
		{
			inConfigString: configWithoutFIPSKex,
			inFIPSMode:     false,
			outError:       false,
		},
		{
			inConfigString: configWithFIPSKex,
			inFIPSMode:     true,
			outError:       false,
		},
		{
			inConfigString: configWithFIPSKex,
			inFIPSMode:     false,
			outError:       false,
		},
	}

	for i, tt := range tests {
		comment := fmt.Sprintf("Test %v", i)

		clf := CommandLineFlags{
			ConfigString: base64.StdEncoding.EncodeToString([]byte(tt.inConfigString)),
			FIPS:         tt.inFIPSMode,
		}

		cfg := servicecfg.MakeDefaultConfig()
		servicecfg.ApplyDefaults(cfg)
		servicecfg.ApplyFIPSDefaults(cfg)

		err := Configure(&clf, cfg, false)
		if tt.outError {
			require.Error(t, err, comment)
		} else {
			require.NoError(t, err, comment)
		}
	}
}

func TestProxyKube(t *testing.T) {
	tests := []struct {
		desc     string
		cfg      Proxy
		version  string
		want     servicecfg.KubeProxyConfig
		checkErr require.ErrorAssertionFunc
	}{
		{
			desc: "not configured",
			cfg:  Proxy{},
			want: servicecfg.KubeProxyConfig{
				Enabled: false,
			},
			checkErr: require.NoError,
		},
		{
			desc: "legacy format, no local cluster",
			cfg: Proxy{Kube: KubeProxy{
				Service: Service{EnabledFlag: "yes", ListenAddress: "0.0.0.0:8080"},
			}},
			want: servicecfg.KubeProxyConfig{
				Enabled:         true,
				ListenAddr:      *utils.MustParseAddr("0.0.0.0:8080"),
				LegacyKubeProxy: true,
			},
			checkErr: require.NoError,
		},
		{
			desc: "legacy format, with local cluster",
			cfg: Proxy{Kube: KubeProxy{
				Service:        Service{EnabledFlag: "yes", ListenAddress: "0.0.0.0:8080"},
				KubeconfigFile: "/tmp/kubeconfig",
				PublicAddr:     apiutils.Strings([]string{constants.KubeTeleportProxyALPNPrefix + "example.com:443"}),
			}},
			want: servicecfg.KubeProxyConfig{
				Enabled:         true,
				ListenAddr:      *utils.MustParseAddr("0.0.0.0:8080"),
				KubeconfigPath:  "/tmp/kubeconfig",
				PublicAddrs:     []utils.NetAddr{*utils.MustParseAddr(constants.KubeTeleportProxyALPNPrefix + "example.com:443")},
				LegacyKubeProxy: true,
			},
			checkErr: require.NoError,
		},
		{
			desc: "new format",
			cfg:  Proxy{KubeAddr: "0.0.0.0:8080"},
			want: servicecfg.KubeProxyConfig{
				Enabled:    true,
				ListenAddr: *utils.MustParseAddr("0.0.0.0:8080"),
			},
			checkErr: require.NoError,
		},
		{
			desc: "new and old formats",
			cfg: Proxy{
				KubeAddr: "0.0.0.0:8080",
				Kube: KubeProxy{
					Service: Service{EnabledFlag: "yes", ListenAddress: "0.0.0.0:8080"},
				},
			},
			checkErr: require.Error,
		},
		{
			desc: "new format and old explicitly disabled",
			cfg: Proxy{
				KubeAddr: "0.0.0.0:8080",
				Kube: KubeProxy{
					Service:        Service{EnabledFlag: "no", ListenAddress: "0.0.0.0:8080"},
					KubeconfigFile: "/tmp/kubeconfig",
					PublicAddr:     apiutils.Strings([]string{constants.KubeTeleportProxyALPNPrefix + "example.com:443"}),
				},
			},
			want: servicecfg.KubeProxyConfig{
				Enabled:    true,
				ListenAddr: *utils.MustParseAddr("0.0.0.0:8080"),
			},
			checkErr: require.NoError,
		},
		{
			desc:    "v2 kube service should be enabled by default",
			version: defaults.TeleportConfigVersionV2,
			cfg:     Proxy{},
			want: servicecfg.KubeProxyConfig{
				Enabled: true,
			},
			checkErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			fc := &FileConfig{
				Version: tt.version,
				Proxy:   tt.cfg,
			}
			cfg := &servicecfg.Config{
				Version: tt.version,
			}
			err := applyProxyConfig(fc, cfg)
			tt.checkErr(t, err)
			require.Empty(t, cmp.Diff(cfg.Proxy.Kube, tt.want, cmpopts.EquateEmpty()))
		})
	}
}

// Test default values generated by v1 and v2 configuration versions.
func TestProxyConfigurationVersion(t *testing.T) {
	tests := []struct {
		desc     string
		fc       FileConfig
		want     servicecfg.ProxyConfig
		checkErr require.ErrorAssertionFunc
	}{
		{
			desc: "v2 config with default web address",
			fc: FileConfig{
				Version: defaults.TeleportConfigVersionV2,
				Proxy: Proxy{
					Service: Service{
						defaultEnabled: true,
					},
				},
			},
			want: servicecfg.ProxyConfig{
				WebAddr: *utils.MustParseAddr("0.0.0.0:3080"),
				Enabled: true,
				Kube: servicecfg.KubeProxyConfig{
					Enabled: true,
				},
				Limiter: limiter.Config{
					MaxConnections:   defaults.LimiterMaxConnections,
					MaxNumberOfUsers: 250,
				},
				IdP: servicecfg.IdP{
					SAMLIdP: servicecfg.SAMLIdP{
						Enabled: true,
					},
				},
			},
			checkErr: require.NoError,
		},
		{
			desc: "v2 config with custom web address",
			fc: FileConfig{
				Version: defaults.TeleportConfigVersionV2,
				Proxy: Proxy{
					Service: Service{
						defaultEnabled: true,
					},
					WebAddr: "0.0.0.0:9999",
				},
			},
			want: servicecfg.ProxyConfig{
				Enabled: true,
				WebAddr: *utils.MustParseAddr("0.0.0.0:9999"),
				Kube: servicecfg.KubeProxyConfig{
					Enabled: true,
				},
				Limiter: limiter.Config{
					MaxConnections:   defaults.LimiterMaxConnections,
					MaxNumberOfUsers: 250,
				},
				IdP: servicecfg.IdP{
					SAMLIdP: servicecfg.SAMLIdP{
						Enabled: true,
					},
				},
			},
			checkErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			cfg := servicecfg.MakeDefaultConfig()
			err := ApplyFileConfig(&tt.fc, cfg)
			tt.checkErr(t, err)
			opts := cmp.Options{
				cmpopts.EquateEmpty(),
				cmpopts.IgnoreFields(servicecfg.ProxyConfig{}, "AutomaticUpgradesChannels"),
			}
			require.Empty(t, cmp.Diff(cfg.Proxy, tt.want, opts...))
		})
	}
}

func TestWindowsDesktopService(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		desc        string
		mutate      func(fc *FileConfig)
		expectError require.ErrorAssertionFunc
	}{
		{
			desc:        "NOK - invalid static host addr",
			expectError: require.Error,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.ADHosts = []string{"badscheme://foo:1:2"}
			},
		},
		{
			desc:        "NOK - invalid host label key",
			expectError: require.Error,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.HostLabels = []WindowsHostLabelRule{
					{Match: ".*", Labels: map[string]string{"invalid label key": "value"}},
				}
			},
		},
		{
			desc:        "NOK - invalid host label regexp",
			expectError: require.Error,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.HostLabels = []WindowsHostLabelRule{
					{Match: "g(-z]+ invalid regex", Labels: map[string]string{"key": "value"}},
				}
			},
		},
		{
			desc:        "NOK - invalid label key for LDAP attribute",
			expectError: require.Error,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.Discovery.LabelAttributes = []string{"this?is not* a valid key 🚨"}
			},
		},
		{
			desc:        "NOK - hosts specified but ldap not specified",
			expectError: require.Error,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.ADHosts = []string{"127.0.0.1:3389"}
				fc.WindowsDesktop.LDAP = LDAPConfig{
					Addr: "",
				}
			},
		},
		{
			desc:        "OK - hosts specified and ldap specified",
			expectError: require.NoError,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.ADHosts = []string{"127.0.0.1:3389"}
				fc.WindowsDesktop.LDAP = LDAPConfig{
					Addr: "something",
				}
			},
		},
		{
			desc:        "OK - no hosts specified and ldap not specified",
			expectError: require.NoError,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.ADHosts = []string{}
				fc.WindowsDesktop.LDAP = LDAPConfig{
					Addr: "",
				}
			},
		},
		{
			desc:        "OK - no hosts specified and ldap specified",
			expectError: require.NoError,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.ADHosts = []string{}
				fc.WindowsDesktop.LDAP = LDAPConfig{
					Addr: "something",
				}
			},
		},
		{
			desc:        "NOK - discovery specified but ldap not specified",
			expectError: require.Error,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.Discovery = LDAPDiscoveryConfig{
					BaseDN: "something",
				}
				fc.WindowsDesktop.LDAP = LDAPConfig{
					Addr: "",
				}
			},
		},
		{
			desc:        "OK - discovery specified and ldap specified",
			expectError: require.NoError,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.Discovery = LDAPDiscoveryConfig{
					BaseDN: "something",
				}
				fc.WindowsDesktop.LDAP = LDAPConfig{
					Addr: "something",
				}
			},
		},
		{
			desc:        "OK - discovery not specified and ldap not specified",
			expectError: require.NoError,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.Discovery = LDAPDiscoveryConfig{
					BaseDN: "",
				}
				fc.WindowsDesktop.LDAP = LDAPConfig{
					Addr: "",
				}
			},
		},
		{
			desc:        "OK - discovery not specified and ldap specified",
			expectError: require.NoError,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.Discovery = LDAPDiscoveryConfig{
					BaseDN: "",
				}
				fc.WindowsDesktop.LDAP = LDAPConfig{
					Addr: "something",
				}
			},
		},
		{
			desc:        "OK - valid config",
			expectError: require.NoError,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.EnabledFlag = "yes"
				fc.WindowsDesktop.ListenAddress = "0.0.0.0:3028"
				fc.WindowsDesktop.ADHosts = []string{"127.0.0.1:3389"}
				fc.WindowsDesktop.LDAP = LDAPConfig{
					Addr: "something",
				}
				fc.WindowsDesktop.HostLabels = []WindowsHostLabelRule{
					{Match: ".*", Labels: map[string]string{"key": "value"}},
				}
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			fc := &FileConfig{}
			test.mutate(fc)
			cfg := &servicecfg.Config{}
			err := applyWindowsDesktopConfig(fc, cfg)
			test.expectError(t, err)
		})
	}
}

func TestApps(t *testing.T) {
	tests := []struct {
		inConfigString string
		inComment      string
		outError       bool
	}{
		{
			inConfigString: `
app_service:
  enabled: true
  apps:
    -
      name: foo
      public_addr: "foo.example.com"
      uri: "http://127.0.0.1:8080"
  resources:
  - labels:
      '*': '*'
`,
			inComment: "config is valid",
			outError:  false,
		},
		{
			inConfigString: `
app_service:
  enabled: true
  apps:
    -
      public_addr: "foo.example.com"
      uri: "http://127.0.0.1:8080"
`,
			inComment: "config is missing name",
			outError:  true,
		},
		{
			inConfigString: `
app_service:
  enabled: true
  apps:
    -
      name: foo
      uri: "http://127.0.0.1:8080"
`,
			inComment: "config is valid",
			outError:  false,
		},
		{
			inConfigString: `
app_service:
  enabled: true
  apps:
    -
      name: foo
      public_addr: "foo.example.com"
`,
			inComment: "config is missing internal address",
			outError:  true,
		},
		{
			inConfigString: `
app_service:
  enabled: true
  apps:
    -
      name: foo
      public_addr: "foo.example.com"
      uri: "http://127.0.0.1:8080"
  resources:
  - labels:
      '*': '*'
    aws:
      assume_role_arn: "arn:aws:iam::123456789012:role/AppAccess"
`,
			inComment: "assume_role_arn is not supported",
			outError:  true,
		},
	}

	for _, tt := range tests {
		clf := CommandLineFlags{
			ConfigString: base64.StdEncoding.EncodeToString([]byte(tt.inConfigString)),
		}
		cfg := servicecfg.MakeDefaultConfig()

		err := Configure(&clf, cfg, false)
		require.Equal(t, err != nil, tt.outError, tt.inComment)
	}
}

// TestAppsCLF checks that validation runs on application configuration passed
// in on the command line.
func TestAppsCLF(t *testing.T) {
	tests := []struct {
		desc             string
		inRoles          string
		inAppName        string
		inAppURI         string
		inAppCloud       string
		inLegacyAppFlags bool
		outApps          []servicecfg.App
		requireError     require.ErrorAssertionFunc
	}{
		{
			desc:      "role provided, valid name and uri",
			inRoles:   defaults.RoleApp,
			inAppName: "foo",
			inAppURI:  "http://localhost:8080",
			outApps: []servicecfg.App{
				{
					Name:          "foo",
					URI:           "http://localhost:8080",
					StaticLabels:  map[string]string{"teleport.dev/origin": "config-file"},
					DynamicLabels: map[string]types.CommandLabel{},
				},
			},
			requireError: require.NoError,
		},
		{
			desc:             "role provided, name not provided, uri not provided, legacy flags",
			inRoles:          defaults.RoleApp,
			inAppName:        "",
			inAppURI:         "",
			inLegacyAppFlags: true,
			outApps:          nil,
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "application name (--app-name) and URI (--app-uri) flags are both required to join application proxy to the cluster")
			},
		},
		{
			desc:      "role provided, name not provided, uri not provided, regular flags",
			inRoles:   defaults.RoleApp,
			inAppName: "",
			inAppURI:  "",
			outApps:   nil,
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "to join application proxy to the cluster provide application name (--name) and either URI (--uri) or Cloud type (--cloud)")
			},
		},
		{
			desc:             "role provided, name not provided, legacy flags",
			inRoles:          defaults.RoleApp,
			inAppName:        "",
			inAppURI:         "http://localhost:8080",
			inLegacyAppFlags: true,
			outApps:          nil,
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "application name (--app-name) is required to join application proxy to the cluster")
			},
		},
		{
			desc:      "role provided, name not provided, regular flags",
			inRoles:   defaults.RoleApp,
			inAppName: "",
			inAppURI:  "http://localhost:8080",
			outApps:   nil,
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "to join application proxy to the cluster provide application name (--name)")
			},
		},
		{
			desc:             "role provided, uri not provided, legacy flags",
			inRoles:          defaults.RoleApp,
			inAppName:        "foo",
			inAppURI:         "",
			inLegacyAppFlags: true,
			outApps:          nil,
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "URI (--app-uri) flag is required to join application proxy to the cluster")
			},
		},
		{
			desc:      "role provided, uri not provided, regular flags",
			inRoles:   defaults.RoleApp,
			inAppName: "foo",
			inAppURI:  "",
			outApps:   nil,
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "to join application proxy to the cluster provide URI (--uri) or Cloud type (--cloud)")
			},
		},
		{
			desc:      "valid name and uri",
			inAppName: "foo",
			inAppURI:  "http://localhost:8080",
			outApps: []servicecfg.App{
				{
					Name:          "foo",
					URI:           "http://localhost:8080",
					StaticLabels:  map[string]string{"teleport.dev/origin": "config-file"},
					DynamicLabels: map[string]types.CommandLabel{},
				},
			},
			requireError: require.NoError,
		},
		{
			desc:       "valid name and cloud",
			inAppName:  "foo",
			inAppCloud: types.CloudGCP,
			outApps: []servicecfg.App{
				{
					Name:          "foo",
					URI:           "cloud://GCP",
					Cloud:         "GCP",
					StaticLabels:  map[string]string{"teleport.dev/origin": "config-file"},
					DynamicLabels: map[string]types.CommandLabel{},
				},
			},
			requireError: require.NoError,
		},
		{
			desc:      "invalid name",
			inAppName: "-foo",
			inAppURI:  "http://localhost:8080",
			outApps:   nil,
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "application name \"-foo\" must be a valid DNS subdomain: https://goteleport.com/docs/application-access/guides/connecting-apps/#application-name")
			},
		},
		{
			desc:      "missing uri",
			inAppName: "foo",
			outApps:   nil,
			requireError: func(t require.TestingT, err error, i ...interface{}) {
				require.True(t, trace.IsBadParameter(err))
				require.ErrorContains(t, err, "missing application \"foo\" URI")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			clf := CommandLineFlags{
				Roles:    tt.inRoles,
				AppName:  tt.inAppName,
				AppURI:   tt.inAppURI,
				AppCloud: tt.inAppCloud,
			}
			cfg := servicecfg.MakeDefaultConfig()
			err := Configure(&clf, cfg, tt.inLegacyAppFlags)
			tt.requireError(t, err)
			require.Equal(t, tt.outApps, cfg.Apps.Apps)
		})
	}
}

func TestDatabaseConfig(t *testing.T) {
	tests := []struct {
		inConfigString string
		desc           string
		outError       string
	}{
		{
			desc: "valid database config",
			inConfigString: `
db_service:
  enabled: true
  resources:
  - labels:
      '*': '*'
  aws:
  - types: ["rds", "redshift"]
    regions: ["us-east-1", "us-west-1"]
    tags:
      '*': '*'
  azure:
  - subscriptions: ["foo", "bar"]
    types: ["mysql", "postgres"]
    regions: ["eastus", "westus"]
    tags:
      '*': '*'
  databases:
  - name: foo
    protocol: postgres
    uri: localhost:5432
    static_labels:
      env: test
    dynamic_labels:
    - name: arch
      command: ["uname", "-p"]
      period: 1h
`,
			outError: "",
		},
		{
			desc: "missing database name",
			inConfigString: `
db_service:
  enabled: true
  databases:
  - protocol: postgres
    uri: localhost:5432
`,
			outError: "empty database name",
		},
		{
			desc: "unsupported database protocol",
			inConfigString: `
db_service:
  enabled: true
  databases:
  - name: foo
    protocol: unknown
    uri: localhost:5432
`,
			outError: `unsupported database "foo" protocol`,
		},
		{
			desc: "missing database uri",
			inConfigString: `
db_service:
  enabled: true
  databases:
  - name: foo
    protocol: postgres
`,
			outError: `database "foo" URI is empty`,
		},
		{
			desc: "invalid database uri (missing port)",
			inConfigString: `
db_service:
  enabled: true
  databases:
  - name: foo
    protocol: postgres
    uri: 192.168.1.1
`,
			outError: `invalid database "foo" address`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			clf := CommandLineFlags{
				ConfigString: base64.StdEncoding.EncodeToString([]byte(tt.inConfigString)),
			}
			err := Configure(&clf, servicecfg.MakeDefaultConfig(), false)
			if tt.outError != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.outError)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestDatabaseCLIFlags verifies database service can be configured with CLI flags.
func TestDatabaseCLIFlags(t *testing.T) {
	// Prepare test CA certificate used to configure some databases.
	testCertPath := filepath.Join(t.TempDir(), "cert.pem")
	err := os.WriteFile(testCertPath, fixtures.LocalhostCert, 0o644)
	require.NoError(t, err)
	tests := []struct {
		inFlags     CommandLineFlags
		desc        string
		outDatabase servicecfg.Database
		outError    string
	}{
		{
			desc: "valid database config",
			inFlags: CommandLineFlags{
				DatabaseName:     "foo",
				DatabaseProtocol: defaults.ProtocolPostgres,
				DatabaseURI:      "localhost:5432",
				Labels:           "env=test,hostname=[1h:hostname]",
			},
			outDatabase: servicecfg.Database{
				Name:     "foo",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				StaticLabels: map[string]string{
					"env":             "test",
					types.OriginLabel: types.OriginConfigFile,
				},
				DynamicLabels: services.CommandLabels{
					"hostname": &types.CommandLabelV2{
						Period:  types.Duration(time.Hour),
						Command: []string{"hostname"},
					},
				},
				TLS: servicecfg.DatabaseTLS{
					Mode: servicecfg.VerifyFull,
				},
			},
		},
		{
			desc: "unsupported database protocol",
			inFlags: CommandLineFlags{
				DatabaseName:     "foo",
				DatabaseProtocol: "unknown",
				DatabaseURI:      "localhost:5432",
			},
			outError: `unsupported database "foo" protocol`,
		},
		{
			desc: "missing database uri",
			inFlags: CommandLineFlags{
				DatabaseName:     "foo",
				DatabaseProtocol: defaults.ProtocolPostgres,
			},
			outError: `database "foo" URI is empty`,
		},
		{
			desc: "invalid database uri (missing port)",
			inFlags: CommandLineFlags{
				DatabaseName:     "foo",
				DatabaseProtocol: defaults.ProtocolPostgres,
				DatabaseURI:      "localhost",
			},
			outError: `invalid database "foo" address`,
		},
		{
			desc: "RDS database",
			inFlags: CommandLineFlags{
				DatabaseName:             "rds",
				DatabaseProtocol:         defaults.ProtocolMySQL,
				DatabaseURI:              "localhost:3306",
				DatabaseAWSRegion:        "us-east-1",
				DatabaseAWSAccountID:     "123456789012",
				DatabaseAWSAssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				DatabaseAWSExternalID:    "externalID123",
			},
			outDatabase: servicecfg.Database{
				Name:     "rds",
				Protocol: defaults.ProtocolMySQL,
				URI:      "localhost:3306",
				AWS: servicecfg.DatabaseAWS{
					Region:        "us-east-1",
					AccountID:     "123456789012", // this gets derived from the assumed role.
					AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
					ExternalID:    "externalID123",
				},
				StaticLabels: map[string]string{
					types.OriginLabel: types.OriginConfigFile,
				},
				DynamicLabels: services.CommandLabels{},
				TLS: servicecfg.DatabaseTLS{
					Mode: servicecfg.VerifyFull,
				},
			},
		},
		{
			desc: "Redshift database",
			inFlags: CommandLineFlags{
				DatabaseName:                 "redshift",
				DatabaseProtocol:             defaults.ProtocolPostgres,
				DatabaseURI:                  "localhost:5432",
				DatabaseAWSRegion:            "us-east-1",
				DatabaseAWSRedshiftClusterID: "redshift-cluster-1",
				DatabaseAWSAssumeRoleARN:     "arn:aws:iam::123456789012:role/DBDiscoverer",
				DatabaseAWSExternalID:        "externalID123",
			},
			outDatabase: servicecfg.Database{
				Name:     "redshift",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				AWS: servicecfg.DatabaseAWS{
					Region: "us-east-1",
					Redshift: servicecfg.DatabaseAWSRedshift{
						ClusterID: "redshift-cluster-1",
					},
					AccountID:     "123456789012", // this gets derived from the assumed role.
					AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
					ExternalID:    "externalID123",
				},
				StaticLabels: map[string]string{
					types.OriginLabel: types.OriginConfigFile,
				},
				DynamicLabels: services.CommandLabels{},
				TLS: servicecfg.DatabaseTLS{
					Mode: servicecfg.VerifyFull,
				},
			},
		},
		{
			desc: "Cloud SQL database",
			inFlags: CommandLineFlags{
				DatabaseName:          "gcp",
				DatabaseProtocol:      defaults.ProtocolPostgres,
				DatabaseURI:           "localhost:5432",
				DatabaseCACertFile:    testCertPath,
				DatabaseGCPProjectID:  "gcp-project-1",
				DatabaseGCPInstanceID: "gcp-instance-1",
			},
			outDatabase: servicecfg.Database{
				Name:     "gcp",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				TLS: servicecfg.DatabaseTLS{
					Mode:   servicecfg.VerifyFull,
					CACert: fixtures.LocalhostCert,
				},
				GCP: servicecfg.DatabaseGCP{
					ProjectID:  "gcp-project-1",
					InstanceID: "gcp-instance-1",
				},
				StaticLabels: map[string]string{
					types.OriginLabel: types.OriginConfigFile,
				},
				DynamicLabels: services.CommandLabels{},
			},
		},
		{
			desc: "SQL Server",
			inFlags: CommandLineFlags{
				DatabaseName:         "sqlserver",
				DatabaseProtocol:     defaults.ProtocolSQLServer,
				DatabaseURI:          "sqlserver.example.com:1433",
				DatabaseADKeytabFile: "/etc/keytab",
				DatabaseADDomain:     "EXAMPLE.COM",
				DatabaseADSPN:        "MSSQLSvc/sqlserver.example.com:1433",
			},
			outDatabase: servicecfg.Database{
				Name:     "sqlserver",
				Protocol: defaults.ProtocolSQLServer,
				URI:      "sqlserver.example.com:1433",
				TLS: servicecfg.DatabaseTLS{
					Mode: servicecfg.VerifyFull,
				},
				AD: servicecfg.DatabaseAD{
					KeytabFile: "/etc/keytab",
					Krb5File:   defaults.Krb5FilePath,
					Domain:     "EXAMPLE.COM",
					SPN:        "MSSQLSvc/sqlserver.example.com:1433",
				},
				StaticLabels: map[string]string{
					types.OriginLabel: types.OriginConfigFile,
				},
				DynamicLabels: services.CommandLabels{},
			},
		},
		{
			desc: "MySQL version",
			inFlags: CommandLineFlags{
				DatabaseName:               "mysql-foo",
				DatabaseProtocol:           defaults.ProtocolMySQL,
				DatabaseURI:                "localhost:3306",
				DatabaseMySQLServerVersion: "8.0.28",
			},
			outDatabase: servicecfg.Database{
				Name:     "mysql-foo",
				Protocol: defaults.ProtocolMySQL,
				URI:      "localhost:3306",
				MySQL: servicecfg.MySQLOptions{
					ServerVersion: "8.0.28",
				},
				TLS: servicecfg.DatabaseTLS{
					Mode: servicecfg.VerifyFull,
				},
				StaticLabels: map[string]string{
					types.OriginLabel: types.OriginConfigFile,
				},
				DynamicLabels: services.CommandLabels{},
			},
		},
		{
			desc: "AWS Keyspaces",
			inFlags: CommandLineFlags{
				DatabaseName:             "keyspace",
				DatabaseProtocol:         defaults.ProtocolCassandra,
				DatabaseURI:              "cassandra.us-east-1.amazonaws.com:9142",
				DatabaseAWSAccountID:     "123456789012",
				DatabaseAWSRegion:        "us-east-1",
				DatabaseAWSAssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				DatabaseAWSExternalID:    "externalID123",
			},
			outDatabase: servicecfg.Database{
				Name:     "keyspace",
				Protocol: defaults.ProtocolCassandra,
				URI:      "cassandra.us-east-1.amazonaws.com:9142",
				AWS: servicecfg.DatabaseAWS{
					Region:        "us-east-1",
					AccountID:     "123456789012",
					AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
					ExternalID:    "externalID123",
				},
				StaticLabels: map[string]string{
					types.OriginLabel: types.OriginConfigFile,
				},
				DynamicLabels: services.CommandLabels{},
				TLS: servicecfg.DatabaseTLS{
					Mode: servicecfg.VerifyFull,
				},
			},
		},
		{
			desc: "AWS DynamoDB",
			inFlags: CommandLineFlags{
				DatabaseName:             "ddb",
				DatabaseProtocol:         defaults.ProtocolDynamoDB,
				DatabaseURI:              "dynamodb.us-east-1.amazonaws.com",
				DatabaseAWSAccountID:     "123456789012",
				DatabaseAWSRegion:        "us-east-1",
				DatabaseAWSAssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				DatabaseAWSExternalID:    "externalID123",
			},
			outDatabase: servicecfg.Database{
				Name:     "ddb",
				Protocol: defaults.ProtocolDynamoDB,
				URI:      "dynamodb.us-east-1.amazonaws.com",
				AWS: servicecfg.DatabaseAWS{
					Region:        "us-east-1",
					AccountID:     "123456789012",
					AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
					ExternalID:    "externalID123",
				},
				StaticLabels: map[string]string{
					types.OriginLabel: types.OriginConfigFile,
				},
				DynamicLabels: services.CommandLabels{},
				TLS: servicecfg.DatabaseTLS{
					Mode: servicecfg.VerifyFull,
				},
			},
		},
		{
			desc: "AWS DynamoDB with session tags",
			inFlags: CommandLineFlags{
				DatabaseName:             "ddb",
				DatabaseProtocol:         defaults.ProtocolDynamoDB,
				DatabaseURI:              "dynamodb.us-east-1.amazonaws.com",
				DatabaseAWSAccountID:     "123456789012",
				DatabaseAWSRegion:        "us-east-1",
				DatabaseAWSAssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
				DatabaseAWSExternalID:    "externalID123",
				DatabaseAWSSessionTags:   "database_name=hello,something=else",
			},
			outDatabase: servicecfg.Database{
				Name:     "ddb",
				Protocol: defaults.ProtocolDynamoDB,
				URI:      "dynamodb.us-east-1.amazonaws.com",
				AWS: servicecfg.DatabaseAWS{
					Region:        "us-east-1",
					AccountID:     "123456789012",
					AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
					ExternalID:    "externalID123",
					SessionTags: map[string]string{
						"database_name": "hello",
						"something":     "else",
					},
				},
				StaticLabels: map[string]string{
					types.OriginLabel: types.OriginConfigFile,
				},
				DynamicLabels: services.CommandLabels{},
				TLS: servicecfg.DatabaseTLS{
					Mode: servicecfg.VerifyFull,
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			config := servicecfg.MakeDefaultConfig()
			err := Configure(&tt.inFlags, config, false)
			if tt.outError != "" {
				require.Contains(t, err.Error(), tt.outError)
			} else {
				require.NoError(t, err)
				require.Equal(t, []servicecfg.Database{tt.outDatabase}, config.Databases.Databases)
			}
		})
	}
}

func TestTLSCert(t *testing.T) {
	tmpDir := t.TempDir()
	tmpCA := path.Join(tmpDir, "ca.pem")

	err := os.WriteFile(tmpCA, fixtures.LocalhostCert, 0o644)
	require.NoError(t, err)

	tests := []struct {
		name string
		conf *FileConfig
	}{
		{
			"read deprecated DB cert field",
			&FileConfig{
				Databases: Databases{
					Service: Service{
						EnabledFlag: "true",
					},
					Databases: []*Database{
						{
							Name:       "test-db-1",
							Protocol:   "mysql",
							URI:        "localhost:1234",
							CACertFile: tmpCA,
						},
					},
				},
			},
		},
		{
			"read DB cert field",
			&FileConfig{
				Databases: Databases{
					Service: Service{
						EnabledFlag: "true",
					},
					Databases: []*Database{
						{
							Name:     "test-db-1",
							Protocol: "mysql",
							URI:      "localhost:1234",
							TLS: DatabaseTLS{
								CACertFile: tmpCA,
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := servicecfg.MakeDefaultConfig()

			err = ApplyFileConfig(tt.conf, cfg)
			require.NoError(t, err)

			require.Len(t, cfg.Databases.Databases, 1)
			require.Equal(t, fixtures.LocalhostCert, cfg.Databases.Databases[0].TLS.CACert)
		})
	}
}

func TestApplyKeyStoreConfig(t *testing.T) {
	slotNumber := 1

	tempDir := t.TempDir()

	worldReadablePinFilePath := filepath.Join(tempDir, "world-readable-pin-file")
	err := os.WriteFile(worldReadablePinFilePath, []byte("world-readable-pin-file"), 0o644)
	require.NoError(t, err)
	securePinFilePath := filepath.Join(tempDir, "secure-pin-file")
	err = os.WriteFile(securePinFilePath, []byte("secure-pin-file"), 0o600)
	require.NoError(t, err)

	worldWritablePKCS11LibPath := filepath.Join(tempDir, "world-writable-pkcs1")
	err = os.WriteFile(worldWritablePKCS11LibPath, []byte("pkcs11"), 0o666)
	require.NoError(t, err)
	require.NoError(t, os.Chmod(worldWritablePKCS11LibPath, 0o666))
	securePKCS11LibPath := filepath.Join(tempDir, "secure-pkcs11")
	err = os.WriteFile(securePKCS11LibPath, []byte("pkcs11"), 0o600)
	require.NoError(t, err)

	tests := []struct {
		name string

		auth Auth

		want       keystore.Config
		errMessage string
	}{
		{
			name: "handle nil configuration",
			auth: Auth{
				CAKeyParams: nil,
			},
			want: servicecfg.MakeDefaultConfig().Auth.KeyStore,
		},
		{
			name: "correct config",
			auth: Auth{
				CAKeyParams: &CAKeyParams{
					PKCS11: &PKCS11{
						ModulePath: securePKCS11LibPath,
						TokenLabel: "foo",
						SlotNumber: &slotNumber,
						Pin:        "pin",
					},
				},
			},
			want: keystore.Config{
				PKCS11: keystore.PKCS11Config{
					TokenLabel: "foo",
					SlotNumber: &slotNumber,
					Pin:        "pin",
					Path:       securePKCS11LibPath,
				},
			},
		},
		{
			name: "correct config with pin file",
			auth: Auth{
				CAKeyParams: &CAKeyParams{
					PKCS11: &PKCS11{
						ModulePath: securePKCS11LibPath,
						TokenLabel: "foo",
						SlotNumber: &slotNumber,
						PinPath:    securePinFilePath,
					},
				},
			},
			want: keystore.Config{
				PKCS11: keystore.PKCS11Config{
					TokenLabel: "foo",
					SlotNumber: &slotNumber,
					Pin:        "secure-pin-file",
					Path:       securePKCS11LibPath,
				},
			},
		},
		{
			name: "err when pin and pin path configured",
			auth: Auth{
				CAKeyParams: &CAKeyParams{
					PKCS11: &PKCS11{
						Pin:     "oops",
						PinPath: securePinFilePath,
					},
				},
			},
			errMessage: "can not set both pin and pin_path",
		},
		{
			name: "err when pkcs11 world writable",
			auth: Auth{
				CAKeyParams: &CAKeyParams{
					PKCS11: &PKCS11{
						ModulePath: worldWritablePKCS11LibPath,
					},
				},
			},
			errMessage: fmt.Sprintf(
				"PKCS11 library (%s) must not be world-writable",
				worldWritablePKCS11LibPath,
			),
		},
		{
			name: "err when pin file world-readable",
			auth: Auth{
				CAKeyParams: &CAKeyParams{
					PKCS11: &PKCS11{
						PinPath: worldReadablePinFilePath,
					},
				},
			},
			errMessage: fmt.Sprintf(
				"HSM pin file (%s) must not be world-readable",
				worldReadablePinFilePath,
			),
		},
		{
			name: "correct gcp config",
			auth: Auth{
				CAKeyParams: &CAKeyParams{
					GoogleCloudKMS: &GoogleCloudKMS{
						KeyRing:         "/projects/my-project/locations/global/keyRings/my-keyring",
						ProtectionLevel: "HSM",
					},
				},
			},
			want: keystore.Config{
				GCPKMS: keystore.GCPKMSConfig{
					KeyRing:         "/projects/my-project/locations/global/keyRings/my-keyring",
					ProtectionLevel: "HSM",
				},
			},
		},
		{
			name: "gcp config no protection level",
			auth: Auth{
				CAKeyParams: &CAKeyParams{
					GoogleCloudKMS: &GoogleCloudKMS{
						KeyRing: "/projects/my-project/locations/global/keyRings/my-keyring",
					},
				},
			},
			errMessage: "must set protection_level in ca_key_params.gcp_kms",
		},
		{
			name: "gcp config no keyring",
			auth: Auth{
				CAKeyParams: &CAKeyParams{
					GoogleCloudKMS: &GoogleCloudKMS{
						ProtectionLevel: "HSM",
					},
				},
			},
			errMessage: "must set keyring in ca_key_params.gcp_kms",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := servicecfg.MakeDefaultConfig()

			err := applyKeyStoreConfig(&FileConfig{
				Auth: tt.auth,
			}, cfg)
			if tt.errMessage != "" {
				require.EqualError(t, err, tt.errMessage)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.want, cfg.Auth.KeyStore)
			}
		})
	}
}

// TestApplyConfigSessionRecording checks if the session recording origin is
// set correct and if file configuration is read and applied correctly.
func TestApplyConfigSessionRecording(t *testing.T) {
	tests := []struct {
		desc                   string
		inSessionRecording     string
		inProxyChecksHostKeys  string
		outOrigin              string
		outSessionRecording    string
		outProxyChecksHostKeys bool
	}{
		{
			desc:                   "both-empty",
			inSessionRecording:     "",
			inProxyChecksHostKeys:  "",
			outOrigin:              "defaults",
			outSessionRecording:    "node",
			outProxyChecksHostKeys: true,
		},
		{
			desc:                   "proxy-checks-empty",
			inSessionRecording:     "session_recording: proxy-sync",
			inProxyChecksHostKeys:  "",
			outOrigin:              "config-file",
			outSessionRecording:    "proxy-sync",
			outProxyChecksHostKeys: true,
		},
		{
			desc:                   "session-recording-empty",
			inSessionRecording:     "",
			inProxyChecksHostKeys:  "proxy_checks_host_keys: true",
			outOrigin:              "config-file",
			outSessionRecording:    "node",
			outProxyChecksHostKeys: true,
		},
		{
			desc:                   "both-set",
			inSessionRecording:     "session_recording: node-sync",
			inProxyChecksHostKeys:  "proxy_checks_host_keys: false",
			outOrigin:              "config-file",
			outSessionRecording:    "node-sync",
			outProxyChecksHostKeys: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			fileconfig := fmt.Sprintf(configSessionRecording,
				tt.inSessionRecording,
				tt.inProxyChecksHostKeys)
			conf, err := ReadConfig(bytes.NewBufferString(fileconfig))
			require.NoError(t, err)

			cfg := servicecfg.MakeDefaultConfig()
			err = ApplyFileConfig(conf, cfg)
			require.NoError(t, err)

			require.Equal(t, tt.outOrigin, cfg.Auth.SessionRecordingConfig.Origin())
			require.Equal(t, tt.outSessionRecording, cfg.Auth.SessionRecordingConfig.GetMode())
			require.Equal(t, tt.outProxyChecksHostKeys, cfg.Auth.SessionRecordingConfig.GetProxyChecksHostKeys())
		})
	}
}

func TestJoinParams(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		desc             string
		input            string
		expectToken      string
		expectJoinMethod types.JoinMethod
		expectError      bool
	}{
		{
			desc: "empty",
		},
		{
			desc: "auth_token",
			input: `
teleport:
  auth_token: xxxyyy
`,
			expectToken:      "xxxyyy",
			expectJoinMethod: types.JoinMethodToken,
		},
		{
			desc: "join_params token",
			input: `
teleport:
  join_params:
    token_name: xxxyyy
    method: token
`,
			expectToken:      "xxxyyy",
			expectJoinMethod: types.JoinMethodToken,
		},
		{
			desc: "join_params ec2",
			input: `
teleport:
  join_params:
    token_name: xxxyyy
    method: ec2
`,
			expectToken:      "xxxyyy",
			expectJoinMethod: types.JoinMethodEC2,
		},
		{
			desc: "join_params iam",
			input: `
teleport:
  join_params:
    token_name: xxxyyy
    method: iam
`,
			expectToken:      "xxxyyy",
			expectJoinMethod: types.JoinMethodIAM,
		},
		{
			desc: "join_params invalid",
			input: `
teleport:
  join_params:
    token_name: xxxyyy
    method: invalid
`,
			expectError: true,
		},
		{
			desc: "both set",
			input: `
teleport:
  auth_token: xxxyyy
  join_params:
    token_name: xxxyyy
    method: iam
`,
			expectError: true,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			conf, err := ReadConfig(strings.NewReader(tc.input))
			require.NoError(t, err)

			cfg := servicecfg.MakeDefaultConfig()
			err = ApplyFileConfig(conf, cfg)

			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			token, err := cfg.Token()
			require.NoError(t, err)
			require.Equal(t, tc.expectToken, token)
			require.Equal(t, tc.expectJoinMethod, cfg.JoinMethod)
		})
	}
}

func TestApplyFileConfig_deviceTrustMode_errors(t *testing.T) {
	tests := []struct {
		name        string
		buildType   string
		deviceTrust *DeviceTrust
		wantErr     bool
	}{
		{
			name:      "ok: OSS Mode=off",
			buildType: modules.BuildOSS,
			deviceTrust: &DeviceTrust{
				Mode: constants.DeviceTrustModeOff,
			},
		},
		{
			name:      "nok: OSS Mode=required",
			buildType: modules.BuildOSS,
			deviceTrust: &DeviceTrust{
				Mode: constants.DeviceTrustModeRequired,
			},
			wantErr: true,
		},
		{
			name:      "ok: Enterprise Mode=required",
			buildType: modules.BuildEnterprise,
			deviceTrust: &DeviceTrust{
				Mode: constants.DeviceTrustModeRequired,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			modules.SetTestModules(t, &modules.TestModules{
				TestBuildType: test.buildType,
			})

			defaultCfg := servicecfg.MakeDefaultConfig()
			err := ApplyFileConfig(&FileConfig{
				Auth: Auth{
					Service: Service{
						EnabledFlag: "yes",
					},
					Authentication: &AuthenticationConfig{
						DeviceTrust: test.deviceTrust,
					},
				},
			}, defaultCfg)
			if test.wantErr {
				assert.Error(t, err, "ApplyFileConfig mismatch")
			} else {
				assert.NoError(t, err, "ApplyFileConfig mismatch")
			}
		})
	}
}

func TestApplyConfig_JamfService(t *testing.T) {
	tempDir := t.TempDir()

	// Write a password file, valid configs require one.
	const password = "supersecret!!1!"
	passwordFile := filepath.Join(tempDir, "test_jamf_password.txt")
	require.NoError(t,
		os.WriteFile(passwordFile, []byte(password+"\n"), 0o400),
		"WriteFile(%q) failed", passwordFile)

	minimalYAML := fmt.Sprintf(`
jamf_service:
  enabled: true
  api_endpoint: https://yourtenant.jamfcloud.com
  username: llama
  password_file: %v
`, passwordFile)

	tests := []struct {
		name    string
		yaml    string
		wantErr string
		want    servicecfg.JamfConfig
	}{
		{
			name: "minimal config",
			yaml: minimalYAML,
			want: servicecfg.JamfConfig{
				Spec: &types.JamfSpecV1{
					Enabled:     true,
					ApiEndpoint: "https://yourtenant.jamfcloud.com",
					Username:    "llama",
					Password:    password,
				},
			},
		},
		{
			name: "all fields",
			yaml: minimalYAML + `  name: jamf2
  sync_delay: 1m
  exit_on_sync: true
  inventory:
  - filter_rsql: 1==1
    sync_period_partial: 4h
    sync_period_full: 48h
    on_missing: NOOP
    page_size: 10
  - {}`,
			want: servicecfg.JamfConfig{
				Spec: &types.JamfSpecV1{
					Enabled:     true,
					Name:        "jamf2",
					SyncDelay:   types.Duration(1 * time.Minute),
					ApiEndpoint: "https://yourtenant.jamfcloud.com",
					Username:    "llama",
					Password:    password,
					Inventory: []*types.JamfInventoryEntry{
						{
							FilterRsql:        "1==1",
							SyncPeriodPartial: types.Duration(4 * time.Hour),
							SyncPeriodFull:    types.Duration(48 * time.Hour),
							OnMissing:         "NOOP",
							PageSize:          10,
						},
						{},
					},
				},
				ExitOnSync: true,
			},
		},

		{
			name:    "listen_addr not supported",
			yaml:    minimalYAML + `  listen_addr: localhost:55555`,
			wantErr: "listen_addr",
		},
		{
			name: "password_file empty",
			yaml: `
jamf_service:
  enabled: true
  api_endpoint: https://yourtenant.jamfcloud.com
  username: llama`,
			wantErr: "password_file required",
		},
		{
			name: "password_file invalid",
			yaml: `
jamf_service:
  enabled: true
  api_endpoint: https://yourtenant.jamfcloud.com
  username: llama
  password_file: /path/to/file/that/doesnt/exist.txt`,
			wantErr: "password_file",
		},
		{
			name: "spec is validated",
			yaml: minimalYAML + `  inventory:
  - on_missing: BANANA`,
			wantErr: "on_missing",
		},

		{
			name: "absent config ignored",
			yaml: ``,
		},
		{
			name: "empty config ignored",
			yaml: `jamf_service: {}`,
		},
		{
			name: "disabled config is validated",
			yaml: `
jamf_service:
  enabled: false
  api_endpoint: https://yourtenant.jamfcloud.com
  username: llama`,
			wantErr: "password_file",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fc, err := ReadConfig(strings.NewReader(test.yaml))
			require.NoError(t, err, "ReadConfig failed")

			cfg := servicecfg.MakeDefaultConfig()
			err = ApplyFileConfig(fc, cfg)
			if test.wantErr == "" {
				require.NoError(t, err, "ApplyFileConfig failed")
			} else {
				assert.ErrorContains(t, err, test.wantErr, "ApplyFileConfig error mismatch")
				return
			}

			if diff := cmp.Diff(test.want, cfg.Jamf, protocmp.Transform()); diff != "" {
				t.Errorf("ApplyFileConfig: JamfConfig mismatch (-want +got)\n%s", diff)
			}
		})
	}
}

func TestAuthHostedPlugins(t *testing.T) {
	t.Parallel()

	badParameter := func(t require.TestingT, err error, msgAndArgs ...interface{}) {
		require.Error(t, err)
		require.True(t, trace.IsBadParameter(err), `expected "bad parameter", but got %v`, err)
	}
	notExist := func(t require.TestingT, err error, msgAndArgs ...interface{}) {
		require.Error(t, err)
		require.ErrorIs(t, err, os.ErrNotExist, `expected "does not exist", but got %v`, err)
	}

	tmpDir := t.TempDir()
	clientIDFile := filepath.Join(tmpDir, "id")
	clientSecretFile := filepath.Join(tmpDir, "secret")
	err := os.WriteFile(clientIDFile, []byte("foo\n"), 0o777)
	require.NoError(t, err)
	err = os.WriteFile(clientSecretFile, []byte("bar\n"), 0o777)
	require.NoError(t, err)

	tests := []struct {
		desc     string
		config   string
		readErr  require.ErrorAssertionFunc
		applyErr require.ErrorAssertionFunc
		assert   func(t *testing.T, p servicecfg.HostedPluginsConfig)
	}{
		{
			desc: "Plugins enabled by default",
			config: strings.Join([]string{
				"auth_service:",
				"  enabled: yes",
			}, "\n"),
			readErr:  require.NoError,
			applyErr: require.NoError,
			assert: func(t *testing.T, p servicecfg.HostedPluginsConfig) {
				require.True(t, p.Enabled)
			},
		},
		{
			desc: "Unknown OAuth provider specified",
			config: strings.Join([]string{
				"auth_service:",
				"  enabled: yes",
				"  hosted_plugins:",
				"    enabled: yes",
				"    oauth_providers:",
				"      acmecorp:",
				"        client_id: foo",
				"        client_secret: bar",
			}, "\n"),
			readErr:  require.Error,
			applyErr: require.NoError,
		},
		{
			desc: "OAuth client ID without the secret",
			config: strings.Join([]string{
				"auth_service:",
				"  enabled: yes",
				"  hosted_plugins:",
				"    enabled: yes",
				"    oauth_providers:",
				"      slack:",
				"        client_id: foo",
			}, "\n"),
			readErr:  require.NoError,
			applyErr: badParameter,
		},
		{
			desc: "OAuth client secret without the ID",
			config: strings.Join([]string{
				"auth_service:",
				"  enabled: yes",
				"  hosted_plugins:",
				"    enabled: yes",
				"    oauth_providers:",
				"      slack:",
				"        client_secret: bar",
			}, "\n"),
			readErr:  require.NoError,
			applyErr: badParameter,
		},
		{
			desc: "OAuth provider in non-existent file",
			config: strings.Join([]string{
				"auth_service:",
				"  enabled: yes",
				"",
				"  hosted_plugins:",
				"    enabled: yes",
				"    oauth_providers:",
				"      slack:",
				"        client_id: /tmp/this-does-not-exist",
				"        client_secret: " + clientSecretFile,
			}, "\n"),
			readErr:  require.NoError,
			applyErr: notExist,
		},
		{
			desc: "OAuth provider in existent files",
			config: strings.Join([]string{
				"auth_service:",
				"  enabled: yes",
				"",
				"  hosted_plugins:",
				"    enabled: yes",
				"    oauth_providers:",
				"      slack:",
				"        client_id: " + clientIDFile,
				"        client_secret: " + clientSecretFile,
			}, "\n"),
			readErr:  require.NoError,
			applyErr: require.NoError,
			assert: func(t *testing.T, p servicecfg.HostedPluginsConfig) {
				require.True(t, p.Enabled)
				require.NotNil(t, p.OAuthProviders.Slack)
				require.Equal(t, "foo", p.OAuthProviders.Slack.ID)
				require.Equal(t, "bar", p.OAuthProviders.Slack.Secret)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			conf, err := ReadConfig(bytes.NewBufferString(tc.config))
			tc.readErr(t, err)

			cfg := servicecfg.MakeDefaultConfig()
			err = ApplyFileConfig(conf, cfg)
			tc.applyErr(t, err)
			if tc.assert != nil {
				tc.assert(t, cfg.Auth.HostedPlugins)
			}
		})
	}
}

func TestApplyDiscoveryConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		discoveryConfig   Discovery
		expectedDiscovery servicecfg.DiscoveryConfig
	}{
		{
			name:            "no matchers",
			discoveryConfig: Discovery{},
			expectedDiscovery: servicecfg.DiscoveryConfig{
				Enabled: true,
			},
		},
		{
			name: "azure matchers",
			discoveryConfig: Discovery{
				AzureMatchers: []AzureMatcher{
					{
						Types:         []string{"aks", "vm"},
						Subscriptions: []string{"abcd"},
						InstallParams: &InstallParams{
							JoinParams: JoinParams{
								TokenName: "azure-token",
								Method:    "azure",
							},
							ScriptName:      "default-installer",
							PublicProxyAddr: "proxy.example.com",
							Azure: &AzureInstallParams{
								ClientID: "abcd1234",
							},
						},
					},
				},
			},
			expectedDiscovery: servicecfg.DiscoveryConfig{
				Enabled: true,
				AzureMatchers: []types.AzureMatcher{
					{
						Subscriptions: []string{"abcd"},
						Types:         []string{"aks", "vm"},
						Params: &types.InstallerParams{
							JoinMethod:      "azure",
							JoinToken:       "azure-token",
							ScriptName:      "default-installer",
							PublicProxyAddr: "proxy.example.com",
							Azure: &types.AzureInstallerParams{
								ClientID: "abcd1234",
							},
						},
						Regions:        []string{"*"},
						ResourceTags:   types.Labels{"*": []string{"*"}},
						ResourceGroups: []string{"*"},
					},
				},
			},
		},
		{
			name: "azure matchers no installer",
			discoveryConfig: Discovery{
				AzureMatchers: []AzureMatcher{
					{
						Types:         []string{"aks"},
						Subscriptions: []string{"abcd"},
					},
				},
			},
			expectedDiscovery: servicecfg.DiscoveryConfig{
				Enabled: true,
				AzureMatchers: []types.AzureMatcher{
					{
						Subscriptions:  []string{"abcd"},
						Types:          []string{"aks"},
						Regions:        []string{"*"},
						ResourceTags:   types.Labels{"*": []string{"*"}},
						ResourceGroups: []string{"*"},
					},
				},
			},
		},
		{
			name: "tag matchers",
			discoveryConfig: Discovery{
				AccessGraph: &AccessGraphSync{
					AWS: []AccessGraphAWSSync{
						{
							Regions:       []string{"us-west-2", "us-east-1"},
							AssumeRoleARN: "arn:aws:iam::123456789012:role/DBDiscoverer",
							ExternalID:    "externalID123",
						},
					},
				},
			},
			expectedDiscovery: servicecfg.DiscoveryConfig{
				Enabled: true,
				AccessGraph: &types.AccessGraphSync{
					AWS: []*types.AccessGraphAWSSync{
						{
							Regions: []string{"us-west-2", "us-east-1"},
							AssumeRole: &types.AssumeRole{
								RoleARN:    "arn:aws:iam::123456789012:role/DBDiscoverer",
								ExternalID: "externalID123",
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fc, err := ReadConfig(bytes.NewBufferString(NoServicesConfigString))
			require.NoError(t, err)
			fc.Discovery = tc.discoveryConfig
			fc.Discovery.EnabledFlag = "yes"
			cfg := servicecfg.MakeDefaultConfig()
			require.NoError(t, applyDiscoveryConfig(fc, cfg))
			require.Equal(t, tc.expectedDiscovery, cfg.Discovery)
		})
	}
}

func TestApplyOktaConfig(t *testing.T) {
	t.Parallel()
	tests := []struct {
		desc             string
		createTokenFile  bool
		oktaConfig       Okta
		expectedOkta     servicecfg.OktaConfig
		errAssertionFunc require.ErrorAssertionFunc
	}{
		{
			desc:            "valid config (access list sync defaults to false)",
			createTokenFile: true,
			oktaConfig: Okta{
				Service: Service{
					EnabledFlag: "yes",
				},
				APIEndpoint: "https://test-endpoint",
			},
			expectedOkta: servicecfg.OktaConfig{
				Enabled:     true,
				APIEndpoint: "https://test-endpoint",
				SyncSettings: servicecfg.OktaSyncSettings{
					SyncAccessLists: false,
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			desc:            "valid config (access list sync enabled)",
			createTokenFile: true,
			oktaConfig: Okta{
				Service: Service{
					EnabledFlag: "yes",
				},
				APIEndpoint: "https://test-endpoint",
				Sync: OktaSync{
					SyncAccessListsFlag: "yes",
					DefaultOwners:       []string{"owner1"},
				},
			},
			expectedOkta: servicecfg.OktaConfig{
				Enabled:     true,
				APIEndpoint: "https://test-endpoint",
				SyncSettings: servicecfg.OktaSyncSettings{
					SyncAccessLists: true,
					DefaultOwners:   []string{"owner1"},
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			desc:            "valid config (access list sync with filters)",
			createTokenFile: true,
			oktaConfig: Okta{
				Service: Service{
					EnabledFlag: "yes",
				},
				APIEndpoint: "https://test-endpoint",
				Sync: OktaSync{
					SyncAccessListsFlag: "yes",
					DefaultOwners:       []string{"owner1"},
					GroupFilters: []string{
						"group*",
						"^admin-.*$",
					},
					AppFilters: []string{
						"app*",
						"^admin-.*$",
					},
				},
			},
			expectedOkta: servicecfg.OktaConfig{
				Enabled:     true,
				APIEndpoint: "https://test-endpoint",
				SyncSettings: servicecfg.OktaSyncSettings{
					SyncAccessLists: true,
					DefaultOwners:   []string{"owner1"},
					GroupFilters: []string{
						"group*",
						"^admin-.*$",
					},
					AppFilters: []string{
						"app*",
						"^admin-.*$",
					},
				},
			},
			errAssertionFunc: require.NoError,
		},
		{
			desc:            "empty URL",
			createTokenFile: true,
			oktaConfig: Okta{
				Service: Service{
					EnabledFlag: "yes",
				},
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter(`okta_service is enabled but no api_endpoint is specified`))
			},
		},
		{
			desc:            "bad url",
			createTokenFile: true,
			oktaConfig: Okta{
				Service: Service{
					EnabledFlag: "yes",
				},
				APIEndpoint: `bad%url`,
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter(`malformed URL bad%%url`))
			},
		},
		{
			desc:            "no host",
			createTokenFile: true,
			oktaConfig: Okta{
				Service: Service{
					EnabledFlag: "yes",
				},
				APIEndpoint: `http://`,
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter(`api_endpoint has no host`))
			},
		},
		{
			desc:            "no scheme",
			createTokenFile: true,
			oktaConfig: Okta{
				Service: Service{
					EnabledFlag: "yes",
				},
				APIEndpoint: `//hostname`,
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter(`api_endpoint has no scheme`))
			},
		},
		{
			desc: "empty file",
			oktaConfig: Okta{
				Service: Service{
					EnabledFlag: "yes",
				},
				APIEndpoint: "https://test-endpoint",
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter(`okta_service is enabled but no api_token_path is specified`))
			},
		},
		{
			desc: "bad file",
			oktaConfig: Okta{
				Service: Service{
					EnabledFlag: "yes",
				},
				APIEndpoint:  "https://test-endpoint",
				APITokenPath: "/non-existent/path",
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter(`error trying to find file %s`, i...))
			},
		},
		{
			desc:            "no default owners",
			createTokenFile: true,
			oktaConfig: Okta{
				Service: Service{
					EnabledFlag: "yes",
				},
				APIEndpoint: "https://test-endpoint",
				Sync: OktaSync{
					SyncAccessListsFlag: "yes",
				},
			},
			errAssertionFunc: func(tt require.TestingT, err error, i ...interface{}) {
				require.ErrorIs(t, err, trace.BadParameter("default owners must be set when access list import is enabled"))
			},
		},
		{
			desc:            "bad group filter",
			createTokenFile: true,
			oktaConfig: Okta{
				Service: Service{
					EnabledFlag: "yes",
				},
				APIEndpoint: "https://test-endpoint",
				Sync: OktaSync{
					SyncAccessListsFlag: "yes",
					DefaultOwners:       []string{"owner1"},
					GroupFilters: []string{
						"^admin-.[[[*$",
					},
				},
			},
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "error parsing group filter: ^admin-.[[[*$")
			},
		},
		{
			desc:            "bad app filter",
			createTokenFile: true,
			oktaConfig: Okta{
				Service: Service{
					EnabledFlag: "yes",
				},
				APIEndpoint: "https://test-endpoint",
				Sync: OktaSync{
					SyncAccessListsFlag: "yes",
					DefaultOwners:       []string{"owner1"},
					AppFilters: []string{
						"^admin-.[[[*$",
					},
				},
			},
			errAssertionFunc: func(t require.TestingT, err error, i ...interface{}) {
				require.ErrorContains(t, err, "error parsing app filter: ^admin-.[[[*$")
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			fc, err := ReadConfig(bytes.NewBufferString(NoServicesConfigString))
			require.NoError(t, err)

			expectedOkta := test.expectedOkta

			fc.Okta = test.oktaConfig
			if test.createTokenFile {
				file, err := os.CreateTemp("", "")
				require.NoError(t, err)
				t.Cleanup(func() {
					require.NoError(t, os.Remove(file.Name()))
				})
				fc.Okta.APITokenPath = file.Name()
				expectedOkta.APITokenPath = file.Name()
			}
			cfg := servicecfg.MakeDefaultConfig()
			err = applyOktaConfig(fc, cfg)
			test.errAssertionFunc(t, err, fc.Okta.APITokenPath)
			if err == nil {
				require.Equal(t, expectedOkta, cfg.Okta)
			}
		})
	}
}

func TestAssistKey(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		desc        string
		input       string
		expectKey   string
		expectError bool
	}{
		{
			desc: "api token is set",
			input: `
teleport:
proxy_service:
  assist:
    openai:
      api_token_path: testdata/test-api-key
`,
			expectKey: "123-abc-zzz",
		},
		{
			desc: "api token file does not exist",
			input: `
teleport:
proxy_service:
  assist:
    openai:
      api_token_path: testdata/non-existent-file
`,
			expectError: true,
		},
		{
			desc: "missing api token doesn't error",
			input: `
teleport:
proxy_service:
  assist:
    openai:
`,
			expectKey: "",
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			conf, err := ReadConfig(strings.NewReader(tc.input))
			require.NoError(t, err)

			cfg := servicecfg.MakeDefaultConfig()
			err = ApplyFileConfig(conf, cfg)

			if tc.expectError {
				require.Error(t, err)
				return
			}

			require.Equal(t, tc.expectKey, cfg.Proxy.AssistAPIKey)
		})
	}
}

func TestApplyKubeConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name              string
		inputFileConfig   Kube
		wantServiceConfig servicecfg.KubeConfig
		wantError         bool
	}{
		{
			name: "invalid listener address",
			inputFileConfig: Kube{
				Service: Service{
					ListenAddress: "0.0.0.0:a",
				},
				KubeconfigFile: "path-to-kubeconfig",
			},
			wantError: true,
		},
		{
			name: "assume_role_arn is not supported",
			inputFileConfig: Kube{
				Service: Service{
					ListenAddress: "0.0.0.0:8888",
				},
				KubeconfigFile: "path-to-kubeconfig",
				ResourceMatchers: []ResourceMatcher{
					{
						Labels: map[string]apiutils.Strings{"a": {"b"}},
						AWS: ResourceMatcherAWS{
							AssumeRoleARN: "arn:aws:iam::123456789012:role/KubeAccess",
							ExternalID:    "externalID123",
						},
					},
				},
			},
			wantError: false,
			wantServiceConfig: servicecfg.KubeConfig{
				ListenAddr:     utils.MustParseAddr("0.0.0.0:8888"),
				KubeconfigPath: "path-to-kubeconfig",
				ResourceMatchers: []services.ResourceMatcher{
					{
						Labels: map[string]apiutils.Strings{"a": {"b"}},
						AWS: services.ResourceMatcherAWS{
							AssumeRoleARN: "arn:aws:iam::123456789012:role/KubeAccess",
							ExternalID:    "externalID123",
						},
					},
				},
				Limiter: limiter.Config{
					MaxConnections:   defaults.LimiterMaxConnections,
					MaxNumberOfUsers: 250,
				},
			},
		},
		{
			name: "valid",
			inputFileConfig: Kube{
				Service: Service{
					ListenAddress: "0.0.0.0:8888",
				},
				PublicAddr:      apiutils.Strings{"example.com", "example.with.port.com:4444"},
				KubeconfigFile:  "path-to-kubeconfig",
				KubeClusterName: "kube-name",
				ResourceMatchers: []ResourceMatcher{{
					Labels: map[string]apiutils.Strings{"a": {"b"}},
				}},
				StaticLabels: map[string]string{
					"env":     "dev",
					"product": "test",
				},
				DynamicLabels: []CommandLabel{{
					Name:    "hostname",
					Command: []string{"hostname"},
					Period:  time.Hour,
				}},
			},
			wantServiceConfig: servicecfg.KubeConfig{
				ListenAddr:      utils.MustParseAddr("0.0.0.0:8888"),
				PublicAddrs:     []utils.NetAddr{*utils.MustParseAddr("example.com:3026"), *utils.MustParseAddr("example.with.port.com:4444")},
				KubeconfigPath:  "path-to-kubeconfig",
				KubeClusterName: "kube-name",
				ResourceMatchers: []services.ResourceMatcher{
					{
						Labels: map[string]apiutils.Strings{"a": {"b"}},
					},
				},
				StaticLabels: map[string]string{
					"env":     "dev",
					"product": "test",
				},
				DynamicLabels: services.CommandLabels{
					"hostname": &types.CommandLabelV2{
						Period:  types.Duration(time.Hour),
						Command: []string{"hostname"},
					},
				},
				Limiter: limiter.Config{
					MaxConnections:   defaults.LimiterMaxConnections,
					MaxNumberOfUsers: 250,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fc, err := ReadConfig(bytes.NewBufferString(NoServicesConfigString))
			require.NoError(t, err)
			fc.Kube = test.inputFileConfig
			fc.Kube.EnabledFlag = "yes"

			cfg := servicecfg.MakeDefaultConfig()
			err = applyKubeConfig(fc, cfg)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.wantServiceConfig, cfg.Kube)
			}
		})
	}
}

func TestGetInstallerProxyAddr(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name              string
		installParams     *InstallParams
		fc                *FileConfig
		expectedProxyAddr string
	}{
		{
			name:              "empty",
			fc:                &FileConfig{},
			expectedProxyAddr: "",
		},
		{
			name: "explicit proxy addr",
			installParams: &InstallParams{
				PublicProxyAddr: "explicit.example.com",
			},
			fc: &FileConfig{
				Global: Global{
					ProxyServer: "proxy.example.com",
				},
			},
			expectedProxyAddr: "explicit.example.com",
		},
		{
			name: "proxy server",
			fc: &FileConfig{
				Global: Global{
					ProxyServer: "proxy.example.com",
				},
			},
			expectedProxyAddr: "proxy.example.com",
		},
		{
			name: "local proxy service",
			fc: &FileConfig{
				Global: Global{
					AuthServer: "auth.example.com",
				},
				Proxy: Proxy{
					Service: Service{
						EnabledFlag: "yes",
					},
					PublicAddr: apiutils.Strings{"proxy.example.com"},
				},
			},
			expectedProxyAddr: "proxy.example.com",
		},
		{
			name: "v1/v2 auth servers",
			fc: &FileConfig{
				Version: "v2",
				Global: Global{
					AuthServers: []string{"proxy.example.com"},
				},
			},
			expectedProxyAddr: "proxy.example.com",
		},
		{
			name: "auth server",
			fc: &FileConfig{
				Global: Global{
					AuthServer: "auth.example.com",
				},
			},
			expectedProxyAddr: "auth.example.com",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expectedProxyAddr, getInstallerProxyAddr(tc.installParams, tc.fc))
		})
	}
}

func TestDiscoveryConfig(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		desc                  string
		mutate                func(cfgMap)
		expectError           require.ErrorAssertionFunc
		expectEnabled         require.BoolAssertionFunc
		expectedTotalMatchers int
		expectedAWSMatchers   []types.AWSMatcher
		expectedAzureMatchers []types.AzureMatcher
		expectedGCPMatchers   []types.GCPMatcher
	}{
		{
			desc:          "default",
			mutate:        func(cfgMap) {},
			expectError:   require.NoError,
			expectEnabled: require.False,
		},
		{
			desc:          "GCP section without project_ids",
			expectError:   require.Error,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["gcp"] = []cfgMap{
					{
						"types": []string{"gke"},
					},
				}
			},
		},
		{
			desc:          "GCP section is filled with defaults",
			expectError:   require.NoError,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["gcp"] = []cfgMap{
					{
						"types":       []string{"gke"},
						"project_ids": []string{"p1", "p2"},
					},
				}
			},
			expectedGCPMatchers: []types.GCPMatcher{{
				Types:     []string{"gke"},
				Locations: []string{"*"},
				Labels: map[string]apiutils.Strings{
					"*": []string{"*"},
				},
				ProjectIDs: []string{"p1", "p2"},
			}},
		},
		{
			desc:          "GCP section is filled",
			expectError:   require.NoError,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["gcp"] = []cfgMap{
					{
						"types":     []string{"gke"},
						"locations": []string{"eucentral1"},
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
						"project_ids": []string{"p1", "p2"},
					},
				}
			},
			expectedGCPMatchers: []types.GCPMatcher{{
				Types:     []string{"gke"},
				Locations: []string{"eucentral1"},
				Labels: map[string]apiutils.Strings{
					"discover_teleport": []string{"yes"},
				},
				Tags: map[string]apiutils.Strings{
					"discover_teleport": []string{"yes"},
				},
				ProjectIDs: []string{"p1", "p2"},
			}},
		},
		{
			desc:          "GCP section is filled with installer",
			expectError:   require.NoError,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["gcp"] = []cfgMap{
					{
						"types":     []string{"gce"},
						"locations": []string{"eucentral1"},
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
						"project_ids":      []string{"p1", "p2"},
						"service_accounts": []string{"a@example.com", "b@example.com"},
					},
				}
				cfg["version"] = "v3"
				cfg["teleport"].(cfgMap)["proxy_server"] = "example.com"
				cfg["proxy_service"] = cfgMap{
					"enabled": "no",
				}
			},
			expectedGCPMatchers: []types.GCPMatcher{{
				Types:     []string{"gce"},
				Locations: []string{"eucentral1"},
				Labels: map[string]apiutils.Strings{
					"discover_teleport": []string{"yes"},
				},
				Tags: map[string]apiutils.Strings{
					"discover_teleport": []string{"yes"},
				},
				ProjectIDs:      []string{"p1", "p2"},
				ServiceAccounts: []string{"a@example.com", "b@example.com"},
				Params: &types.InstallerParams{
					JoinMethod:      types.JoinMethodGCP,
					JoinToken:       types.GCPInviteTokenName,
					ScriptName:      installers.InstallerScriptName,
					PublicProxyAddr: "example.com",
				},
			}},
		},
		{
			desc:          "Azure section is filled with defaults (aks)",
			expectError:   require.NoError,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["azure"] = []cfgMap{
					{
						"types": []string{"aks"},
					},
				}
			},
			expectedAzureMatchers: []types.AzureMatcher{{
				Types:   []string{"aks"},
				Regions: []string{"*"},
				ResourceTags: map[string]apiutils.Strings{
					"*": []string{"*"},
				},
				Subscriptions:  []string{"*"},
				ResourceGroups: []string{"*"},
			}},
		},
		{
			desc:          "Azure section is filled with values",
			expectError:   require.NoError,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["azure"] = []cfgMap{
					{
						"types":   []string{"aks"},
						"regions": []string{"eucentral1"},
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
						"subscriptions":   []string{"sub1", "sub2"},
						"resource_groups": []string{"group1", "group2"},
					},
				}
			},
			expectedAzureMatchers: []types.AzureMatcher{{
				Types:   []string{"aks"},
				Regions: []string{"eucentral1"},
				ResourceTags: map[string]apiutils.Strings{
					"discover_teleport": []string{"yes"},
				},
				Subscriptions:  []string{"sub1", "sub2"},
				ResourceGroups: []string{"group1", "group2"},
			}},
		},
		{
			desc:          "AWS section is filled with defaults",
			expectError:   require.NoError,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["aws"] = []cfgMap{
					{
						"types":   []string{"ec2"},
						"regions": []string{"eu-central-1"},
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
					},
				}
			},
			expectedAWSMatchers: []types.AWSMatcher{{
				Types:   []string{"ec2"},
				Regions: []string{"eu-central-1"},
				Tags: map[string]apiutils.Strings{
					"discover_teleport": []string{"yes"},
				},
				Params: &types.InstallerParams{
					JoinMethod:      types.JoinMethodIAM,
					JoinToken:       types.IAMInviteTokenName,
					SSHDConfig:      "/etc/ssh/sshd_config",
					ScriptName:      installers.InstallerScriptName,
					InstallTeleport: true,
					EnrollMode:      types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
				},
				SSM: &types.AWSSSM{DocumentName: types.AWSInstallerDocument},
			}},
		},
		{
			desc:          "AWS section is filled with custom configs",
			expectError:   require.NoError,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["aws"] = []cfgMap{
					{
						"types":   []string{"ec2"},
						"regions": []string{"eu-central-1"},
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
						"install": cfgMap{
							"join_params": cfgMap{
								"token_name": "hello-iam-a-token",
								"method":     "iam",
							},
							"script_name": "installer-custom",
						},
						"ssm": cfgMap{
							"document_name": "hello_document",
						},
						"assume_role_arn": "arn:aws:iam::123456789012:role/DBDiscoverer",
						"external_id":     "externalID123",
					},
				}
			},
			expectedAWSMatchers: []types.AWSMatcher{{
				Types:   []string{"ec2"},
				Regions: []string{"eu-central-1"},
				Tags: map[string]apiutils.Strings{
					"discover_teleport": []string{"yes"},
				},
				Params: &types.InstallerParams{
					JoinMethod:      types.JoinMethodIAM,
					JoinToken:       "hello-iam-a-token",
					SSHDConfig:      "/etc/ssh/sshd_config",
					ScriptName:      "installer-custom",
					InstallTeleport: true,
					EnrollMode:      types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
				},
				SSM: &types.AWSSSM{DocumentName: "hello_document"},
				AssumeRole: &types.AssumeRole{
					RoleARN:    "arn:aws:iam::123456789012:role/DBDiscoverer",
					ExternalID: "externalID123",
				},
			}},
		},
		{
			desc:          "AWS section with eice enroll mode",
			expectError:   require.NoError,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["aws"] = []cfgMap{
					{
						"types":   []string{"ec2"},
						"regions": []string{"eu-central-1"},
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
						"install": cfgMap{
							"join_params": cfgMap{
								"token_name": "hello-iam-a-token",
								"method":     "iam",
							},
							"script_name": "installer-custom",
							"enroll_mode": "eice",
						},
						"ssm": cfgMap{
							"document_name": "hello_document",
						},
						"assume_role_arn": "arn:aws:iam::123456789012:role/DBDiscoverer",
						"external_id":     "externalID123",
						"integration":     "my-integration",
					},
				}
			},
			expectedAWSMatchers: []types.AWSMatcher{{
				Types:   []string{"ec2"},
				Regions: []string{"eu-central-1"},
				Tags: map[string]apiutils.Strings{
					"discover_teleport": []string{"yes"},
				},
				Params: &types.InstallerParams{
					JoinMethod:      types.JoinMethodIAM,
					JoinToken:       "hello-iam-a-token",
					SSHDConfig:      "/etc/ssh/sshd_config",
					ScriptName:      "installer-custom",
					InstallTeleport: true,
					EnrollMode:      types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_EICE,
				},
				SSM:         &types.AWSSSM{DocumentName: "hello_document"},
				Integration: "my-integration",
				AssumeRole: &types.AssumeRole{
					RoleARN:    "arn:aws:iam::123456789012:role/DBDiscoverer",
					ExternalID: "externalID123",
				},
			}},
		},
		{
			desc:          "AWS cannot use EICE mode without integration",
			expectError:   require.Error,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["aws"] = []cfgMap{
					{
						"types":   []string{"ec2"},
						"regions": []string{"eu-central-1"},
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
						"install": cfgMap{
							"join_params": cfgMap{
								"token_name": "hello-iam-a-token",
								"method":     "iam",
							},
							"script_name": "installer-custom",
							"enroll_mode": "eice",
						},
						"ssm": cfgMap{
							"document_name": "hello_document",
						},
						"assume_role_arn": "arn:aws:iam::123456789012:role/DBDiscoverer",
						"external_id":     "externalID123",
					},
				}
			},
		},
		{
			desc:          "AWS section is filled with invalid region",
			expectError:   require.Error,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["aws"] = []cfgMap{
					{
						"types":   []string{"ec2"},
						"regions": []string{"*"},
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
					},
				}
			},
		},
		{
			desc:          "AWS section is filled with invalid join method",
			expectError:   require.Error,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["aws"] = []cfgMap{
					{
						"install": cfgMap{
							"join_params": cfgMap{
								"token_name": "hello-iam-a-token",
								"method":     "token",
							},
						},
					},
				}
			},
		},
		{
			desc:          "AWS section is filled with external_id but empty assume_role_arn",
			expectError:   require.Error,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["aws"] = []cfgMap{
					{
						"types":           []string{"rds"},
						"regions":         []string{"us-west-1"},
						"assume_role_arn": "",
						"external_id":     "externalid123",
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
					},
				}
			},
		},
		{
			desc:          "AWS section is filled with external_id but empty assume_role_arn is ok for redshift serverless",
			expectError:   require.NoError,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["aws"] = []cfgMap{
					{
						"types":           []string{"redshift-serverless"},
						"regions":         []string{"us-west-1"},
						"assume_role_arn": "",
						"external_id":     "externalid123",
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
					},
				}
			},
			expectedAWSMatchers: []types.AWSMatcher{{
				Types:   []string{"redshift-serverless"},
				Regions: []string{"us-west-1"},
				Tags: map[string]apiutils.Strings{
					"discover_teleport": []string{"yes"},
				},
				Params: &types.InstallerParams{
					JoinMethod:      types.JoinMethodIAM,
					JoinToken:       "aws-discovery-iam-token",
					SSHDConfig:      "/etc/ssh/sshd_config",
					ScriptName:      "default-installer",
					InstallTeleport: true,
					EnrollMode:      types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
				},
				SSM: &types.AWSSSM{DocumentName: "TeleportDiscoveryInstaller"},
				AssumeRole: &types.AssumeRole{
					RoleARN:    "",
					ExternalID: "externalid123",
				},
			}},
		},
		{
			desc:          "AWS section is filled with invalid assume_role_arn",
			expectError:   require.Error,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["aws"] = []cfgMap{
					{
						"types":           []string{"rds"},
						"regions":         []string{"us-west-1"},
						"assume_role_arn": "foobar",
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
					},
				}
			},
		},
		{
			desc:          "AWS section is filled with assume_role_arn that is not an iam ARN",
			expectError:   require.Error,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["aws"] = []cfgMap{
					{
						"types":           []string{"rds"},
						"regions":         []string{"us-west-1"},
						"assume_role_arn": "arn:aws:sts::123456789012:federated-user/Alice",
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
					},
				}
			},
		},
		{
			desc:          "AWS section is filled with no token",
			expectError:   require.NoError,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["aws"] = []cfgMap{
					{
						"types":   []string{"ec2"},
						"regions": []string{"eu-west-1"},
						"install": cfgMap{
							"join_params": cfgMap{
								"method": "iam",
							},
						},
					},
				}
			},
			expectedAWSMatchers: []types.AWSMatcher{{
				Types: []string{"ec2"},
				SSM: &types.AWSSSM{
					DocumentName: types.AWSInstallerDocument,
				},
				Regions: []string{"eu-west-1"},
				Tags:    map[string]apiutils.Strings{"*": {"*"}},
				Params: &types.InstallerParams{
					JoinMethod:      types.JoinMethodIAM,
					JoinToken:       types.IAMInviteTokenName,
					ScriptName:      installers.InstallerScriptName,
					SSHDConfig:      "/etc/ssh/sshd_config",
					InstallTeleport: true,
					EnrollMode:      types.InstallParamEnrollMode_INSTALL_PARAM_ENROLL_MODE_SCRIPT,
				},
			}},
		},
		{
			desc:          "Azure section is filled with defaults (vm)",
			expectError:   require.NoError,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["azure"] = []cfgMap{
					{
						"types":           []string{"vm"},
						"regions":         []string{"westcentralus"},
						"resource_groups": []string{"rg1"},
						"subscriptions":   []string{"88888888-8888-8888-8888-888888888888"},
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
					},
				}
				cfg["version"] = "v3"
				cfg["teleport"].(cfgMap)["proxy_server"] = "example.com"
				cfg["proxy_service"] = cfgMap{
					"enabled": "no",
				}
			},
			expectedAzureMatchers: []types.AzureMatcher{{
				Types:          []string{"vm"},
				Regions:        []string{"westcentralus"},
				ResourceGroups: []string{"rg1"},
				Subscriptions:  []string{"88888888-8888-8888-8888-888888888888"},
				ResourceTags: map[string]apiutils.Strings{
					"discover_teleport": []string{"yes"},
				},
				Params: &types.InstallerParams{
					JoinMethod:      "azure",
					JoinToken:       "azure-discovery-token",
					ScriptName:      "default-installer",
					PublicProxyAddr: "example.com",
					Azure:           &types.AzureInstallerParams{},
				},
			}},
		},
		{
			desc:          "Azure section is filled with custom config",
			expectError:   require.NoError,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["azure"] = []cfgMap{
					{
						"types":           []string{"vm"},
						"regions":         []string{"westcentralus"},
						"resource_groups": []string{"rg1"},
						"subscriptions":   []string{"88888888-8888-8888-8888-888888888888"},
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
						"install": cfgMap{
							"join_params": cfgMap{
								"token_name": "custom-azure-token",
								"method":     "azure",
							},
							"script_name":       "custom-installer",
							"public_proxy_addr": "teleport.example.com",
						},
					},
				}
			},
			expectedAzureMatchers: []types.AzureMatcher{{
				Types:          []string{"vm"},
				Regions:        []string{"westcentralus"},
				ResourceGroups: []string{"rg1"},
				Subscriptions:  []string{"88888888-8888-8888-8888-888888888888"},
				ResourceTags: map[string]apiutils.Strings{
					"discover_teleport": []string{"yes"},
				},
				Params: &types.InstallerParams{
					JoinMethod:      "azure",
					JoinToken:       "custom-azure-token",
					ScriptName:      "custom-installer",
					PublicProxyAddr: "teleport.example.com",
					Azure:           &types.AzureInstallerParams{},
				},
			}},
		},
		{
			desc:          "Azure section is filled with invalid join method",
			expectError:   require.Error,
			expectEnabled: require.True,
			mutate: func(cfg cfgMap) {
				cfg["discovery_service"].(cfgMap)["enabled"] = "yes"
				cfg["discovery_service"].(cfgMap)["azure"] = []cfgMap{
					{
						"types":           []string{"vm"},
						"regions":         []string{"westcentralus"},
						"resource_groups": []string{"rg1"},
						"subscriptions":   []string{"88888888-8888-8888-8888-888888888888"},
						"tags": cfgMap{
							"discover_teleport": "yes",
						},
						"install": cfgMap{
							"join_params": cfgMap{
								"token_name": "custom-azure-token",
								"method":     "token",
							},
						},
					},
				}
			},
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.desc, func(t *testing.T) {
			text := bytes.NewBuffer(editConfig(t, testCase.mutate))
			fc, err := ReadConfig(text)
			require.NoError(t, err)

			cfg := servicecfg.MakeDefaultConfig()

			err = ApplyFileConfig(fc, cfg)
			testCase.expectError(t, err)
			if cfg == nil {
				return
			}

			testCase.expectEnabled(t, cfg.Discovery.Enabled)
			require.Equal(t, testCase.expectedAWSMatchers, cfg.Discovery.AWSMatchers)
			require.Equal(t, testCase.expectedAzureMatchers, cfg.Discovery.AzureMatchers)
			require.Equal(t, testCase.expectedGCPMatchers, cfg.Discovery.GCPMatchers)
		})
	}
}
