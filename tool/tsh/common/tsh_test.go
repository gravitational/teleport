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

package common

import (
	"bufio"
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ghodss/yaml"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	otlp "go.opentelemetry.io/proto/otlp/trace/v1"
	"golang.org/x/crypto/ssh"
	yamlv2 "gopkg.in/yaml.v2"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/api/utils/keypaths"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/api/utils/keys/hardwarekey"
	"github.com/gravitational/teleport/api/utils/prompt"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/integration/kube"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/cryptosuites"
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
	"github.com/gravitational/teleport/tool/common"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

const (
	mockHeadlessPassword = "password1234"
	staticToken          = "test-static-token"
	// tshBinMainTestEnv allows to execute tsh main function from test binary.
	tshBinMainTestEnv = "TSH_BIN_MAIN_TEST"

	// tshBinMainTestOneshotEnv allows child processes of a tsh reexec process
	// to call teleport instead of tsh to support 'tsh ssh'.
	tshBinMainTestOneshotEnv = "TSH_BIN_MAIN_TEST_ONESHOT"
	// tshBinMockHeadlessAddr allows tests to mock headless auth when the
	// test binary is re-executed.
	tshBinMockHeadlessAddrEnv = "TSH_BIN_MOCK_HEADLESS_ADDR"
)

var ports utils.PortList

const initTestSentinel = "init_test"

func TestMain(m *testing.M) {
	handleReexec()

	var err error
	ports, err = utils.GetFreeTCPPorts(5000, utils.PortStartingNumber)
	if err != nil {
		panic(fmt.Sprintf("failed to allocate tcp ports for tests: %v", err))
	}

	modules.SetModules(&cliModules{})
	modules.SetInsecureTestMode(true)

	utils.InitLoggerForTests()
	cryptosuites.PrecomputeRSATestKeys(m)
	os.Exit(m.Run())
}

func BenchmarkInit(b *testing.B) {
	executable, err := os.Executable()
	require.NoError(b, err)

	for b.Loop() {
		cmd := exec.Command(executable, initTestSentinel)
		err := cmd.Run()
		assert.NoError(b, err)
	}
}

func handleReexec() {
	if slices.Contains(os.Args, initTestSentinel) {
		os.Exit(0)
	}

	var runOpts []CliOption

	// Allows mock headless auth to be implemented when the test binary
	// is re-executed.
	if addr := os.Getenv(tshBinMockHeadlessAddrEnv); addr != "" {
		runOpts = append(runOpts, func(c *CLIConf) error {
			c.MockHeadlessLogin = func(ctx context.Context, keyRing *client.KeyRing) (*authclient.SSHLoginResponse, error) {
				conn, err := net.Dial("tcp", addr)
				if err != nil {
					return nil, trace.Wrap(err, "dialing mock headless server")
				}
				defer conn.Close()

				// send the server the SSH public key
				_, err = conn.Write(keyRing.SSHPrivateKey.MarshalSSHPublicKey())
				if err != nil {
					return nil, trace.Wrap(err, "writing public key to mock headless server")
				}
				// send 0 byte as key delimiter
				if _, err := conn.Write([]byte{0}); err != nil {
					return nil, trace.Wrap(err, "writing delimiter")
				}
				// send the server the TLS public key
				tlsPub, err := keyRing.TLSPrivateKey.MarshalTLSPublicKey()
				if err != nil {
					return nil, trace.Wrap(err)
				}
				_, err = conn.Write(tlsPub)
				if err != nil {
					return nil, trace.Wrap(err, "writing public key to mock headless server")
				}
				// send 0 byte as key delimiter
				if _, err := conn.Write([]byte{0}); err != nil {
					return nil, trace.Wrap(err, "writing delimiter")
				}
				// read and decode response from server
				reply, err := io.ReadAll(conn)
				if err != nil {
					return nil, trace.Wrap(err, "reading reply from mock headless server")
				}
				var loginResp authclient.SSHLoginResponse
				if err := json.Unmarshal(reply, &loginResp); err != nil {
					return nil, trace.Wrap(err, "decoding reply from mock headless server")
				}

				return &loginResp, nil
			}
			return nil
		})
	}

	// Allows test to refer to tsh binary in tests.
	// Needed for tests that generate OpenSSH config by tsh config command where
	// tsh proxy ssh command is used as ProxyCommand.
	if os.Getenv(tshBinMainTestEnv) != "" {
		if os.Getenv(tshBinMainTestOneshotEnv) != "" {
			// unset this env var so child processes started by 'tsh ssh'
			// will be executed correctly below.
			if err := os.Unsetenv(tshBinMainTestEnv); err != nil {
				panic(fmt.Sprintf("failed to unset env var: %v", err))
			}
		}

		err := Run(context.Background(), os.Args[1:], runOpts...)
		if err != nil {
			var exitError *common.ExitCodeError
			if errors.As(err, &exitError) {
				os.Exit(exitError.Code)
			}
			utils.FatalError(err)
		}
		os.Exit(0)
	}

	// If the test is re-executing itself, execute the command that comes over
	// the pipe. Used to test tsh ssh command.
	if srv.IsReexec() {
		srv.RunAndExit(os.Args[1])
	}
}

type cliModules struct{}

func (p *cliModules) GenerateAccessRequestPromotions(_ context.Context, _ modules.AccessResourcesGetter, _ types.AccessRequest) (*types.AccessRequestAllowedPromotions, error) {
	return &types.AccessRequestAllowedPromotions{}, nil
}

func (p *cliModules) GetSuggestedAccessLists(ctx context.Context, _ *tlsca.Identity, _ modules.AccessListSuggestionClient, _ modules.AccessListAndMembersGetter, _ string) ([]*accesslist.AccessList, error) {
	return []*accesslist.AccessList{}, nil
}

// BuildType returns build type (OSS or Enterprise)
func (p *cliModules) BuildType() string {
	return "CLI"
}

// IsEnterpriseBuild returns false for [cliModules].
func (p *cliModules) IsEnterpriseBuild() bool {
	return false
}

// IsOSSBuild returns false for [cliModules].
func (p *cliModules) IsOSSBuild() bool {
	return false
}

// LicenseExpiry returns the expiry date of the enterprise license, if applicable.
func (p *cliModules) LicenseExpiry() time.Time {
	return time.Time{}
}

// PrintVersion prints the Teleport version.
func (p *cliModules) PrintVersion() {
	fmt.Printf("Teleport CLI\n")
}

// Features returns supported features
func (p *cliModules) Features() modules.Features {
	return modules.Features{
		Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
			entitlements.K8s: {Enabled: true},
			entitlements.DB:  {Enabled: true},
			entitlements.App: {Enabled: true},
		},
		AdvancedAccessWorkflows: true,
		AccessControls:          true,
	}
}

// IsBoringBinary checks if the binary was compiled with BoringCrypto.
func (p *cliModules) IsBoringBinary() bool {
	return false
}

// AttestHardwareKey attests a hardware key.
func (p *cliModules) AttestHardwareKey(_ context.Context, _ interface{}, _ *hardwarekey.AttestationStatement, _ crypto.PublicKey, _ time.Duration) (*keys.AttestationData, error) {
	return nil, trace.NotFound("no attestation data for the given key")
}

func (p *cliModules) EnableRecoveryCodes() {
}

func (p *cliModules) EnablePlugins() {
}

func (p *cliModules) SetFeatures(f modules.Features) {
}

func (p *cliModules) EnableAccessGraph() {}

func (p *cliModules) EnableAccessMonitoring() {}

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
			config := &client.TSHConfig{Aliases: tt.aliases}
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

// TestNoEnvVars checks if tsh is able to work without any env vars provided.
// This is important for VNet on macOS. When launchd starts VNet's launch daemon, it executes "tsh
// vnet-daemon" with only the following env vars available:
//
// XPC_SERVICE_NAME=com.goteleport.tshdev.vnetd
// PATH=/usr/bin:/bin:/usr/sbin:/sbin
// XPC_FLAGS=1
//
// …plus whatever is set in the launch daemon plist under the EnvironmentVariables key.
func TestNoEnvVars(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	testExecutable, err := os.Executable()
	require.NoError(t, err)
	// Execute an actual command and not just `tsh help` which goes through a different code path.
	cmd := exec.CommandContext(ctx, testExecutable, "version", "--client")
	// Run the command with no env vars except tshBinMainTestEnv, otherwise the test would hang.
	cmd.Env = []string{fmt.Sprintf("%s=1", tshBinMainTestEnv)}

	t.Logf("running command %v", cmd)
	output, err := cmd.CombinedOutput()
	t.Logf("executable output: %v", string(output))
	// By checking the ctx error together with err, the error report will include "context deadline
	// exceeded" if the command doesn't complete within the timeout. Otherwise the error would be just
	// "signal: killed".
	require.NoError(t, trace.NewAggregate(err, ctx.Err()))
}

func TestFailedLogin(t *testing.T) {
	t.Parallel()
	tmpHomePath := t.TempDir()

	connector := mockConnector(t)

	_, proxyProcess := makeTestServers(t, withBootstrap(connector))

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	// build a mock SSO login function to patch into tsh
	loginFailed := trace.AccessDenied("login failed")
	ssoLogin := func(ctx context.Context, connectorID string, keyRing *client.KeyRing, protocol string) (*authclient.SSHLoginResponse, error) {
		return nil, loginFailed
	}

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLoginCustom(ssoLogin, connector.GetName()))
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

	// set up watcher to approve the automatic request in background
	var didAutoRequest atomic.Bool
	watcher, err := authServer.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{
			{Kind: types.KindAccessRequest},
		},
	})
	require.NoError(t, err)

	// ensure that we observe init event prior to moving watcher to background
	// goroutine (ensures watcher init does not race with request creation).
	select {
	case event := <-watcher.Events():
		require.Equal(t, types.OpInit, event.Type)
	case <-watcher.Done():
		require.FailNow(t, "watcher closed unexpected", "err: %v", watcher.Error())
	}

	go func() {
		select {
		case event := <-watcher.Events():
			if event.Type != types.OpPut {
				panic(fmt.Sprintf("unexpected event type: %v\n", event))
			}
			err := authServer.SetAccessRequestState(ctx, types.AccessRequestUpdate{
				RequestID: event.Resource.(types.AccessRequest).GetName(),
				State:     types.RequestState_APPROVED,
			})
			if err != nil {
				panic(fmt.Sprintf("failed to approve request: %v", err))
			}
			didAutoRequest.Store(true)
		case <-watcher.Done():
			panic(fmt.Sprintf("watcher exited unexpectedly: %v", watcher.Error()))
		}
	}()

	buf := bytes.NewBuffer([]byte{})
	sc := bufio.NewScanner(buf)
	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr.String(),
		"--user", "alice", // explicitly use wrong name
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()), func(c *CLIConf) error {
		c.overrideStderr = buf
		c.SiteName = "localhost"
		return nil
	})

	require.NoError(t, err)

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
				_, err := identityfile.KeyRingFromIdentityFile(identityPath, "proxy.example.com", "")
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
				"--proxy", proxyAddr.String(),
				"--out", identPath,
			}, tt.extraArgs...), setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
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
	_, err = authServer.UpsertClusterNetworkingConfig(context.Background(), networkCfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		networkCfg.SetProxyListenerMode(prevValue)
		_, err = authServer.UpsertClusterNetworkingConfig(context.Background(), networkCfg)
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
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()), func(cf *CLIConf) error {
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
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()), func(cf *CLIConf) error {
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
		"--proxy", proxyAddr.String(),
		"localhost",
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()), func(cf *CLIConf) error {
		cf.overrideStderr = buf
		return nil
	})
	findMOTD(t, sc, motd)
	require.NoError(t, err)
}

