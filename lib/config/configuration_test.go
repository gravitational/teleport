/*
Copyright 2015-2021 Gravitational, Inc.

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

	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth/keystore"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/lite"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/fixtures"
	"github.com/gravitational/teleport/lib/limiter"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
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
			require.Equal(t, fc.Global.DataDir, defaults.DataDir)
			require.Equal(t, fc.Logger.Severity, "INFO")
			require.Equal(t, testCase.expectClusterName, fc.Auth.ClusterName)
			require.Equal(t, testCase.expectLicenseFile, fc.Auth.LicenseFile)
			require.Equal(t, testCase.expectProxyWebAddr, fc.Proxy.WebAddr)
			require.ElementsMatch(t, testCase.expectProxyPublicAddrs, fc.Proxy.PublicAddr)
			require.ElementsMatch(t, testCase.expectProxyKeyPairs, fc.Proxy.KeyPairs)

			require.False(t, lib.IsInsecureDevMode())
		})
	}
}

// TestBooleanParsing tests that boolean options
// are parsed properly
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
auth_service:
  enabled: yes
  disconnect_expired_cert: %v
`, tc.s))))
		require.NoError(t, err, msg)
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

	// static config
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
		},
		Kube: Kube{
			Service: Service{
				EnabledFlag:   "yes",
				ListenAddress: "tcp://kube",
			},
			KubeClusterName: "kube-cluster",
			PublicAddr:      apiutils.Strings([]string{"kube-host:1234"}),
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
						"*": {"*"},
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
				},
				{
					Types:   []string{"rds"},
					Regions: []string{"us-central-1"},
					Tags: map[string]apiutils.Strings{
						"c": {"d"},
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
		WindowsDesktop: WindowsDesktopService{
			Service: Service{
				EnabledFlag:   "yes",
				ListenAddress: "tcp://windows_desktop",
			},
			PublicAddr: apiutils.Strings([]string{"winsrv.example.com:3028", "no-port.winsrv.example.com"}),
			Hosts:      apiutils.Strings([]string{"win.example.com:3389", "no-port.win.example.com"}),
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
	var conf service.SSHConfig
	var err error
	// empty spec. no errors, no labels
	err = parseLabelsApply("", &conf)
	require.Nil(t, err)
	require.Nil(t, conf.CmdLabels)
	require.Nil(t, conf.Labels)

	// simple static labels
	err = parseLabelsApply(`key=value,more="much better"`, &conf)
	require.NoError(t, err)
	require.NotNil(t, conf.CmdLabels)
	require.Len(t, conf.CmdLabels, 0)
	require.Equal(t, map[string]string{
		"key":  "value",
		"more": "much better",
	}, conf.Labels)

	// static labels + command labels
	err = parseLabelsApply(`key=value,more="much better",arch=[5m2s:/bin/uname -m "p1 p2"]`, &conf)
	require.Nil(t, err)
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

func TestTrustedClusters(t *testing.T) {
	err := readTrustedClusters(nil, nil)
	require.NoError(t, err)

	var conf service.Config
	err = readTrustedClusters([]TrustedCluster{
		{
			AllowedLogins: "vagrant, root",
			KeyFile:       "../../fixtures/trusted_clusters/cluster-a",
			TunnelAddr:    "one,two",
		},
	}, &conf)
	require.NoError(t, err)
	authorities := conf.Auth.Authorities
	require.Len(t, authorities, 2)
	require.Equal(t, "cluster-a", authorities[0].GetClusterName())
	require.Equal(t, types.HostCA, authorities[0].GetType())
	require.Len(t, authorities[0].GetActiveKeys().SSH, 1)
	require.Equal(t, "cluster-a", authorities[1].GetClusterName())
	require.Equal(t, types.UserCA, authorities[1].GetType())
	require.Len(t, authorities[1].GetActiveKeys().SSH, 1)
	_, _, _, _, err = ssh.ParseAuthorizedKey(authorities[1].GetActiveKeys().SSH[0].PublicKey)
	require.NoError(t, err)

	tunnels := conf.ReverseTunnels
	require.Len(t, tunnels, 1)
	require.Equal(t, "cluster-a", tunnels[0].GetClusterName())
	require.Len(t, tunnels[0].GetDialAddrs(), 2)
	require.Equal(t, "tcp://one:3024", tunnels[0].GetDialAddrs()[0])
	require.Equal(t, "tcp://two:3024", tunnels[0].GetDialAddrs()[1])

	// invalid data:
	err = readTrustedClusters([]TrustedCluster{
		{
			AllowedLogins: "vagrant, root",
			KeyFile:       "non-existing",
			TunnelAddr:    "one,two",
		},
	}, &conf)
	require.Error(t, err)
	require.Contains(t, err.Error(), "reading trusted cluster keys")
	err = readTrustedClusters([]TrustedCluster{
		{
			KeyFile:    "../../fixtures/trusted_clusters/cluster-a",
			TunnelAddr: "one,two",
		},
	}, &conf)
	require.Error(t, err)
	require.Contains(t, err.Error(), "needs allow_logins parameter")
	conf.ReverseTunnels = nil
	err = readTrustedClusters([]TrustedCluster{
		{
			KeyFile:       "../../fixtures/trusted_clusters/cluster-a",
			AllowedLogins: "vagrant",
			TunnelAddr:    "",
		},
	}, &conf)
	require.NoError(t, err)
	require.Len(t, conf.ReverseTunnels, 0)
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

	conf, err := ReadConfig(bytes.NewBufferString(fmt.Sprintf(
		SmallConfigString,
		authTokenPath,
		caPinPath,
		staticTokenPath,
		pkcs11LibPath,
	)))
	require.NoError(t, err)
	require.NotNil(t, conf)
	require.Equal(t, apiutils.Strings{"web3:443"}, conf.Proxy.PublicAddr)

	cfg := service.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.NoError(t, err)

	require.Equal(t, "join-token", cfg.Token)
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
	require.Equal(t, "tcp://peerhost:1234", cfg.Proxy.PeerAddr.FullAddress())

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
			SecondFactor: constants.SecondFactorOTP,
			U2F: &types.U2F{
				AppID: "app-id",
				DeviceAttestationCAs: []string{
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
			AllowPasswordless:     types.NewBoolOption(false),
		},
	}))

	require.Equal(t, pkcs11LibPath, cfg.Auth.KeyStore.Path)
	require.Equal(t, "example_token", cfg.Auth.KeyStore.TokenLabel)
	require.Equal(t, 1, *cfg.Auth.KeyStore.SlotNumber)
	require.Equal(t, "example_pin", cfg.Auth.KeyStore.Pin)
	require.ElementsMatch(t, []string{"ca-pin-from-string", "ca-pin-from-file1", "ca-pin-from-file2"}, cfg.CAPins)
}

// TestApplyConfigNoneEnabled makes sure that if a section is not enabled,
// it's fields are not read in.
func TestApplyConfigNoneEnabled(t *testing.T) {
	conf, err := ReadConfig(bytes.NewBufferString(NoServicesConfigString))
	require.NoError(t, err)

	cfg := service.MakeDefaultConfig()
	err = ApplyFileConfig(conf, cfg)
	require.NoError(t, err)

	require.False(t, cfg.Auth.Enabled)
	require.Empty(t, cfg.Auth.PublicAddrs)
	require.False(t, cfg.Proxy.Enabled)
	require.Empty(t, cfg.Proxy.PublicAddrs)
	require.False(t, cfg.SSH.Enabled)
	require.Empty(t, cfg.SSH.PublicAddrs)
	require.False(t, cfg.Apps.Enabled)
	require.False(t, cfg.Databases.Enabled)
	require.False(t, cfg.Metrics.Enabled)
	require.False(t, cfg.WindowsDesktop.Enabled)
	require.Empty(t, cfg.Proxy.PostgresPublicAddrs)
	require.Empty(t, cfg.Proxy.MySQLPublicAddrs)
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
			cfg := service.MakeDefaultConfig()
			err := applyProxyConfig(test.fc, cfg)
			require.NoError(t, err)
			require.EqualValues(t, test.out, utils.NetAddrsToStrings(cfg.Proxy.PostgresPublicAddrs))
		})
	}
}

func TestBackendDefaults(t *testing.T) {
	read := func(val string) *service.Config {
		// Default value is lite backend.
		conf, err := ReadConfig(bytes.NewBufferString(val))
		require.NoError(t, err)
		require.NotNil(t, conf)

		cfg := service.MakeDefaultConfig()
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
	require.Equal(t, cfg.Auth.StorageConfig.Type, lite.GetName())
	require.Equal(t, cfg.Auth.StorageConfig.Params[defaults.BackendPath], "/var/lib/teleport/mybackend")

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

			cfg := service.MakeDefaultConfig()
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

// TestParseKey ensures that keys are parsed correctly if they are in
// authorized_keys format or known_hosts format.
func TestParseKey(t *testing.T) {
	tests := []struct {
		inCABytes      []byte
		outType        types.CertAuthType
		outClusterName string
	}{
		// 0 - host ca in known_hosts format
		{
			[]byte(`@cert-authority *.foo ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCz+PzY6z2Xa1cMeiJqOH5BRpwY+PlS3Q6C4e3Yj8xjLW1zD3Cehm71zjsYmrpuFTmdylbcKB6CcM6Ft4YbKLG3PTLSKvCPTgfSBk8RCYX02PtOV5ixwa7xl5Gfhc1GRIheXgFO9IT+W9w9ube9r002AGpkMnRRtWAWiZHMGeJoaUoCsjDLDbWsQHj06pr7fD98c7PVcVzCKPTQpadXEP6sF8w417DvypHY1bYsvhRqHw9Njx6T3b9BM3bJ4QXgy18XuO5fCpLjKLsngLwSbqe/1IP4Q0zlUaNOTph3WnjeKJZO9yQeVX1cWDwY4Iz5lSHhsJnQD99hBDdw2RklHU0j type=host`),
			types.HostCA,
			"foo",
		},
		// 1 - user ca in known_hosts format (legacy)
		{
			[]byte(`@cert-authority *.bar ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCfhrvzbHAHrukeDhLSzoXtpctiumao1MQElwhOeuzFRYwrGV/1L2gsx4OJk4ztXKOCpon1FB+dy2aJN0WIr/9qXg37D6K/XJhgDaSfW8cjpl72Lw8kknDpmgSSA3cTvzFNmXfw4DNT/klRwEw6MMrDmfT9QvaV2d35lSoMMeTZ1ilFeJqXdUkY+bgijLBQU5MUjZUfQfS3jpSxVD0DD9D1VbAE1nGSNyFqf34JxJmqJ3R5hfZqNfb9CWouv+uFF99tzOr7tnKM/sQMPGmJ5G+zjTaErNSSLiIU1iCwVKUpNFcGiR1lpOEET+neJVnEeqEqKv2ookkXaIdKjk1UKZEn type=user`),
			types.UserCA,
			"bar",
		},
		// 2 - user ca in authorized_keys format
		{
			[]byte(`cert-authority ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCiIxyz0ctsyQbKLpWVYNF+ZIOrF150Wma2GqkrOWZaOzu5NSnt9Hmp7DaIa2Gn8fh8+8vjP02qp3i43SDOlLyYSn05nJjEXaz7QGysgeppN8ayojl5dkOhA00ROpCl5HhS9cmga7fy1Uwy4jhxenNpfQ5ap0COQi3UrXPepaq8z+I4XQK//qFWnkgyD1VXCnRKXXiajOf3dShYJqLCgwYiViuFmzi2p3lysoYS5eRwTCKiyyBtlkUtpTAse455yGf3QCpe+UOBiJ/4AElxacDndtMkjjctHSPCiztnph1xej64vSy8C2nGsnPIK7RfiOzSEdd5hwva+wPLgNTcKXZz type=user&clustername=baz`),
			types.UserCA,
			"baz",
		},
	}

	// run tests
	for i, tt := range tests {
		comment := fmt.Sprintf("Test %v", i)

		ca, _, err := parseCAKey(tt.inCABytes, []string{"foo"})
		require.NoError(t, err, comment)
		require.Equal(t, ca.GetType(), tt.outType)
		require.Equal(t, ca.GetClusterName(), tt.outClusterName)
	}
}

func TestParseCachePolicy(t *testing.T) {
	tcs := []struct {
		in  *CachePolicy
		out *service.CachePolicy
		err error
	}{
		{in: &CachePolicy{EnabledFlag: "yes", TTL: "never"}, out: &service.CachePolicy{Enabled: true}},
		{in: &CachePolicy{EnabledFlag: "true", TTL: "10h"}, out: &service.CachePolicy{Enabled: true}},
		{in: &CachePolicy{Type: "whatever", EnabledFlag: "false", TTL: "10h"}, out: &service.CachePolicy{Enabled: false}},
		{in: &CachePolicy{Type: "name", EnabledFlag: "yes", TTL: "never"}, out: &service.CachePolicy{Enabled: true}},
		{in: &CachePolicy{EnabledFlag: "no"}, out: &service.CachePolicy{Enabled: false}},
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
	require.Equal(t, conf.AuthToken, "xxxyyy")
	require.Equal(t, conf.AdvertiseIP, "10.10.10.1:3022")
	require.Equal(t, conf.PIDFile, "/var/run/teleport.pid")

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

	// ssh service:
	conf.SSH.EnabledFlag = "true"
	conf.SSH.ListenAddress = "tcp://ssh"
	conf.SSH.Labels = Labels
	conf.SSH.Commands = CommandLabels

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

	// kubernetes service:
	conf.Kube = Kube{
		Service: Service{
			EnabledFlag:   "yes",
			ListenAddress: "tcp://kube",
		},
		KubeClusterName: "kube-cluster",
		PublicAddr:      apiutils.Strings([]string{"kube-host:1234"}),
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
			Labels: map[string]apiutils.Strings{"*": {"*"}},
		},
	}
	conf.Databases.AWSMatchers = []AWSMatcher{
		{
			Types:   []string{"rds"},
			Regions: []string{"us-west-1", "us-east-1"},
			Tags:    map[string]apiutils.Strings{"a": {"b"}},
		},
		{
			Types:   []string{"rds"},
			Regions: []string{"us-central-1"},
			Tags:    map[string]apiutils.Strings{"c": {"d"}},
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

	// Windows Desktop Service
	conf.WindowsDesktop = WindowsDesktopService{
		Service: Service{
			EnabledFlag:   "yes",
			ListenAddress: "tcp://windows_desktop",
		},
		PublicAddr: apiutils.Strings([]string{"winsrv.example.com:3028", "no-port.winsrv.example.com"}),
		Hosts:      apiutils.Strings([]string{"win.example.com:3389", "no-port.win.example.com"}),
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
		cfg := service.MakeDefaultConfig()

		err := Configure(&clf, cfg)
		require.NoError(t, err, comment)
		require.Equal(t, tt.outPermitUserEnvironment, cfg.SSH.PermitUserEnvironment, comment)
	}
}

// TestDebugFlag ensures that the debug command-line flag is correctly set in the config.
func TestDebugFlag(t *testing.T) {
	clf := CommandLineFlags{
		Debug: true,
	}
	cfg := service.MakeDefaultConfig()
	require.False(t, cfg.Debug)
	err := Configure(&clf, cfg)
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
			cfg := service.MakeDefaultConfig()
			require.Empty(t, cfg.CAPins)
			err := Configure(&clf, cfg)
			require.NoError(t, err)
			require.ElementsMatch(t, tt.want, cfg.CAPins)
		})
	}
}

func TestLicenseFile(t *testing.T) {
	testCases := []struct {
		path   string
		result string
	}{
		// 0 - no license
		{
			path:   "",
			result: filepath.Join(defaults.DataDir, defaults.LicenseFile),
		},
		// 1 - relative path
		{
			path:   "lic.pem",
			result: filepath.Join(defaults.DataDir, "lic.pem"),
		},
		// 2 - absolute path
		{
			path:   "/etc/teleport/license",
			result: "/etc/teleport/license",
		},
	}

	cfg := service.MakeDefaultConfig()
	require.Equal(t, filepath.Join(defaults.DataDir, defaults.LicenseFile), cfg.Auth.LicenseFile)

	for _, tc := range testCases {
		fc := new(FileConfig)
		require.NoError(t, fc.CheckAndSetDefaults())
		fc.Auth.LicenseFile = tc.path
		err := ApplyFileConfig(fc, cfg)
		require.NoError(t, err)
		require.Equal(t, tc.result, cfg.Auth.LicenseFile)
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

		cfg := service.MakeDefaultConfig()
		service.ApplyDefaults(cfg)
		service.ApplyFIPSDefaults(cfg)

		err := Configure(&clf, cfg)
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
		want     service.KubeProxyConfig
		checkErr require.ErrorAssertionFunc
	}{
		{
			desc: "not configured",
			cfg:  Proxy{},
			want: service.KubeProxyConfig{
				Enabled: false,
			},
			checkErr: require.NoError,
		},
		{
			desc: "legacy format, no local cluster",
			cfg: Proxy{Kube: KubeProxy{
				Service: Service{EnabledFlag: "yes", ListenAddress: "0.0.0.0:8080"},
			}},
			want: service.KubeProxyConfig{
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
				PublicAddr:     apiutils.Strings([]string{"kube.example.com:443"}),
			}},
			want: service.KubeProxyConfig{
				Enabled:         true,
				ListenAddr:      *utils.MustParseAddr("0.0.0.0:8080"),
				KubeconfigPath:  "/tmp/kubeconfig",
				PublicAddrs:     []utils.NetAddr{*utils.MustParseAddr("kube.example.com:443")},
				LegacyKubeProxy: true,
			},
			checkErr: require.NoError,
		},
		{
			desc: "new format",
			cfg:  Proxy{KubeAddr: "0.0.0.0:8080"},
			want: service.KubeProxyConfig{
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
					PublicAddr:     apiutils.Strings([]string{"kube.example.com:443"}),
				},
			},
			want: service.KubeProxyConfig{
				Enabled:    true,
				ListenAddr: *utils.MustParseAddr("0.0.0.0:8080"),
			},
			checkErr: require.NoError,
		},
		{
			desc:    "v2 kube service should be enabled by default",
			version: defaults.TeleportConfigVersionV2,
			cfg:     Proxy{},
			want: service.KubeProxyConfig{
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
			cfg := &service.Config{
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
		want     service.ProxyConfig
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
			want: service.ProxyConfig{
				WebAddr:             *utils.MustParseAddr("0.0.0.0:3080"),
				Enabled:             true,
				EnableProxyProtocol: true,
				Kube: service.KubeProxyConfig{
					Enabled: true,
				},
				Limiter: limiter.Config{
					MaxConnections:   defaults.LimiterMaxConnections,
					MaxNumberOfUsers: 250,
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
			want: service.ProxyConfig{
				Enabled:             true,
				EnableProxyProtocol: true,
				WebAddr:             *utils.MustParseAddr("0.0.0.0:9999"),
				Kube: service.KubeProxyConfig{
					Enabled: true,
				},
				Limiter: limiter.Config{
					MaxConnections:   defaults.LimiterMaxConnections,
					MaxNumberOfUsers: 250,
				},
			},
			checkErr: require.NoError,
		},
		{
			desc: "v1 config should generate default listener addresses",
			fc: FileConfig{
				Version: defaults.TeleportConfigVersionV1,
				Proxy: Proxy{
					Service: Service{
						defaultEnabled: true,
					},
				},
			},
			want: service.ProxyConfig{
				Enabled:             true,
				EnableProxyProtocol: true,
				Limiter: limiter.Config{
					MaxConnections:   defaults.LimiterMaxConnections,
					MaxNumberOfUsers: 250,
				},
				WebAddr: *defaults.ProxyWebListenAddr(),
				Kube: service.KubeProxyConfig{
					Enabled: false,
				},
				ReverseTunnelListenAddr: *defaults.ReverseTunnelListenAddr(),
				SSHAddr:                 *defaults.ProxyListenAddr(),
			},
			checkErr: require.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			cfg := service.MakeDefaultConfig()
			err := ApplyFileConfig(&tt.fc, cfg)
			tt.checkErr(t, err)
			require.Empty(t, cmp.Diff(cfg.Proxy, tt.want, cmpopts.EquateEmpty()))
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
				fc.WindowsDesktop.Hosts = []string{"badscheme://foo:1:2"}
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
			desc:        "OK - valid config",
			expectError: require.NoError,
			mutate: func(fc *FileConfig) {
				fc.WindowsDesktop.EnabledFlag = "yes"
				fc.WindowsDesktop.ListenAddress = "0.0.0.0:3028"
				fc.WindowsDesktop.Hosts = []string{"127.0.0.1:3389"}
				fc.WindowsDesktop.HostLabels = []WindowsHostLabelRule{
					{Match: ".*", Labels: map[string]string{"key": "value"}},
				}
			},
		},
	} {
		t.Run(test.desc, func(t *testing.T) {
			fc := &FileConfig{}
			test.mutate(fc)
			cfg := &service.Config{}
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
	}

	for _, tt := range tests {
		clf := CommandLineFlags{
			ConfigString: base64.StdEncoding.EncodeToString([]byte(tt.inConfigString)),
		}
		cfg := service.MakeDefaultConfig()

		err := Configure(&clf, cfg)
		require.Equal(t, err != nil, tt.outError, tt.inComment)
	}
}

// TestAppsCLF checks that validation runs on application configuration passed
// in on the command line.
func TestAppsCLF(t *testing.T) {
	tests := []struct {
		desc      string
		inRoles   string
		inAppName string
		inAppURI  string
		outError  error
	}{
		{
			desc:      "role provided, valid name and uri",
			inRoles:   defaults.RoleApp,
			inAppName: "foo",
			inAppURI:  "http://localhost:8080",
			outError:  nil,
		},
		{
			desc:      "role provided, name not provided",
			inRoles:   defaults.RoleApp,
			inAppName: "",
			inAppURI:  "http://localhost:8080",
			outError:  trace.BadParameter(""),
		},
		{
			desc:      "role provided, uri not provided",
			inRoles:   defaults.RoleApp,
			inAppName: "foo",
			inAppURI:  "",
			outError:  trace.BadParameter(""),
		},
		{
			desc:      "valid name and uri",
			inAppName: "foo",
			inAppURI:  "http://localhost:8080",
			outError:  nil,
		},
		{
			desc:      "invalid name",
			inAppName: "-foo",
			inAppURI:  "http://localhost:8080",
			outError:  trace.BadParameter(""),
		},
		{
			desc:      "missing uri",
			inAppName: "foo",
			outError:  trace.BadParameter(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			clf := CommandLineFlags{
				Roles:   tt.inRoles,
				AppName: tt.inAppName,
				AppURI:  tt.inAppURI,
			}
			cfg := service.MakeDefaultConfig()
			err := Configure(&clf, cfg)
			if err != nil {
				require.IsType(t, err, tt.outError)
			} else {
				require.NoError(t, err)
			}
			if tt.outError != nil {
				return
			}
			require.True(t, cfg.Apps.Enabled)
			require.Len(t, cfg.Apps.Apps, 1)
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
			outError: `invalid database "foo" address`,
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
			err := Configure(&clf, service.MakeDefaultConfig())
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
		outDatabase service.Database
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
			outDatabase: service.Database{
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
				TLS: service.DatabaseTLS{
					Mode: service.VerifyFull,
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
			outError: `invalid database "foo" address`,
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
				DatabaseName:      "rds",
				DatabaseProtocol:  defaults.ProtocolMySQL,
				DatabaseURI:       "localhost:3306",
				DatabaseAWSRegion: "us-east-1",
			},
			outDatabase: service.Database{
				Name:     "rds",
				Protocol: defaults.ProtocolMySQL,
				URI:      "localhost:3306",
				AWS: service.DatabaseAWS{
					Region: "us-east-1",
				},
				StaticLabels: map[string]string{
					types.OriginLabel: types.OriginConfigFile,
				},
				DynamicLabels: services.CommandLabels{},
				TLS: service.DatabaseTLS{
					Mode: service.VerifyFull,
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
			},
			outDatabase: service.Database{
				Name:     "redshift",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				AWS: service.DatabaseAWS{
					Region: "us-east-1",
					Redshift: service.DatabaseAWSRedshift{
						ClusterID: "redshift-cluster-1",
					},
				},
				StaticLabels: map[string]string{
					types.OriginLabel: types.OriginConfigFile,
				},
				DynamicLabels: services.CommandLabels{},
				TLS: service.DatabaseTLS{
					Mode: service.VerifyFull,
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
			outDatabase: service.Database{
				Name:     "gcp",
				Protocol: defaults.ProtocolPostgres,
				URI:      "localhost:5432",
				TLS: service.DatabaseTLS{
					Mode:   service.VerifyFull,
					CACert: fixtures.LocalhostCert,
				},
				GCP: service.DatabaseGCP{
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
				DatabaseURI:          "localhost:1433",
				DatabaseADKeytabFile: "/etc/keytab",
				DatabaseADDomain:     "EXAMPLE.COM",
				DatabaseADSPN:        "MSSQLSvc/sqlserver.example.com:1433",
			},
			outDatabase: service.Database{
				Name:     "sqlserver",
				Protocol: defaults.ProtocolSQLServer,
				URI:      "localhost:1433",
				TLS: service.DatabaseTLS{
					Mode: service.VerifyFull,
				},
				AD: service.DatabaseAD{
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
			outDatabase: service.Database{
				Name:     "mysql-foo",
				Protocol: defaults.ProtocolMySQL,
				URI:      "localhost:3306",
				MySQL: service.MySQLOptions{
					ServerVersion: "8.0.28",
				},
				TLS: service.DatabaseTLS{
					Mode: service.VerifyFull,
				},
				StaticLabels: map[string]string{
					types.OriginLabel: types.OriginConfigFile,
				},
				DynamicLabels: services.CommandLabels{},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.desc, func(t *testing.T) {
			t.Parallel()

			config := service.MakeDefaultConfig()
			err := Configure(&tt.inFlags, config)
			if tt.outError != "" {
				require.Contains(t, err.Error(), tt.outError)
			} else {
				require.NoError(t, err)
				require.Equal(t,
					config.Databases.Databases,
					[]service.Database{tt.outDatabase},
				)
			}
		})
	}
}

func TestTextFormatter(t *testing.T) {
	tests := []struct {
		comment      string
		formatConfig []string
		assertErr    require.ErrorAssertionFunc
	}{
		{
			comment:      "invalid key (does not exist)",
			formatConfig: []string{"level", "invalid key"},
			assertErr:    require.Error,
		},
		{
			comment:      "valid keys and formatting",
			formatConfig: []string{"level", "component", "timestamp"},
			assertErr:    require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			formatter := &utils.TextFormatter{
				ExtraFields: tt.formatConfig,
			}
			tt.assertErr(t, formatter.CheckAndSetDefaults())
		})
	}
}

func TestJSONFormatter(t *testing.T) {
	tests := []struct {
		comment     string
		extraFields []string
		assertErr   require.ErrorAssertionFunc
	}{
		{
			comment:     "invalid key (does not exist)",
			extraFields: []string{"level", "invalid key"},
			assertErr:   require.Error,
		},
		{
			comment:     "valid keys and formatting",
			extraFields: []string{"level", "caller", "component", "timestamp"},
			assertErr:   require.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.comment, func(t *testing.T) {
			formatter := &utils.JSONFormatter{
				ExtraFields: tt.extraFields,
			}
			tt.assertErr(t, formatter.CheckAndSetDefaults())
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
			cfg := service.MakeDefaultConfig()

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
			want: service.MakeDefaultConfig().Auth.KeyStore,
		},
		{
			name: "correct config",
			auth: Auth{
				CAKeyParams: &CAKeyParams{
					PKCS11: PKCS11{
						ModulePath: securePKCS11LibPath,
						TokenLabel: "foo",
						SlotNumber: &slotNumber,
						Pin:        "pin",
					},
				},
			},
			want: keystore.Config{
				TokenLabel: "foo",
				SlotNumber: &slotNumber,
				Pin:        "pin",
				Path:       securePKCS11LibPath,
			},
		},
		{
			name: "correct config with pin file",
			auth: Auth{
				CAKeyParams: &CAKeyParams{
					PKCS11: PKCS11{
						ModulePath: securePKCS11LibPath,
						TokenLabel: "foo",
						SlotNumber: &slotNumber,
						PinPath:    securePinFilePath,
					},
				},
			},
			want: keystore.Config{
				TokenLabel: "foo",
				SlotNumber: &slotNumber,
				Pin:        "secure-pin-file",
				Path:       securePKCS11LibPath,
			},
		},
		{
			name: "err when pin and pin path configured",
			auth: Auth{
				CAKeyParams: &CAKeyParams{
					PKCS11: PKCS11{
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
					PKCS11: PKCS11{
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
					PKCS11: PKCS11{
						PinPath: worldReadablePinFilePath,
					},
				},
			},
			errMessage: fmt.Sprintf(
				"HSM pin file (%s) must not be world-readable",
				worldReadablePinFilePath,
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := service.MakeDefaultConfig()

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

			cfg := service.MakeDefaultConfig()
			err = ApplyFileConfig(conf, cfg)
			require.NoError(t, err)

			require.Equal(t, tt.outOrigin, cfg.Auth.SessionRecordingConfig.Origin())
			require.Equal(t, tt.outSessionRecording, cfg.Auth.SessionRecordingConfig.GetMode())
			require.Equal(t, tt.outProxyChecksHostKeys, cfg.Auth.SessionRecordingConfig.GetProxyChecksHostKeys())
		})
	}
}
