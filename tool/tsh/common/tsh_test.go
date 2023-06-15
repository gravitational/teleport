/*
Copyright 2015-2017 Gravitational, Inc.

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

package common

import (
	"bufio"
	"bytes"
	"context"
	"crypto"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otlp "go.opentelemetry.io/proto/otlp/trace/v1"
	"golang.org/x/exp/slices"
	yamlv2 "gopkg.in/yaml.v2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/integration/kube"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/auth/native"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/prompt"
	"github.com/gravitational/teleport/tool/common"
)

const (
	staticToken = "test-static-token"
	// tshBinMainTestEnv allows to execute tsh main function from test binary.
	tshBinMainTestEnv = "TSH_BIN_MAIN_TEST"
)

var ports utils.PortList

func init() {
	// Allows test to refer to tsh binary in tests.
	// Needed for tests that generate OpenSSH config by tsh config command where
	// tsh proxy ssh command is used as ProxyCommand.
	if os.Getenv(tshBinMainTestEnv) != "" {
		Main()
		// main will only exit if there is an error.
		// since we are here, there was no error, so we must do so ourselves.
		os.Exit(0)
		return
	}

	// If the test is re-executing itself, execute the command that comes over
	// the pipe. Used to test tsh ssh command.
	if srv.IsReexec() {
		srv.RunAndExit(os.Args[1])
		return
	}

	var err error
	ports, err = utils.GetFreeTCPPorts(5000, utils.PortStartingNumber)
	if err != nil {
		panic(fmt.Sprintf("failed to allocate tcp ports for tests: %v", err))
	}

	modules.SetModules(&cliModules{})
}

func TestMain(m *testing.M) {
	utils.InitLoggerForTests()
	native.PrecomputeTestKeys(m)
	os.Exit(m.Run())
}

type cliModules struct{}

// BuildType returns build type (OSS or Enterprise)
func (p *cliModules) BuildType() string {
	return "CLI"
}

// PrintVersion prints the Teleport version.
func (p *cliModules) PrintVersion() {
	fmt.Printf("Teleport CLI\n")
}

// Features returns supported features
func (p *cliModules) Features() modules.Features {
	return modules.Features{
		Kubernetes:              true,
		DB:                      true,
		App:                     true,
		AdvancedAccessWorkflows: true,
		AccessControls:          true,
	}
}

// IsBoringBinary checks if the binary was compiled with BoringCrypto.
func (p *cliModules) IsBoringBinary() bool {
	return false
}

// AttestHardwareKey attests a hardware key.
func (p *cliModules) AttestHardwareKey(_ context.Context, _ interface{}, _ keys.PrivateKeyPolicy, _ *keys.AttestationStatement, _ crypto.PublicKey, _ time.Duration) (keys.PrivateKeyPolicy, error) {
	return keys.PrivateKeyPolicyNone, nil
}

func (p *cliModules) EnableRecoveryCodes() {
}

func (p *cliModules) EnablePlugins() {
}

func (p *cliModules) SetFeatures(f modules.Features) {

}

func TestAlias(t *testing.T) {
	testExecutable, err := os.Executable()
	require.NoError(t, err)

	tests := []struct {
		name           string
		aliases        map[string]string
		args           []string
		wantErr        bool
		validateOutput func(t *testing.T, output string)
	}{
		{
			name: "loop",
			aliases: map[string]string{
				"loop": fmt.Sprintf("%v loop", testExecutable),
			},
			args:    []string{"loop"},
			wantErr: true,
			validateOutput: func(t *testing.T, output string) {
				require.Contains(t, output, "recursive alias \"loop\"; correct alias definition and try again")
			},
		},
		{
			name: "loop via other",
			aliases: map[string]string{
				"loop":      fmt.Sprintf("%v loop", testExecutable),
				"loop-call": fmt.Sprintf("%v loop", testExecutable),
			},
			args:    []string{"loop-call"},
			wantErr: true,
			validateOutput: func(t *testing.T, output string) {
				require.Contains(t, output, "recursive alias \"loop\"; correct alias definition and try again")
			},
		},
		{
			name: "r1 -> r2 -> r1",
			aliases: map[string]string{
				"r1": fmt.Sprintf("%v r2", testExecutable),
				"r2": fmt.Sprintf("%v r1", testExecutable),
			},
			args:    []string{"r2"},
			wantErr: true,
			validateOutput: func(t *testing.T, output string) {
				require.Contains(t, output, "recursive alias \"r2\"; correct alias definition and try again")
			},
		},
		{
			name: "set default flag to command",
			aliases: map[string]string{
				"version": fmt.Sprintf("%v version --format=json", testExecutable),
			},
			args:    []string{"version"},
			wantErr: false,
			validateOutput: func(t *testing.T, output string) {
				require.Contains(t, output, `"version"`)
				require.Contains(t, output, `"gitref"`)
				require.Contains(t, output, `"runtime"`)
			},
		},
		{
			name: "default flag and alias",
			aliases: map[string]string{
				"version": fmt.Sprintf("%v version --format=json", testExecutable),
				"v":       fmt.Sprintf("%v version", testExecutable),
			},
			args:    []string{"v"},
			wantErr: false,
			validateOutput: func(t *testing.T, output string) {
				require.Contains(t, output, `"version"`)
				require.Contains(t, output, `"gitref"`)
				require.Contains(t, output, `"runtime"`)
			},
		},
		{
			name: "call external program, pass non-zero exit code",
			aliases: map[string]string{
				"ss": fmt.Sprintf("%v status", testExecutable),
				"bb": fmt.Sprintf("bash -c '%v ss'", testExecutable),
			},
			args:    []string{"bb"},
			wantErr: true,
			validateOutput: func(t *testing.T, output string) {
				require.Contains(t, output, fmt.Sprintf("%vNot logged in", utils.Color(utils.Red, "ERROR: ")))
				require.Contains(t, output, fmt.Sprintf("%vexit status 1", utils.Color(utils.Red, "ERROR: ")))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// make sure we have fresh configs for the tests:
			// - new home
			// - new global config
			tmpHomePath := t.TempDir()
			t.Setenv(types.HomeEnvVar, tmpHomePath)
			t.Setenv(globalTshConfigEnvVar, filepath.Join(tmpHomePath, "tsh_global.yaml"))

			// make the re-exec behave as `tsh` instead of test binary.
			t.Setenv(tshBinMainTestEnv, "1")

			// write config to use
			config := &TshConfig{Aliases: tt.aliases}
			configBytes, err := yamlv2.Marshal(config)
			require.NoError(t, err)
			err = os.WriteFile(filepath.Join(tmpHomePath, "tsh_global.yaml"), configBytes, 0o777)
			require.NoError(t, err)

			// run command
			cmd := exec.Command(testExecutable, tt.args...)
			t.Logf("running command %v", cmd)
			output, err := cmd.CombinedOutput()
			t.Logf("executable output: %v", string(output))

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}

			tt.validateOutput(t, string(output))
		})
	}
}

func TestFailedLogin(t *testing.T) {
	tmpHomePath := t.TempDir()

	connector := mockConnector(t)

	_, proxyProcess := makeTestServers(t, withBootstrap(connector))

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	// build a mock SSO login function to patch into tsh
	loginFailed := trace.AccessDenied("login failed")
	ssoLogin := func(ctx context.Context, connectorID string, priv *keys.PrivateKey, protocol string) (*auth.SSHLoginResponse, error) {
		return nil, loginFailed
	}

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
		cf.MockSSOLogin = ssoLogin
		return nil
	})
	require.ErrorIs(t, err, loginFailed)
}

func TestOIDCLogin(t *testing.T) {
	tmpHomePath := t.TempDir()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// set up an initial role with `request_access: always` in order to
	// trigger automatic post-login escalation.
	populist, err := types.NewRole("populist", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles: []string{"dictator"},
			},
		},
		Options: types.RoleOptions{
			RequestAccess: types.RequestStrategyAlways,
		},
	})
	require.NoError(t, err)

	// empty role which serves as our escalation target
	dictator, err := types.NewRole("dictator", types.RoleSpecV6{})
	require.NoError(t, err)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"populist"})

	connector := mockConnector(t)

	motd := "MESSAGE_OF_THE_DAY_OIDC"
	authProcess, proxyProcess := makeTestServers(t,
		withBootstrap(populist, dictator, connector, alice),
		withMOTD(t, motd),
	)

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	var didAutoRequest atomic.Bool

	errCh := make(chan error)
	go func() {
		watcher, err := authServer.NewWatcher(ctx, types.Watch{
			Kinds: []types.WatchKind{
				{Kind: types.KindAccessRequest},
			},
		})
		if err != nil {
			errCh <- err
			return
		}
		for {
			select {
			case event := <-watcher.Events():
				if event.Type != types.OpPut {
					continue
				}
				err = authServer.SetAccessRequestState(ctx, types.AccessRequestUpdate{
					RequestID: event.Resource.(types.AccessRequest).GetName(),
					State:     types.RequestState_APPROVED,
				})
				didAutoRequest.Store(true)
				errCh <- err
				return
			case <-watcher.Done():
				errCh <- nil
				return
			case <-ctx.Done():
				errCh <- nil
				return
			}
		}
	}()

	buf := bytes.NewBuffer([]byte{})
	sc := bufio.NewScanner(buf)
	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr.String(),
		"--user", "alice", // explicitly use wrong name
	}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
		cf.MockSSOLogin = mockSSOLogin(t, authServer, alice)
		cf.SiteName = "localhost"
		cf.overrideStderr = buf
		return nil
	})

	require.NoError(t, err)
	require.NoError(t, <-errCh)

	// verify that auto-request happened
	require.True(t, didAutoRequest.Load())

	findMOTD(t, sc, motd)
	// if we got this far, then tsh successfully registered name change from `alice` to
	// `alice@example.com`, since the correct name needed to be used for the access
	// request to be generated.
}

func findMOTD(t *testing.T, sc *bufio.Scanner, motd string) {
	t.Helper()
	for sc.Scan() {
		if strings.Contains(sc.Text(), motd) {
			return
		}
	}
	require.Fail(t, "Failed to find %q MOTD in the logs", motd)
}

// TestLoginIdentityOut makes sure that "tsh login --out <ident>" command
// writes identity credentials to the specified path. It also supports
// specifying the output format via `--format=<format>`.
func TestLoginIdentityOut(t *testing.T) {
	const kubeClusterName = "kubeTest"
	tmpHomePath := t.TempDir()

	connector := mockConnector(t)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, alice))
	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	cluster, err := types.NewKubernetesClusterV3(types.Metadata{
		Name:   kubeClusterName,
		Labels: map[string]string{},
	},
		types.KubernetesClusterSpecV3{},
	)
	require.NoError(t, err)

	kubeServer, err := types.NewKubernetesServerV3FromCluster(cluster, kubeClusterName, kubeClusterName)
	require.NoError(t, err)
	_, err = authServer.UpsertKubernetesServer(context.Background(), kubeServer)
	require.NoError(t, err)

	cases := []struct {
		name               string
		extraArgs          []string
		validationFunc     func(t *testing.T, identityPath string)
		requiresTLSRouting bool
	}{
		{
			name: "write identity out",
			validationFunc: func(t *testing.T, identityPath string) {
				_, err := identityfile.KeyFromIdentityFile(identityPath, "proxy.example.com", "")
				require.NoError(t, err)
			},
		},
		{
			name:      "write identity in kubeconfig format with tls routing enabled",
			extraArgs: []string{"--format", "kubernetes", "--kube-cluster", kubeClusterName},
			validationFunc: func(t *testing.T, identityPath string) {
				cfg, err := kubeconfig.Load(identityPath)
				require.NoError(t, err)
				kubeCtx := cfg.Contexts[cfg.CurrentContext]
				require.NotNil(t, kubeCtx)
				cluster := cfg.Clusters[kubeCtx.Cluster]
				require.NotNil(t, cluster)
				require.NotEmpty(t, cluster.TLSServerName)
			},
			requiresTLSRouting: true,
		},
		{
			name:      "write identity in kubeconfig format with tls routing disabled",
			extraArgs: []string{"--format", "kubernetes", "--kube-cluster", kubeClusterName},
			validationFunc: func(t *testing.T, identityPath string) {
				cfg, err := kubeconfig.Load(identityPath)
				require.NoError(t, err)
				kubeCtx := cfg.Contexts[cfg.CurrentContext]
				require.NotNil(t, kubeCtx)
				cluster := cfg.Clusters[kubeCtx.Cluster]
				require.NotNil(t, cluster)
				require.Empty(t, cluster.TLSServerName)
			},
		},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			identPath := filepath.Join(t.TempDir(), "ident")
			if tt.requiresTLSRouting {
				switchProxyListenerMode(t, authServer, types.ProxyListenerMode_Multiplex)
			}
			err = Run(context.Background(), append([]string{
				"login",
				"--insecure",
				"--debug",
				"--auth", connector.GetName(),
				"--proxy", proxyAddr.String(),
				"--out", identPath,
			}, tt.extraArgs...), setHomePath(tmpHomePath), func(cf *CLIConf) error {
				cf.MockSSOLogin = mockSSOLogin(t, authServer, alice)
				return nil
			})
			require.NoError(t, err)
			tt.validationFunc(t, identPath)
		})
	}
}

// switchProxyListenerMode switches the proxy listener mode to the specified mode
// and schedules a reversion to the previous value once the sub-test completes.
func switchProxyListenerMode(t *testing.T, authServer *auth.Server, mode types.ProxyListenerMode) {
	networkCfg, err := authServer.GetClusterNetworkingConfig(context.Background())
	require.NoError(t, err)
	prevValue := networkCfg.GetProxyListenerMode()
	networkCfg.SetProxyListenerMode(mode)
	err = authServer.SetClusterNetworkingConfig(context.Background(), networkCfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		networkCfg.SetProxyListenerMode(prevValue)
		err = authServer.SetClusterNetworkingConfig(context.Background(), networkCfg)
		require.NoError(t, err)
	})
}

func TestRelogin(t *testing.T) {
	t.Parallel()

	tmpHomePath := t.TempDir()

	connector := mockConnector(t)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	motd := "RELOGIN MOTD PRESENT"
	authProcess, proxyProcess := makeTestServers(t,
		withBootstrap(connector, alice),
		withMOTD(t, motd),
	)

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	buf := bytes.NewBuffer([]byte{})
	sc := bufio.NewScanner(buf)
	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
		cf.MockSSOLogin = mockSSOLogin(t, authServer, alice)
		cf.overrideStderr = buf
		return nil
	})
	require.NoError(t, err)
	findMOTD(t, sc, motd)

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr.String(),
		"localhost",
	}, setHomePath(tmpHomePath),
		func(cf *CLIConf) error {
			cf.MockSSOLogin = mockSSOLogin(t, authServer, alice)
			cf.overrideStderr = buf
			return nil
		})
	require.NoError(t, err)
	findMOTD(t, sc, motd)

	err = Run(context.Background(), []string{"logout"}, setHomePath(tmpHomePath),
		func(cf *CLIConf) error {
			cf.overrideStderr = buf
			return nil
		})
	require.NoError(t, err)

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr.String(),
		"localhost",
	}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
		cf.MockSSOLogin = mockSSOLogin(t, authServer, alice)
		cf.overrideStderr = buf
		return nil
	})
	findMOTD(t, sc, motd)
	require.NoError(t, err)
}

func TestSwitchingProxies(t *testing.T) {
	t.Parallel()

	tmpHomePath := t.TempDir()

	connector := mockConnector(t)
	// Connector need not be functional since we are going to mock the actual
	// login operation.

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	auth1, proxy1 := makeTestServers(t,
		withBootstrap(connector, alice),
	)

	auth2, proxy2 := makeTestServers(t,
		withBootstrap(connector, alice),
	)
	authServer1 := auth1.GetAuthServer()
	require.NotNil(t, authServer1)

	proxyAddr1, err := proxy1.ProxyWebAddr()
	require.NoError(t, err)

	authServer2 := auth2.GetAuthServer()
	require.NotNil(t, authServer2)

	proxyAddr2, err := proxy2.ProxyWebAddr()
	require.NoError(t, err)

	// perform initial login to both proxies

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr1.String(),
	}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
		cf.MockSSOLogin = mockSSOLogin(t, authServer1, alice)
		return nil
	})
	require.NoError(t, err)

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr2.String(),
	}, setHomePath(tmpHomePath),
		func(cf *CLIConf) error {
			cf.MockSSOLogin = mockSSOLogin(t, authServer2, alice)
			return nil
		})

	require.NoError(t, err)

	// login again while both proxies are still valid and ensure it is successful without an SSO login provided

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr1.String(),
	}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
		return nil
	})

	require.NoError(t, err)

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr2.String(),
	}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
		return nil
	})

	require.NoError(t, err)

	// logout

	err = Run(context.Background(), []string{"logout"}, setHomePath(tmpHomePath),
		func(cf *CLIConf) error {
			return nil
		})
	require.NoError(t, err)

	// after logging out, make sure that any attempt to log in without providing a valid login function fails

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr1.String(),
	}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
		return nil
	})

	require.Error(t, err)

	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr2.String(),
	}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
		return nil
	})

	require.Error(t, err)

	cancel()
}

func TestMakeClient(t *testing.T) {
	t.Parallel()

	var conf CLIConf
	conf.HomePath = t.TempDir()

	// empty config won't work:
	tc, err := makeClient(&conf)
	require.Nil(t, tc)
	require.Error(t, err)

	// minimal configuration (with defaults)
	conf.Proxy = "proxy:3080"
	conf.UserHost = "localhost"
	conf.HomePath = t.TempDir()

	// Create a empty profile so we don't ping proxy.
	clientStore, err := initClientStore(&conf, conf.Proxy)
	require.NoError(t, err)
	profile := &profile.Profile{
		SSHProxyAddr: "proxy:3023",
		WebProxyAddr: "proxy:3080",
	}
	err = clientStore.SaveProfile(profile, true)
	require.NoError(t, err)

	tc, err = makeClient(&conf)
	require.NoError(t, err)
	require.NotNil(t, tc)

	// profile info should be loaded into client
	require.Equal(t, "proxy:3023", tc.Config.SSHProxyAddr)
	require.Equal(t, "proxy:3080", tc.Config.WebProxyAddr)

	localUser, err := client.Username()
	require.NoError(t, err)

	require.Equal(t, localUser, tc.Config.HostLogin)
	require.Equal(t, apidefaults.CertDuration, tc.Config.KeyTTL)

	// specific configuration
	conf.MinsToLive = 5
	conf.UserHost = "root@localhost"
	conf.NodePort = 46528
	conf.LocalForwardPorts = []string{"80:remote:180"}
	conf.DynamicForwardedPorts = []string{":8080"}
	tc, err = makeClient(&conf)
	require.NoError(t, err)
	require.Equal(t, time.Minute*time.Duration(conf.MinsToLive), tc.Config.KeyTTL)
	require.Equal(t, "root", tc.Config.HostLogin)
	require.Equal(t, client.ForwardedPorts{
		{
			SrcIP:    "127.0.0.1",
			SrcPort:  80,
			DestHost: "remote",
			DestPort: 180,
		},
	}, tc.Config.LocalForwardPorts)
	require.Equal(t, client.DynamicForwardedPorts{
		{
			SrcIP:   "127.0.0.1",
			SrcPort: 8080,
		},
	}, tc.Config.DynamicForwardedPorts)

	// specific configuration with email like user
	conf.MinsToLive = 5
	conf.UserHost = "root@example.com@localhost"
	conf.NodePort = 46528
	conf.LocalForwardPorts = []string{"80:remote:180"}
	conf.DynamicForwardedPorts = []string{":8080"}
	conf.TshConfig.ExtraHeaders = []ExtraProxyHeaders{
		{Proxy: "proxy:3080", Headers: map[string]string{"A": "B"}},
		{Proxy: "*roxy:3080", Headers: map[string]string{"C": "D"}},
		{Proxy: "*hello:3080", Headers: map[string]string{"E": "F"}}, // shouldn't get included
	}
	tc, err = makeClient(&conf)
	require.NoError(t, err)
	require.Equal(t, time.Minute*time.Duration(conf.MinsToLive), tc.Config.KeyTTL)
	require.Equal(t, "root@example.com", tc.Config.HostLogin)
	require.Equal(t, client.ForwardedPorts{
		{
			SrcIP:    "127.0.0.1",
			SrcPort:  80,
			DestHost: "remote",
			DestPort: 180,
		},
	}, tc.Config.LocalForwardPorts)
	require.Equal(t, client.DynamicForwardedPorts{
		{
			SrcIP:   "127.0.0.1",
			SrcPort: 8080,
		},
	}, tc.Config.DynamicForwardedPorts)

	require.Equal(t,
		map[string]string{"A": "B", "C": "D"},
		tc.ExtraProxyHeaders)

	_, proxy := makeTestServers(t)

	proxyWebAddr, err := proxy.ProxyWebAddr()
	require.NoError(t, err)

	proxySSHAddr, err := proxy.ProxySSHAddr()
	require.NoError(t, err)

	// If profile is missing, makeClient should call Ping on the proxy to fetch SSHProxyAddr
	conf = CLIConf{
		Proxy:              proxyWebAddr.String(),
		Context:            context.Background(),
		InsecureSkipVerify: true,
	}
	tc, err = makeClient(&conf)
	require.NoError(t, err)
	require.NotNil(t, tc)
	require.Equal(t, proxyWebAddr.String(), tc.Config.WebProxyAddr)
	require.Equal(t, proxySSHAddr.Addr, tc.Config.SSHProxyAddr)
	require.NotNil(t, tc.LocalAgent().ExtendedAgent)

	// With provided identity file.
	//
	// makeClient should call Ping on the proxy to fetch SSHProxyAddr
	conf = CLIConf{
		Proxy:              proxyWebAddr.String(),
		IdentityFileIn:     "../../../fixtures/certs/identities/tls.pem",
		Context:            context.Background(),
		InsecureSkipVerify: true,
	}
	tc, err = makeClient(&conf)
	require.NoError(t, err)
	require.NotNil(t, tc)
	require.Equal(t, proxyWebAddr.String(), tc.Config.WebProxyAddr)
	require.Equal(t, proxySSHAddr.Addr, tc.Config.SSHProxyAddr)
	require.NotNil(t, tc.LocalAgent().ExtendedAgent)

	// Client should have an in-memory agent with keys loaded, in case agent
	// forwarding is required for proxy recording mode.
	agentKeys, err := tc.LocalAgent().ExtendedAgent.List()
	require.NoError(t, err)
	require.Greater(t, len(agentKeys), 0)
	require.Equal(t, keys.PrivateKeyPolicyNone, tc.PrivateKeyPolicy,
		"private key policy should be configured from the identity file temp profile")
}

// accessApprover allows watching and updating access requests
type accessApprover interface {
	types.Events
	SetAccessRequestState(ctx context.Context, params types.AccessRequestUpdate) error
}

// approveAllAccessRequests starts a loop which watches for pending AccessRequests
// and automatically approves them.
func approveAllAccessRequests(ctx context.Context, approver accessApprover) error {
	watcher, err := approver.NewWatcher(ctx, types.Watch{
		Name:  types.KindAccessRequest,
		Kinds: []types.WatchKind{{Kind: types.KindAccessRequest}},
	})
	if err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-watcher.Done():
			return watcher.Error()
		case evt := <-watcher.Events():
			if evt.Type != types.OpPut {
				continue
			}

			request, ok := evt.Resource.(types.AccessRequest)
			if !ok {
				return trace.BadParameter("unexpected event type received: %q", evt.Resource.GetKind())
			}

			if request.GetState() == types.RequestState_APPROVED {
				continue
			}

			if err := approver.SetAccessRequestState(ctx, types.AccessRequestUpdate{
				RequestID: request.GetName(),
				State:     types.RequestState_APPROVED,
			}); err != nil {
				return trace.Wrap(err)
			}
		}
	}
}

// TestSSHOnMultipleNodes validates that mfa is enforced when creating SSH
// sessions when set either via role or cluster auth preference.
// Sessions created via hostname and by matched labels are
// verified.
//
// NOTE: This test must NOT be run in parallel because it updates
// the global [client.PromptWebauthn] in multiple test cases.
func TestSSHOnMultipleNodes(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	isInsecure := lib.IsInsecureDevMode()
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() {
		lib.SetInsecureDevMode(isInsecure)
	})

	origin := func(cluster string) string {
		return fmt.Sprintf("https://%s", cluster)
	}
	connector := mockConnector(t)

	user, err := user.Current()
	require.NoError(t, err)

	noAccessRole, err := types.NewRole("no-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins: []string{user.Username},
		},
		Deny: types.RoleConditions{
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
	})
	require.NoError(t, err)

	sshLoginRole, err := types.NewRole("ssh-login", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{user.Username},
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
	})
	require.NoError(t, err)

	perSessionMFARole, err := types.NewRole("mfa-login", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{user.Username},
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
		Options: types.RoleOptions{RequireMFAType: types.RequireMFAType_SESSION},
	})
	require.NoError(t, err)

	alice, err := types.NewUser("alice")
	require.NoError(t, err)
	alice.SetRoles([]string{"access", "ssh-login"})
	const password = "supersecretpassword"

	device, err := mocku2f.Create()
	require.NoError(t, err)
	device.SetPasswordless()

	rootAuth, rootProxy := makeTestServers(t, withBootstrap(connector, alice, noAccessRole, sshLoginRole, perSessionMFARole))

	rootAuthAddr, err := rootAuth.AuthAddr()
	require.NoError(t, err)

	rootProxyAddr, err := rootProxy.ProxyWebAddr()
	require.NoError(t, err)
	rootTunnelAddr, err := rootProxy.ProxyTunnelAddr()
	require.NoError(t, err)

	trustedCluster, err := types.NewTrustedCluster("localhost", types.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{},
		Token:                staticToken,
		ProxyAddress:         rootProxyAddr.String(),
		ReverseTunnelAddress: rootTunnelAddr.String(),
		RoleMap: []types.RoleMapping{
			{
				Remote: "access",
				Local:  []string{"access"},
			},
			{
				Remote: perSessionMFARole.GetName(),
				Local:  []string{perSessionMFARole.GetName()},
			},
			{
				Remote: sshLoginRole.GetName(),
				Local:  []string{sshLoginRole.GetName()},
			},
		},
	})
	require.NoError(t, err)

	leafAuth, leafProxy := makeTestServers(t, withClusterName(t, "leafcluster"), withBootstrap(connector, alice, sshLoginRole, perSessionMFARole))
	tryCreateTrustedCluster(t, leafAuth.GetAuthServer(), trustedCluster)

	leafAuthAddr, err := leafAuth.AuthAddr()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		conns, err := rootAuth.GetAuthServer().GetTunnelConnections("leafcluster")
		return err == nil && len(conns) == 1
	}, 10*time.Second, 100*time.Millisecond, "leaf cluster never heart beated")

	leafProxyAddr := leafProxy.Config.Proxy.WebAddr.String()

	stage1Hostname := "test-stage-1"
	node := makeTestSSHNode(t, rootAuthAddr, withHostname(stage1Hostname), withSSHLabel("env", "stage"))
	sshHostID := node.Config.HostUUID

	stage2Hostname := "test-stage-2"
	node2 := makeTestSSHNode(t, rootAuthAddr, withHostname(stage2Hostname), withSSHLabel("env", "stage"))
	sshHostID2 := node2.Config.HostUUID

	prodHostname := "test-prod-1"
	nodeProd := makeTestSSHNode(t, rootAuthAddr, withHostname(prodHostname), withSSHLabel("env", "prod"))
	sshHostID3 := nodeProd.Config.HostUUID

	leafHostname := "leaf-node"
	leafNode := makeTestSSHNode(t, leafAuthAddr, withHostname(leafHostname), withSSHLabel("animal", "llama"))
	sshLeafHostID := leafNode.Config.HostUUID

	hasNodes := func(asrv *auth.Server, hostIDs ...string) func() bool {
		return func() bool {
			nodes, err := asrv.GetNodes(ctx, apidefaults.Namespace)
			if err != nil {
				return false
			}

			foundCount := 0
			for _, node := range nodes {
				if slices.Contains(hostIDs, node.GetName()) {
					foundCount++
				}
			}
			return foundCount == len(hostIDs)
		}
	}

	// wait for auth to see nodes
	require.Eventually(t, hasNodes(rootAuth.GetAuthServer(), sshHostID, sshHostID2, sshHostID3),
		5*time.Second, 100*time.Millisecond, "nodes never joined root cluster")

	require.Eventually(t, hasNodes(leafAuth.GetAuthServer(), sshLeafHostID),
		5*time.Second, 100*time.Millisecond, "nodes never joined leaf cluster")

	defaultPreference, err := rootAuth.GetAuthServer().GetAuthPreference(ctx)
	require.NoError(t, err)

	webauthnPreference := func(cluster string) *types.AuthPreferenceV2 {
		return &types.AuthPreferenceV2{
			Spec: types.AuthPreferenceSpecV2{
				Type:         constants.Local,
				SecondFactor: constants.SecondFactorOptional,
				Webauthn: &types.Webauthn{
					RPID: cluster,
				},
			},
		}
	}

	setupUser := func(cluster string, asrv *auth.Server) {
		// set the default auth preference
		err = asrv.SetAuthPreference(ctx, webauthnPreference(cluster))
		require.NoError(t, err)

		token, err := asrv.CreateResetPasswordToken(ctx, auth.CreateUserTokenRequest{
			Name: "alice",
		})
		require.NoError(t, err)
		tokenID := token.GetName()
		res, err := asrv.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
			TokenID:     tokenID,
			DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
			DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
		})
		require.NoError(t, err)
		cc := wanlib.CredentialCreationFromProto(res.GetWebauthn())

		ccr, err := device.SignCredentialCreation(origin(cluster), cc)
		require.NoError(t, err)
		_, err = asrv.ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
			TokenID:     tokenID,
			NewPassword: []byte(password),
			NewMFARegisterResponse: &proto.MFARegisterResponse{
				Response: &proto.MFARegisterResponse_Webauthn{
					Webauthn: wanlib.CredentialCreationResponseToProto(ccr),
				},
			},
		})
		require.NoError(t, err)
	}

	setupUser("localhost", rootAuth.GetAuthServer())
	setupUser("leafcluster", leafAuth.GetAuthServer())

	successfulChallenge := func(cluster string) func(ctx context.Context, realOrigin string, assertion *wanlib.CredentialAssertion, prompt wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
		return func(ctx context.Context, realOrigin string, assertion *wanlib.CredentialAssertion, prompt wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
			car, err := device.SignAssertion(origin(cluster), assertion) // use the fake origin to prevent a mismatch
			if err != nil {
				return nil, "", err
			}
			return &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: wanlib.CredentialAssertionResponseToProto(car),
				},
			}, "", nil
		}
	}

	failedChallenge := func(cluster string) func(ctx context.Context, realOrigin string, assertion *wanlib.CredentialAssertion, prompt wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
		return func(ctx context.Context, realOrigin string, assertion *wanlib.CredentialAssertion, prompt wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {

			car, err := device.SignAssertion(origin(cluster), assertion) // use the fake origin to prevent a mismatch
			if err != nil {
				return nil, "", err
			}
			carProto := wanlib.CredentialAssertionResponseToProto(car)
			carProto.Type = "NOT A VALID TYPE" // set to an invalid type so the ceremony fails

			return &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: carProto,
				},
			}, "", nil
		}
	}

	type mfaPrompt = func(ctx context.Context, origin string, assertion *wanlib.CredentialAssertion, prompt wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error)
	setupChallengeSolver := func(mfaPrompt mfaPrompt) func(t *testing.T) {
		return func(t *testing.T) {
			inputReader := prompt.NewFakeReader().
				AddString(password).
				AddReply(func(ctx context.Context) (string, error) {
					panic("this should not be called")
				})

			oldStdin, oldWebauthn := prompt.Stdin(), *client.PromptWebauthn
			t.Cleanup(func() {
				prompt.SetStdin(oldStdin)
				*client.PromptWebauthn = oldWebauthn
			})

			prompt.SetStdin(inputReader)
			*client.PromptWebauthn = mfaPrompt
		}
	}

	cases := []struct {
		name            string
		target          string
		authPreference  types.AuthPreference
		roles           []string
		setup           func(t *testing.T)
		errAssertion    require.ErrorAssertionFunc
		stdoutAssertion require.ValueAssertionFunc
		mfaPromptCount  int
		headless        bool
		proxyAddr       string
		auth            *auth.Server
		cluster         string
	}{
		{
			name:           "default auth preference runs commands on multiple nodes without mfa",
			authPreference: defaultPreference,
			proxyAddr:      rootProxyAddr.String(),
			auth:           rootAuth.GetAuthServer(),
			target:         "env=stage",
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\ntest\n", i, i2...)
			},
			errAssertion: require.NoError,
		},
		{
			name:      "webauthn auth preference runs commands on multiple matches without mfa",
			target:    "env=stage",
			proxyAddr: rootProxyAddr.String(),
			auth:      rootAuth.GetAuthServer(),
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\ntest\n", i, i2...)
			},
			errAssertion: require.NoError,
		},
		{
			name:      "webauthn auth preference runs commands on a single match without mfa",
			target:    "env=prod",
			proxyAddr: rootProxyAddr.String(),
			auth:      rootAuth.GetAuthServer(),
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			errAssertion: require.NoError,
		},
		{
			name:            "no matching hosts",
			target:          "env=dev",
			proxyAddr:       rootProxyAddr.String(),
			auth:            rootAuth.GetAuthServer(),
			errAssertion:    require.Error,
			stdoutAssertion: require.Empty,
		},
		{
			name: "command runs on multiple matches with mfa set via auth preference",
			authPreference: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOptional,
					Webauthn: &types.Webauthn{
						RPID: "localhost",
					},
					RequireMFAType: types.RequireMFAType_SESSION,
				},
			},
			proxyAddr: rootProxyAddr.String(),
			auth:      rootAuth.GetAuthServer(),
			setup:     setupChallengeSolver(successfulChallenge("localhost")),
			target:    "env=stage",
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\ntest\n", i, i2...)
			},
			mfaPromptCount: 2,
			errAssertion:   require.NoError,
		},
		{
			name: "command runs on a single match with mfa set via auth preference",
			authPreference: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOptional,
					Webauthn: &types.Webauthn{
						RPID: "localhost",
					},
					RequireMFAType: types.RequireMFAType_SESSION,
				},
			},
			proxyAddr: rootProxyAddr.String(),
			auth:      rootAuth.GetAuthServer(),
			setup:     setupChallengeSolver(successfulChallenge("localhost")),
			target:    "env=prod",
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			mfaPromptCount: 1,
			errAssertion:   require.NoError,
		},
		{
			name: "no matching hosts with mfa",
			authPreference: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOptional,
					Webauthn: &types.Webauthn{
						RPID: "localhost",
					},
					RequireMFAType: types.RequireMFAType_SESSION,
				},
			},
			proxyAddr:       rootProxyAddr.String(),
			auth:            rootAuth.GetAuthServer(),
			setup:           setupChallengeSolver(successfulChallenge("localhost")),
			target:          "env=dev",
			errAssertion:    require.Error,
			stdoutAssertion: require.Empty,
		},
		{
			name: "command runs on a multiple matches with mfa set via role",
			authPreference: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOptional,
					Webauthn: &types.Webauthn{
						RPID: "localhost",
					},
				},
			},
			proxyAddr: rootProxyAddr.String(),
			auth:      rootAuth.GetAuthServer(),
			roles:     []string{"access", sshLoginRole.GetName(), perSessionMFARole.GetName()},
			setup:     setupChallengeSolver(successfulChallenge("localhost")),
			target:    "env=stage",
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\ntest\n", i, i2...)
			},
			mfaPromptCount: 2,
			errAssertion:   require.NoError,
		},
		{
			name:      "role permits access without mfa",
			target:    sshHostID,
			proxyAddr: rootProxyAddr.String(),
			auth:      rootAuth.GetAuthServer(),
			roles:     []string{sshLoginRole.GetName()},
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			errAssertion: require.NoError,
		},
		{
			name:            "role prevents access",
			target:          sshHostID,
			proxyAddr:       rootProxyAddr.String(),
			auth:            rootAuth.GetAuthServer(),
			roles:           []string{noAccessRole.GetName()},
			stdoutAssertion: require.Empty,
			errAssertion:    require.Error,
		},
		{
			name:      "command runs on a hostname with mfa set via role",
			target:    sshHostID,
			proxyAddr: rootProxyAddr.String(),
			auth:      rootAuth.GetAuthServer(),
			roles:     []string{perSessionMFARole.GetName()},
			setup:     setupChallengeSolver(successfulChallenge("localhost")),
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			mfaPromptCount: 1,
			errAssertion:   require.NoError,
		},
		{
			name: "failed ceremony when role requires per session mfa",
			authPreference: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOptional,
					Webauthn: &types.Webauthn{
						RPID: "localhost",
					},
				},
			},
			proxyAddr:       rootProxyAddr.String(),
			auth:            rootAuth.GetAuthServer(),
			target:          sshHostID,
			roles:           []string{perSessionMFARole.GetName()},
			setup:           setupChallengeSolver(failedChallenge("localhost")),
			stdoutAssertion: require.Empty,
			mfaPromptCount:  1,
			errAssertion:    require.Error,
		},
		{
			name: "mfa ceremony prevented when using headless auth",
			authPreference: &types.AuthPreferenceV2{
				Spec: types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOptional,
					Webauthn: &types.Webauthn{
						RPID: "localhost",
					},
				},
			},
			proxyAddr: rootProxyAddr.String(),
			auth:      rootAuth.GetAuthServer(),
			target:    sshHostID,
			roles:     []string{perSessionMFARole.GetName()},
			setup:     setupChallengeSolver(failedChallenge("localhost")),
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			errAssertion: require.NoError,
			headless:     true,
		},
		{
			name:      "command runs on a leaf node with mfa set via role",
			target:    sshLeafHostID,
			proxyAddr: leafProxyAddr,
			auth:      leafAuth.GetAuthServer(),
			roles:     []string{perSessionMFARole.GetName()},
			setup:     setupChallengeSolver(successfulChallenge("leafcluster")),
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			mfaPromptCount: 1,
			errAssertion:   require.NoError,
		}, {
			name:      "command runs on a leaf node via root without mfa",
			target:    sshLeafHostID,
			proxyAddr: rootProxyAddr.String(),
			auth:      rootAuth.GetAuthServer(),
			cluster:   "leafcluster",
			roles:     []string{sshLoginRole.GetName()},
			setup:     setupChallengeSolver(successfulChallenge("localhost")),
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			errAssertion: require.NoError,
		},
		{
			name:      "command runs on a leaf node without mfa",
			target:    sshLeafHostID,
			proxyAddr: leafProxyAddr,
			auth:      leafAuth.GetAuthServer(),
			roles:     []string{sshLoginRole.GetName()},
			setup:     setupChallengeSolver(successfulChallenge("leafcluster")),
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			errAssertion: require.NoError,
		}, {
			name:      "command runs on a leaf node via root with mfa set via role",
			target:    sshLeafHostID,
			proxyAddr: rootProxyAddr.String(),
			auth:      rootAuth.GetAuthServer(),
			cluster:   "leafcluster",
			roles:     []string{perSessionMFARole.GetName()},
			setup:     setupChallengeSolver(successfulChallenge("localhost")),
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			mfaPromptCount: 1,
			errAssertion:   require.NoError,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			tmpHomePath := t.TempDir()

			clusterName, err := tt.auth.GetClusterName()
			require.NoError(t, err)

			if tt.authPreference != nil {
				require.NoError(t, tt.auth.SetAuthPreference(ctx, tt.authPreference))
				t.Cleanup(func() {
					require.NoError(t, tt.auth.SetAuthPreference(ctx, webauthnPreference(clusterName.GetClusterName())))
				})
			}

			if tt.setup != nil {
				tt.setup(t)
			}

			if tt.roles != nil {
				roles := alice.GetRoles()
				t.Cleanup(func() {
					alice.SetRoles(roles)
					require.NoError(t, tt.auth.UpsertUser(alice))
				})
				alice.SetRoles(tt.roles)
				require.NoError(t, tt.auth.UpsertUser(alice))
			}

			err = Run(ctx, []string{
				"login",
				"-d",
				"--insecure",
				"--auth", connector.GetName(),
				"--proxy", tt.proxyAddr,
				"--user", "alice",
				tt.cluster,
			}, setHomePath(tmpHomePath),
				func(cf *CLIConf) error {
					cf.MockSSOLogin = mockSSOLogin(t, tt.auth, alice)
					return nil
				},
			)
			require.NoError(t, err)

			stdout := &output{buf: bytes.Buffer{}}
			// Clear counter before each ssh command,
			// so we can assert how many times sign was called.
			device.SetCounter(0)

			args := []string{"ssh", "-d", "--insecure"}
			if tt.headless {
				args = append(args, "--headless", "--proxy", tt.proxyAddr, "--user", alice.GetName())
			}
			args = append(args, tt.target, "echo", "test")

			err = Run(ctx,
				args,
				setHomePath(tmpHomePath),
				func(conf *CLIConf) error {
					conf.overrideStdin = &bytes.Buffer{}
					conf.OverrideStdout = stdout
					conf.MockHeadlessLogin = mockHeadlessLogin(t, tt.auth, alice)
					return nil
				},
			)

			tt.errAssertion(t, err)
			tt.stdoutAssertion(t, stdout.String())
			require.Equal(t, tt.mfaPromptCount, int(device.Counter()), "device sign count mismatch")
		})
	}
}

type output struct {
	lock sync.Mutex
	buf  bytes.Buffer
}

func (o *output) Write(p []byte) (int, error) {
	o.lock.Lock()
	defer o.lock.Unlock()

	return o.buf.Write(p)
}

func (o *output) String() string {
	o.lock.Lock()
	defer o.lock.Unlock()

	return o.buf.String()
}

// TestSSHAccessRequest tests that a user can automatically request access to a
// ssh server using a resource access request when "tsh ssh" fails with
// AccessDenied.
func TestSSHAccessRequest(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})
	tmpHomePath := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	requester, err := types.NewRole("requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				SearchAsRoles: []string{"node-access"},
			},
		},
	})
	require.NoError(t, err)

	nodeAccessRole, err := types.NewRole("node-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"access": {"true"},
			},
			Logins: []string{"{{internal.logins}}"},
		},
	})
	require.NoError(t, err)

	connector := mockConnector(t)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"requester"})
	user, err := user.Current()
	require.NoError(t, err)
	traits := map[string][]string{
		constants.TraitLogins: {user.Username},
	}
	alice.SetTraits(traits)

	rootAuth, rootProxy := makeTestServers(t, withBootstrap(requester, nodeAccessRole, connector, alice))

	authAddr, err := rootAuth.AuthAddr()
	require.NoError(t, err)

	proxyAddr, err := rootProxy.ProxyWebAddr()
	require.NoError(t, err)

	sshHostname := "test-ssh-server"
	node := makeTestSSHNode(t, authAddr, withHostname(sshHostname), withSSHLabel("access", "true"))
	sshHostID := node.Config.HostUUID

	sshHostname2 := "test-ssh-server-2"
	node2 := makeTestSSHNode(t, authAddr, withHostname(sshHostname2), withSSHLabel("access", "true"))
	sshHostID2 := node2.Config.HostUUID

	sshHostnameNoAccess := "test-ssh-server-no-access"
	nodeNoAccess := makeTestSSHNode(t, authAddr, withHostname(sshHostnameNoAccess), withSSHLabel("access", "false"))
	sshHostIDNoAccess := nodeNoAccess.Config.HostUUID

	hasNodes := func(hostIDs ...string) func() bool {
		return func() bool {
			nodes, err := rootAuth.GetAuthServer().GetNodes(ctx, apidefaults.Namespace)
			require.NoError(t, err)
			foundCount := 0
			for _, node := range nodes {
				if slices.Contains(hostIDs, node.GetName()) {
					foundCount++
				}
			}
			return foundCount == len(hostIDs)
		}
	}

	// wait for auth to see nodes
	require.Eventually(t, hasNodes(sshHostID, sshHostID2, sshHostIDNoAccess),
		10*time.Second, 100*time.Millisecond, "nodes never showed up")

	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr.String(),
		"--user", "alice",
	}, setHomePath(tmpHomePath), CliOption(func(cf *CLIConf) error {
		cf.MockSSOLogin = mockSSOLogin(t, rootAuth.GetAuthServer(), alice)
		return nil
	}))
	require.NoError(t, err)

	// won't request if can't list node
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--request-reason", "reason here to bypass prompt",
		fmt.Sprintf("%s@%s", user.Username, sshHostnameNoAccess),
		"echo", "test",
	}, setHomePath(tmpHomePath))
	require.Error(t, err)

	// won't request if can't login with username
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--request-reason", "reason here to bypass prompt",
		fmt.Sprintf("%s@%s", "not-a-username", sshHostname),
		"echo", "test",
	}, setHomePath(tmpHomePath))
	require.Error(t, err)

	// won't request to non-existent node
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		fmt.Sprintf("%s@unknown", user.Username),
		"echo", "test",
	}, setHomePath(tmpHomePath))
	require.Error(t, err)

	// approve all requests as they're created
	errChan := make(chan error)
	t.Cleanup(func() {
		require.ErrorIs(t, <-errChan, context.Canceled, "unexpected error from approveAllAccessRequests")
	})
	go func() {
		err := approveAllAccessRequests(ctx, rootAuth.GetAuthServer())
		// Cancel the context, so Run calls don't block
		cancel()
		errChan <- err
	}()

	// won't request if explicitly disabled
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--request-reason", "reason here to bypass prompt",
		"--disable-access-request",
		fmt.Sprintf("%s@%s", user.Username, sshHostname),
		"echo", "test",
	}, setHomePath(tmpHomePath))
	require.Error(t, err)

	// the first ssh request can fail if the proxy node watcher doesn't know
	// about the nodes yet, retry a few times until it works
	require.Eventually(t, func() bool {
		// ssh with request, by hostname
		err := Run(ctx, []string{
			"ssh",
			"--debug",
			"--insecure",
			"--request-reason", "reason here to bypass prompt",
			fmt.Sprintf("%s@%s", user.Username, sshHostname),
			"echo", "test",
		}, setHomePath(tmpHomePath))
		if err != nil {
			t.Logf("Got error while trying to SSH to node, retrying. Error: %v", err)
		}
		return err == nil
	}, 10*time.Second, 100*time.Millisecond, "failed to ssh with retries")

	// now that we have an approved access request, it should work without
	// prompting for a request reason
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		fmt.Sprintf("%s@%s", user.Username, sshHostname),
		"echo", "test",
	}, setHomePath(tmpHomePath))
	require.NoError(t, err)

	// log out and back in with no access request
	err = Run(ctx, []string{
		"logout",
	}, setHomePath(tmpHomePath))
	require.NoError(t, err)
	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr.String(),
		"--user", "alice",
	}, setHomePath(tmpHomePath), CliOption(func(cf *CLIConf) error {
		cf.MockSSOLogin = mockSSOLogin(t, rootAuth.GetAuthServer(), alice)
		return nil
	}))
	require.NoError(t, err)

	// ssh with request, by host ID
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--request-reason", "reason here to bypass prompt",
		fmt.Sprintf("%s@%s", user.Username, sshHostID),
		"echo", "test",
	}, setHomePath(tmpHomePath))
	require.NoError(t, err)

	// fail to ssh to other non-approved node, do not prompt for request
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		fmt.Sprintf("%s@%s", user.Username, sshHostname2),
		"echo", "test",
	}, setHomePath(tmpHomePath))
	require.Error(t, err)

	// drop the current access request
	err = Run(ctx, []string{
		"--insecure",
		"request",
		"drop",
	}, setHomePath(tmpHomePath))
	require.NoError(t, err)

	// fail to ssh to other node with no active request
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--disable-access-request",
		fmt.Sprintf("%s@%s", user.Username, sshHostname2),
		"echo", "test",
	}, setHomePath(tmpHomePath))
	require.Error(t, err)

	// successfully ssh to other node, with new request
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--request-reason", "reason here to bypass prompt",
		fmt.Sprintf("%s@%s", user.Username, sshHostname2),
		"echo", "test",
	}, setHomePath(tmpHomePath))
	require.NoError(t, err)
}

func TestAccessRequestOnLeaf(t *testing.T) {
	t.Parallel()

	tmpHomePath := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	isInsecure := lib.IsInsecureDevMode()
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() {
		lib.SetInsecureDevMode(isInsecure)
	})

	requester, err := types.NewRole("requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles: []string{"access"},
			},
		},
	})
	require.NoError(t, err)

	connector := mockConnector(t)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"requester"})

	rootAuth, rootProxy := makeTestServers(t,
		withBootstrap(requester, connector, alice),
	)

	rootAuthServer := rootAuth.GetAuthServer()
	require.NotNil(t, rootAuthServer)
	rootProxyAddr, err := rootProxy.ProxyWebAddr()
	require.NoError(t, err)
	rootTunnelAddr, err := rootProxy.ProxyTunnelAddr()
	require.NoError(t, err)

	trustedCluster, err := types.NewTrustedCluster("localhost", types.TrustedClusterSpecV2{
		Enabled:              true,
		Roles:                []string{},
		Token:                staticToken,
		ProxyAddress:         rootProxyAddr.String(),
		ReverseTunnelAddress: rootTunnelAddr.String(),
		RoleMap: []types.RoleMapping{
			{
				Remote: "access",
				Local:  []string{"access"},
			},
		},
	})
	require.NoError(t, err)

	leafAuth, _ := makeTestServers(t, withClusterName(t, "leafcluster"))
	tryCreateTrustedCluster(t, leafAuth.GetAuthServer(), trustedCluster)

	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", rootProxyAddr.String(),
	}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
		cf.MockSSOLogin = mockSSOLogin(t, rootAuthServer, alice)
		return nil
	})
	require.NoError(t, err)

	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", rootProxyAddr.String(),
		"leafcluster",
	}, setHomePath(tmpHomePath))
	require.NoError(t, err)

	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", rootProxyAddr.String(),
		"localhost",
	}, setHomePath(tmpHomePath))
	require.NoError(t, err)

	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", rootProxyAddr.String(),
		"leafcluster",
	}, setHomePath(tmpHomePath))
	require.NoError(t, err)

	// approve all requests as they're created
	errChan := make(chan error)
	t.Cleanup(func() {
		require.ErrorIs(t, <-errChan, context.Canceled, "unexpected error from approveAllAccessRequests")
	})
	go func() {
		err := approveAllAccessRequests(ctx, rootAuth.GetAuthServer())
		// Cancel the context, so Run calls don't block
		cancel()
		errChan <- err
	}()

	err = Run(ctx, []string{
		"request",
		"new",
		"--insecure",
		"--debug",
		"--proxy", rootProxyAddr.String(),
		"--roles=access",
	}, setHomePath(tmpHomePath))
	require.NoError(t, err)
}

// tryCreateTrustedCluster performs several attempts to create a trusted cluster,
// retries on connection problems and access denied errors to let caches
// propagate and services to start
//
// Duplicated in integration/integration_test.go
func tryCreateTrustedCluster(t *testing.T, authServer *auth.Server, trustedCluster types.TrustedCluster) {
	ctx := context.TODO()
	for i := 0; i < 10; i++ {
		log.Debugf("Will create trusted cluster %v, attempt %v.", trustedCluster, i)
		_, err := authServer.UpsertTrustedCluster(ctx, trustedCluster)
		if err == nil {
			return
		}
		if trace.IsConnectionProblem(err) {
			log.Debugf("Retrying on connection problem: %v.", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if trace.IsAccessDenied(err) {
			log.Debugf("Retrying on access denied: %v.", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}
		require.FailNow(t, "Terminating on unexpected problem", "%v.", err)
	}
	require.FailNow(t, "Timeout creating trusted cluster")
}

func TestKubeCredentialsLock(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	const kubeClusterName = "kube-cluster"

	t.Run("failed client creation doesn't create lockfile", func(t *testing.T) {
		tmpHomePath := t.TempDir()

		firstErr := Run(ctx, []string{
			"kube",
			"credentials",
			"--proxy", "fake-proxy",
			"--teleport-cluster", "teleport",
			"--kube-cluster", kubeClusterName,
		}, setHomePath(tmpHomePath))
		require.Error(t, firstErr) // Fails because fake proxy doesn't exist
		require.NoFileExists(t, keypaths.KubeCredLockfilePath(tmpHomePath, "fake-proxy"))
	})

	t.Run("kube credentials called multiple times, SSO login called only once", func(t *testing.T) {
		tmpHomePath := t.TempDir()
		connector := mockConnector(t)
		alice, err := types.NewUser("alice@example.com")
		require.NoError(t, err)

		kubeRole, err := types.NewRole("kube-access", types.RoleSpecV6{
			Allow: types.RoleConditions{
				KubernetesLabels: types.Labels{types.Wildcard: apiutils.Strings{types.Wildcard}},
				KubeGroups:       []string{kube.TestImpersonationGroup},
				KubeUsers:        []string{alice.GetName()},
				KubernetesResources: []types.KubernetesResource{
					{
						Kind: types.KindKubePod, Name: types.Wildcard, Namespace: types.Wildcard,
					},
				},
			},
		})
		require.NoError(t, err)
		alice.SetRoles([]string{"access", kubeRole.GetName()})

		require.NoError(t, err)
		authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, alice, kubeRole))
		authServer := authProcess.GetAuthServer()
		require.NotNil(t, authServer)
		proxyAddr, err := proxyProcess.ProxyWebAddr()
		require.NoError(t, err)

		teleportClusterName, err := authServer.GetClusterName()
		require.NoError(t, err)

		kubeCluster, err := types.NewKubernetesClusterV3(types.Metadata{
			Name:   kubeClusterName,
			Labels: map[string]string{},
		},
			types.KubernetesClusterSpecV3{},
		)
		require.NoError(t, err)
		kubeServer, err := types.NewKubernetesServerV3FromCluster(kubeCluster, kubeClusterName, kubeClusterName)
		require.NoError(t, err)
		_, err = authServer.UpsertKubernetesServer(context.Background(), kubeServer)
		require.NoError(t, err)

		var ssoCalls atomic.Int32
		mockSSO := mockSSOLogin(t, authServer, alice)
		ssoFunc := func(ctx context.Context, connectorID string, priv *keys.PrivateKey, protocol string) (*auth.SSHLoginResponse, error) {
			ssoCalls.Add(1)
			return mockSSO(ctx, connectorID, priv, protocol)
		}

		err = Run(context.Background(), []string{
			"login",
			"--insecure",
			"--debug",
			"--auth", connector.GetName(),
			"--proxy", proxyAddr.String(),
		}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
			cf.MockSSOLogin = ssoFunc
			return nil
		})
		require.NoError(t, err)
		_, err = profile.FromDir(tmpHomePath, "")
		require.NoError(t, err)
		ssoCalls.Store(0) // Reset number of calls after setup login

		// Overwrite profile data to simulate expired user certificate
		expiredSSHCert := `
		ssh-rsa-cert-v01@openssh.com AAAAHHNzaC1yc2EtY2VydC12MDFAb3BlbnNzaC5jb20AAAAgefu/ZQ70TbBMZfGUFHluE7PCu6PiWN0SsA5xrKbkzCkAAAADAQABAAABAQCyGzVvW7vgsK1P2Rtg55DTjL4We0WjSYYdzXJnVbyTxqrEYDOkhSnw4tZTS9KgALb698g0vrqy5bSJXB90d8uLdTmCmPngPbYpSN+p3P2SbIdkB5cRIMspB22qSkfHUARQlYM4PrMYIznWwQRFBvrRNOVdTdbMywlQGMUb0jdxK7JFBx1LC76qfHJhrD7jZS+MtygFIqhAJS9CQXW314p3FmL9s1cPV5lQfY527np8580qMKPkdeowPd/hVGcPA/C+ZxLcN9LqnuTZEFoDvYtwjfofOGUpANwtENBNZbNTxHDk7shYCRN9aZJ50zdFq3rMNdzFlEyJwm2ca+7aRDLlAAAAAAAAAAAAAAABAAAAEWFsaWNlQGV4YW1wbGUuY29tAAAAVQAAADYtdGVsZXBvcnQtbm9sb2dpbi1hZDVhYTViMi00MDBlLTQ2ZmUtOThjOS02ZjRhNDA2YzdlZGMAAAAXLXRlbGVwb3J0LWludGVybmFsLWpvaW4AAAAAZG9PKAAAAABkb0+gAAAAAAAAAQkAAAAXcGVybWl0LWFnZW50LWZvcndhcmRpbmcAAAAAAAAAFnBlcm1pdC1wb3J0LWZvcndhcmRpbmcAAAAAAAAACnBlcm1pdC1wdHkAAAAAAAAAEnByaXZhdGUta2V5LXBvbGljeQAAAAgAAAAEbm9uZQAAAA50ZWxlcG9ydC1yb2xlcwAAADUAAAAxeyJ2ZXJzaW9uIjoidjEiLCJyb2xlcyI6WyJhY2Nlc3MiLCJrdWJlLWFjY2VzcyJdfQAAABl0ZWxlcG9ydC1yb3V0ZS10by1jbHVzdGVyAAAADQAAAAlsb2NhbGhvc3QAAAAPdGVsZXBvcnQtdHJhaXRzAAAACAAAAARudWxsAAAAAAAAARcAAAAHc3NoLXJzYQAAAAMBAAEAAAEBAK/vBVOnf+QLSF0aKsEpQuof1o/5EJJ25C07tljSWvF2wNixHOyHZj8kAwO3f2XmWQd/XBddvZtLETvTbdBum8T37oOLepnDR32TzTV7cR7XVvo0pSqwrg0jWuAxt67b2n2BnWOCULdV9mPM8X9q4wRhqQHFGB3+7dD24x5YmVIBFUFJKYfFYh516giKAcNPFSK6eD381+cNXYx3yDO6i/iyrsuhbYVcTlWSV2Zhc0Gytf83QRmpM6hXW8b8hGCui36ffXSYu/9nWHcK7OeaHZzePT7jHptyqcrYSs52VuzikO74jpw8htU6maUeEcR5TBbeBlB+hmHQfwl8bEUeszMAAAEUAAAADHJzYS1zaGEyLTUxMgAAAQAwb7OpVkJP8pz9M4VIoG0DzXe5W2GN2pH0eWq+n/+YgshzcWPyHsPbckCu9IleHhrp6IK8ZyCt2qLi77o9XJxJUCiJxmsnfJYTs5DtWoCqiIRWKtYvSNpML1PH/badQsS/Stg7VUs48Yftg4eJOo4PYfJqDoHRfGimMwTNQ+aIWAYek3QwydlDdEtJ6X8kkHhNnZb0fzUbUyQLzFDXPu++di+AHNOpMOWDBtNZ1Lm3WWja/t5zIv7j6L67ZTzS5JtV7TlcD+lJ7RcBfWow8OtEW5RyyI8918A2q/zbe3OhGD4D2dZxPUhyGPHLLgy9NJqJoueR8qLPQQyMdDl4JKUq
		`
		privKey := `
-----BEGIN RSA PRIVATE KEY-----
MIIEowIBAAKCAQEAshs1b1u74LCtT9kbYOeQ04y+FntFo0mGHc1yZ1W8k8aqxGAz
pIUp8OLWU0vSoAC2+vfINL66suW0iVwfdHfLi3U5gpj54D22KUjfqdz9kmyHZAeX
ESDLKQdtqkpHx1AEUJWDOD6zGCM51sEERQb60TTlXU3WzMsJUBjFG9I3cSuyRQcd
Swu+qnxyYaw+42UvjLcoBSKoQCUvQkF1t9eKdxZi/bNXD1eZUH2Odu56fOfNKjCj
5HXqMD3f4VRnDwPwvmcS3DfS6p7k2RBaA72LcI36HzhlKQDcLRDQTWWzU8Rw5O7I
WAkTfWmSedM3Rat6zDXcxZRMicJtnGvu2kQy5QIDAQABAoIBAFBPIn4PAB1lrRBX
FhhQ8iXhzYi3lwP00Cu6Cr77kueTakbYFhE2Fl5O+lNe2h9ZkyiA996IrgiiuRBC
4NAUgExm1ELGFc3+JZhiCrA+PHx8wWPiZETN462hctqZWdpOg1OOxzdiVkEpCRiD
uhgh+JDC6DV1NsjrOEzMjnxoAqXdS8R8HRSr7ATV/28rXCpzBevtsQFWsHFQ069H
uL9JY4AnBdvnu749ClFYuhux/C0zSAsZIu47WUmmyZdIXY/B32vkhbDfKPpCp7Il
5sE9reNGf22jkqjC4wLpCT0L8wuUnJil0Sj1JUQjtZc4vn7Fc1RWZWRZKC4IlI/+
OUSdXhUCgYEA6krEs7B1W3mO8sCXEUpFZv8KfLaa4BeDncGS++qGrz4qSI8JiuUI
M4uLBec+9FHAAMVd5F/JxhcA5J1BA2e9mITEf582ur/lTU2cSYBIY64IPBMR0ask
Q9UdAdu0r/xQ91cdwKiaSrC3bPgCX/Xe6MzaEWMrmdnse3Kl+E49n0cCgYEAwpvB
gtCk/L6lOsQDgLH3zO27qlYUGSPqhy8iIF+4HMMIIvdIOrSMHspEHhbnypQe9cYO
GRcimlb4U0FbHsOmpkfcNHhvBTegmYEYjuWJR0AN8cgaV2b6XytqYB7Gv2kXhjnF
9dvamhy9+4SngywbqZshUlazVW/RfO4+OqXsKnMCgYBwJJOcUqUJwNhsV0S30O4B
S6gwY5MkGf00oHgDPpFzBfVlP5nYsqHHUk6b58DZXtvhQpcbfcHtoAscYizBPYGh
pEMNtx6SKtHNu41IHTAJDj8AyjvoONul4Db/MbN93O7ARSGHmuwnPgi+DsPMPLqS
gaMLWYWAIbAwsoLApGqYdwKBgAZALHoAK5x2nyYBD7+9d6Ecba+t7h1Umv7Wk7kI
eghqd0NwP+Cq1elTQ9bXk4BdO5VXVDKYHKNqcbVy3vNhA2RJ4JfK2n4HaGAl1l0Y
oE0qkIgYjkgKZbZS1arasjWJsZi9GE+qTR4wGCYQ/7Rl4UmUUwCrCj2PRuJFYLhP
hgNjAoGBAKwqiQpwNzbKOq3+pxau6Y32BqUaTV5ut9FEUz0/qzuNoc2S5bCf4wq+
cc/nvPBnXrP+rsubJXjFDfcIjcZ7x41bRMENvP50xD/J94IpK88TGTVa04VHKExx
iUK/veLmZ6XoouiWLCdU1VJz/1Fcwe/IEamg6ETfofvsqOCgcNYJ
-----END RSA PRIVATE KEY-----
`
		pubKey := `ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCyGzVvW7vgsK1P2Rtg55DTjL4We0WjSYYdzXJnVbyTxqrEYDOkhSnw4tZTS9KgALb698g0vrqy5bSJXB90d8uLdTmCmPngPbYpSN+p3P2SbIdkB5cRIMspB22qSkfHUARQlYM4PrMYIznWwQRFBvrRNOVdTdbMywlQGMUb0jdxK7JFBx1LC76qfHJhrD7jZS+MtygFIqhAJS9CQXW314p3FmL9s1cPV5lQfY527np8580qMKPkdeowPd/hVGcPA/C+ZxLcN9LqnuTZEFoDvYtwjfofOGUpANwtENBNZbNTxHDk7shYCRN9aZJ50zdFq3rMNdzFlEyJwm2ca+7aRDLl
`
		err = os.WriteFile(fmt.Sprintf("%s/%s", tmpHomePath, "keys/127.0.0.1/alice@example.com"), []byte(privKey), 0666)
		require.NoError(t, err)
		err = os.WriteFile(fmt.Sprintf("%s/%s", tmpHomePath, "keys/127.0.0.1/alice@example.com.pub"), []byte(pubKey), 0666)
		require.NoError(t, err)
		err = os.WriteFile(fmt.Sprintf("%s/%s", tmpHomePath, "keys/127.0.0.1/alice@example.com-ssh/localhost-cert.pub"), []byte(expiredSSHCert), 0666)
		require.NoError(t, err)

		errChan := make(chan error)
		runCreds := func() {
			credErr := Run(context.Background(), []string{
				"kube",
				"credentials",
				"--insecure",
				"--proxy", proxyAddr.String(),
				"--auth", connector.GetName(),
				"--teleport-cluster", teleportClusterName.GetClusterName(),
				"--kube-cluster", kubeClusterName,
			}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
				cf.MockSSOLogin = ssoFunc
				return nil
			})
			errChan <- credErr
		}

		// Run kube credentials calls in parallel, only one should actually call SSO login
		runsCount := 3
		for i := 0; i < runsCount; i++ {
			go runCreds()
		}
		for i := 0; i < runsCount; i++ {
			select {
			case err := <-errChan:
				if err != nil {
					require.ErrorIs(t, err, errKubeCredLockfileFound)
				}

			case <-time.After(time.Second * 5):
				require.Fail(t, "Running kube credentials timed out")
			}
		}
		require.Equal(t, 1, int(ssoCalls.Load()), "SSO login should have been called exactly once")
	})
}

func TestSSHHeadlessCLIFlags(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name          string
		modifyCLIConf func(c *CLIConf)
		assertErr     require.ErrorAssertionFunc
		assertConfig  func(t require.TestingT, c *client.Config)
	}{
		{
			name: "OK --auth headless",
			modifyCLIConf: func(c *CLIConf) {
				c.AuthConnector = constants.HeadlessConnector
				c.ExplicitUsername = true
			},
			assertErr: require.NoError,
			assertConfig: func(t require.TestingT, c *client.Config) {
				require.Equal(t, constants.HeadlessConnector, c.AuthConnector)
			},
		}, {
			name: "OK --headless",
			modifyCLIConf: func(c *CLIConf) {
				c.Headless = true
				c.ExplicitUsername = true
			},
			assertErr: require.NoError,
			assertConfig: func(t require.TestingT, c *client.Config) {
				require.Equal(t, constants.HeadlessConnector, c.AuthConnector)
			},
		}, {
			name: "NOK --headless with mismatched auth connector",
			modifyCLIConf: func(c *CLIConf) {
				c.Headless = true
				c.AuthConnector = constants.LocalConnector
				c.ExplicitUsername = true
			},
			assertErr: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected trace.BadParameter error but got %v", err)
			},
		}, {
			name: "NOK --auth headless without explicit user",
			modifyCLIConf: func(c *CLIConf) {
				c.AuthConnector = constants.HeadlessConnector
			},
			assertErr: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected trace.BadParameter error but got %v", err)
			},
		}, {
			name: "NOK --headless without explicit user",
			modifyCLIConf: func(c *CLIConf) {
				c.Headless = true
			},
			assertErr: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected trace.BadParameter error but got %v", err)
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			// minimal configuration (with defaults)
			conf := &CLIConf{
				Proxy:    "proxy:3080",
				UserHost: "localhost",
				HomePath: t.TempDir(),
			}

			tc.modifyCLIConf(conf)

			c, err := loadClientConfigFromCLIConf(conf, "proxy:3080")
			tc.assertErr(t, err)
			if tc.assertConfig != nil {
				tc.assertConfig(t, c)
			}
		})
	}
}

func TestSSHHeadless(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	user, err := user.Current()
	require.NoError(t, err)

	// Headless ssh should pass session mfa requirements
	nodeAccess, err := types.NewRole("node-access", types.RoleSpecV6{
		Options: types.RoleOptions{
			RequireMFAType: types.RequireMFAType_SESSION,
		},
		Allow: types.RoleConditions{
			Logins:     []string{user.Username},
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
	})
	require.NoError(t, err)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"node-access"})

	requester, err := types.NewRole("requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				SearchAsRoles: []string{"node-access"},
			},
		},
	})
	require.NoError(t, err)

	bob, err := types.NewUser("bob@example.com")
	require.NoError(t, err)
	bob.SetRoles([]string{"requester"})

	sshHostname := "test-ssh-host"
	rootAuth, rootProxy := makeTestServers(t, withBootstrap(nodeAccess, alice, requester, bob), withConfig(func(cfg *servicecfg.Config) {
		cfg.Hostname = sshHostname
		cfg.SSH.Enabled = true
		cfg.SSH.Addr = utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("127.0.0.1", ports.Pop())}
	}))

	proxyAddr, err := rootProxy.ProxyWebAddr()
	require.NoError(t, err)

	require.NoError(t, rootAuth.GetAuthServer().SetAuthPreference(ctx, &types.AuthPreferenceV2{
		Spec: types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.SecondFactorOptional,
			Webauthn: &types.Webauthn{
				RPID: "127.0.0.1",
			},
		},
	}))

	go func() {
		if err := approveAllAccessRequests(ctx, rootAuth.GetAuthServer()); err != nil {
			assert.ErrorIs(t, err, context.Canceled, "unexpected error from approveAllAccessRequests")
		}
		// Cancel the context, so Run calls don't block
		cancel()
	}()

	for _, tc := range []struct {
		name      string
		args      []string
		envFlags  map[string]string
		assertErr require.ErrorAssertionFunc
	}{
		{
			name:      "node access",
			args:      []string{"--headless", "--user", "alice", "--proxy", proxyAddr.String()},
			assertErr: require.NoError,
		}, {
			name:      "resource request",
			args:      []string{"--headless", "--user", "bob", "--request-reason", "reason here to bypass prompt", "--proxy", proxyAddr.String()},
			assertErr: require.NoError,
		}, {
			name: "ssh env variables",
			args: []string{"--headless"},
			envFlags: map[string]string{
				teleport.SSHSessionWebProxyAddr: proxyAddr.String(),
				teleport.SSHTeleportUser:        "alice",
			},
			assertErr: require.NoError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envFlags {
				t.Setenv(k, v)
			}

			args := append([]string{
				"ssh",
				"-d",
				"--insecure",
			}, tc.args...)
			args = append(args,
				fmt.Sprintf("%s@%s", user.Username, sshHostname),
				"echo", "test",
			)

			err := Run(ctx, args, CliOption(func(cf *CLIConf) error {
				cf.MockHeadlessLogin = mockHeadlessLogin(t, rootAuth.GetAuthServer(), alice)
				return nil
			}))
			tc.assertErr(t, err)
		})
	}
}

func TestFormatConnectCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		clusterFlag string
		comment     string
		db          tlsca.RouteToDatabase
		command     string
	}{
		{
			comment: "no default user/database are specified",
			db: tlsca.RouteToDatabase{
				ServiceName: "test",
				Protocol:    defaults.ProtocolPostgres,
			},
			command: `tsh db connect --db-user=<user> --db-name=<name> test`,
		},
		{
			comment: "default user is specified",
			db: tlsca.RouteToDatabase{
				ServiceName: "test",
				Protocol:    defaults.ProtocolPostgres,
				Username:    "postgres",
			},
			command: `tsh db connect --db-name=<name> test`,
		},
		{
			comment: "default database is specified",
			db: tlsca.RouteToDatabase{
				ServiceName: "test",
				Protocol:    defaults.ProtocolPostgres,
				Database:    "postgres",
			},
			command: `tsh db connect --db-user=<user> test`,
		},
		{
			comment: "default user/database are specified",
			db: tlsca.RouteToDatabase{
				ServiceName: "test",
				Protocol:    defaults.ProtocolPostgres,
				Username:    "postgres",
				Database:    "postgres",
			},
			command: `tsh db connect test`,
		},
		{
			comment:     "extra cluster flag",
			clusterFlag: "leaf",
			db: tlsca.RouteToDatabase{
				ServiceName: "test",
				Protocol:    defaults.ProtocolPostgres,
				Database:    "postgres",
			},
			command: `tsh db connect --cluster=leaf --db-user=<user> test`,
		},
	}
	for _, test := range tests {
		t.Run(test.comment, func(t *testing.T) {
			require.Equal(t, test.command, formatDatabaseConnectCommand(test.clusterFlag, test.db))
		})
	}
}

func TestKubeConfigUpdate(t *testing.T) {
	t.Parallel()
	// don't need real creds for this test, just something to compare against
	creds := &client.Key{KeyIndex: client.KeyIndex{ProxyHost: "a.example.com"}}
	tests := []struct {
		desc           string
		cf             *CLIConf
		kubeStatus     *kubernetesStatus
		errorAssertion require.ErrorAssertionFunc
		expectedValues *kubeconfig.Values
	}{
		{
			desc: "selected cluster",
			cf: &CLIConf{
				executablePath:    "/bin/tsh",
				KubernetesCluster: "dev",
			},
			kubeStatus: &kubernetesStatus{
				clusterAddr:         "https://a.example.com:3026",
				teleportClusterName: "a.example.com",
				kubeClusters: []types.KubeCluster{
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name: "dev",
						},
					},
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name: "prod",
						},
					},
				},
				credentials: creds,
			},
			errorAssertion: require.NoError,
			expectedValues: &kubeconfig.Values{
				Credentials:         creds,
				ClusterAddr:         "https://a.example.com:3026",
				TeleportClusterName: "a.example.com",
				KubeClusters:        []string{"dev"},
				SelectCluster:       "dev",
				Exec: &kubeconfig.ExecValues{
					TshBinaryPath: "/bin/tsh",
					Env:           make(map[string]string),
				},
			},
		},
		{
			desc: "selected cluster with impersonation and namespace",
			cf: &CLIConf{
				executablePath:    "/bin/tsh",
				KubernetesCluster: "dev",
				kubeNamespace:     "namespace1",
				kubernetesImpersonationConfig: impersonationConfig{
					kubernetesUser:   "user1",
					kubernetesGroups: []string{"group1", "group2"},
				},
			},
			kubeStatus: &kubernetesStatus{
				clusterAddr:         "https://a.example.com:3026",
				teleportClusterName: "a.example.com",
				kubeClusters: []types.KubeCluster{
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name: "dev",
						},
					},
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name: "prod",
						},
					},
				},
				credentials: creds,
			},
			errorAssertion: require.NoError,
			expectedValues: &kubeconfig.Values{
				Credentials:         creds,
				ClusterAddr:         "https://a.example.com:3026",
				TeleportClusterName: "a.example.com",
				Impersonate:         "user1",
				ImpersonateGroups:   []string{"group1", "group2"},
				Namespace:           "namespace1",
				KubeClusters:        []string{"dev"},
				SelectCluster:       "dev",
				Exec: &kubeconfig.ExecValues{
					TshBinaryPath: "/bin/tsh",
					Env:           make(map[string]string),
				},
			},
		},
		{
			desc: "no selected cluster",
			cf: &CLIConf{
				executablePath:    "/bin/tsh",
				KubernetesCluster: "",
			},
			kubeStatus: &kubernetesStatus{
				clusterAddr:         "https://a.example.com:3026",
				teleportClusterName: "a.example.com",
				kubeClusters: []types.KubeCluster{
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name: "dev",
						},
					},
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name: "prod",
						},
					},
				},
				credentials: creds,
			},
			errorAssertion: require.NoError,
			expectedValues: &kubeconfig.Values{
				Credentials:         creds,
				ClusterAddr:         "https://a.example.com:3026",
				TeleportClusterName: "a.example.com",
				KubeClusters:        []string{"dev", "prod"},
				SelectCluster:       "",
				Exec: &kubeconfig.ExecValues{
					TshBinaryPath: "/bin/tsh",
					Env:           make(map[string]string),
				},
			},
		},
		{
			desc: "invalid selected cluster",
			cf: &CLIConf{
				executablePath:    "/bin/tsh",
				KubernetesCluster: "invalid",
			},
			kubeStatus: &kubernetesStatus{
				clusterAddr:         "https://a.example.com:3026",
				teleportClusterName: "a.example.com",
				kubeClusters: []types.KubeCluster{
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name: "dev",
						},
					},
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name: "prod",
						},
					},
				},
				credentials: creds,
			},
			errorAssertion: func(t require.TestingT, err error, _ ...interface{}) {
				require.True(t, trace.IsNotFound(err))
			},
			expectedValues: nil,
		},
		{
			desc: "no kube clusters",
			cf: &CLIConf{
				executablePath:    "/bin/tsh",
				KubernetesCluster: "",
			},
			kubeStatus: &kubernetesStatus{
				clusterAddr:         "https://a.example.com:3026",
				teleportClusterName: "a.example.com",
				kubeClusters:        []types.KubeCluster{},
				credentials:         creds,
			},
			errorAssertion: require.NoError,
			expectedValues: &kubeconfig.Values{
				Credentials:         creds,
				ClusterAddr:         "https://a.example.com:3026",
				TeleportClusterName: "a.example.com",
				Exec:                nil,
			},
		},
		{
			desc: "no tsh path",
			cf: &CLIConf{
				executablePath:    "",
				KubernetesCluster: "dev",
			},
			kubeStatus: &kubernetesStatus{
				clusterAddr:         "https://a.example.com:3026",
				teleportClusterName: "a.example.com",
				kubeClusters: []types.KubeCluster{
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name: "dev",
						},
					},
					&types.KubernetesClusterV3{
						Metadata: types.Metadata{
							Name: "prod",
						},
					},
				},
				credentials: creds,
			},
			errorAssertion: require.NoError,
			expectedValues: &kubeconfig.Values{
				Credentials:         creds,
				ClusterAddr:         "https://a.example.com:3026",
				TeleportClusterName: "a.example.com",
				Exec:                nil,
				SelectCluster:       "dev",
			},
		},
	}
	for _, testcase := range tests {
		t.Run(testcase.desc, func(t *testing.T) {
			values, err := buildKubeConfigUpdate(testcase.cf, testcase.kubeStatus, "")
			testcase.errorAssertion(t, err)
			require.Equal(t, testcase.expectedValues, values)
		})
	}
}

func TestSetX11Config(t *testing.T) {
	t.Parallel()

	envMapGetter := func(envMap map[string]string) envGetter {
		return func(s string) string {
			return envMap[s]
		}
	}

	for _, tc := range []struct {
		desc         string
		cf           CLIConf
		opts         []string
		envMap       map[string]string
		assertError  require.ErrorAssertionFunc
		expectConfig client.Config
	}{
		// Test Teleport flag usage
		{
			desc: "-X",
			cf: CLIConf{
				X11ForwardingUntrusted: true,
			},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				EnableX11Forwarding:  true,
				X11ForwardingTrusted: false,
			},
		},
		{
			desc: "-Y",
			cf: CLIConf{
				X11ForwardingTrusted: true,
			},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				EnableX11Forwarding:  true,
				X11ForwardingTrusted: true,
			},
		},
		{
			desc: "--x11-untrustedTimeout=1m",
			cf: CLIConf{
				X11ForwardingTimeout: time.Minute,
			},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				X11ForwardingTimeout: time.Minute,
			},
		},
		{
			desc: "$DISPLAY not set",
			cf: CLIConf{
				X11ForwardingUntrusted: true,
			},
			assertError: require.Error,
			expectConfig: client.Config{
				EnableX11Forwarding: false,
			},
		},
		// Test OpenSSH flag usage
		{
			desc:        "-oForwardX11=yes",
			opts:        []string{"ForwardX11=yes"},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				EnableX11Forwarding:  true,
				X11ForwardingTrusted: true,
			},
		},
		{
			desc:        "-oForwardX11Trusted=yes",
			opts:        []string{"ForwardX11Trusted=yes"},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				X11ForwardingTrusted: true,
			},
		},
		{
			desc:        "-oForwardX11Trusted=yes",
			opts:        []string{"ForwardX11Trusted=no"},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				X11ForwardingTrusted: false,
			},
		},
		{
			desc:        "-oForwardX11=yes with -oForwardX11Trusted=yes",
			opts:        []string{"ForwardX11=yes", "ForwardX11Trusted=yes"},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				EnableX11Forwarding:  true,
				X11ForwardingTrusted: true,
			},
		},
		{
			desc:        "-oForwardX11=yes with -oForwardX11Trusted=no",
			opts:        []string{"ForwardX11=yes", "ForwardX11Trusted=no"},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				EnableX11Forwarding:  true,
				X11ForwardingTrusted: false,
			},
		},
		{
			desc:        "-oForwardX11Timeout=60",
			opts:        []string{"ForwardX11Timeout=60"},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				X11ForwardingTimeout: time.Minute,
			},
		},
		// Test Combined usage - options generally take priority
		{
			desc: "-X with -oForwardX11=yes",
			cf: CLIConf{
				X11ForwardingUntrusted: true,
			},
			opts:        []string{"ForwardX11=yes"},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				EnableX11Forwarding:  true,
				X11ForwardingTrusted: true,
			},
		},
		{
			desc: "-X with -oForwardX11Trusted=yes",
			cf: CLIConf{
				X11ForwardingUntrusted: true,
			},
			opts:        []string{"ForwardX11Trusted=yes"},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				EnableX11Forwarding:  true,
				X11ForwardingTrusted: true,
			},
		},
		{
			desc: "-X with -oForwardX11Trusted=no",
			cf: CLIConf{
				X11ForwardingUntrusted: true,
			},
			opts:        []string{"ForwardX11Trusted=no"},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				EnableX11Forwarding:  true,
				X11ForwardingTrusted: false,
			},
		},
		{
			desc: "-Y with -oForwardX11Trusted=yes",
			cf: CLIConf{
				X11ForwardingTrusted: true,
			},
			opts:        []string{"ForwardX11Trusted=yes"},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				EnableX11Forwarding:  true,
				X11ForwardingTrusted: true,
			},
		},
		{
			desc: "-Y with -oForwardX11Trusted=no",
			cf: CLIConf{
				X11ForwardingTrusted: true,
			},
			opts:        []string{"ForwardX11Trusted=no"},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				EnableX11Forwarding:  true,
				X11ForwardingTrusted: true,
			},
		},
		{
			desc: "--x11-untrustedTimeout=1m with -oForwardX11Timeout=120",
			cf: CLIConf{
				X11ForwardingTimeout: time.Minute,
			},
			opts:        []string{"ForwardX11Timeout=120"},
			envMap:      map[string]string{x11.DisplayEnv: ":0"},
			assertError: require.NoError,
			expectConfig: client.Config{
				X11ForwardingTimeout: time.Minute * 2,
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			opts, err := parseOptions(tc.opts)
			require.NoError(t, err)

			clt := client.Config{}
			err = setX11Config(&clt, &tc.cf, opts, envMapGetter(tc.envMap))
			tc.assertError(t, err)
			require.Equal(t, tc.expectConfig, clt)
		})
	}
}

// TestAuthClientFromTSHProfile tests if API Client can be successfully created from tsh profile where clusters
// certs are stored separately in CAS directory and in case where legacy certs.pem file was used.
func TestAuthClientFromTSHProfile(t *testing.T) {
	t.Parallel()

	tmpHomePath := t.TempDir()

	connector := mockConnector(t)
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})
	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, alice))
	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)
	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
		cf.MockSSOLogin = mockSSOLogin(t, authServer, alice)
		return nil
	})
	require.NoError(t, err)

	profile, err := profile.FromDir(tmpHomePath, "")
	require.NoError(t, err)

	mustCreateAuthClientFormUserProfile(t, tmpHomePath, proxyAddr.String())

	// Simulate legacy tsh client behavior where all clusters certs were stored in the certs.pem file.
	require.NoError(t, os.RemoveAll(profile.TLSClusterCASDir()))

	// Verify that authClient created from profile will create a valid client in case where cas dir doesn't exit.
	mustCreateAuthClientFormUserProfile(t, tmpHomePath, proxyAddr.String())
}

type testServersOpts struct {
	bootstrap   []types.Resource
	configFuncs []func(cfg *servicecfg.Config)
}

type testServerOptFunc func(o *testServersOpts)

func withBootstrap(bootstrap ...types.Resource) testServerOptFunc {
	return func(o *testServersOpts) {
		o.bootstrap = bootstrap
	}
}

func withConfig(fn func(cfg *servicecfg.Config)) testServerOptFunc {
	return func(o *testServersOpts) {
		o.configFuncs = append(o.configFuncs, fn)
	}
}

func withAuthConfig(fn func(*servicecfg.AuthConfig)) testServerOptFunc {
	return withConfig(func(cfg *servicecfg.Config) {
		fn(&cfg.Auth)
	})
}

func withClusterName(t *testing.T, n string) testServerOptFunc {
	return withAuthConfig(func(cfg *servicecfg.AuthConfig) {
		clusterName, err := services.NewClusterNameWithRandomID(
			types.ClusterNameSpecV2{
				ClusterName: n,
			})
		require.NoError(t, err)
		cfg.ClusterName = clusterName
	})
}

func withMOTD(t *testing.T, motd string) testServerOptFunc {
	oldStdin := prompt.Stdin()
	t.Cleanup(func() {
		prompt.SetStdin(oldStdin)
	})
	prompt.SetStdin(prompt.NewFakeReader().
		AddString(""). // 3x to allow multiple logins
		AddString("").
		AddString(""))
	return withAuthConfig(func(cfg *servicecfg.AuthConfig) {
		fmt.Printf("\n\n Setting MOTD: '%s' \n\n", motd)
		cfg.Preference.SetMessageOfTheDay(motd)
	})
}

func withHostname(hostname string) testServerOptFunc {
	return withConfig(func(cfg *servicecfg.Config) {
		cfg.Hostname = hostname
	})
}

func withSSHLabel(key, value string) testServerOptFunc {
	return withConfig(func(cfg *servicecfg.Config) {
		if cfg.SSH.Labels == nil {
			cfg.SSH.Labels = make(map[string]string)
		}
		cfg.SSH.Labels[key] = value
	})
}

func makeTestSSHNode(t *testing.T, authAddr *utils.NetAddr, opts ...testServerOptFunc) *service.TeleportProcess {
	var options testServersOpts
	for _, opt := range opts {
		opt(&options)
	}

	// Set up a test ssh service.
	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.Hostname = "node"
	cfg.DataDir = t.TempDir()

	cfg.SetAuthServerAddress(*authAddr)
	cfg.SetToken(staticToken)
	cfg.Auth.Enabled = false
	cfg.Proxy.Enabled = false
	cfg.SSH.Enabled = true
	cfg.SSH.Addr = *utils.MustParseAddr("127.0.0.1:0")
	cfg.SSH.PublicAddrs = []utils.NetAddr{cfg.SSH.Addr}
	cfg.SSH.DisableCreateHostUser = true
	cfg.Log = utils.NewLoggerForTests()

	for _, fn := range options.configFuncs {
		fn(cfg)
	}

	return runTeleport(t, cfg)
}

func makeTestServers(t *testing.T, opts ...testServerOptFunc) (auth *service.TeleportProcess, proxy *service.TeleportProcess) {
	t.Helper()

	var options testServersOpts
	for _, opt := range opts {
		opt(&options)
	}

	authAddr := utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("127.0.0.1", ports.Pop())}
	var err error
	// Set up a test auth server.
	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.Hostname = "localhost"
	cfg.DataDir = t.TempDir()
	cfg.SetAuthServerAddress(authAddr)
	cfg.Auth.BootstrapResources = options.bootstrap
	cfg.Auth.StorageConfig.Params = backend.Params{defaults.BackendPath: filepath.Join(cfg.DataDir, defaults.BackendDir)}
	cfg.Auth.StaticTokens, err = types.NewStaticTokens(types.StaticTokensSpecV2{
		StaticTokens: []types.ProvisionTokenV1{{
			Roles:   []types.SystemRole{types.RoleProxy, types.RoleDatabase, types.RoleTrustedCluster, types.RoleNode, types.RoleApp},
			Expires: time.Now().Add(time.Minute),
			Token:   staticToken,
		}},
	})
	require.NoError(t, err)
	cfg.SetToken(staticToken)
	cfg.SSH.Enabled = false
	cfg.Auth.Enabled = true
	cfg.Auth.ListenAddr = authAddr
	cfg.Proxy.Enabled = true
	cfg.Proxy.WebAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("127.0.0.1", ports.Pop())}
	cfg.Proxy.SSHAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("127.0.0.1", ports.Pop())}
	cfg.Proxy.ReverseTunnelListenAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("127.0.0.1", ports.Pop())}
	cfg.Proxy.DisableWebInterface = true
	cfg.Log = utils.NewLoggerForTests()

	for _, fn := range options.configFuncs {
		fn(cfg)
	}

	auth = runTeleport(t, cfg)

	// Wait for auth to become ready.
	_, err = auth.WaitForEventTimeout(30*time.Second, service.AuthTLSReady)
	// in reality, the auth server should start *much* sooner than this.  we use a very large
	// timeout here because this isn't the kind of problem that this test is meant to catch.
	require.NoError(t, err, "auth server didn't start after 30s")

	return auth, auth
}

func mockConnector(t *testing.T) types.OIDCConnector {
	// Connector need not be functional since we are going to mock the actual
	// login operation.
	connector, err := types.NewOIDCConnector("auth.example.com", types.OIDCConnectorSpecV3{
		IssuerURL:    "https://auth.example.com",
		RedirectURLs: []string{"https://cluster.example.com"},
		ClientID:     "fake-client",
		ClaimsToRoles: []types.ClaimMapping{
			{
				Claim: "groups",
				Value: "dummy",
				Roles: []string{"dummy"},
			},
		},
	})
	require.NoError(t, err)
	return connector
}

func mockSSOLogin(t *testing.T, authServer *auth.Server, user types.User) client.SSOLoginFunc {
	return func(ctx context.Context, connectorID string, priv *keys.PrivateKey, protocol string) (*auth.SSHLoginResponse, error) {
		// generate certificates for our user
		clusterName, err := authServer.GetClusterName()
		require.NoError(t, err)
		sshCert, tlsCert, err := authServer.GenerateUserTestCerts(auth.GenerateUserTestCertsRequest{
			Key:            priv.MarshalSSHPublicKey(),
			Username:       user.GetName(),
			TTL:            time.Hour,
			Compatibility:  constants.CertificateFormatStandard,
			RouteToCluster: clusterName.GetClusterName(),
		})
		require.NoError(t, err)

		// load CA cert
		authority, err := authServer.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, false)
		require.NoError(t, err)

		// build login response
		return &auth.SSHLoginResponse{
			Username:    user.GetName(),
			Cert:        sshCert,
			TLSCert:     tlsCert,
			HostSigners: auth.AuthoritiesToTrustedCerts([]types.CertAuthority{authority}),
		}, nil
	}
}

func mockHeadlessLogin(t *testing.T, authServer *auth.Server, user types.User) client.SSHLoginFunc {
	return func(ctx context.Context, priv *keys.PrivateKey) (*auth.SSHLoginResponse, error) {
		// generate certificates for our user
		clusterName, err := authServer.GetClusterName()
		require.NoError(t, err)
		sshCert, tlsCert, err := authServer.GenerateUserTestCerts(auth.GenerateUserTestCertsRequest{
			Key:            priv.MarshalSSHPublicKey(),
			Username:       user.GetName(),
			TTL:            time.Hour,
			Compatibility:  constants.CertificateFormatStandard,
			RouteToCluster: clusterName.GetClusterName(),
			MFAVerified:    "mfa-verified",
		})
		require.NoError(t, err)

		// load CA cert
		authority, err := authServer.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, false)
		require.NoError(t, err)

		// build login response
		return &auth.SSHLoginResponse{
			Username:    user.GetName(),
			Cert:        sshCert,
			TLSCert:     tlsCert,
			HostSigners: auth.AuthoritiesToTrustedCerts([]types.CertAuthority{authority}),
		}, nil
	}
}

func setOverrideStdout(stdout io.Writer) CliOption {
	return func(cf *CLIConf) error {
		cf.OverrideStdout = stdout
		return nil
	}
}

func setCopyStdout(stdout io.Writer) CliOption {
	return setOverrideStdout(io.MultiWriter(os.Stdout, stdout))
}

func setHomePath(path string) CliOption {
	return func(cf *CLIConf) error {
		cf.HomePath = path
		return nil
	}
}

func setKubeConfigPath(path string) CliOption {
	return func(cf *CLIConf) error {
		cf.KubeConfigPath = path
		return nil
	}
}

func setIdentity(path string) CliOption {
	return func(cf *CLIConf) error {
		cf.IdentityFileIn = path
		return nil
	}
}

func setCmdRunner(cmdRunner func(*exec.Cmd) error) CliOption {
	return func(cf *CLIConf) error {
		cf.cmdRunner = cmdRunner
		return nil
	}
}

func testSerialization(t *testing.T, expected string, serializer func(string) (string, error)) {
	out, err := serializer(teleport.JSON)
	require.NoError(t, err)
	require.JSONEq(t, expected, out)

	out, err = serializer(teleport.YAML)
	require.NoError(t, err)
	outJSON, err := yaml.YAMLToJSON([]byte(out))
	require.NoError(t, err)
	require.JSONEq(t, expected, string(outJSON))
}

func TestSerializeVersion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		expected string

		proxyVersion       string
		proxyPublicAddress string
	}{
		{
			name: "no proxy version provided",
			expected: fmt.Sprintf(
				`{"version": %q, "gitref": %q, "runtime": %q}`,
				teleport.Version, teleport.Gitref, runtime.Version(),
			),
		},
		{
			name:               "proxy version provided",
			proxyVersion:       "1.33.7",
			proxyPublicAddress: "teleport.example.com:443",
			expected: fmt.Sprintf(
				`{"version": %q, "gitref": %q, "runtime": %q, "proxyVersion": %q, "proxyPublicAddress": %q}`,
				teleport.Version, teleport.Gitref, runtime.Version(), "1.33.7", "teleport.example.com:443"),
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			testSerialization(t, tC.expected, func(fmt string) (string, error) {
				return serializeVersion(fmt, tC.proxyVersion, tC.proxyPublicAddress)
			})
		})
	}
}

func TestSerializeApps(t *testing.T) {
	t.Parallel()

	expected := `
	[{
		"kind": "app",
		"version": "v3",
		"metadata": {
			"name": "my app",
			"description": "this is the description",
			"labels": {
				"a": "1",
				"b": "2"
			}
		},
		"spec": {
			"uri": "https://example.com",
			"insecure_skip_verify": false
		}
	}]
	`
	app, err := types.NewAppV3(types.Metadata{
		Name:        "my app",
		Description: "this is the description",
		Labels:      map[string]string{"a": "1", "b": "2"},
	}, types.AppSpecV3{
		URI: "https://example.com",
	})
	require.NoError(t, err)
	testSerialization(t, expected, func(f string) (string, error) {
		return serializeApps([]types.Application{app}, f)
	})
}

func TestSerializeAppsEmpty(t *testing.T) {
	t.Parallel()

	testSerialization(t, "[]", func(f string) (string, error) {
		return serializeApps(nil, f)
	})
}

func TestSerializeAppConfig(t *testing.T) {
	t.Parallel()

	expected := `
	{
		"name": "my app",
		"uri": "https://example.com",
		"ca": "/path/to/ca",
		"cert": "/path/to/cert",
		"key": "/path/to/key",
		"curl": "curl https://example.com"
	}
	`
	appConfig := &appConfigInfo{
		Name: "my app",
		URI:  "https://example.com",
		CA:   "/path/to/ca",
		Cert: "/path/to/cert",
		Key:  "/path/to/key",
		Curl: "curl https://example.com",
	}
	testSerialization(t, expected, func(f string) (string, error) {
		return serializeAppConfig(appConfig, f)
	})
}

func TestSerializeDatabases(t *testing.T) {
	t.Parallel()

	expectedFmt := `
	[{
    "kind": "db",
    "version": "v3",
    "metadata": {
      "name": "my db",
      "description": "this is the description",
      "labels": {"a": "1", "b": "2"}
    },
    "spec": {
      "protocol": "mongodb",
      "uri": "mongodb://example.com",
      "aws": {
        "redshift": {},
        "rds": {
          "iam_auth": false
        },
        "elasticache": {},
        "secret_store": {},
        "memorydb": {},
        "rdsproxy": {},
        "redshift_serverless": {}
      },
      "mysql": {},
      "gcp": {},
      "azure": {
	    "redis": {}
	  },
      "tls": {
        "mode": 0
      },
      "ad": {
        "domain": "",
        "spn": ""
      },
      "mongo_atlas": {}
    },
    "status": {
      "mysql": {},
      "aws": {
        "redshift": {},
        "rds": {
          "iam_auth": false
        },
        "elasticache": {},
        "secret_store": {},
        "memorydb": {},
        "rdsproxy": {},
        "redshift_serverless": {}
      },
      "azure": {
	    "redis": {}
	  }
    }%v
  }]
	`
	db, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my db",
		Description: "this is the description",
		Labels:      map[string]string{"a": "1", "b": "2"},
	}, types.DatabaseSpecV3{
		Protocol: "mongodb",
		URI:      "mongodb://example.com",
	})
	require.NoError(t, err)

	tests := []struct {
		name        string
		dbUsersData string
		roles       services.RoleSet
	}{
		{
			name: "without db users",
		},
		{
			name: "with wildcard in allowed db users",
			dbUsersData: `,
    "users": {
      "allowed": [
        "*",
        "bar",
        "foo"
      ],
      "denied": [
        "baz",
        "qux"
      ]
     }`,
			roles: services.RoleSet{
				&types.RoleV6{
					Metadata: types.Metadata{Name: "role1", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							Namespaces:     []string{apidefaults.Namespace},
							DatabaseLabels: types.Labels{"*": []string{"*"}},
							DatabaseUsers:  []string{"foo", "bar", "*"},
						},
						Deny: types.RoleConditions{
							Namespaces:    []string{apidefaults.Namespace},
							DatabaseUsers: []string{"baz", "qux"},
						},
					},
				},
			},
		},
		{
			name: "without wildcard in allowed db users",
			dbUsersData: `,
    "users": {
      "allowed": [
        "bar",
        "foo"
      ]
     }`,
			roles: services.RoleSet{
				&types.RoleV6{
					Metadata: types.Metadata{Name: "role2", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							Namespaces:     []string{apidefaults.Namespace},
							DatabaseLabels: types.Labels{"*": []string{"*"}},
							DatabaseUsers:  []string{"foo", "bar"},
						},
						Deny: types.RoleConditions{
							Namespaces:    []string{apidefaults.Namespace},
							DatabaseUsers: []string{"baz", "qux"},
						},
					},
				},
			},
		},
		{
			name: "with no denied db users",
			dbUsersData: `,
    "users": {
      "allowed": [
        "*",
        "bar",
        "foo"
      ]
     }`,
			roles: services.RoleSet{
				&types.RoleV6{
					Metadata: types.Metadata{Name: "role2", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							Namespaces:     []string{apidefaults.Namespace},
							DatabaseLabels: types.Labels{"*": []string{"*"}},
							DatabaseUsers:  []string{"*", "foo", "bar"},
						},
						Deny: types.RoleConditions{
							Namespaces:    []string{apidefaults.Namespace},
							DatabaseUsers: []string{},
						},
					},
				},
			},
		},
		{
			name: "with no allowed db users",
			dbUsersData: `,
    "users": {}`,
			roles: services.RoleSet{
				&types.RoleV6{
					Metadata: types.Metadata{Name: "role2", Namespace: apidefaults.Namespace},
					Spec: types.RoleSpecV6{
						Allow: types.RoleConditions{
							Namespaces:     []string{apidefaults.Namespace},
							DatabaseLabels: types.Labels{"*": []string{"*"}},
							DatabaseUsers:  []string{},
						},
						Deny: types.RoleConditions{
							Namespaces:    []string{apidefaults.Namespace},
							DatabaseUsers: []string{"baz", "qux"},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			accessChecker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clustername", tt.roles)
			expected := fmt.Sprintf(expectedFmt, tt.dbUsersData)
			testSerialization(t, expected, func(f string) (string, error) {
				return serializeDatabases([]types.Database{db}, f, accessChecker)
			})
		})
	}
}

func TestSerializeDatabasesEmpty(t *testing.T) {
	t.Parallel()

	testSerialization(t, "[]", func(f string) (string, error) {
		return serializeDatabases(nil, f, nil)
	})
}

func TestSerializeDatabaseEnvironment(t *testing.T) {
	t.Parallel()

	expected := `
	{
		"A": "1",
		"B": "2"
	}
	`
	env := map[string]string{
		"A": "1",
		"B": "2",
	}
	testSerialization(t, expected, func(f string) (string, error) {
		return serializeDatabaseEnvironment(env, f)
	})
}

func TestSerializeDatabaseConfig(t *testing.T) {
	t.Parallel()

	expected := `
	{
		"name": "my db",
		"host": "example.com",
		"port": 27017,
		"ca": "/path/to/ca",
		"cert": "/path/to/cert",
		"key": "/path/to/key"
	}
	`
	configInfo := &dbConfigInfo{
		Name: "my db",
		Host: "example.com",
		Port: 27017,
		CA:   "/path/to/ca",
		Cert: "/path/to/cert",
		Key:  "/path/to/key",
	}
	testSerialization(t, expected, func(f string) (string, error) {
		return serializeDatabaseConfig(configInfo, f)
	})
}

func TestSerializeNodes(t *testing.T) {
	t.Parallel()

	expected := `
	[{
    "kind": "node",
    "version": "v2",
    "metadata": {
      "name": "my server"
    },
    "spec": {
      "addr": "https://example.com",
      "hostname": "example.com",
      "rotation": {
        "current_id": "",
        "started": "0001-01-01T00:00:00Z",
        "last_rotated": "0001-01-01T00:00:00Z",
        "schedule": {
          "update_clients": "0001-01-01T00:00:00Z",
          "update_servers": "0001-01-01T00:00:00Z",
          "standby": "0001-01-01T00:00:00Z"
        }
      },
      "version": "v2"
    }
  }]
	`
	node, err := types.NewServer("my server", "node", types.ServerSpecV2{
		Addr:     "https://example.com",
		Hostname: "example.com",
		Version:  "v2",
	})
	require.NoError(t, err)
	testSerialization(t, expected, func(f string) (string, error) {
		return serializeNodes([]types.Server{node}, f)
	})
}

func TestSerializeNodesEmpty(t *testing.T) {
	t.Parallel()

	testSerialization(t, "[]", func(f string) (string, error) {
		return serializeNodes(nil, f)
	})
}

func TestSerializeClusters(t *testing.T) {
	t.Parallel()

	expected := `
	[
		{
			"cluster_name": "rootCluster",
			"status": "online",
			"cluster_type": "root",
			"labels": null,
			"selected": true
		},
		{
			"cluster_name": "leafCluster",
			"status": "offline",
			"cluster_type": "leaf",
			"labels": {"foo": "bar", "baz": "boof"},
			"selected": false
		}
	]
	`
	root := clusterInfo{
		ClusterName: "rootCluster",
		Status:      teleport.RemoteClusterStatusOnline,
		ClusterType: "root",
		Selected:    true,
	}
	leafClusters := []clusterInfo{
		{
			ClusterName: "leafCluster",
			Status:      teleport.RemoteClusterStatusOffline,
			ClusterType: "leaf",
			Labels: map[string]string{
				"foo": "bar",
				"baz": "boof",
			},
			Selected: false,
		},
	}
	testSerialization(t, expected, func(f string) (string, error) {
		return serializeClusters(root, leafClusters, f)
	})
}

func TestSerializeProfiles(t *testing.T) {
	t.Parallel()

	expected := `
	{
  "active": {
    "profile_url": "example.com",
    "username": "test",
    "active_requests": [
      "1",
      "2",
      "3"
    ],
    "cluster": "main",
    "roles": [
      "a",
      "b",
      "c"
    ],
    "traits": {
      "a": [
  "1",
  "2",
  "3"
]
    },
    "logins": [
      "a",
      "b",
      "c"
    ],
    "kubernetes_enabled": true,
    "kubernetes_users": [
      "x"
    ],
    "kubernetes_groups": [
      "y"
    ],
    "databases": [
      "z"
    ],
    "valid_until": "1970-01-01T00:00:00Z",
    "extensions": [
      "7",
      "8",
      "9"
    ]
  },
  "profiles": [
    {
      "profile_url": "example.com",
      "username": "test2",
      "cluster": "other",
      "kubernetes_enabled": false,
      "valid_until": "1970-01-01T00:00:00Z"
    }
  ]
}
	`
	aTime := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	p, err := url.Parse("example.com")
	require.NoError(t, err)
	activeProfile := &client.ProfileStatus{
		ProxyURL:       *p,
		Username:       "test",
		ActiveRequests: services.RequestIDs{AccessRequests: []string{"1", "2", "3"}},
		Cluster:        "main",
		Roles:          []string{"a", "b", "c"},
		Traits:         wrappers.Traits{"a": []string{"1", "2", "3"}},
		Logins:         []string{"a", "b", "c"},
		KubeEnabled:    true,
		KubeUsers:      []string{"x"},
		KubeGroups:     []string{"y"},
		Databases:      []tlsca.RouteToDatabase{{ServiceName: "z"}},
		ValidUntil:     aTime,
		Extensions:     []string{"7", "8", "9"},
	}
	otherProfile := &client.ProfileStatus{
		ProxyURL:   *p,
		Username:   "test2",
		Cluster:    "other",
		ValidUntil: aTime,
	}

	testSerialization(t, expected, func(f string) (string, error) {
		activeInfo, othersInfo := makeAllProfileInfo(activeProfile, []*client.ProfileStatus{otherProfile}, nil)
		return serializeProfiles(activeInfo, othersInfo, nil, f)
	})
}

func TestSerializeProfilesNoOthers(t *testing.T) {
	t.Parallel()

	expected := `
	{
		"active": {
      "profile_url": "example.com",
      "username": "test",
      "cluster": "main",
      "kubernetes_enabled": false,
      "valid_until": "1970-01-01T00:00:00Z"
    },
		"profiles": []
	}
	`
	aTime := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	p, err := url.Parse("example.com")
	require.NoError(t, err)
	profile := &client.ProfileStatus{
		ProxyURL:   *p,
		Username:   "test",
		Cluster:    "main",
		ValidUntil: aTime,
	}
	testSerialization(t, expected, func(f string) (string, error) {
		active, _ := makeAllProfileInfo(profile, nil, nil)
		return serializeProfiles(active, nil, nil, f)
	})
}

func TestSerializeProfilesNoActive(t *testing.T) {
	t.Parallel()

	expected := `
	{
		"profiles": []
	}
	`
	testSerialization(t, expected, func(f string) (string, error) {
		return serializeProfiles(nil, nil, nil, f)
	})
}

func TestSerializeProfilesWithEnvVars(t *testing.T) {
	cluster := "someCluster"
	siteName := "someSiteName"
	proxy := "example.com"
	kubeCluster := "someKubeCluster"
	kubeConfig := "someKubeConfigPath"
	t.Setenv(proxyEnvVar, proxy)
	t.Setenv(clusterEnvVar, cluster)
	t.Setenv(siteEnvVar, siteName)
	t.Setenv(kubeClusterEnvVar, kubeCluster)
	t.Setenv(teleport.EnvKubeConfig, kubeConfig)
	expected := fmt.Sprintf(`
{
  "profiles": [],
  "environment": {
    %q: %q,
    %q: %q,
    %q: %q,
    %q: %q,
    %q: %q
  }
}
`, teleport.EnvKubeConfig, kubeConfig,
		clusterEnvVar, cluster,
		kubeClusterEnvVar, kubeCluster,
		proxyEnvVar, proxy,
		siteEnvVar, siteName)
	testSerialization(t, expected, func(f string) (string, error) {
		env := getTshEnv()
		return serializeProfiles(nil, nil, env, f)
	})
}

func TestSerializeEnvironment(t *testing.T) {
	t.Parallel()

	expected := fmt.Sprintf(`
	{
		%q: "example.com",
		%q: "main"
	}
	`, proxyEnvVar, clusterEnvVar)
	p, err := url.Parse("https://example.com")
	require.NoError(t, err)
	profile := &client.ProfileStatus{
		ProxyURL: *p,
		Cluster:  "main",
	}
	testSerialization(t, expected, func(f string) (string, error) {
		return serializeEnvironment(profile, f)
	})
}

func TestSerializeAccessRequests(t *testing.T) {
	t.Parallel()

	expected := `
	{
    "kind": "access_request",
    "version": "v3",
    "metadata": {
      "name": "test"
    },
    "spec": {
      "user": "user",
      "roles": [
        "a",
        "b",
        "c"
      ],
      "state": 1,
      "created": "0001-01-01T00:00:00Z",
      "expires": "0001-01-01T00:00:00Z"
    }
  }
	`
	req, err := types.NewAccessRequest("test", "user", "a", "b", "c")
	require.NoError(t, err)
	testSerialization(t, expected, func(f string) (string, error) {
		return serializeAccessRequest(req, f)
	})
	expected2 := fmt.Sprintf("[%v]", expected)
	testSerialization(t, expected2, func(f string) (string, error) {
		return serializeAccessRequests([]types.AccessRequest{req}, f)
	})
}

func TestSerializeKubeSessions(t *testing.T) {
	t.Parallel()

	aTime := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	expected := `
	[
  {
    "kind": "session_tracker",
    "version": "v1",
    "metadata": {
      "name": "id",
      "expires": "1970-01-01T00:00:00Z"
    },
    "spec": {
      "session_id": "id",
      "kind": "session-kind",
      "state": 1,
      "created": "1970-01-01T00:00:00Z",
      "expires": "1970-01-01T00:00:00Z",
      "attached": "arbitrary attached data",
      "reason": "some reason",
      "invited": [
        "a",
        "b",
        "c"
      ],
      "target_hostname": "example.com",
      "target_address": "https://example.com",
      "cluster_name": "cluster",
      "login": "login",
      "participants": [
        {
          "id": "some-id",
          "user": "test",
          "mode": "mode",
          "last_active": "1970-01-01T00:00:00Z"
        }
      ],
      "kubernetes_cluster": "kc",
      "host_user": "test",
      "host_roles": [
        {
          "name": "policy",
          "version": "v1",
          "require_session_join": [
            {
              "name": "policy",
              "filter": "filter",
              "kinds": [
                "x",
                "y",
                "z"
              ],
              "count": 1,
              "modes": [
                "mode",
                "mode-1",
                "mode-2"
              ],
              "on_leave": "do something"
            }
          ]
        }
      ]
    }
  }
]
	`
	tracker, err := types.NewSessionTracker(types.SessionTrackerSpecV1{
		SessionID:    "id",
		Kind:         "session-kind",
		State:        types.SessionState_SessionStateRunning,
		Created:      aTime,
		Expires:      aTime,
		AttachedData: "arbitrary attached data",
		Reason:       "some reason",
		Invited:      []string{"a", "b", "c"},
		Hostname:     "example.com",
		Address:      "https://example.com",
		ClusterName:  "cluster",
		Login:        "login",
		Participants: []types.Participant{
			{
				ID:         "some-id",
				User:       "test",
				Mode:       "mode",
				LastActive: aTime,
			},
		},
		KubernetesCluster: "kc",
		HostUser:          "test",
		HostPolicies: []*types.SessionTrackerPolicySet{
			{
				Name:    "policy",
				Version: "v1",
				RequireSessionJoin: []*types.SessionRequirePolicy{
					{
						Name:    "policy",
						Filter:  "filter",
						Kinds:   []string{"x", "y", "z"},
						Count:   1,
						Modes:   []string{"mode", "mode-1", "mode-2"},
						OnLeave: "do something",
					},
				},
			},
		},
	})
	require.NoError(t, err)
	testSerialization(t, expected, func(f string) (string, error) {
		return serializeKubeSessions([]types.SessionTracker{tracker}, f)
	})
}

func TestSerializeKubeClusters(t *testing.T) {
	t.Parallel()

	expected := `
	[
		{
			"kube_cluster_name": "cluster1",
			"labels": {"cmd": "result", "foo": "bar"},
			"selected": true
		},
		{
			"kube_cluster_name": "cluster2",
			"labels": null,
			"selected": false
		}
	]
	`
	c1, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: "cluster1",
			Labels: map[string]string{
				"foo": "bar",
			},
		},
		types.KubernetesClusterSpecV3{
			DynamicLabels: map[string]types.CommandLabelV2{
				"cmd": {
					Result: "result",
				},
			},
		},
	)
	require.NoError(t, err)
	c2, err := types.NewKubernetesClusterV3(
		types.Metadata{
			Name: "cluster2",
		},
		types.KubernetesClusterSpecV3{},
	)

	require.NoError(t, err)
	clusters := []types.KubeCluster{
		c1, c2,
	}
	testSerialization(t, expected, func(f string) (string, error) {
		return serializeKubeClusters(clusters, "cluster1", f)
	})
}

func TestSerializeMFADevices(t *testing.T) {
	t.Parallel()

	aTime := time.Date(1970, time.January, 1, 0, 0, 0, 0, time.UTC)
	expected := `
	[
  {"metadata":{"Name":"my device"},"id":"id","addedAt":"1970-01-01T00:00:00Z","lastUsed":"1970-01-01T00:00:00Z"}
	]
	`
	dev := types.NewMFADevice("my device", "id", aTime)
	testSerialization(t, expected, func(f string) (string, error) {
		return serializeMFADevices([]*types.MFADevice{dev}, f)
	})
}

func TestListDatabasesWithUsers(t *testing.T) {
	t.Parallel()
	dbStage, err := types.NewDatabaseV3(types.Metadata{
		Name:   "stage",
		Labels: map[string]string{"env": "stage"},
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)

	dbProd, err := types.NewDatabaseV3(types.Metadata{
		Name:   "prod",
		Labels: map[string]string{"env": "prod"},
	}, types.DatabaseSpecV3{
		Protocol: "protocol",
		URI:      "uri",
	})
	require.NoError(t, err)

	roleDevStage := &types.RoleV6{
		Metadata: types.Metadata{Name: "dev-stage", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"stage"}},
				DatabaseUsers:  []string{types.Wildcard},
			},
			Deny: types.RoleConditions{
				Namespaces:    []string{apidefaults.Namespace},
				DatabaseUsers: []string{"superuser"},
			},
		},
	}
	roleDevProd := &types.RoleV6{
		Metadata: types.Metadata{Name: "dev-prod", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"prod"}},
				DatabaseUsers:  []string{"dev"},
			},
		},
	}

	tests := []struct {
		name      string
		roles     services.RoleSet
		database  types.Database
		wantUsers *dbUsers
		wantText  string
	}{
		{
			name:     "developer allowed any username in stage database except superuser",
			roles:    services.RoleSet{roleDevStage, roleDevProd},
			database: dbStage,
			wantUsers: &dbUsers{
				Allowed: []string{"*", "dev"},
				Denied:  []string{"superuser"},
			},
			wantText: "[* dev], except: [superuser]",
		},
		{
			name:     "developer allowed only specific username/database in prod database",
			roles:    services.RoleSet{roleDevStage, roleDevProd},
			database: dbProd,
			wantUsers: &dbUsers{
				Allowed: []string{"dev"},
			},
			wantText: "[dev]",
		},
		{
			name:     "roleDevStage x dbStage",
			roles:    services.RoleSet{roleDevStage},
			database: dbStage,
			wantUsers: &dbUsers{
				Allowed: []string{"*"},
				Denied:  []string{"superuser"},
			},
			wantText: "[*], except: [superuser]",
		},
		{
			name:      "roleDevStage x dbProd",
			roles:     services.RoleSet{roleDevStage},
			database:  dbProd,
			wantUsers: &dbUsers{},
			wantText:  "(none)",
		},
		{
			name:      "roleDevProd x dbStage",
			roles:     services.RoleSet{roleDevProd},
			database:  dbStage,
			wantUsers: &dbUsers{},
			wantText:  "(none)",
		},
		{
			name:     "roleDevProd x dbProd",
			roles:    services.RoleSet{roleDevProd},
			database: dbProd,
			wantUsers: &dbUsers{
				Allowed: []string{"dev"},
			},
			wantText: "[dev]",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accessChecker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{}, "clustername", tt.roles)

			gotUsers := getDBUsers(tt.database, accessChecker)
			require.Equal(t, tt.wantUsers, gotUsers)

			gotText := formatUsersForDB(tt.database, accessChecker)
			require.Equal(t, tt.wantText, gotText)
		})
	}
}

func spanAssertion(containsTSH, empty bool) func(t *testing.T, spans []*otlp.ScopeSpans) {
	return func(t *testing.T, spans []*otlp.ScopeSpans) {
		if empty {
			require.Empty(t, spans)
			return
		}

		require.NotEmpty(t, spans)

		var scopes []string
		for _, span := range spans {
			scopes = append(scopes, span.Scope.Name)
		}

		if containsTSH {
			require.Contains(t, scopes, teleport.ComponentTSH)
		} else {
			require.NotContains(t, scopes, teleport.ComponentTSH)
		}
	}
}

func TestForwardingTraces(t *testing.T) {
	cases := []struct {
		name          string
		cfg           func(c *tracing.Collector) servicecfg.TracingConfig
		spanAssertion func(t *testing.T, spans []*otlp.ScopeSpans)
	}{
		{
			name: "spans exported with auth sampling all",
			cfg: func(c *tracing.Collector) servicecfg.TracingConfig {
				return servicecfg.TracingConfig{
					Enabled:      true,
					ExporterURL:  c.GRPCAddr(),
					SamplingRate: 1.0,
				}
			},
			spanAssertion: spanAssertion(true, false),
		},
		{
			name: "spans exported with auth sampling none",
			cfg: func(c *tracing.Collector) servicecfg.TracingConfig {
				return servicecfg.TracingConfig{
					Enabled:      true,
					ExporterURL:  c.HTTPAddr(),
					SamplingRate: 0.0,
				}
			},
			spanAssertion: spanAssertion(true, false),
		},
		{
			name: "spans not exported when tracing disabled",
			cfg: func(c *tracing.Collector) servicecfg.TracingConfig {
				return servicecfg.TracingConfig{}
			},
			spanAssertion: func(t *testing.T, spans []*otlp.ScopeSpans) {
				require.Empty(t, spans)
			},
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			tmpHomePath := t.TempDir()

			connector := mockConnector(t)
			alice, err := types.NewUser("alice@example.com")
			require.NoError(t, err)
			alice.SetRoles([]string{"access"})

			collector, err := tracing.NewCollector(tracing.CollectorConfig{})
			require.NoError(t, err)

			errCh := make(chan error)
			go func() {
				errCh <- collector.Start()
			}()
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
				defer cancel()
				require.NoError(t, collector.Shutdown(ctx))
				require.NoError(t, <-errCh)
			})

			traceCfg := tt.cfg(collector)
			authProcess, proxyProcess := makeTestServers(t,
				withBootstrap(connector, alice),
				withConfig(func(cfg *servicecfg.Config) {
					cfg.Tracing = traceCfg
				}),
			)

			authServer := authProcess.GetAuthServer()
			require.NotNil(t, authServer)

			proxyAddr, err := proxyProcess.ProxyWebAddr()
			require.NoError(t, err)

			// --trace should have no impact on login, since login is ignored
			err = Run(context.Background(), []string{
				"login",
				"--insecure",
				"--auth", connector.GetName(),
				"--proxy", proxyAddr.String(),
				"--trace",
			}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
				cf.MockSSOLogin = mockSSOLogin(t, authServer, alice)
				return nil
			})
			require.NoError(t, err)

			if traceCfg.Enabled && traceCfg.SamplingRate > 0 {
				collector.WaitForExport()
			}

			// ensure login doesn't generate any spans from tsh if spans are being sampled
			loginAssertion := spanAssertion(false, !traceCfg.Enabled || traceCfg.SamplingRate <= 0)
			loginAssertion(t, collector.Spans())

			err = Run(context.Background(), []string{
				"ls",
				"--insecure",
				"--auth", connector.GetName(),
				"--proxy", proxyAddr.String(),
				"--trace",
			}, setHomePath(tmpHomePath))
			require.NoError(t, err)

			if traceCfg.Enabled {
				collector.WaitForExport()
			}

			tt.spanAssertion(t, collector.Spans())
		})
	}
}

func TestExportingTraces(t *testing.T) {
	cases := []struct {
		name                  string
		cfg                   func(c *tracing.Collector) servicecfg.TracingConfig
		teleportSpanAssertion func(t *testing.T, spans []*otlp.ScopeSpans)
		tshSpanAssertion      func(t *testing.T, spans []*otlp.ScopeSpans)
	}{
		{
			name: "spans exported with auth sampling all",
			cfg: func(c *tracing.Collector) servicecfg.TracingConfig {
				return servicecfg.TracingConfig{
					Enabled:      true,
					ExporterURL:  c.GRPCAddr(),
					SamplingRate: 1.0,
				}
			},
			teleportSpanAssertion: spanAssertion(false, false),
			tshSpanAssertion:      spanAssertion(true, false),
		},
		{
			name: "spans exported with auth sampling none",
			cfg: func(c *tracing.Collector) servicecfg.TracingConfig {
				return servicecfg.TracingConfig{
					Enabled:      true,
					ExporterURL:  c.HTTPAddr(),
					SamplingRate: 0.0,
				}
			},
			teleportSpanAssertion: spanAssertion(false, false),
			tshSpanAssertion:      spanAssertion(true, false),
		},
		{
			name: "spans not exported when tracing disabled",
			cfg: func(c *tracing.Collector) servicecfg.TracingConfig {
				return servicecfg.TracingConfig{}
			},
			teleportSpanAssertion: spanAssertion(false, true),
			tshSpanAssertion:      spanAssertion(true, false),
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			connector := mockConnector(t)
			alice, err := types.NewUser("alice@example.com")
			require.NoError(t, err)
			alice.SetRoles([]string{"access"})

			tmpHomePath := t.TempDir()

			teleportCollector, err := tracing.NewCollector(tracing.CollectorConfig{})
			require.NoError(t, err)

			tshCollector, err := tracing.NewCollector(tracing.CollectorConfig{})
			require.NoError(t, err)

			errCh := make(chan error, 2)
			go func() {
				errCh <- teleportCollector.Start()
			}()
			go func() {
				errCh <- tshCollector.Start()
			}()

			traceCfg := tt.cfg(teleportCollector)
			authProcess, proxyProcess := makeTestServers(t,
				withBootstrap(connector, alice),
				withConfig(func(cfg *servicecfg.Config) {
					cfg.Tracing = traceCfg
				}),
			)

			authServer := authProcess.GetAuthServer()
			require.NotNil(t, authServer)

			proxyAddr, err := proxyProcess.ProxyWebAddr()
			require.NoError(t, err)

			// login events should be included since there is
			// no forwarding
			err = Run(context.Background(), []string{
				"login",
				"--insecure",
				"--auth", connector.GetName(),
				"--proxy", proxyAddr.String(),
				"--trace",
				"--trace-exporter", tshCollector.GRPCAddr(),
			}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
				cf.MockSSOLogin = mockSSOLogin(t, authServer, alice)
				return nil
			})
			require.NoError(t, err)

			if traceCfg.Enabled {
				teleportCollector.WaitForExport()
			}
			tshCollector.WaitForExport()

			tt.teleportSpanAssertion(t, teleportCollector.Spans())
			tt.tshSpanAssertion(t, tshCollector.Spans())

			err = Run(context.Background(), []string{
				"ls",
				"--insecure",
				"--auth", connector.GetName(),
				"--proxy", proxyAddr.String(),
				"--trace",
				"--trace-exporter", tshCollector.GRPCAddr(),
			}, setHomePath(tmpHomePath))
			require.NoError(t, err)

			if traceCfg.Enabled {
				teleportCollector.WaitForExport()
			}
			tshCollector.WaitForExport()

			tt.teleportSpanAssertion(t, teleportCollector.Spans())
			tt.tshSpanAssertion(t, tshCollector.Spans())
		})
	}
}

func TestShowSessions(t *testing.T) {
	t.Parallel()

	expected := `[
    {
        "ei": 0,
        "event": "",
        "uid": "someID1",
        "time": "0001-01-01T00:00:00Z",
        "sid": "",
        "server_id": "",
        "enhanced_recording": false,
        "interactive": false,
        "participants": [
            "someParticipant"
        ],
        "session_start": "0001-01-01T00:00:00Z",
        "session_stop": "0001-01-01T00:00:00Z"
    },
    {
        "ei": 0,
        "event": "",
        "uid": "someID2",
        "time": "0001-01-01T00:00:00Z",
        "sid": "",
        "server_id": "",
        "enhanced_recording": false,
        "interactive": false,
        "participants": [
            "someParticipant"
        ],
        "session_start": "0001-01-01T00:00:00Z",
        "session_stop": "0001-01-01T00:00:00Z"
    },
    {
        "ei": 0,
        "event": "",
        "uid": "someID3",
        "time": "0001-01-01T00:00:00Z",
        "sid": "",
        "windows_desktop_service": "",
        "desktop_addr": "",
        "windows_domain": "",
        "windows_user": "",
        "desktop_labels": null,
        "session_start": "0001-01-01T00:00:00Z",
        "session_stop": "0001-01-01T00:00:00Z",
        "desktop_name": "",
        "recorded": false,
        "participants": [
            "someParticipant"
        ]
    }
]`
	sessions := []events.AuditEvent{
		&events.SessionEnd{
			Metadata: events.Metadata{
				ID: "someID1",
			},
			StartTime:    time.Time{},
			EndTime:      time.Time{},
			Participants: []string{"someParticipant"},
		},
		&events.SessionEnd{
			Metadata: events.Metadata{
				ID: "someID2",
			},
			StartTime:    time.Time{},
			EndTime:      time.Time{},
			Participants: []string{"someParticipant"},
		},
		&events.WindowsDesktopSessionEnd{
			Metadata: events.Metadata{
				ID: "someID3",
			},
			StartTime:    time.Time{},
			EndTime:      time.Time{},
			Participants: []string{"someParticipant"},
		},
	}
	var buf bytes.Buffer
	err := common.ShowSessions(sessions, teleport.JSON, &buf)
	require.NoError(t, err)
	require.Equal(t, expected, buf.String())
}

func TestMakeProfileInfo_NoInternalLogins(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		profile        *client.ProfileStatus
		expectedLogins []string
	}{
		{
			name: "with internal logins",
			profile: &client.ProfileStatus{
				Logins: []string{constants.NoLoginPrefix, teleport.SSHSessionJoinPrincipal, "-teleport-something-else"},
			},
			expectedLogins: nil,
		},
		{
			name: "with valid logins and internal logins",
			profile: &client.ProfileStatus{
				Logins: []string{constants.NoLoginPrefix, "alpaca", teleport.SSHSessionJoinPrincipal, "llama"},
			},
			expectedLogins: []string{"alpaca", "llama"},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			madeProfile := makeProfileInfo(test.profile, nil /* env map */, false /* inactive */)
			require.Equal(t, test.expectedLogins, madeProfile.Logins)
		})
	}
}