// Test when https:// is included in --proxy address
func TestIgnoreHTTPSPrefix(t *testing.T) {
	t.Parallel()

	tmpHomePath := t.TempDir()

	connector := mockConnector(t)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	authProcess, proxyProcess := makeTestServers(t,
		withBootstrap(connector, alice),
	)

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	var buf bytes.Buffer

	proxyAddress := "https://" + proxyAddr.String()
	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddress,
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()), func(cf *CLIConf) error {
		cf.overrideStderr = &buf
		return nil
	})
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
		"--proxy", proxyAddr1.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer1, alice, connector.GetName()))
	require.NoError(t, err)

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr2.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer2, alice, connector.GetName()))

	require.NoError(t, err)

	// login again while both proxies are still valid and ensure it is successful without an SSO login provided
	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr1.String(),
	}, setHomePath(tmpHomePath))

	require.NoError(t, err)

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr2.String(),
	}, setHomePath(tmpHomePath))

	require.NoError(t, err)

	// logout

	err = Run(context.Background(), []string{"logout"}, setHomePath(tmpHomePath))
	require.NoError(t, err)

	// after logging out, make sure that any attempt to log in without providing a valid login function fails

	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond*50)
	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr1.String(),
	}, setHomePath(tmpHomePath))
	require.Error(t, err)

	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr2.String(),
	}, setHomePath(tmpHomePath))
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
	profile := &profile.Profile{
		SSHProxyAddr: "proxy:3023",
		WebProxyAddr: "proxy:3080",
	}

	err = conf.getClientStore().SaveProfile(profile, true)
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
	require.Equal(t, time.Duration(0), tc.Config.KeyTTL)

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
	conf.TSHConfig.ExtraHeaders = []client.ExtraProxyHeaders{
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
	require.NotEmpty(t, agentKeys)
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
func TestSSHOnMultipleNodes(t *testing.T) {
	t.Parallel()

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

	bob, err := types.NewUser("bob")
	require.NoError(t, err)
	bob.SetRoles([]string{"access", "ssh-login"})

	device, err := mocku2f.Create()
	require.NoError(t, err)
	device.SetPasswordless()

	rootAuth, rootProxy := makeTestServers(t, withBootstrap(connector, alice, bob, noAccessRole, sshLoginRole, perSessionMFARole))

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

	setupUser := func(cluster, name string, withDevices bool, asrv *auth.Server) {
		// set the default auth preference
		_, err = asrv.UpsertAuthPreference(ctx, webauthnPreference(cluster))
		require.NoError(t, err)

		if !withDevices {
			return
		}

		token, err := asrv.CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
			Name: name,
		})
		require.NoError(t, err)
		tokenID := token.GetName()
		res, err := asrv.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
			TokenID:     tokenID,
			DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
			DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
		})
		require.NoError(t, err)
		cc := wantypes.CredentialCreationFromProto(res.GetWebauthn())

		ccr, err := device.SignCredentialCreation(origin(cluster), cc)
		require.NoError(t, err)
		_, err = asrv.ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
			TokenID:     tokenID,
			NewPassword: []byte(password),
			NewMFARegisterResponse: &proto.MFARegisterResponse{
				Response: &proto.MFARegisterResponse_Webauthn{
					Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
				},
			},
		})
		require.NoError(t, err)
	}

	setupUser("localhost", "alice", true, rootAuth.GetAuthServer())
	setupUser("leafcluster", "alice", true, leafAuth.GetAuthServer())
	setupUser("localhost", "bob", false, rootAuth.GetAuthServer())

	successfulChallenge := func(cluster string) func(ctx context.Context, realOrigin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
		return func(ctx context.Context, realOrigin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
			car, err := device.SignAssertion(origin(cluster), assertion) // use the fake origin to prevent a mismatch
			if err != nil {
				return nil, "", err
			}
			return &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: wantypes.CredentialAssertionResponseToProto(car),
				},
			}, "", nil
		}
	}

	failedChallenge := func(cluster string) func(ctx context.Context, realOrigin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
		return func(ctx context.Context, realOrigin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
			car, err := device.SignAssertion(origin(cluster), assertion) // use the fake origin to prevent a mismatch
			if err != nil {
				return nil, "", err
			}
			carProto := wantypes.CredentialAssertionResponseToProto(car)
			carProto.Type = "NOT A VALID TYPE" // set to an invalid type so the ceremony fails

			return &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: carProto,
				},
			}, "", nil
		}
	}

	abortedChallenge := func(ctx context.Context, realOrigin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, _ *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
		return nil, "", errors.New("aborted challenge")
	}

	cases := []struct {
		name            string
		target          string
		authPreference  types.AuthPreference
		roles           []string
		webauthnLogin   client.WebauthnLoginFunc
		errAssertion    require.ErrorAssertionFunc
		stdoutAssertion require.ValueAssertionFunc
		stderrAssertion require.ValueAssertionFunc
		mfaPromptCount  int
		headless        bool
		proxyAddr       string
		auth            *auth.Server
		cluster         string
		user            types.User
		logSuccess      []string
	}{
		{
			name:           "default auth preference runs commands on multiple nodes without mfa",
			authPreference: defaultPreference,
			proxyAddr:      rootProxyAddr.String(),
			auth:           rootAuth.GetAuthServer(),
			target:         "env=stage",
			stderrAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Contains(t, i, "[test-stage-1] error\n", i2...)
				require.Contains(t, i, "[test-stage-2] error\n", i2...)
			},
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Contains(t, i, "[test-stage-1] test\n", i2...)
				require.Contains(t, i, "[test-stage-2] test\n", i2...)
			},
			errAssertion: require.NoError,
			logSuccess:   []string{stage1Hostname, stage2Hostname},
		},
		{
			name:      "webauthn auth preference runs commands on multiple matches without mfa",
			target:    "env=stage",
			proxyAddr: rootProxyAddr.String(),
			auth:      rootAuth.GetAuthServer(),
			stderrAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Contains(t, i, "[test-stage-1] error\n", i2...)
				require.Contains(t, i, "[test-stage-2] error\n", i2...)
			},
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Contains(t, i, "[test-stage-1] test\n", i2...)
				require.Contains(t, i, "[test-stage-2] test\n", i2...)
			},
			errAssertion: require.NoError,
			logSuccess:   []string{stage1Hostname, stage2Hostname},
		},
		{
			name:      "webauthn auth preference runs commands on a single match without mfa",
			target:    "env=prod",
			proxyAddr: rootProxyAddr.String(),
			auth:      rootAuth.GetAuthServer(),
			stderrAssertion: func(tt require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "error\n", i, i2...)
			},
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
			stderrAssertion: require.Empty,
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
			proxyAddr:     rootProxyAddr.String(),
			auth:          rootAuth.GetAuthServer(),
			webauthnLogin: successfulChallenge("localhost"),
			target:        "env=stage",
			stderrAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Contains(t, i, "[test-stage-1] error\n", i2...)
				require.Contains(t, i, "[test-stage-2] error\n", i2...)
			},
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Contains(t, i, "[test-stage-1] test\n", i2...)
				require.Contains(t, i, "[test-stage-2] test\n", i2...)
			},
			mfaPromptCount: 2,
			errAssertion:   require.NoError,
			logSuccess:     []string{stage1Hostname, stage2Hostname},
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
			proxyAddr:     rootProxyAddr.String(),
			auth:          rootAuth.GetAuthServer(),
			webauthnLogin: successfulChallenge("localhost"),
			target:        "env=prod",
			stderrAssertion: func(tt require.TestingT, i interface{}, i2 ...interface{}) {
				require.Contains(t, i, "error\n", i2...)
			},
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
			webauthnLogin:   successfulChallenge("localhost"),
			target:          "env=dev",
			errAssertion:    require.Error,
			stderrAssertion: require.Empty,
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
			proxyAddr:     rootProxyAddr.String(),
			auth:          rootAuth.GetAuthServer(),
			roles:         []string{"access", sshLoginRole.GetName(), perSessionMFARole.GetName()},
			webauthnLogin: successfulChallenge("localhost"),
			target:        "env=stage",
			stderrAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Contains(t, i, "[test-stage-1] error\n", i2...)
				require.Contains(t, i, "[test-stage-2] error\n", i2...)
			},
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Contains(t, i, "[test-stage-1] test\n", i2...)
				require.Contains(t, i, "[test-stage-2] test\n", i2...)
			},
			mfaPromptCount: 2,
			errAssertion:   require.NoError,
			logSuccess:     []string{stage1Hostname, stage2Hostname},
		},
		{
			name:      "role permits access without mfa",
			target:    sshHostID,
			proxyAddr: rootProxyAddr.String(),
			auth:      rootAuth.GetAuthServer(),
			roles:     []string{sshLoginRole.GetName()},
			stderrAssertion: func(tt require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "error\n", i, i2...)
			},
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
			stderrAssertion: func(t require.TestingT, v any, i ...any) {
				out, ok := v.(string)
				require.True(t, ok, i...)
				require.Contains(t, out, fmt.Sprintf("access denied to %s connecting to", user.Username), i...)
			},
			errAssertion: require.Error,
		},
		{
			name:          "command runs on a hostname with mfa set via role",
			target:        sshHostID,
			proxyAddr:     rootProxyAddr.String(),
			auth:          rootAuth.GetAuthServer(),
			roles:         []string{perSessionMFARole.GetName()},
			webauthnLogin: successfulChallenge("localhost"),
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			stderrAssertion: func(tt require.TestingT, i interface{}, i2 ...interface{}) {
				require.Contains(t, i, "error\n", i2...)
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
			webauthnLogin:   failedChallenge("localhost"),
			stdoutAssertion: require.Empty,
			stderrAssertion: func(t require.TestingT, v any, i ...any) {
				out, ok := v.(string)
				require.True(t, ok, i...)
				require.Contains(t, out, "MFA response validation failed", i...)
			},
			mfaPromptCount: 1,
			errAssertion:   require.Error,
		},
		{
			name: "aborted ceremony when role requires per session mfa",
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
			webauthnLogin:   abortedChallenge,
			stdoutAssertion: require.Empty,
			stderrAssertion: func(t require.TestingT, v any, i ...any) {
				out, ok := v.(string)
				require.True(t, ok, i...)
				require.Contains(t, out, "aborted challenge", i...)
			},
			errAssertion: require.Error,
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
			proxyAddr:     rootProxyAddr.String(),
			auth:          rootAuth.GetAuthServer(),
			target:        sshHostID,
			roles:         []string{perSessionMFARole.GetName()},
			webauthnLogin: failedChallenge("localhost"),
			stderrAssertion: func(tt require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "error\n", i, i2...)
			},
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			errAssertion: require.NoError,
			headless:     true,
		},
		{
			name:          "command runs on a leaf node with mfa set via role",
			target:        sshLeafHostID,
			proxyAddr:     leafProxyAddr,
			auth:          leafAuth.GetAuthServer(),
			roles:         []string{perSessionMFARole.GetName()},
			webauthnLogin: successfulChallenge("leafcluster"),
			stderrAssertion: func(tt require.TestingT, i interface{}, i2 ...interface{}) {
				require.Contains(t, i, "error\n", i2...)
			},
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			mfaPromptCount: 1,
			errAssertion:   require.NoError,
		},
		{
			name:          "command runs on a leaf node via root without mfa",
			target:        sshLeafHostID,
			proxyAddr:     rootProxyAddr.String(),
			auth:          rootAuth.GetAuthServer(),
			cluster:       "leafcluster",
			roles:         []string{sshLoginRole.GetName()},
			webauthnLogin: successfulChallenge("localhost"),
			stderrAssertion: func(tt require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "error\n", i, i2...)
			},
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
			stderrAssertion: func(tt require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "error\n", i, i2...)
			},
			webauthnLogin: successfulChallenge("leafcluster"),
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			errAssertion: require.NoError,
		},
		{
			name:          "command runs on a leaf node via root with mfa set via role",
			target:        sshLeafHostID,
			proxyAddr:     rootProxyAddr.String(),
			auth:          rootAuth.GetAuthServer(),
			cluster:       "leafcluster",
			roles:         []string{perSessionMFARole.GetName()},
			webauthnLogin: successfulChallenge("localhost"),
			stderrAssertion: func(tt require.TestingT, i interface{}, i2 ...interface{}) {
				require.Contains(t, i, "error\n", i2...)
			},
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			mfaPromptCount: 1,
			errAssertion:   require.NoError,
		},
		{
			name:            "invalid login on leaf node with no devices enrolled in root",
			target:          "invalid@" + sshLeafHostID,
			proxyAddr:       rootProxyAddr.String(),
			auth:            rootAuth.GetAuthServer(),
			roles:           []string{sshLoginRole.GetName()},
			cluster:         "leafcluster",
			user:            bob,
			stdoutAssertion: require.Empty,
			stderrAssertion: func(t require.TestingT, v any, i ...any) {
				out, ok := v.(string)
				require.True(t, ok, i...)
				require.Contains(t, out, "access denied to invalid connecting to", i...)
			},
			errAssertion: require.Error,
		},
		{
			name:            "invalid login on leaf node with devices enrolled in root",
			target:          "invalid@" + sshLeafHostID,
			proxyAddr:       rootProxyAddr.String(),
			auth:            rootAuth.GetAuthServer(),
			roles:           []string{sshLoginRole.GetName()},
			cluster:         "leafcluster",
			stdoutAssertion: require.Empty,
			stderrAssertion: func(t require.TestingT, v any, i ...any) {
				out, ok := v.(string)
				require.True(t, ok, i...)
				require.Contains(t, out, "access denied to invalid connecting to", i...)
			},
			errAssertion: require.Error,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			tmpHomePath := t.TempDir()

			clusterName, err := tt.auth.GetClusterName(ctx)
			require.NoError(t, err)

			user := alice
			if tt.user != nil {
				user = tt.user
			}

			if tt.authPreference != nil {
				_, err = tt.auth.UpsertAuthPreference(ctx, tt.authPreference)
				require.NoError(t, err)
				t.Cleanup(func() {
					_, err = tt.auth.UpsertAuthPreference(ctx, webauthnPreference(clusterName.GetClusterName()))
					require.NoError(t, err)
				})
			}

			if tt.roles != nil {
				roles := user.GetRoles()
				t.Cleanup(func() {
					user.SetRoles(roles)
					_, err = tt.auth.UpsertUser(ctx, user)
					require.NoError(t, err)
				})
				user.SetRoles(tt.roles)
				user, err = tt.auth.UpsertUser(ctx, user)
				require.NoError(t, err)
			}

			err = Run(ctx, []string{
				"login",
				"-d",
				"--insecure",
				"--proxy", tt.proxyAddr,
				"--user", user.GetName(),
				tt.cluster,
			}, setHomePath(tmpHomePath), setMockSSOLogin(tt.auth, user, connector.GetName()),
				func(cf *CLIConf) error {
					cf.WebauthnLogin = tt.webauthnLogin
					return nil
				},
			)
			require.NoError(t, err)

			stdout := &output{buf: bytes.Buffer{}}
			stderr := &output{buf: bytes.Buffer{}}
			// Clear counter before each ssh command,
			// so we can assert how many times sign was called.
			device.SetCounter(0)

			args := []string{"ssh", "-d", "--insecure"}
			if tt.headless {
				args = append(args, "--headless", "--proxy", tt.proxyAddr, "--user", user.GetName())
			}
			var logDir string
			if len(tt.logSuccess) > 0 {
				logDir = t.TempDir()
				args = append(args, "--log-dir", logDir)
			}
			args = append(args, tt.target, "echo", "test", "&&", "echo", "error", ">&2")

			err = Run(ctx,
				args,
				setHomePath(tmpHomePath),
				func(conf *CLIConf) error {
					conf.overrideStdin = &bytes.Buffer{}
					conf.OverrideStdout = stdout
					conf.overrideStderr = stderr
					conf.MockHeadlessLogin = mockHeadlessLogin(t, tt.auth, user)
					conf.WebauthnLogin = tt.webauthnLogin
					return nil
				},
			)

			tt.errAssertion(t, err)
			tt.stdoutAssertion(t, stdout.String())
			tt.stderrAssertion(t, stderr.String())
			require.Equal(t, tt.mfaPromptCount, int(device.Counter()), "device sign count mismatch")

			// Check for logs if enabled.
			if len(tt.logSuccess) > 0 {
				succeededFile := filepath.Join(logDir, "hosts.succeeded")
				assert.FileExists(t, succeededFile)
				succeededContents, err := os.ReadFile(succeededFile)
				assert.NoError(t, err)

				for _, host := range tt.logSuccess {
					assert.Contains(t, string(succeededContents), host)
					stdoutFile := filepath.Join(logDir, host+".stdout")
					assert.FileExists(t, stdoutFile)
					contents, err := os.ReadFile(stdoutFile)
					assert.NoError(t, err)
					assert.Equal(t, "test\n", string(contents))

					stderrFile := filepath.Join(logDir, host+".stderr")
					assert.FileExists(t, stderrFile)
					contents, err = os.ReadFile(stderrFile)
					assert.NoError(t, err)
					assert.Equal(t, "error\n", string(contents))
				}
			}
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

func TestNoRelogin(t *testing.T) {
	t.Parallel()
	tmpHomePath := t.TempDir()
	connector := mockConnector(t)
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	authProcess, proxyProcess := makeTestServers(t,
		withBootstrap(connector, alice),
	)
	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)
	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--proxy", proxyAddr.String(),
		"--user", "alice",
	}, setHomePath(tmpHomePath), setMockSSOLogin(authProcess.GetAuthServer(), alice, connector.GetName()))
	require.NoError(t, err)

	reloginErr := errors.New("relogin failed")
	trackingLoginFunc := func(ctx context.Context, connectorID string, keyRing *client.KeyRing, protocol string) (*authclient.SSHLoginResponse, error) {
		return nil, reloginErr
	}

	for _, tc := range []struct {
		desc           string
		extraArgs      []string
		errorAssertion func(*testing.T, error)
	}{
		{
			// Should try to relogin due to bad parameter without passing --relogin.
			desc: "default",
			errorAssertion: func(t *testing.T, err error) {
				require.ErrorIs(t, err, reloginErr)
			},
		},
		{
			// Should try to relogin due to bad parameter when passing --relogin.
			desc:      "relogin",
			extraArgs: []string{"--relogin"},
			errorAssertion: func(t *testing.T, err error) {
				require.ErrorIs(t, err, reloginErr)
			},
		},
		{
			// Should skip relogin and fail instantly when passing --no-relogin.
			desc:      "no relogin",
			extraArgs: []string{"--no-relogin"},
			errorAssertion: func(t *testing.T, err error) {
				require.Error(t, err)
				require.NotErrorIs(t, err, reloginErr)
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			t.Parallel()
			tshArgs := []string{"ssh"}
			tshArgs = append(tshArgs, tc.extraArgs...)
			tshArgs = append(tshArgs,
				"--insecure",
				"--user", "alice",
				"--proxy", proxyAddr.String(),
				"12.12.12.12:8080",
				"uptime",
			)
			err := Run(context.Background(), tshArgs, setHomePath(tmpHomePath), setMockSSOLoginCustom(trackingLoginFunc, connector.GetName()))
			tc.errorAssertion(t, err)
		})
	}
}

// TestSSHAccessRequest tests that a user can automatically request access to a
// ssh server using a resource access request when "tsh ssh" fails with
// AccessDenied.
func TestSSHAccessRequest(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	requester, err := types.NewRole("requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles:         []string{"node-access"},
				SearchAsRoles: []string{"node-access"},
			},
		},
	})
	require.NoError(t, err)

	emptyRole, err := types.NewRole("empty", types.RoleSpecV6{})
	require.NoError(t, err)
	searchOnlyRequester, err := types.NewRole("search-only-requester", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Request: &types.AccessRequestConditions{
				Roles:         []string{"empty"},
				SearchAsRoles: []string{"node-access"},
			},
		},
	})
	require.NoError(t, err)

	user, err := user.Current()
	require.NoError(t, err)
	nodeAccessRole, err := types.NewRole("node-access", types.RoleSpecV6{
		Allow: types.RoleConditions{
			NodeLabels: types.Labels{
				"access": {"true"},
			},
			Logins: []string{user.Username},
		},
	})
	require.NoError(t, err)

	connector := mockConnector(t)

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"requester"})
	traits := map[string][]string{
		constants.TraitLogins: {user.Username},
	}
	alice.SetTraits(traits)

	rootAuth, rootProxy := makeTestServers(t,
		withBootstrap(requester, searchOnlyRequester, nodeAccessRole, emptyRole, connector, alice),
		// Do not use a fake clock to better imitate real-world behavior.
	)

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

	tmpHomePath := t.TempDir()

	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--proxy", proxyAddr.String(),
		"--user", "alice",
	}, setHomePath(tmpHomePath), setMockSSOLogin(rootAuth.GetAuthServer(), alice, connector.GetName()))
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

	// won't request if explicitly disabled
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--request-reason", "reason here to bypass prompt",
		"--request-mode", accessRequestModeOff,
		fmt.Sprintf("%s@%s", user.Username, sshHostname),
		"echo", "test",
	}, setHomePath(tmpHomePath))
	require.Error(t, err)
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--request-reason", "reason here to bypass prompt",
		"--disable-access-request",
		fmt.Sprintf("%s@%s", user.Username, sshHostname),
		"echo", "test",
	}, setHomePath(tmpHomePath))
	require.Error(t, err)

	tests := []struct {
		name                        string
		requestMode                 string
		assertNonApprovedNodeAccess require.ErrorAssertionFunc
		assertSearchRolesOnly       require.ErrorAssertionFunc
	}{
		{
			name:                        "resource-based",
			requestMode:                 accessRequestModeResource,
			assertNonApprovedNodeAccess: require.Error,
			assertSearchRolesOnly:       require.NoError,
		},
		{
			name:                        "role-based",
			requestMode:                 accessRequestModeRole,
			assertNonApprovedNodeAccess: require.NoError,
			assertSearchRolesOnly:       require.Error,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			alice.SetRoles([]string{"requester"})
			_, err = rootAuth.GetAuthServer().UpsertUser(ctx, alice)
			require.NoError(t, err)

			err = Run(ctx, []string{
				"logout",
			}, setHomePath(tmpHomePath))
			require.NoError(t, err)

			err = Run(ctx, []string{
				"login",
				"--insecure",
				"--proxy", proxyAddr.String(),
				"--user", "alice",
			}, setHomePath(tmpHomePath), setMockSSOLogin(rootAuth.GetAuthServer(), alice, connector.GetName()))
			require.NoError(t, err)

			requestReason := uuid.New().String()
			// the first ssh request can fail if the proxy node watcher doesn't know
			// about the nodes yet, retry a few times until it works
			require.Eventually(t, func() bool {
				// ssh with request, by hostname
				err := Run(ctx, []string{
					"ssh",
					"--debug",
					"--insecure",
					"--request-mode", tc.requestMode,
					"--request-reason", requestReason,
					fmt.Sprintf("%s@%s", user.Username, sshHostname),
					"echo", "test",
				}, setHomePath(tmpHomePath))
				if err != nil {
					t.Logf("Got error while trying to SSH to node, retrying. Error: %v", err)
				}
				return err == nil
			}, 10*time.Second, 100*time.Millisecond, "failed to ssh with retries")

			requests, err := rootAuth.GetAuthServer().GetAccessRequests(ctx, types.AccessRequestFilter{})
			require.NoError(t, err)
			require.True(t, slices.ContainsFunc(requests, func(request types.AccessRequest) bool {
				return request.GetRequestReason() == requestReason
			}), "access request with the specified reason was not found")

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
				"--proxy", proxyAddr.String(),
				"--user", "alice",
			}, setHomePath(tmpHomePath), setMockSSOLogin(rootAuth.GetAuthServer(), alice, connector.GetName()))
			require.NoError(t, err)

			// ssh with request, by host ID
			err = Run(ctx, []string{
				"ssh",
				"--insecure",
				"--request-mode", tc.requestMode,
				"--request-reason", "reason here to bypass prompt",
				fmt.Sprintf("%s@%s", user.Username, sshHostID),
				"echo", "test",
			}, setHomePath(tmpHomePath))
			require.NoError(t, err)

			// check access to non-requested node
			err = Run(ctx, []string{
				"ssh",
				"--insecure",
				fmt.Sprintf("%s@%s", user.Username, sshHostname2),
				"echo", "test",
			}, setHomePath(tmpHomePath))
			tc.assertNonApprovedNodeAccess(t, err)

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
				"--request-mode", accessRequestModeOff,
				fmt.Sprintf("%s@%s", user.Username, sshHostname2),
				"echo", "test",
			}, setHomePath(tmpHomePath))
			require.Error(t, err)

			// successfully ssh to other node, with new request
			err = Run(ctx, []string{
				"ssh",
				"--insecure",
				"--request-mode", tc.requestMode,
				"--request-reason", "reason here to bypass prompt",
				fmt.Sprintf("%s@%s", user.Username, sshHostname2),
				"echo", "test",
			}, setHomePath(tmpHomePath))
			require.NoError(t, err)

			// Check access to nodes when only search_as_roles are available
			alice.SetRoles([]string{"search-only-requester"})
			_, err = rootAuth.GetAuthServer().UpsertUser(ctx, alice)
			require.NoError(t, err)
			err = Run(ctx, []string{
				"--insecure",
				"request",
				"drop",
			}, setHomePath(tmpHomePath))
			require.NoError(t, err)
			err = Run(ctx, []string{
				"ssh",
				"--insecure",
				"--request-mode", tc.requestMode,
				"--request-reason", "reason here to bypass prompt",
				fmt.Sprintf("%s@%s", user.Username, sshHostname),
				"echo", "test",
			}, setHomePath(tmpHomePath))
			tc.assertSearchRolesOnly(t, err)
		})
	}
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
		"--proxy", rootProxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(rootAuthServer, alice, connector.GetName()))
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

// TestSSHCommand tests that a user can access a single SSH node and run commands.
func TestSSHCommands(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	accessRoleName := "access"
	sshHostname := "test-ssh-server"

	accessUser, err := types.NewUser(accessRoleName)
	require.NoError(t, err)
	accessUser.SetRoles([]string{accessRoleName})

	user, err := user.Current()
	require.NoError(t, err)
	accessUser.SetLogins([]string{user.Username})

	traits := map[string][]string{
		constants.TraitLogins: {user.Username},
	}
	accessUser.SetTraits(traits)

	connector := mockConnector(t)
	rootServerOpts := []testserver.TestServerOptFunc{
		testserver.WithBootstrap(connector, accessUser),
		testserver.WithHostname(sshHostname),
		testserver.WithClusterName(t, "root"),
		testserver.WithSSHLabel(accessRoleName, "true"),
		testserver.WithSSHPublicAddrs("127.0.0.1:0"),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.SSH.Enabled = true
			cfg.SSH.PublicAddrs = []utils.NetAddr{cfg.SSH.Addr}
			cfg.SSH.DisableCreateHostUser = true
		}),
	}
	rootServer := testserver.MakeTestServer(t, rootServerOpts...)

	rootProxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		rootNodes, err := rootServer.GetAuthServer().GetNodes(ctx, apidefaults.Namespace)
		if !assert.NoError(t, err) || !assert.Len(t, rootNodes, 1) {
			return
		}
	}, 10*time.Second, 100*time.Millisecond)

	tmpHomePath := t.TempDir()
	rootAuth := rootServer.GetAuthServer()

	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--proxy", rootProxyAddr.String(),
		"--user", user.Username,
	}, setHomePath(tmpHomePath), setMockSSOLogin(rootAuth, accessUser, connector.GetName()))
	require.NoError(t, err)

	tests := []struct {
		name      string
		args      []string
		expected  string
		shouldErr bool
	}{
		{
			// Test that a simple echo works.
			name:     "ssh simple command",
			expected: "this is a test message",
			args: []string{
				fmt.Sprintf("%s@%s", user.Username, sshHostname),
				"echo",
				"this is a test message",
			},
			shouldErr: false,
		},
		{
			// Test that commands can be prefixed with a double dash.
			name:     "ssh command with double dash",
			expected: "this is a test message",
			args: []string{
				fmt.Sprintf("%s@%s", user.Username, sshHostname),
				"--",
				"echo",
				"this is a test message",
			},
			shouldErr: false,
		},
		{
			// Test that a double dash is not removed from the middle of a command.
			name:     "ssh command with double dash in the middle",
			expected: "-- this is a test message",
			args: []string{
				fmt.Sprintf("%s@%s", user.Username, sshHostname),
				"echo",
				"--",
				"this is a test message",
			},
			shouldErr: false,
		},
		{
			// Test that quoted commands work (e.g. `tsh ssh 'echo test'`)
			name:     "ssh command literal",
			expected: "this is a test message",
			args: []string{
				fmt.Sprintf("%s@%s", user.Username, sshHostname),
				"echo this is a test message",
			},
			shouldErr: false,
		},
		{
			// Test that a double dash is passed as-is in a quoted command (which should fail).
			name:     "ssh command literal with double dash err",
			expected: "",
			args: []string{
				fmt.Sprintf("%s@%s", user.Username, sshHostname),
				"-- echo this is a test message",
			},
			shouldErr: true,
		},
		{
			// Test that a double dash is not removed from the middle of a quoted command.
			name:     "ssh command literal with double dash in the middle",
			expected: "-- this is a test message",
			args: []string{
				fmt.Sprintf("%s@%s", user.Username, sshHostname),
				"echo", "-- this is a test message",
			},
			shouldErr: false,
		},
		{
			// Test tsh ssh -- hostname command
			name:     "delimiter before host and command",
			expected: "this is a test message",
			args: []string{
				"--", sshHostname, "echo", "this is a test message",
			},
			shouldErr: false,
		},
	}

	for _, test := range tests {
		test := test
		ctx := context.Background()
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			stdout := &output{buf: bytes.Buffer{}}
			stderr := &output{buf: bytes.Buffer{}}
			args := append(
				[]string{
					"ssh",
					"--insecure",
					"--proxy", rootProxyAddr.String(),
				},
				test.args...,
			)

			err := Run(ctx, args, setHomePath(tmpHomePath),
				func(conf *CLIConf) error {
					conf.overrideStdin = &bytes.Buffer{}
					conf.OverrideStdout = stdout
					conf.overrideStderr = stderr
					return nil
				},
			)

			if test.shouldErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expected, strings.TrimSpace(stdout.String()))
				require.Empty(t, stderr.String())
			}
		})
	}
}

// tryCreateTrustedCluster performs several attempts to create a trusted cluster,
// retries on connection problems and access denied errors to let caches
// propagate and services to start
//
// Duplicated in integration/integration_test.go
func tryCreateTrustedCluster(t *testing.T, authServer *auth.Server, trustedCluster types.TrustedCluster) {
	ctx := context.TODO()
	for i := 0; i < 10; i++ {
		_, err := authServer.UpsertTrustedClusterV2(ctx, trustedCluster)
		if err == nil {
			return
		}
		if trace.IsConnectionProblem(err) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if trace.IsAccessDenied(err) {
			time.Sleep(500 * time.Millisecond)
			continue
		}
		require.FailNow(t, "Terminating on unexpected problem", "%v.", err)
	}
	require.FailNow(t, "Timeout creating trusted cluster")
}

func TestKubeCredentialsLock(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	const kubeClusterName = "kube-cluster"

	t.Run("failed client creation doesn't create lockfile", func(t *testing.T) {
		t.Parallel()
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
		t.Parallel()
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
						Kind: "pods", Name: types.Wildcard, Namespace: types.Wildcard, APIGroup: types.Wildcard,
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

		teleportClusterName, err := authServer.GetClusterName(ctx)
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

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			gotNames := map[string]struct{}{}
			for ks, err := range authServer.UnifiedResourceCache.KubernetesServers(ctx, services.UnifiedResourcesIterateParams{}) {
				if !assert.NoError(t, err) {
					return
				}

				gotNames[ks.GetCluster().GetName()] = struct{}{}

			}

			assert.Contains(t, gotNames, kubeCluster.GetName(), "missing kube cluster")
		}, 15*time.Second, 100*time.Millisecond)

		var ssoCalls atomic.Int32
		mockSSOLogin := mockSSOLogin(authServer, alice)
		mockSSOLoginWithCountCalls := func(ctx context.Context, connectorID string, keyRing *client.KeyRing, protocol string) (*authclient.SSHLoginResponse, error) {
			ssoCalls.Add(1)
			return mockSSOLogin(ctx, connectorID, keyRing, protocol)
		}

		err = Run(context.Background(), []string{
			"login",
			"--insecure",
			"--debug",
			"--proxy", proxyAddr.String(),
		}, setHomePath(tmpHomePath), setMockSSOLoginCustom(mockSSOLoginWithCountCalls, connector.GetName()))
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
		err = os.WriteFile(fmt.Sprintf("%s/%s", tmpHomePath, "keys/127.0.0.1/alice@example.com"), []byte(privKey), 0o666)
		require.NoError(t, err)
		err = os.WriteFile(fmt.Sprintf("%s/%s", tmpHomePath, "keys/127.0.0.1/alice@example.com.pub"), []byte(pubKey), 0o666)
		require.NoError(t, err)
		err = os.WriteFile(fmt.Sprintf("%s/%s", tmpHomePath, "keys/127.0.0.1/alice@example.com-ssh/localhost-cert.pub"), []byte(expiredSSHCert), 0o666)
		require.NoError(t, err)

		errChan := make(chan error)
		runCreds := func() {
			credErr := Run(context.Background(), []string{
				"kube",
				"credentials",
				"--insecure",
				"--proxy", proxyAddr.String(),
				"--teleport-cluster", teleportClusterName.GetClusterName(),
				"--kube-cluster", kubeClusterName,
			}, setHomePath(tmpHomePath), setMockSSOLoginCustom(mockSSOLoginWithCountCalls, connector.GetName()))
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
	var (
		proxy       = "proxy.example.com:3080"
		username    = "alice"
		clustername = "root-cluster"
	)
	for _, tc := range []struct {
		name         string
		cliConf      CLIConf
		envMap       map[string]string
		assertErr    require.ErrorAssertionFunc
		assertConfig func(t require.TestingT, c *client.Config)
	}{
		{
			name: "OK --auth headless",
			cliConf: CLIConf{
				Proxy:         proxy,
				AuthConnector: constants.HeadlessConnector,
				Username:      "username",
			},
			assertErr: require.NoError,
			assertConfig: func(t require.TestingT, c *client.Config) {
				require.Equal(t, constants.HeadlessConnector, c.AuthConnector)
			},
		}, {
			name: "OK --headless",
			cliConf: CLIConf{
				Proxy:    proxy,
				Headless: true,
				Username: "username",
			},
			assertErr: require.NoError,
			assertConfig: func(t require.TestingT, c *client.Config) {
				require.Equal(t, constants.HeadlessConnector, c.AuthConnector)
			},
		}, {
			name: "OK use ssh session env with headless cli flag",
			cliConf: CLIConf{
				Headless: true,
			},
			envMap: map[string]string{
				teleport.SSHSessionWebProxyAddr: proxy,
				teleport.SSHTeleportUser:        username,
				teleport.SSHTeleportClusterName: clustername,
			},
			assertErr: require.NoError,
			assertConfig: func(t require.TestingT, c *client.Config) {
				require.Equal(t, constants.HeadlessConnector, c.AuthConnector)
				require.Equal(t, proxy, c.WebProxyAddr)
				require.Equal(t, username, c.Username)
				require.Equal(t, clustername, c.SiteName)
			},
		}, {
			name: "OK use ssh session env with headless auth connector cli flag",
			cliConf: CLIConf{
				AuthConnector: constants.HeadlessConnector,
			},
			envMap: map[string]string{
				teleport.SSHSessionWebProxyAddr: proxy,
				teleport.SSHTeleportUser:        username,
				teleport.SSHTeleportClusterName: clustername,
			},
			assertErr: require.NoError,
			assertConfig: func(t require.TestingT, c *client.Config) {
				require.Equal(t, constants.HeadlessConnector, c.AuthConnector)
				require.Equal(t, proxy, c.WebProxyAddr)
				require.Equal(t, username, c.Username)
				require.Equal(t, clustername, c.SiteName)
			},
		}, {
			name: "OK ignore ssh session env without headless",
			cliConf: CLIConf{
				Proxy:    "other-proxy:3080",
				Headless: false,
			},
			envMap: map[string]string{
				teleport.SSHSessionWebProxyAddr: proxy,
				teleport.SSHTeleportUser:        username,
				teleport.SSHTeleportClusterName: clustername,
			},
			assertErr: require.NoError,
			assertConfig: func(t require.TestingT, c *client.Config) {
				require.Equal(t, "other-proxy:3080", c.WebProxyAddr)
				require.Empty(t, c.Username)
				require.Empty(t, c.SiteName)
			},
		}, {
			name: "NOK --headless with mismatched auth connector",
			cliConf: CLIConf{
				Proxy:         proxy,
				Headless:      true,
				AuthConnector: constants.LocalConnector,
				Username:      "username",
			},
			assertErr: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected trace.BadParameter error but got %v", err)
			},
		}, {
			name: "NOK --auth headless without explicit user",
			cliConf: CLIConf{
				Proxy:         proxy,
				AuthConnector: constants.HeadlessConnector,
			},
			assertErr: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected trace.BadParameter error but got %v", err)
			},
		}, {
			name: "NOK --headless without explicit user",
			cliConf: CLIConf{
				Proxy:    proxy,
				Headless: true,
			},
			assertErr: func(t require.TestingT, err error, msgAndArgs ...interface{}) {
				require.True(t, trace.IsBadParameter(err), "expected trace.BadParameter error but got %v", err)
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envMap {
				t.Setenv(k, v)
			}

			tc.cliConf.HomePath = t.TempDir()
			c, err := loadClientConfigFromCLIConf(&tc.cliConf, proxy)
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
	server := testserver.MakeTestServer(t,
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Hostname = sshHostname
			cfg.Auth.Enabled = true
			cfg.Proxy.Enabled = true
			cfg.SSH.Enabled = true
			cfg.SSH.DisableCreateHostUser = true

			cfg.Auth.BootstrapResources = []types.Resource{nodeAccess, alice, requester, bob}
			cfg.Auth.Preference = &types.AuthPreferenceV2{
				Metadata: types.Metadata{
					Labels: map[string]string{types.OriginLabel: types.OriginConfigFile},
				},
				Spec: types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOptional,
					Webauthn: &types.Webauthn{
						RPID: "127.0.0.1",
					},
					AllowHeadless: types.NewBoolOption(true),
				},
			}
		}),
	)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		found, err := server.GetAuthServer().GetNodes(ctx, apidefaults.Namespace)
		assert.NoError(t, err)
		assert.Len(t, found, 1)
	}, 10*time.Second, 100*time.Millisecond)

	go func() {
		// Ensure the context is canceled, so that Run calls don't block
		defer cancel()
		if err := approveAllAccessRequests(ctx, server.GetAuthServer()); err != nil {
			assert.ErrorIs(t, err, context.Canceled, "unexpected error from approveAllAccessRequests")
		}
	}()

	proxyAddr, err := server.ProxyWebAddr()
	require.NoError(t, err)

	for _, tc := range []struct {
		name      string
		args      []string
		envMap    map[string]string
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
			envMap: map[string]string{
				teleport.SSHSessionWebProxyAddr: proxyAddr.String(),
				teleport.SSHTeleportUser:        "alice",
			},
			assertErr: require.NoError,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envMap {
				t.Setenv(k, v)
			}

			args := append([]string{
				"ssh",
				"-d",
				"--insecure",
				"--proxy", proxyAddr.String(),
			}, tc.args...)
			args = append(args,
				fmt.Sprintf("%s@%s", user.Username, sshHostname),
				"echo", "test",
			)

			err := Run(ctx, args, func(cf *CLIConf) error {
				cf.MockHeadlessLogin = mockHeadlessLogin(t, server.GetAuthServer(), alice)
				return nil
			})
			tc.assertErr(t, err)
		})
	}
}

func TestHeadlessDoesNotAddKeysToAgent(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})
	agentKeyring, _ := createAgent(t)

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

	sshHostname := "test-ssh-host"

	server := testserver.MakeTestServer(t,
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Hostname = sshHostname
			cfg.Auth.Enabled = true
			cfg.Proxy.Enabled = true
			cfg.SSH.Enabled = true
			cfg.SSH.DisableCreateHostUser = true
			cfg.Auth.BootstrapResources = []types.Resource{nodeAccess, alice}
			cfg.Auth.Preference = &types.AuthPreferenceV2{
				Metadata: types.Metadata{
					Labels: map[string]string{types.OriginLabel: types.OriginConfigFile},
				},
				Spec: types.AuthPreferenceSpecV2{
					Type:         constants.Local,
					SecondFactor: constants.SecondFactorOptional,
					Webauthn: &types.Webauthn{
						RPID: "127.0.0.1",
					},
					AllowHeadless: types.NewBoolOption(true),
				},
			}
		}))

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		found, err := server.GetAuthServer().GetNodes(ctx, apidefaults.Namespace)
		assert.NoError(t, err)
		assert.Len(t, found, 1)
	}, 10*time.Second, 100*time.Millisecond)

	proxyAddr, err := server.ProxyWebAddr()
	require.NoError(t, err)

	go func() {
		// Ensure the context is canceled, so that Run calls don't block
		defer cancel()
		if err := approveAllAccessRequests(ctx, server.GetAuthServer()); err != nil {
			assert.ErrorIs(t, err, context.Canceled, "unexpected error from approveAllAccessRequests")
		}
	}()

	err = Run(ctx, []string{
		"ssh",
		"-d",
		"--insecure",
		"--proxy", proxyAddr.String(),
		"--headless",
		"--user", "alice",
		"--add-keys-to-agent=yes",
		fmt.Sprintf("%s@%s", user.Username, sshHostname),
		"echo", "test",
	}, func(cf *CLIConf) error {
		cf.MockHeadlessLogin = mockHeadlessLogin(t, server.GetAuthServer(), alice)
		return nil
	})
	require.NoError(t, err)

	keys, err := agentKeyring.List()
	require.NoError(t, err)
	require.Empty(t, keys)
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

func TestEnvFlags(t *testing.T) {
	type testCase struct {
		inCLIConf  CLIConf
		envMap     map[string]string
		outCLIConf CLIConf
	}

	testEnvFlag := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			for k, v := range tc.envMap {
				t.Setenv(k, v)
			}
			setEnvFlags(&tc.inCLIConf)
			require.Equal(t, tc.outCLIConf, tc.inCLIConf)
		}
	}

	t.Run("cluster env", func(t *testing.T) {
		t.Run("nothing set", testEnvFlag(testCase{
			outCLIConf: CLIConf{
				SiteName: "",
			},
		}))
		t.Run("CLI flag is set", testEnvFlag(testCase{
			inCLIConf: CLIConf{
				SiteName: "a.example.com",
			},
			outCLIConf: CLIConf{
				SiteName: "a.example.com",
			},
		}))
		t.Run("TELEPORT_SITE set", testEnvFlag(testCase{
			envMap: map[string]string{
				siteEnvVar: "a.example.com",
			},
			outCLIConf: CLIConf{
				SiteName: "a.example.com",
			},
		}))
		t.Run("TELEPORT_CLUSTER set", testEnvFlag(testCase{
			envMap: map[string]string{
				clusterEnvVar: "b.example.com",
			},
			outCLIConf: CLIConf{
				SiteName: "b.example.com",
			},
		}))
		t.Run("TELEPORT_SITE and TELEPORT_CLUSTER set, prefer TELEPORT_CLUSTER", testEnvFlag(testCase{
			envMap: map[string]string{
				clusterEnvVar: "d.example.com",
				siteEnvVar:    "c.example.com",
			},
			outCLIConf: CLIConf{
				SiteName: "d.example.com",
			},
		}))
		t.Run("TELEPORT_SITE and TELEPORT_CLUSTER and CLI flag is set, prefer CLI", testEnvFlag(testCase{
			inCLIConf: CLIConf{
				SiteName: "e.example.com",
			},
			envMap: map[string]string{
				clusterEnvVar: "g.example.com",
				siteEnvVar:    "f.example.com",
			},
			outCLIConf: CLIConf{
				SiteName: "e.example.com",
			},
		}))
	})

	t.Run("kube cluster env", func(t *testing.T) {
		t.Run("nothing set", testEnvFlag(testCase{
			outCLIConf: CLIConf{
				KubernetesCluster: "",
			},
		}))
		t.Run("CLI flag is set", testEnvFlag(testCase{
			inCLIConf: CLIConf{
				KubernetesCluster: "a.example.com",
			},
			outCLIConf: CLIConf{
				KubernetesCluster: "a.example.com",
			},
		}))
		t.Run("TELEPORT_KUBE_CLUSTER set", testEnvFlag(testCase{
			envMap: map[string]string{
				kubeClusterEnvVar: "a.example.com",
			},
			outCLIConf: CLIConf{
				KubernetesCluster: "a.example.com",
			},
		}))
		t.Run("TELEPORT_KUBE_CLUSTER and CLI flag is set, prefer CLI", testEnvFlag(testCase{
			inCLIConf: CLIConf{
				KubernetesCluster: "e.example.com",
			},
			envMap: map[string]string{
				kubeClusterEnvVar: "g.example.com",
			},
			outCLIConf: CLIConf{
				KubernetesCluster: "e.example.com",
			},
		}))
	})

	t.Run("teleport home env", func(t *testing.T) {
		t.Run("nothing set", testEnvFlag(testCase{
			outCLIConf: CLIConf{},
		}))
		t.Run("CLI flag is set", testEnvFlag(testCase{
			inCLIConf: CLIConf{
				HomePath: "teleport-data",
			},
			outCLIConf: CLIConf{
				HomePath: "teleport-data",
			},
		}))
		t.Run("TELEPORT_HOME set", testEnvFlag(testCase{
			envMap: map[string]string{
				types.HomeEnvVar: "teleport-data/",
			},
			outCLIConf: CLIConf{
				HomePath: "teleport-data",
			},
		}))
		t.Run("TELEPORT_HOME and CLI flag is set, prefer env", testEnvFlag(testCase{
			inCLIConf: CLIConf{
				HomePath: "teleport-data",
			},
			envMap: map[string]string{
				types.HomeEnvVar: "teleport-data/",
			},
			outCLIConf: CLIConf{
				HomePath: "teleport-data",
			},
		}))
	})

	t.Run("tsh global config path", func(t *testing.T) {
		t.Run("nothing set", testEnvFlag(testCase{
			outCLIConf: CLIConf{},
		}))
		t.Run("TELEPORT_GLOBAL_TSH_CONFIG set", testEnvFlag(testCase{
			envMap: map[string]string{
				globalTshConfigEnvVar: "/opt/teleport/tsh.yaml",
			},
			outCLIConf: CLIConf{
				GlobalTshConfigPath: "/opt/teleport/tsh.yaml",
			},
		}))
	})

	t.Run("tsh ssh session env", func(t *testing.T) {
		t.Run("does not overwrite cli flags", testEnvFlag(testCase{
			inCLIConf: CLIConf{
				Headless: true,
				Proxy:    "proxy.example.com",
				Username: "alice",
				SiteName: "root-cluster",
			},
			envMap: map[string]string{
				teleport.SSHSessionWebProxyAddr: "other.example.com",
				teleport.SSHTeleportUser:        "bob",
				teleport.SSHTeleportClusterName: "leaf-cluster",
			},
			outCLIConf: CLIConf{
				Headless: true,
				Proxy:    "proxy.example.com",
				Username: "alice",
				SiteName: "root-cluster",
			},
		}))
	})
}

func TestKubeConfigUpdate(t *testing.T) {
	t.Parallel()
	// don't need real creds for this test, just something to compare against
	creds := &client.KeyRing{KeyRingIndex: client.KeyRingIndex{ProxyHost: "a.example.com"}}
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
			envMap:      map[string]string{x11.DisplayEnv: ""},
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
			for k, v := range tc.envMap {
				t.Setenv(k, v)
			}

			opts, err := parseOptions(tc.opts)
			require.NoError(t, err)

			clt := client.Config{}
			err = setX11Config(&clt, &tc.cf, opts)
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
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
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

// deprecated: Use `tools/teleport/testenv.MakeTestServer` instead.
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
	cfg.Logger = utils.NewSlogLoggerForTests()
	// Disabling debug service for tests so that it doesn't break if the data
	// directory path is too long.
	cfg.DebugService.Enabled = false

	for _, fn := range options.configFuncs {
		fn(cfg)
	}

	return runTeleport(t, cfg)
}

// deprecated: Use `tools/teleport/testenv.MakeTestServer` instead.
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
	cfg.Auth.Preference.SetSignatureAlgorithmSuite(types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1)
	cfg.Proxy.Enabled = true
	cfg.Proxy.WebAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("127.0.0.1", ports.Pop())}
	cfg.Proxy.SSHAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("127.0.0.1", ports.Pop())}
	cfg.Proxy.ReverseTunnelListenAddr = utils.NetAddr{AddrNetwork: "tcp", Addr: net.JoinHostPort("127.0.0.1", ports.Pop())}
	cfg.Proxy.DisableWebInterface = true
	cfg.Logger = utils.NewSlogLoggerForTests()
	// Disabling debug service for tests so that it doesn't break if the data
	// directory path is too long.
	cfg.DebugService.Enabled = false

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
		ClientSecret: "fake-secret",
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

func mockSSOLogin(authServer *auth.Server, user types.User) client.SSOLoginFunc {
	return func(ctx context.Context, connectorID string, keyRing *client.KeyRing, protocol string) (*authclient.SSHLoginResponse, error) {
		// generate certificates for our user
		clusterName, err := authServer.GetClusterName(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		tlsPub, err := keyRing.TLSPrivateKey.MarshalTLSPublicKey()
		if err != nil {
			return nil, trace.Wrap(err)
		}
		sshCert, tlsCert, err := authServer.GenerateUserTestCerts(auth.GenerateUserTestCertsRequest{
			SSHPubKey:               keyRing.SSHPrivateKey.MarshalSSHPublicKey(),
			TLSPubKey:               tlsPub,
			Username:                user.GetName(),
			TTL:                     time.Hour,
			Compatibility:           constants.CertificateFormatStandard,
			RouteToCluster:          clusterName.GetClusterName(),
			SSHAttestationStatement: keyRing.SSHPrivateKey.GetAttestationStatement(),
			TLSAttestationStatement: keyRing.TLSPrivateKey.GetAttestationStatement(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// load CA cert
		authority, err := authServer.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName.GetClusterName(),
		}, false)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// build login response
		return &authclient.SSHLoginResponse{
			Username:    user.GetName(),
			Cert:        sshCert,
			TLSCert:     tlsCert,
			HostSigners: authclient.AuthoritiesToTrustedCerts([]types.CertAuthority{authority}),
		}, nil
	}
}

func mockHeadlessLogin(t *testing.T, authServer *auth.Server, user types.User) client.SSHLoginFunc {
	return func(ctx context.Context, keyRing *client.KeyRing) (*authclient.SSHLoginResponse, error) {
		// generate certificates for our user
		clusterName, err := authServer.GetClusterName(ctx)
		require.NoError(t, err)
		tlsPub, err := keyRing.TLSPrivateKey.MarshalTLSPublicKey()
		require.NoError(t, err)
		sshCert, tlsCert, err := authServer.GenerateUserTestCerts(auth.GenerateUserTestCertsRequest{
			SSHPubKey:      keyRing.SSHPrivateKey.MarshalSSHPublicKey(),
			TLSPubKey:      tlsPub,
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
		return &authclient.SSHLoginResponse{
			Username:    user.GetName(),
			Cert:        sshCert,
			TLSCert:     tlsCert,
			HostSigners: authclient.AuthoritiesToTrustedCerts([]types.CertAuthority{authority}),
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
		// The TSHConfig is populated prior to applying any options, and
		// if the home directory is being overridden, then any proxy templates
		// and aliases that may have been loaded from the default home directory
		// should be returned to default to avoid altering tests run from machines
		// that may have a tsh config file present in the home directory.
		cf.TSHConfig = client.TSHConfig{}
		return nil
	}
}

func setTSHConfig(cfg client.TSHConfig) CliOption {
	return func(cf *CLIConf) error {
		for _, template := range cfg.ProxyTemplates {
			if err := template.Check(); err != nil {
				return err
			}
		}

		cf.TSHConfig = cfg
		return nil
	}
}

func setNoRelogin() CliOption {
	return func(cf *CLIConf) error {
		cf.Relogin = false
		return nil
	}
}

func setOverrideMySQLConfigPath(path string) CliOption {
	return func(cf *CLIConf) error {
		cf.overrideMySQLOptionFilePath = path
		return nil
	}
}

func setOverridePostgresConfigPath(path string) CliOption {
	return func(cf *CLIConf) error {
		cf.overridePostgresServiceFilePath = path
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

func setMockSSOLogin(authServer *auth.Server, user types.User, connectorName string) CliOption {
	return setMockSSOLoginCustom(mockSSOLogin(authServer, user), connectorName)
}

func setMockSSOLoginCustom(mockSSOLogin client.SSOLoginFunc, connectorName string) CliOption {
	return func(cf *CLIConf) error {
		cf.MockSSOLogin = mockSSOLogin
		cf.AuthConnector = connectorName
		return nil
	}
}

func testSerialization(t *testing.T, expected string, serializer func(string) (string, error)) {
	t.Helper()
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
		reExecFromVersion  string
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
		{
			name:               "re-exec version provided",
			proxyVersion:       "3.2.1",
			proxyPublicAddress: "teleport.example.com:443",
			reExecFromVersion:  "1.2.3",
			expected: fmt.Sprintf(
				`{"version": %q, "gitref": %q, "runtime": %q, "proxyVersion": %q, "reExecutedFromVersion": %q, "proxyPublicAddress": %q}`,
				teleport.Version, teleport.Gitref, runtime.Version(), "3.2.1", "1.2.3", "teleport.example.com:443"),
		},
	}

	for _, tC := range testCases {
		t.Run(tC.name, func(t *testing.T) {
			testSerialization(t, tC.expected, func(fmt string) (string, error) {
				return serializeVersion(fmt, tC.proxyVersion, tC.proxyPublicAddress, tC.reExecFromVersion)
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
      "name": "my-db",
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
        "iam_policy_status": "IAM_POLICY_STATUS_UNSPECIFIED",
        "elasticache": {},
        "secret_store": {},
        "memorydb": {},
        "opensearch": {},
        "rdsproxy": {},
        "redshift_serverless": {},
        "docdb": {}
      },
      "mysql": {},
      "oracle": {
        "audit_user": ""
      },
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
        "iam_policy_status": "IAM_POLICY_STATUS_UNSPECIFIED",
        "elasticache": {},
        "secret_store": {},
        "memorydb": {},
        "opensearch": {},
        "rdsproxy": {},
        "redshift_serverless": {},
        "docdb": {}
      },
      "azure": {
	    "redis": {}
	  }
    }%v
  }]
	`
	db, err := types.NewDatabaseV3(types.Metadata{
		Name:        "my-db",
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
		ActiveRequests: []string{"1", "2", "3"},
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
	}`
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
			"expires": "0001-01-01T00:00:00Z",
			"max_duration": "0001-01-01T00:00:00Z",
			"session_ttl": "0001-01-01T00:00:00Z"
		}
	}`
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
	]`
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
		var b bytes.Buffer
		err := serializeSessions([]types.SessionTracker{tracker}, f, &b)
		return b.String(), err
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
		{"metadata":{"Name":"my device","Namespace":"default"},"id":"id","addedAt":"1970-01-01T00:00:00Z","lastUsed":"1970-01-01T00:00:00Z","kind":"mfa_device","totp":{"key":"key"},"version":"v1"}
	]
	`

	dev, err := types.NewMFADevice("my device", "id", aTime, &types.MFADevice_Totp{
		Totp: &types.TOTPDevice{
			Key: "key",
		},
	})
	require.NoError(t, err)

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

	dbWithAutoUser, err := types.NewDatabaseV3(types.Metadata{
		Name:   "auto-user",
		Labels: map[string]string{"env": "prod"},
	}, types.DatabaseSpecV3{
		Protocol: "postgres",
		URI:      "localhost:5432",
		AdminUser: &types.DatabaseAdminUser{
			Name: "teleport-admin",
		},
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
	roleAutoUser := &types.RoleV6{
		Metadata: types.Metadata{Name: "auto-user", Namespace: apidefaults.Namespace},
		Spec: types.RoleSpecV6{
			Options: types.RoleOptions{
				CreateDatabaseUserMode: types.CreateDatabaseUserMode_DB_USER_MODE_KEEP,
			},
			Allow: types.RoleConditions{
				Namespaces:     []string{apidefaults.Namespace},
				DatabaseLabels: types.Labels{"env": []string{"prod"}},
				DatabaseRoles:  []string{"dev"},
				DatabaseNames:  []string{"*"},
				DatabaseUsers:  []string{types.Wildcard},
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
				Allowed: []string{"*"},
				Denied:  []string{"superuser"},
			},
			wantText: "[*], except: [superuser]",
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
		{
			name:     "db with admin user and role with auto-user",
			database: dbWithAutoUser,
			roles:    services.RoleSet{roleAutoUser},
			wantUsers: &dbUsers{
				Allowed: []string{"alice"},
			},
			wantText: "[alice] (Auto-provisioned)",
		},
		{
			name:     "db with admin user but role without auto-user",
			database: dbWithAutoUser,
			roles:    services.RoleSet{roleDevProd},
			wantUsers: &dbUsers{
				Allowed: []string{"dev"},
			},
			wantText: "[dev]",
		},
		{
			name:     "db without admin user but role with auto-user",
			database: dbProd,
			roles:    services.RoleSet{roleAutoUser},
			wantUsers: &dbUsers{
				Allowed: []string{"*"},
			},
			wantText: "[*]",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			accessChecker := services.NewAccessCheckerWithRoleSet(&services.AccessInfo{
				Username: "alice",
			}, "clustername", tt.roles)

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
				"--proxy", proxyAddr.String(),
				"--trace",
			}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
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
				"--proxy", proxyAddr.String(),
				"--trace",
				"--trace-exporter", tshCollector.GRPCAddr(),
			}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
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
	},
	{
		"ei": 0,
		"event": "",
		"uid": "someID4",
		"time": "0001-01-01T00:00:00Z",
		"user": "someUser",
		"sid": "",
		"db_protocol": "postgres",
		"db_uri": "",
		"session_start": "0001-01-01T00:00:00Z",
		"session_stop": "0001-01-01T00:00:00Z"
    } ]`
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
		&events.DatabaseSessionEnd{
			Metadata: events.Metadata{
				ID: "someID4",
			},
			UserMetadata: events.UserMetadata{
				User: "someUser",
			},
			DatabaseMetadata: events.DatabaseMetadata{
				DatabaseProtocol: "postgres",
			},
			StartTime: time.Time{},
			EndTime:   time.Time{},
		},
	}
	var buf bytes.Buffer
	err := common.ShowSessions(sessions, teleport.JSON, &buf)
	require.NoError(t, err)
	require.JSONEq(t, expected, buf.String())
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

func TestBenchmarkPostgres(t *testing.T) {
	t.Parallel()

	// Canceling the context right after the test ensures the local proxy
	// started by the benchmark command is closed and the remaining connections
	// (if any) are dropped.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpHomePath := t.TempDir()
	connector := mockConnector(t)
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetDatabaseUsers([]string{"*"})
	alice.SetDatabaseNames([]string{"*"})
	alice.SetRoles([]string{"access"})

	authProcess := testserver.MakeTestServer(
		t,
		testserver.WithBootstrap(connector, alice),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{
				{
					Name:     "postgres-local",
					Protocol: defaults.ProtocolPostgres,
					URI:      "external-pg:5432",
				},
				{
					Name:     "mysql-local",
					Protocol: defaults.ProtocolMySQL,
					URI:      "external-mysql:3306",
				},
			}
		}),
	)

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := authProcess.ProxyWebAddr()
	require.NoError(t, err)

	// Log into Teleport cluster.
	err = Run(ctx, []string{
		"login", "--insecure", "--debug", "--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
	require.NoError(t, err)

	benchmarkErrorLineParser := regexp.MustCompile("`host=(.+) +user=(.+) database=(.+)`: (.+)$")
	args := []string{
		"bench", "postgres", "--insecure",
		// Benchmark options to limit benchmark to a single execution.
		"--rate", "1", "--duration", "1s",
	}

	for name, tc := range map[string]struct {
		database            string
		additionalFlags     []string
		expectCommandErr    bool
		expectedErrContains string
		expectedHost        string
		expectedUser        string
		expectedDatabase    string
	}{
		"connect to database": {
			database:            "postgres-local",
			additionalFlags:     []string{"--db-user", "username", "--db-name", "database"},
			expectedErrContains: "server error",
			// When connecting to Teleport databases, it will use a local proxy.
			expectedHost:     "127.0.0.1",
			expectedUser:     "username",
			expectedDatabase: "database",
		},
		"direct connection": {
			database:            "postgres://direct_user@test:5432/direct_database",
			expectedErrContains: "hostname resolving error",
			expectedHost:        "test",
			expectedUser:        "direct_user",
			expectedDatabase:    "direct_database",
		},
		"no postgres database found": {
			database:         "mysql-local",
			expectCommandErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			commandOutput := new(bytes.Buffer)
			err := Run(
				ctx,
				append(args, append(tc.additionalFlags, tc.database)...),
				setCopyStdout(commandOutput), setHomePath(tmpHomePath),
			)
			if tc.expectCommandErr {
				require.Error(t, err)
				return
			}

			lines := bytes.Split(commandOutput.Bytes(), []byte("\n"))
			var errorLine string
			for _, line := range lines {
				if bytes.HasPrefix(line, []byte("* Last error:")) {
					errorLine = string(line)
					break
				}
			}
			require.NotEmpty(t, errorLine, "expected benchmark to fail")

			parsed := benchmarkErrorLineParser.FindStringSubmatch(errorLine)
			require.Len(t, parsed, 5, "unexpecter benchmark error: %q", errorLine)

			host, username, database, benchmarkError := parsed[1], parsed[2], parsed[3], parsed[4]

			require.Contains(t, benchmarkError, tc.expectedErrContains)
			require.Equal(t, tc.expectedHost, host)
			require.Equal(t, tc.expectedUser, username)
			require.Equal(t, tc.expectedDatabase, database)
		})
	}
}

func TestBenchmarkMySQL(t *testing.T) {
	t.Parallel()

	// Canceling the context right after the test ensures the local proxy
	// started by the benchmark command is closed and the remaining connections
	// (if any) are dropped.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tmpHomePath := t.TempDir()
	connector := mockConnector(t)
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetDatabaseUsers([]string{"*"})
	alice.SetDatabaseNames([]string{"*"})
	alice.SetRoles([]string{"access"})

	authProcess := testserver.MakeTestServer(
		t,
		testserver.WithBootstrap(connector, alice),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{
				{
					Name:     "postgres-local",
					Protocol: defaults.ProtocolPostgres,
					URI:      "external-pg:5432",
				},
				{
					Name:     "mysql-local",
					Protocol: defaults.ProtocolMySQL,
					URI:      "external-mysql:3306",
				},
			}
		}),
	)

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := authProcess.ProxyWebAddr()
	require.NoError(t, err)

	// Log into Teleport cluster.
	err = Run(ctx, []string{
		"login", "--insecure", "--debug", "--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, alice, connector.GetName()))
	require.NoError(t, err)

	args := []string{
		"bench", "mysql", "--insecure",
		// Benchmark options to limit benchmark to a single execution.
		"--rate", "1", "--duration", "1s",
	}

	for name, tc := range map[string]struct {
		database            string
		additionalFlags     []string
		expectCommandErr    bool
		expectedErrContains string
	}{
		"connect to database": {
			database:        "mysql-local",
			additionalFlags: []string{"--db-user", "username", "--db-name", "database"},
			// Expect a MySQL driver error where the server is not working correctly.
			expectedErrContains: "ERROR 1105 (HY000)",
		},
		"direct connection": {
			database:            "mysql://direct_user@test:3306/direct_database",
			expectedErrContains: "lookup test",
		},
		"no mysql database found": {
			database:         "postgres-local",
			expectCommandErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			commandOutput := new(bytes.Buffer)
			err := Run(
				ctx,
				append(args, append(tc.additionalFlags, tc.database)...),
				setCopyStdout(commandOutput), setHomePath(tmpHomePath),
			)
			if tc.expectCommandErr {
				require.Error(t, err)
				return
			}

			lines := bytes.Split(commandOutput.Bytes(), []byte("\n"))
			var errorLine string
			for _, line := range lines {
				if bytes.HasPrefix(line, []byte("* Last error:")) {
					errorLine = string(line)
					break
				}
			}
			require.NotEmpty(t, errorLine, "expected benchmark to fail")
			require.Contains(t, errorLine, tc.expectedErrContains)
		})
	}
}

func TestLogout(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	sshKey, tlsKey, err := cryptosuites.GenerateUserSSHAndTLSKey(ctx, func(_ context.Context) (types.SignatureAlgorithmSuite, error) {
		return types.SignatureAlgorithmSuite_SIGNATURE_ALGORITHM_SUITE_BALANCED_V1, nil
	})
	require.NoError(t, err)
	sshPriv, err := keys.NewPrivateKey(sshKey)
	require.NoError(t, err)
	tlsPriv, err := keys.NewPrivateKey(tlsKey)
	require.NoError(t, err)
	clientKeyRing := &client.KeyRing{
		KeyRingIndex: client.KeyRingIndex{
			ProxyHost:   "proxy",
			Username:    "user",
			ClusterName: "cluster",
		},
		SSHPrivateKey: sshPriv,
		TLSPrivateKey: tlsPriv,
	}
	profile := &profile.Profile{
		WebProxyAddr: clientKeyRing.ProxyHost,
		Username:     clientKeyRing.Username,
		SiteName:     clientKeyRing.ClusterName,
	}

	for _, tt := range []struct {
		name         string
		modifyKeyDir func(t *testing.T, homePath string)
	}{
		{
			name:         "normal home dir",
			modifyKeyDir: func(t *testing.T, homePath string) {},
		},
		{
			name: "public key missing",
			modifyKeyDir: func(t *testing.T, homePath string) {
				pubKeyPath := keypaths.PublicKeyPath(homePath, clientKeyRing.ProxyHost, clientKeyRing.Username)
				require.NoError(t, os.Remove(pubKeyPath))
			},
		},
		{
			name: "SSH private key missing",
			modifyKeyDir: func(t *testing.T, homePath string) {
				privKeyPath := keypaths.UserSSHKeyPath(homePath, clientKeyRing.ProxyHost, clientKeyRing.Username)
				require.NoError(t, os.Remove(privKeyPath))
			},
		},
		{
			name: "TLS private key missing",
			modifyKeyDir: func(t *testing.T, homePath string) {
				privKeyPath := keypaths.UserTLSKeyPath(homePath, clientKeyRing.ProxyHost, clientKeyRing.Username)
				require.NoError(t, os.Remove(privKeyPath))
			},
		},
		{
			name: "public key mismatch",
			modifyKeyDir: func(t *testing.T, homePath string) {
				newKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				require.NoError(t, err)
				sshPub, err := ssh.NewPublicKey(newKey.Public())
				require.NoError(t, err)

				pubKeyPath := keypaths.PublicKeyPath(homePath, clientKeyRing.ProxyHost, clientKeyRing.Username)
				err = os.WriteFile(pubKeyPath, ssh.MarshalAuthorizedKey(sshPub), 0o600)
				require.NoError(t, err)
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tmpHomePath := t.TempDir()

			store := client.NewFSClientStore(tmpHomePath)
			err := store.AddKeyRing(clientKeyRing)
			require.NoError(t, err)
			store.SaveProfile(profile, true)

			tt.modifyKeyDir(t, tmpHomePath)

			_, err = os.Lstat(tmpHomePath)
			require.NoError(t, err)

			err = Run(context.Background(), []string{"logout"}, setHomePath(tmpHomePath))
			require.NoError(t, err, trace.DebugReport(err))

			// directory should be empty.
			f, err := os.Open(tmpHomePath)
			require.NoError(t, err)
			entries, err := f.ReadDir(1)
			require.ErrorIs(t, err, io.EOF, "expected empty directory, but found %v", entries)
		})
	}
}

func Test_formatActiveDB(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		active      tlsca.RouteToDatabase
		displayName string
		expect      string
	}{
		{
			name: "no route details",
			active: tlsca.RouteToDatabase{
				ServiceName: "my-db",
			},
			displayName: "my-db",
			expect:      "> my-db",
		},
		{
			name: "different display name",
			active: tlsca.RouteToDatabase{
				ServiceName: "my-db",
			},
			displayName: "display-name",
			expect:      "> display-name",
		},
		{
			name: "user only",
			active: tlsca.RouteToDatabase{
				ServiceName: "my-db",
				Username:    "alice",
			},
			displayName: "my-db",
			expect:      "> my-db (user: alice)",
		},
		{
			name: "db only",
			active: tlsca.RouteToDatabase{
				ServiceName: "my-db",
				Database:    "sales",
			},
			displayName: "my-db",
			expect:      "> my-db (db: sales)",
		},
		{
			name: "user & db",
			active: tlsca.RouteToDatabase{
				ServiceName: "my-db",
				Username:    "alice",
				Database:    "sales",
			},
			displayName: "my-db",
			expect:      "> my-db (user: alice, db: sales)",
		},
		{
			name: "db & roles",
			active: tlsca.RouteToDatabase{
				ServiceName: "my-db",
				Database:    "sales",
				Roles:       []string{"reader", "writer"},
			},
			displayName: "my-db",
			expect:      "> my-db (db: sales, roles: [reader writer])",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			require.Equal(t, test.expect, formatActiveDB(test.active, test.displayName))
		})
	}
}

func TestFlatten(t *testing.T) {
	// Test setup: create a server and a user
	home := t.TempDir()
	identityPath := filepath.Join(t.TempDir(), "identity.pem")

	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})

	connector := mockConnector(t)

	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, alice))
	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	// Test setup: log in and obtain a valid identity for the user
	conf := CLIConf{
		Username:           alice.GetName(),
		Proxy:              proxyAddr.String(),
		InsecureSkipVerify: true,
		IdentityFileOut:    identityPath,
		IdentityFormat:     identityfile.FormatFile,
		HomePath:           home,
		AuthConnector:      connector.GetName(),
		MockSSOLogin:       mockSSOLogin(authServer, alice),
		Context:            context.Background(),
	}
	require.NoError(t, onLogin(&conf))

	// Test setup: validate we got a valid identity
	_, err = identityfile.KeyRingFromIdentityFile(identityPath, "proxy.example.com", "")
	require.NoError(t, err)

	// Test execution: flatten the identity previously obtained in a new home.
	freshHome := t.TempDir()
	conf = CLIConf{
		Proxy:              proxyAddr.String(),
		InsecureSkipVerify: true,
		IdentityFileIn:     identityPath,
		HomePath:           freshHome,
		Context:            context.Background(),
	}
	require.NoError(t, flattenIdentity(&conf))

	// Test execution: validate that the newly created profile can be used to build a valid client.
	clt, err := makeClient(&conf)
	require.NoError(t, err)

	_, err = clt.Ping(context.Background())
	require.NoError(t, err)

	// Test execution: validate that flattening succeeds if a profile already exists.
	conf = CLIConf{
		Proxy:              proxyAddr.String(),
		InsecureSkipVerify: true,
		IdentityFileIn:     identityPath,
		HomePath:           freshHome,
		Context:            context.Background(),
	}
	require.NoError(t, flattenIdentity(&conf), "unexpected error when overwriting a tsh profile")
}

// TestListingResourcesAcrossClusters validates that tsh ls -R
// returns expected results for root and leaf clusters.
func TestListingResourcesAcrossClusters(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	createAgent(t)

	accessUser, err := types.NewUser("access")
	require.NoError(t, err)
	accessUser.SetRoles([]string{"access"})

	user, err := user.Current()
	require.NoError(t, err)
	accessUser.SetLogins([]string{user.Name})

	connector := mockConnector(t)
	rootServerOpts := []testserver.TestServerOptFunc{
		testserver.WithBootstrap(connector, accessUser),
		testserver.WithHostname("node01"),
		testserver.WithClusterName(t, "root"),
		testserver.WithDebugApp(),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			// Enable DB
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{
				{
					Name:     "db01",
					Protocol: defaults.ProtocolPostgres,
					URI:      "localhost:5432",
				},
			}
		}),
	}
	rootServer := testserver.MakeTestServer(t, rootServerOpts...)

	leafServerOpts := []testserver.TestServerOptFunc{
		testserver.WithBootstrap(connector, accessUser),
		testserver.WithHostname("node02"),
		testserver.WithClusterName(t, "leaf"),
		testserver.WithDebugApp(),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			// Enable DB
			cfg.Databases.Enabled = true
			cfg.Databases.Databases = []servicecfg.Database{
				{
					Name:     "db02",
					Protocol: defaults.ProtocolPostgres,
					URI:      "localhost:5432",
				},
			}
		}),
	}
	leafServer := testserver.MakeTestServer(t, leafServerOpts...)
	testserver.SetupTrustedCluster(ctx, t, rootServer, leafServer)

	var (
		rootNode, leafNode *types.ServerV2
		rootDB, leafDB     *types.DatabaseV3
		rootApp, leafApp   *types.AppV3
	)
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		rootNodes, err := rootServer.GetAuthServer().GetNodes(ctx, apidefaults.Namespace)
		if !assert.NoError(t, err) || !assert.Len(t, rootNodes, 1) {
			return
		}

		leafNodes, err := leafServer.GetAuthServer().GetNodes(ctx, apidefaults.Namespace)
		if !assert.NoError(t, err) || !assert.Len(t, leafNodes, 1) {
			return
		}

		rootDatabases, err := rootServer.GetAuthServer().GetDatabaseServers(ctx, apidefaults.Namespace)
		if !assert.NoError(t, err) || !assert.Len(t, rootDatabases, 1) {
			return
		}

		leafDatabases, err := leafServer.GetAuthServer().GetDatabaseServers(ctx, apidefaults.Namespace)
		if !assert.NoError(t, err) || !assert.Len(t, leafDatabases, 1) {
			return
		}

		rootApps, err := rootServer.GetAuthServer().GetApplicationServers(ctx, apidefaults.Namespace)
		if !assert.NoError(t, err) || !assert.Len(t, rootApps, 1) {
			return
		}

		leafApps, err := leafServer.GetAuthServer().GetApplicationServers(ctx, apidefaults.Namespace)
		if !assert.NoError(t, err) || !assert.Len(t, leafApps, 1) {
			return
		}

		rootNode = rootNodes[0].(*types.ServerV2)
		leafNode = leafNodes[0].(*types.ServerV2)
		rootDB = rootDatabases[0].GetDatabase().(*types.DatabaseV3)
		leafDB = leafDatabases[0].GetDatabase().(*types.DatabaseV3)
		rootApp = rootApps[0].GetApp().(*types.AppV3)
		leafApp = leafApps[0].GetApp().(*types.AppV3)
	}, 10*time.Second, 100*time.Millisecond)

	t.Run("nodes", func(t *testing.T) {
		t.Parallel()

		testListingResources(t,
			listPack[*types.ServerV2]{
				rootProcess:   rootServer,
				leafProcess:   leafServer,
				rootResource:  rootNode,
				leafResource:  leafNode,
				user:          accessUser,
				connectorName: connector.GetName(),
				args:          []string{"ls"},
			},
			func(t *testing.T, proxyAddr string, recursive bool, raw []byte) []*types.ServerV2 {
				type recursiveServer struct {
					Proxy   string          `json:"proxy"`
					Cluster string          `json:"cluster"`
					Node    *types.ServerV2 `json:"node"`
				}

				var nodes []*types.ServerV2
				if recursive {
					var servers []recursiveServer
					require.NoError(t, json.Unmarshal(raw, &servers))
					require.NotEmpty(t, servers)

					nodes = make([]*types.ServerV2, 0, len(servers))
					for _, s := range servers {
						if s.Node.GetHostname() == "node01" {
							assert.Equal(t, "root", s.Cluster)
						} else {
							assert.Equal(t, "leaf", s.Cluster)
						}

						assert.Equal(t, proxyAddr, s.Proxy)
						require.NoError(t, s.Node.CheckAndSetDefaults())
						nodes = append(nodes, s.Node)
					}
				} else {
					err := json.Unmarshal(raw, &nodes)
					assert.NoError(t, err)
					require.NotEmpty(t, nodes)

					for _, s := range nodes {
						require.NoError(t, s.CheckAndSetDefaults())
					}
				}

				return nodes
			},
			func(a, b *types.ServerV2) bool {
				return strings.Compare(a.GetName(), b.GetName()) < 0
			},
		)
	})

	t.Run("databases", func(t *testing.T) {
		t.Parallel()

		testListingResources(t,
			listPack[*types.DatabaseV3]{
				rootProcess:   rootServer,
				leafProcess:   leafServer,
				rootResource:  rootDB,
				leafResource:  leafDB,
				user:          accessUser,
				connectorName: connector.GetName(),
				args:          []string{"db", "ls"},
			},
			func(t *testing.T, proxyAddr string, recursive bool, raw []byte) []*types.DatabaseV3 {
				type recursiveServer struct {
					Proxy    string            `json:"proxy"`
					Cluster  string            `json:"cluster"`
					Database *types.DatabaseV3 `json:"database"`
				}

				var databases []*types.DatabaseV3
				if recursive {
					var servers []recursiveServer
					assert.NoError(t, json.Unmarshal(raw, &servers))
					require.NotEmpty(t, servers)

					databases = make([]*types.DatabaseV3, 0, len(servers))
					for _, s := range servers {
						if s.Database.GetName() == "db01" {
							assert.Equal(t, "root", s.Cluster)
						} else {
							assert.Equal(t, "leaf", s.Cluster)
						}

						assert.Equal(t, proxyAddr, s.Proxy)
						require.NoError(t, s.Database.CheckAndSetDefaults())
						databases = append(databases, s.Database)
					}
				} else {
					var servers []*types.DatabaseV3
					assert.NoError(t, json.Unmarshal(raw, &servers))
					require.NotEmpty(t, servers)

					for _, s := range servers {
						require.NoError(t, s.CheckAndSetDefaults())
						databases = append(databases, s)
					}
				}

				return databases
			},
			func(a, b *types.DatabaseV3) bool {
				return strings.Compare(a.GetName(), b.GetName()) < 0
			},
		)
	})

	t.Run("applications", func(t *testing.T) {
		t.Parallel()

		testListingResources(t,
			listPack[*types.AppV3]{
				rootProcess:   rootServer,
				leafProcess:   leafServer,
				rootResource:  rootApp,
				leafResource:  leafApp,
				user:          accessUser,
				connectorName: connector.GetName(),
				args:          []string{"apps", "ls"},
			},
			func(t *testing.T, proxyAddr string, recursive bool, raw []byte) []*types.AppV3 {
				type recursiveServer struct {
					Proxy   string       `json:"proxy"`
					Cluster string       `json:"cluster"`
					App     *types.AppV3 `json:"app"`
				}

				var apps []*types.AppV3
				if recursive {
					var servers []recursiveServer
					assert.NoError(t, json.Unmarshal(raw, &servers))
					require.NotEmpty(t, servers)

					apps = make([]*types.AppV3, 0, len(servers))
					for _, s := range servers {
						if s.App.GetPublicAddr() == "dumper.root" {
							assert.Equal(t, "root", s.Cluster)
						} else {
							assert.Equal(t, "leaf", s.Cluster)
						}

						assert.Equal(t, proxyAddr, s.Proxy)
						require.NoError(t, s.App.CheckAndSetDefaults())
						apps = append(apps, s.App)
					}
				} else {
					assert.NoError(t, json.Unmarshal(raw, &apps))
					require.NotEmpty(t, apps)

					for _, s := range apps {
						require.NoError(t, s.CheckAndSetDefaults())
					}
				}

				return apps
			},
			func(a, b *types.AppV3) bool {
				return strings.Compare(a.GetPublicAddr(), b.GetPublicAddr()) < 0
			},
		)
	})
}

type listPack[T any] struct {
	rootProcess   *service.TeleportProcess
	leafProcess   *service.TeleportProcess
	rootResource  T
	leafResource  T
	user          types.User
	connectorName string
	args          []string
}

func testListingResources[T any](t *testing.T, pack listPack[T], unmarshalFunc func(*testing.T, string, bool, []byte) []T, lessFunc func(a, b T) bool) {
	ctx := context.Background()

	rootProxyAddr, err := pack.rootProcess.ProxyWebAddr()
	require.NoError(t, err)
	leafProxyAddr, err := pack.leafProcess.ProxyWebAddr()
	require.NoError(t, err)

	tests := []struct {
		name      string
		proxyAddr *utils.NetAddr
		cluster   string
		auth      *auth.Server
		recursive bool
		expected  []T
	}{
		{
			name:      "root",
			proxyAddr: rootProxyAddr,
			cluster:   "root",
			auth:      pack.rootProcess.GetAuthServer(),
			expected:  []T{pack.rootResource},
		},
		{
			name:      "leaf",
			proxyAddr: leafProxyAddr,
			cluster:   "leaf",
			auth:      pack.leafProcess.GetAuthServer(),
			expected:  []T{pack.leafResource},
		},
		{
			name:      "leaf via root",
			proxyAddr: rootProxyAddr,
			cluster:   "leaf",
			auth:      pack.rootProcess.GetAuthServer(),
			expected:  []T{pack.leafResource},
		},
		{
			name:      "root recursive",
			proxyAddr: rootProxyAddr,
			cluster:   "root",
			auth:      pack.rootProcess.GetAuthServer(),
			recursive: true,
			expected:  []T{pack.rootResource, pack.leafResource},
		},
		{
			name:      "leaf recursive",
			proxyAddr: leafProxyAddr,
			cluster:   "leaf",
			auth:      pack.leafProcess.GetAuthServer(),
			recursive: true,
			expected:  []T{pack.leafResource},
		},
		{
			name:      "leaf via root recursive",
			proxyAddr: rootProxyAddr,
			cluster:   "leaf",
			auth:      pack.rootProcess.GetAuthServer(),
			recursive: true,
			expected:  []T{pack.leafResource},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tmpHomePath := t.TempDir()
			err = Run(
				ctx,
				[]string{
					"--insecure",
					"login",
					"--proxy", test.proxyAddr.String(),
					test.cluster,
				},
				setHomePath(tmpHomePath),
				setMockSSOLogin(test.auth, pack.user, pack.connectorName),
			)
			require.NoError(t, err)

			stdout := &output{buf: bytes.Buffer{}}

			args := slices.Clone(pack.args)
			args = append(args, "--format", "json")
			if test.recursive {
				args = append(args, "-R")
			}
			err = Run(ctx,
				args,
				setHomePath(tmpHomePath),
				func(conf *CLIConf) error {
					conf.overrideStdin = &bytes.Buffer{}
					conf.OverrideStdout = stdout
					return nil
				},
			)
			require.NoError(t, err)

			out := unmarshalFunc(t, test.proxyAddr.String(), test.recursive, stdout.buf.Bytes())
			require.Empty(t, cmp.Diff(test.expected, out, cmpopts.SortSlices(lessFunc)))
		})
	}
}

// TestProxyTemplates verifies proxy templates apply properly to client config.
func TestProxyTemplatesMakeClient(t *testing.T) {
	t.Parallel()

	tshConfig := client.TSHConfig{
		ProxyTemplates: client.ProxyTemplates{
			{
				Template: `^(.+)\.(us.example.com):(.+)$`,
				Proxy:    "$2:443",
				Cluster:  "$2",
				Host:     "$1:4022",
			},
		},
	}
	require.NoError(t, tshConfig.Check())

	newCLIConf := func(modify func(conf *CLIConf)) *CLIConf {
		// minimal configuration (with defaults)
		conf := &CLIConf{
			Proxy:     "proxy:3080",
			UserHost:  "localhost",
			HomePath:  t.TempDir(),
			TSHConfig: tshConfig,
		}

		// Create a empty profile so we don't ping proxy.
		profile := &profile.Profile{
			SSHProxyAddr: "proxy:3023",
			WebProxyAddr: "proxy:3080",
		}
		err := conf.getClientStore().SaveProfile(profile, true)
		require.NoError(t, err)

		modify(conf)
		return conf
	}

	for _, tt := range []struct {
		name         string
		InConf       *CLIConf
		expectErr    bool
		outHost      string
		outPort      int
		outCluster   string
		outJumpHosts []utils.JumpHost
	}{
		{
			name: "does not match template",
			InConf: newCLIConf(func(conf *CLIConf) {
				conf.UserHost = "node-1.cn.example.com:3022"
			}),
			outHost:    "node-1.cn.example.com:3022",
			outCluster: "proxy",
		},
		{
			name: "does not match template with -J {{proxy}}",
			InConf: newCLIConf(func(conf *CLIConf) {
				conf.UserHost = "node-1.cn.example.com:3022"
				conf.ProxyJump = "{{proxy}}"
			}),
			expectErr: true,
		},
		{
			name: "match with full host set",
			InConf: newCLIConf(func(conf *CLIConf) {
				conf.UserHost = "user@node-1.us.example.com:3022"
			}),
			outHost:    "node-1",
			outPort:    4022,
			outCluster: "us.example.com",
			outJumpHosts: []utils.JumpHost{{
				Addr: utils.NetAddr{
					Addr:        "us.example.com:443",
					AddrNetwork: "tcp",
				},
			}},
		},
		{
			name: "match with host and port set",
			InConf: newCLIConf(func(conf *CLIConf) {
				conf.UserHost = "user@node-1.us.example.com"
				conf.NodePort = 3022
			}),
			outHost:    "node-1",
			outPort:    4022,
			outCluster: "us.example.com",
			outJumpHosts: []utils.JumpHost{{
				Addr: utils.NetAddr{
					Addr:        "us.example.com:443",
					AddrNetwork: "tcp",
				},
			}},
		},
		{
			name: "match with -J {{proxy}} set",
			InConf: newCLIConf(func(conf *CLIConf) {
				conf.UserHost = "node-1.us.example.com:3022"
				conf.ProxyJump = "{{proxy}}"
			}),
			outHost:    "node-1",
			outPort:    4022,
			outCluster: "us.example.com",
			outJumpHosts: []utils.JumpHost{{
				Addr: utils.NetAddr{
					Addr:        "us.example.com:443",
					AddrNetwork: "tcp",
				},
			}},
		},
		{
			name: "match does not overwrite user specified proxy jump",
			InConf: newCLIConf(func(conf *CLIConf) {
				conf.UserHost = "node-1.us.example.com:3022"
				conf.SiteName = "specified.cluster"
				conf.ProxyJump = "specified.proxy.com:443"
			}),
			outHost:    "node-1",
			outPort:    4022,
			outCluster: "us.example.com",
			outJumpHosts: []utils.JumpHost{{
				Addr: utils.NetAddr{
					Addr:        "specified.proxy.com:443",
					AddrNetwork: "tcp",
				},
			}},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			tc, err := makeClient(tt.InConf)
			if tt.expectErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.outHost, tc.Host)
			require.Equal(t, tt.outPort, tc.HostPort)
			require.Equal(t, tt.outJumpHosts, tc.JumpHosts)
			require.Equal(t, tt.outCluster, tc.SiteName)
		})
	}
}

func TestRolesToString(t *testing.T) {
	tests := []struct {
		name     string
		roles    []string
		expected string
		debug    bool
	}{
		{
			name:     "empty",
			roles:    []string{},
			expected: "",
		},
		{
			name:     "exceed threshold okta roles should be squashed",
			roles:    append([]string{"app-figma-reviewer-okta-acl-role", "app-figma-access-okta-acl-role"}, []string{"r1", "r2", "r3", "r4", "r5", "r6", "r7", "r8", "r9", "r10"}...),
			expected: "r1, r10, r2, r3, r4, r5, r6, r7, r8, r9, and 2 more Okta access list roles ...",
		},
		{
			name:     "debug flag",
			roles:    append([]string{"app-figma-reviewer-okta-acl-role", "app-figma-access-okta-acl-role"}, []string{"r1", "r2", "r3", "r4", "r5", "r6", "r7", "r8", "r9", "r10"}...),
			debug:    true,
			expected: "r1, r10, r2, r3, r4, r5, r6, r7, r8, r9, app-figma-access-okta-acl-role, app-figma-reviewer-okta-acl-role",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.expected, rolesToString(tc.debug, tc.roles))
		})
	}
}

// TestResolve tests that host resolution works for various inputs and
// that proxy templates are respected.
func TestResolve(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	accessRoleName := "access"
	sshHostname := "test-ssh-server"

	accessUser, err := types.NewUser(accessRoleName)
	require.NoError(t, err)
	accessUser.SetRoles([]string{accessRoleName})

	user, err := user.Current()
	require.NoError(t, err)
	accessUser.SetLogins([]string{user.Username})

	traits := map[string][]string{
		constants.TraitLogins: {user.Username},
	}
	accessUser.SetTraits(traits)

	connector := mockConnector(t)
	rootServerOpts := []testserver.TestServerOptFunc{
		testserver.WithBootstrap(connector, accessUser),
		testserver.WithHostname(sshHostname),
		testserver.WithClusterName(t, "root"),
		testserver.WithSSHPublicAddrs("127.0.0.1:0"),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.SSH.Enabled = true
			cfg.SSH.PublicAddrs = []utils.NetAddr{cfg.SSH.Addr}
			cfg.SSH.DisableCreateHostUser = true
			cfg.SSH.Labels = map[string]string{
				"animal": "llama",
				"env":    "dev",
			}
		}),
	}
	rootServer := testserver.MakeTestServer(t, rootServerOpts...)

	node := testserver.MakeTestServer(t,
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.SetAuthServerAddresses(rootServer.Config.AuthServerAddresses())
			cfg.Hostname = "second-node"
			cfg.Auth.Enabled = false
			cfg.Proxy.Enabled = false
			cfg.SSH.Enabled = true
			cfg.SSH.DisableCreateHostUser = true
			cfg.SSH.Labels = map[string]string{
				"animal": "shark",
				"env":    "dev",
			}
		}))

	rootProxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		found, err := rootServer.GetAuthServer().GetNodes(ctx, apidefaults.Namespace)
		if !assert.NoError(t, err) || !assert.Len(t, found, 2) {
			return
		}
	}, 10*time.Second, 100*time.Millisecond)

	tmpHomePath := t.TempDir()
	rootAuth := rootServer.GetAuthServer()

	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--proxy", rootProxyAddr.String(),
		"--user", user.Username,
	}, setHomePath(tmpHomePath), setMockSSOLogin(rootAuth, accessUser, connector.GetName()))
	require.NoError(t, err)

	tests := []struct {
		name      string
		hostname  string
		quiet     bool
		assertion require.ErrorAssertionFunc
	}{
		{
			name:      "resolved without using templates",
			hostname:  sshHostname,
			assertion: require.NoError,
		},
		{
			name:      "resolved via predicate from template",
			hostname:  "2.3.4.5",
			assertion: require.NoError,
		},
		{
			name:      "resolved via search from template",
			hostname:  "llama.example.com:3023",
			assertion: require.NoError,
		},
		{
			name:     "no matching host",
			hostname: "asdf",
			assertion: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err, i...)
				require.ErrorContains(tt, err, "no matching hosts", i...)
			},
		},
		{
			name:      "quiet prevents output",
			hostname:  node.Config.Hostname,
			quiet:     true,
			assertion: require.NoError,
		},
		{
			name:     "multiple matching hosts",
			hostname: "dev.example.com",
			assertion: func(tt require.TestingT, err error, i ...interface{}) {
				require.Error(tt, err, i...)
				require.ErrorContains(tt, err, "multiple matching hosts", i...)
			},
		},
	}

	for _, test := range tests {
		test := test
		ctx := context.Background()
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			stdout := &output{buf: bytes.Buffer{}}
			stderr := &output{buf: bytes.Buffer{}}

			args := []string{"resolve"}
			if test.quiet {
				args = append(args, "-q")
			}
			args = append(args, test.hostname)

			err := Run(ctx, args,
				setHomePath(tmpHomePath),
				setTSHConfig(client.TSHConfig{
					ProxyTemplates: client.ProxyTemplates{
						{
							Template: `^([0-9\.]+):\d+$`,
							Query:    `labels["animal"] == "llama"`,
						},
						{
							Template: `^(.*).example.com:\d+$`,
							Search:   "$1",
						},
					},
				}),
				func(conf *CLIConf) error {
					conf.overrideStdin = &bytes.Buffer{}
					conf.OverrideStdout = stdout
					conf.overrideStderr = stderr
					return nil
				},
			)

			test.assertion(t, err)
			if err != nil {
				return
			}

			if test.quiet {
				require.Empty(t, stdout.String())
			} else {
				require.Contains(t, stdout.String(), sshHostname)
			}
		})
	}
}

// TestInteractiveCompatibilityFlags verifies that "tsh ssh -t" and "tsh ssh -T"
// behave similar to OpenSSH.
func TestInteractiveCompatibilityFlags(t *testing.T) {
	// Require the "tty" program exist somewhere in $PATH, otherwise fail.
	tty, err := exec.LookPath("tty")
	require.NoError(t, err)

	// Create roles and users that will be used in test.
	local, err := user.Current()
	require.NoError(t, err)
	nodeAccess, err := types.NewRole("foo", types.RoleSpecV6{
		Allow: types.RoleConditions{
			Logins:     []string{local.Username},
			NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
		},
	})
	require.NoError(t, err)
	user, err := types.NewUser("bar@example.com")
	require.NoError(t, err)
	user.SetRoles([]string{"access", "foo"})

	// Create Auth, Proxy, and Node Service used in tests.
	hostname := uuid.NewString()
	connector := mockConnector(t)
	authProcess, proxyProcess := makeTestServers(t,
		withBootstrap(connector, nodeAccess, user),
		withConfig(func(cfg *servicecfg.Config) {
			cfg.Hostname = hostname
			cfg.SSH.Enabled = true
			cfg.SSH.Addr = utils.NetAddr{
				AddrNetwork: "tcp",
				Addr:        net.JoinHostPort("127.0.0.1", ports.Pop()),
			}
		}))

	// Extract Auth Service and Proxy Service address.
	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)
	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	// Run "tsh login".
	home := t.TempDir()
	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr.String()},
		setHomePath(home),
		setMockSSOLogin(authServer, user, connector.GetName()))
	require.NoError(t, err)

	// Test compatibility with OpenSSH "-T" and "-t" flags. Note that multiple
	// -t options is still not supported.
	//
	// From "man 1 ssh".
	//
	//  -T      Disable pseudo-terminal allocation.
	//  -t      Force pseudo-terminal allocation. This can be used to execute
	//          arbitrary screen-based programs on a remote machine, which can
	//          be very useful, e.g. when implementing menu services. Multiple
	//          -t options force tty allocation, even if ssh has no local tty.
	tests := []struct {
		name        string
		flag        string
		assertError require.ErrorAssertionFunc
	}{
		{
			name: "disable pseudo-terminal allocation",
			flag: "-T",
			assertError: func(t require.TestingT, err error, i ...interface{}) {
				var exiterr *exec.ExitError
				if errors.As(err, &exiterr) {
					require.Equal(t, 1, exiterr.ExitCode())
				} else {
					require.Fail(t, "Non *exec.ExitError type: %T.", err)
				}
			},
		},
		{
			name:        "force pseudo-terminal allocation",
			flag:        "-t",
			assertError: require.NoError,
		},
	}

	// Turn the binary running tests into a fake "tsh" binary so it can
	// re-execute itself.
	t.Setenv(types.HomeEnvVar, home)
	t.Setenv(tshBinMainTestEnv, "1")
	tshBin, err := os.Executable()
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Add some resiliency for cases where connections are attempted
			// prior to the inventory being updated to contain the target node
			// above.
			require.EventuallyWithT(t, func(t *assert.CollectT) {
				out, err := exec.Command(tshBin, "ssh", tt.flag, hostname, tty).CombinedOutput()
				tt.assertError(t, err, string(out))
			}, 20*time.Second, 100*time.Millisecond)
		})
	}
}

// TestVersionCompatibilityFlags verifies that "tsh ssh -V" returns Teleport
// version similar to OpenSSH.
func TestVersionCompatibilityFlags(t *testing.T) {
	t.Setenv(tshBinMainTestEnv, "1")
	tshBin, err := os.Executable()
	require.NoError(t, err)

	output, err := exec.Command(tshBin, "ssh", "-V").CombinedOutput()
	require.NoError(t, err, output)
	require.Equal(t, "Teleport CLI", string(bytes.TrimSpace(output)))
}

// TestSCP validates that tsh scp correctly copy file content while also
// ensuring that proxy templates are respected.
func TestSCP(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{TestBuildType: modules.BuildEnterprise})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	accessRoleName := "access"
	sshHostname := "test-ssh-server"

	accessUser, err := types.NewUser(accessRoleName)
	require.NoError(t, err)
	accessUser.SetRoles([]string{accessRoleName})

	user, err := user.Current()
	require.NoError(t, err)
	accessUser.SetLogins([]string{user.Username})

	traits := map[string][]string{
		constants.TraitLogins: {user.Username},
	}
	accessUser.SetTraits(traits)

	connector := mockConnector(t)
	rootServerOpts := []testserver.TestServerOptFunc{
		testserver.WithBootstrap(connector, accessUser),
		testserver.WithHostname(sshHostname),
		testserver.WithClusterName(t, "root"),
		testserver.WithSSHPublicAddrs("127.0.0.1:0"),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.SSH.Enabled = true
			cfg.SSH.PublicAddrs = []utils.NetAddr{cfg.SSH.Addr}
			cfg.SSH.DisableCreateHostUser = true
			cfg.SSH.Labels = map[string]string{
				"animal": "llama",
				"env":    "dev",
			}
		}),
	}
	rootServer := testserver.MakeTestServer(t, rootServerOpts...)

	// Create a second server to test ambiguous matching.
	testserver.MakeTestServer(t,
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.SetAuthServerAddresses(rootServer.Config.AuthServerAddresses())
			cfg.Hostname = "second-node"
			cfg.Auth.Enabled = false
			cfg.Proxy.Enabled = false
			cfg.SSH.Enabled = true
			cfg.SSH.DisableCreateHostUser = true
			cfg.SSH.Labels = map[string]string{
				"animal": "shark",
				"env":    "dev",
			}
		}))

	rootProxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		found, err := rootServer.GetAuthServer().GetNodes(ctx, apidefaults.Namespace)
		if !assert.NoError(t, err) || !assert.Len(t, found, 2) {
			return
		}
	}, 10*time.Second, 100*time.Millisecond)

	tmpHomePath := t.TempDir()
	rootAuth := rootServer.GetAuthServer()

	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--proxy", rootProxyAddr.String(),
		"--user", user.Username,
	}, setHomePath(tmpHomePath), setMockSSOLogin(rootAuth, accessUser, connector.GetName()))
	require.NoError(t, err)

	sourceFile1 := filepath.Join(t.TempDir(), "source-file")
	expectedFile1 := []byte{6, 7, 8, 9, 0}
	require.NoError(t, os.WriteFile(sourceFile1, expectedFile1, 0o644))

	sourceFile2 := filepath.Join(t.TempDir(), "source-file2")
	expectedFile2 := []byte{1, 2, 3, 4, 5}
	require.NoError(t, os.WriteFile(sourceFile2, expectedFile2, 0o644))

	targetFile1 := uuid.NewString()

	createFile := func(t *testing.T, dir, file string) {
		t.Helper()
		f, err := os.CreateTemp(dir, file)
		require.NoError(t, err)
		require.NoError(t, f.Close())
	}

	tests := []struct {
		name        string
		source      []string
		destination func(t *testing.T, dir string) string
		assertion   require.ErrorAssertionFunc
		expected    map[string][]byte
	}{
		{
			name:        "no paths provided",
			destination: func(*testing.T, string) string { return "" },
			assertion: func(tt require.TestingT, err error, i ...any) {
				require.Error(tt, err, i...)
				require.ErrorContains(tt, err, "local and remote destinations are required", i...)
			},
		},
		{
			name:   "source resolved without using templates",
			source: []string{sshHostname + ":" + sourceFile1},
			destination: func(t *testing.T, dir string) string {
				createFile(t, dir, targetFile1)
				return filepath.Join(dir, targetFile1)
			},
			assertion: require.NoError,
			expected: map[string][]byte{
				targetFile1: expectedFile1,
			},
		},
		{
			name:   "source resolved via predicate from template",
			source: []string{"2.3.4.5:" + sourceFile1},
			destination: func(t *testing.T, dir string) string {
				createFile(t, dir, targetFile1)
				return filepath.Join(dir, targetFile1)
			},
			assertion: require.NoError,
			expected: map[string][]byte{
				targetFile1: expectedFile1,
			},
		},
		{
			name:   "source resolved via search from template",
			source: []string{"llama.example.com:" + sourceFile1},
			destination: func(t *testing.T, dir string) string {
				createFile(t, dir, targetFile1)
				return filepath.Join(dir, targetFile1)
			},
			assertion: require.NoError,
			expected: map[string][]byte{
				targetFile1: expectedFile1,
			},
		},
		{
			name:   "source no matching host",
			source: []string{"asdf.example.com:" + sourceFile1},
			destination: func(t *testing.T, dir string) string {
				createFile(t, dir, targetFile1)
				return filepath.Join(dir, targetFile1)
			},
			assertion: func(tt require.TestingT, err error, i ...any) {
				require.Error(tt, err, i...)
				require.ErrorContains(tt, err, "no matching hosts", i...)
			},
		},
		{
			name:   "source multiple matching hosts",
			source: []string{"dev.example.com:" + sourceFile1},
			destination: func(t *testing.T, dir string) string {
				createFile(t, dir, targetFile1)
				return filepath.Join(dir, targetFile1)
			},
			assertion: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, teleport.ErrNodeIsAmbiguous, i...)
			},
		},
		{
			name:   "destination resolved without using templates",
			source: []string{sourceFile1},
			destination: func(t *testing.T, dir string) string {
				createFile(t, dir, targetFile1)
				return sshHostname + ":" + filepath.Join(dir, targetFile1)
			},
			assertion: require.NoError,
			expected: map[string][]byte{
				targetFile1: expectedFile1,
			},
		},
		{
			name:   "destination resolved via predicate from template",
			source: []string{sourceFile1},
			destination: func(t *testing.T, dir string) string {
				createFile(t, dir, targetFile1)
				return "2.3.4.5:" + filepath.Join(dir, targetFile1)
			},
			assertion: require.NoError,
			expected: map[string][]byte{
				targetFile1: expectedFile1,
			},
		},
		{
			name:   "destination resolved via search from template",
			source: []string{sourceFile1},
			destination: func(t *testing.T, dir string) string {
				createFile(t, dir, targetFile1)
				return "llama.example.com:" + filepath.Join(dir, targetFile1)
			},
			assertion: require.NoError,
			expected: map[string][]byte{
				targetFile1: expectedFile1,
			},
		},
		{
			name:   "destination no matching host",
			source: []string{sourceFile1},
			destination: func(t *testing.T, dir string) string {
				createFile(t, dir, targetFile1)
				return "asdf.example.com:" + filepath.Join(dir, targetFile1)
			},
			assertion: func(tt require.TestingT, err error, i ...any) {
				require.Error(tt, err, i...)
				require.ErrorContains(tt, err, "no matching hosts", i...)
			},
		},
		{
			name:   "destination multiple matching hosts",
			source: []string{sourceFile1},
			destination: func(t *testing.T, dir string) string {
				createFile(t, dir, targetFile1)
				return "dev.example.com:" + filepath.Join(dir, targetFile1)
			},
			assertion: func(tt require.TestingT, err error, i ...any) {
				require.ErrorIs(tt, err, teleport.ErrNodeIsAmbiguous, i...)
			},
		},
		{
			name:        "upload multiple files",
			source:      []string{sourceFile1, sourceFile2},
			destination: func(t *testing.T, dir string) string { return "llama.example.com:" + dir },
			assertion:   require.NoError,
			expected: map[string][]byte{
				filepath.Base(sourceFile1): expectedFile1,
				filepath.Base(sourceFile2): expectedFile2,
			},
		},
	}

	for _, test := range tests {
		test := test
		ctx := context.Background()
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			outputDir := t.TempDir()
			destination := test.destination(t, outputDir)

			args := []string{"scp", "-d", "--no-resume", "--insecure", "-q"}
			args = append(args, test.source...)
			args = append(args, destination)
			err := Run(ctx,
				args,
				setHomePath(tmpHomePath),
				setTSHConfig(client.TSHConfig{
					ProxyTemplates: client.ProxyTemplates{
						{
							Template: `^([0-9\.]+):\d+$`,
							Query:    `labels["animal"] == "llama"`,
						},
						{
							Template: `^(.*).example.com:\d+$`,
							Search:   "$1",
						},
					},
				}),
				func(conf *CLIConf) error {
					// Relogin is disabled since some of the error cases return a
					// BadParameter error which triggers the re-authentication flow
					// and may result in a different authentication related error
					// being returned instead of the expected errors.
					conf.Relogin = false
					return nil
				},
			)
			test.assertion(t, err)
			if err != nil {
				return
			}

			for file, expected := range test.expected {
				got, err := os.ReadFile(filepath.Join(outputDir, file))
				require.NoError(t, err)
				require.Equal(t, expected, got)
			}
		})
	}
}

func TestSetEnvVariables(t *testing.T) {
	testCases := []struct {
		name              string
		envVars           map[string]string
		sendEnvVariables  []string
		expectedExtraEnvs map[string]string
	}{
		{
			name: "Skip unset var",
			envVars: map[string]string{
				"TEST_VAR1": "value1",
				"TEST_VAR2": "value2",
			},
			sendEnvVariables: []string{"TEST_VAR1", "TEST_VAR2", "UNSET_VAR"},
			expectedExtraEnvs: map[string]string{
				"TEST_VAR1": "value1",
				"TEST_VAR2": "value2",
			},
		},
		{
			name:              "Sending empty var",
			envVars:           map[string]string{"EMPTY_VAR": ""},
			sendEnvVariables:  []string{"EMPTY_VAR"},
			expectedExtraEnvs: map[string]string{"EMPTY_VAR": ""},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			for k, v := range tc.envVars {
				t.Setenv(k, v)
			}

			c := &client.Config{}
			options := Options{SendEnvVariables: tc.sendEnvVariables}

			setEnvVariables(c, options)

			require.Equal(t, tc.expectedExtraEnvs, c.ExtraEnvs)
		})
	}
}

func TestLoggingOpts(t *testing.T) {
	tmpHomePath := t.TempDir()

	tests := []struct {
		name      string
		envDebug  *bool
		envOSLog  *bool
		flagDebug *bool
		flagOSLog *bool
		wantDebug require.BoolAssertionFunc
		wantOSLog require.BoolAssertionFunc
	}{
		{
			name:      "no flags or env vars",
			wantDebug: require.False,
			wantOSLog: require.False,
		},
		{
			name:      "just debug flag",
			flagDebug: boolPtr(true),
			wantDebug: require.True,
			wantOSLog: require.False,
		},
		{
			name:      "just os-log flag",
			flagOSLog: boolPtr(true),
			wantDebug: require.True,
			wantOSLog: require.True,
		},
		{
			name:      "debug and os-log flags",
			flagDebug: boolPtr(true),
			flagOSLog: boolPtr(true),
			wantDebug: require.True,
			wantOSLog: require.True,
		},
		{
			name:      "just debug env var",
			envDebug:  boolPtr(true),
			wantDebug: require.True,
			wantOSLog: require.False,
		},
		{
			name:      "just os-log env var",
			envOSLog:  boolPtr(true),
			wantDebug: require.True,
			wantOSLog: require.True,
		},
		{
			name:      "debug and os-log flags",
			envDebug:  boolPtr(true),
			envOSLog:  boolPtr(true),
			wantDebug: require.True,
			wantOSLog: require.True,
		},
		{
			name:      "everything on",
			envDebug:  boolPtr(true),
			envOSLog:  boolPtr(true),
			flagDebug: boolPtr(true),
			flagOSLog: boolPtr(true),
			wantDebug: require.True,
			wantOSLog: require.True,
		},
		{
			name:      "everything explicitly off",
			envDebug:  boolPtr(false),
			envOSLog:  boolPtr(false),
			flagDebug: boolPtr(false),
			flagOSLog: boolPtr(false),
			wantDebug: require.False,
			wantOSLog: require.False,
		},
		{
			name:      "debug enabled by env, disabled by flag",
			envDebug:  boolPtr(true),
			flagDebug: boolPtr(false),
			wantDebug: require.False,
			wantOSLog: require.False,
		},
		{
			name:      "debug disabled by env, enabled by flag",
			envDebug:  boolPtr(false),
			flagDebug: boolPtr(true),
			wantDebug: require.True,
			wantOSLog: require.False,
		},
		{
			name:      "os-log enabled by env, disabled by flag",
			envOSLog:  boolPtr(true),
			flagOSLog: boolPtr(false),
			wantDebug: require.False,
			wantOSLog: require.False,
		},
		{
			name:      "os-log disabled by env, enabled by flag",
			envOSLog:  boolPtr(false),
			flagOSLog: boolPtr(true),
			wantDebug: require.True,
			wantOSLog: require.True,
		},

		// Verify that final values are set after consulting both env and argv, e.g., disabling os-log
		// by flag doesn't disable debug set by env.
		{
			name:      "os-log enabled by env, disabled by flag; debug enabled by env",
			envOSLog:  boolPtr(true),
			flagOSLog: boolPtr(false),
			envDebug:  boolPtr(true),
			wantDebug: require.True,
			wantOSLog: require.False,
		},
		{
			name:      "os-log disabled by env, enabled by flag; debug disabled by flag",
			envOSLog:  boolPtr(false),
			flagOSLog: boolPtr(true),
			flagDebug: boolPtr(false),
			wantDebug: require.True,
			wantOSLog: require.True,
		},

		// Debug is always enabled together with os-log.
		{
			name:      "os-log enabled by flag, debug disabled by env",
			flagOSLog: boolPtr(true),
			envDebug:  boolPtr(false),
			wantDebug: require.True,
			wantOSLog: require.True,
		},
		{
			name:      "os-log enabled by flag, debug disabled by flag",
			flagOSLog: boolPtr(true),
			flagDebug: boolPtr(false),
			wantDebug: require.True,
			wantOSLog: require.True,
		},
		{
			name:      "os-log enabled by env, debug disabled by env",
			envOSLog:  boolPtr(true),
			envDebug:  boolPtr(false),
			wantDebug: require.True,
			wantOSLog: require.True,
		},
		{
			name:      "os-log enabled by env, debug disabled by flag",
			envOSLog:  boolPtr(true),
			flagDebug: boolPtr(false),
			wantDebug: require.True,
			wantOSLog: require.True,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envDebug != nil {
				t.Setenv(debugEnvVar, strconv.FormatBool(*tt.envDebug))
			}

			if tt.envOSLog != nil {
				t.Setenv(osLogEnvVar, strconv.FormatBool(*tt.envOSLog))
			}

			var flags []string

			if tt.flagDebug != nil {
				if *tt.flagDebug {
					flags = append(flags, "--debug")
				} else {
					flags = append(flags, "--no-debug")
				}
			}

			if tt.flagOSLog != nil {
				if *tt.flagOSLog {
					flags = append(flags, "--os-log")
				} else {
					flags = append(flags, "--no-os-log")
				}
			}

			mustReadLoggingOpts, setLoggingOptsFromCLIConf := prepareCLIOptionForReadingLoggingOpts()
			err := Run(t.Context(), append([]string{"version"}, flags...),
				setHomePath(tmpHomePath), setLoggingOptsFromCLIConf)
			require.NoError(t, err)

			opts := mustReadLoggingOpts(t)
			tt.wantDebug(t, opts.debug, "unexpected cf.Debug value")
			tt.wantOSLog(t, opts.osLog, "unexpected cf.OSLog value")
		})
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func prepareCLIOptionForReadingLoggingOpts() (func(t *testing.T) loggingOpts, CliOption) {
	var cf *CLIConf

	setLoggingOptsFromCLIConf := func(cliConfFromRun *CLIConf) error {
		cf = cliConfFromRun
		return nil
	}

	mustReadLoggingOpts := func(t *testing.T) loggingOpts {
		t.Helper()

		if cf == nil {
			t.Fatalf("mustReadLoggingOpts was called before setLoggingOptsFromCLIConf was executed")
		}

		return loggingOpts{
			debug: cf.Debug,
			osLog: cf.OSLog,
		}
	}

	return mustReadLoggingOpts, setLoggingOptsFromCLIConf
}
