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
	"fmt"
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
	"github.com/stretchr/testify/require"
	otlp "go.opentelemetry.io/proto/otlp/trace/v1"
	"golang.org/x/exp/slices"
	yamlv2 "gopkg.in/yaml.v2"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wanlib "github.com/gravitational/teleport/lib/auth/webauthn"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/kube/kubeconfig"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/observability/tracing"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/sshutils/x11"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
)

// TestNodeAccess verifies "tsh ssh" and "tsh scp" command with various configurations.
func TestNodeAccess(t *testing.T) {
	ctx := context.Background()

	createAgent(t)

	user, err := user.Current()
	require.NoError(t, err)

	connector := mockConnector(t)
	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector))

	authAddr, err := authProcess.AuthAddr()
	require.NoError(t, err)
	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)
	makeTestSSHNode(t, authAddr, withHostname("node01"), withSSHLabel("env", "staging"), withSSHAddr("127.0.0.1:0"), withSSHPublicAddrs("localhost:0"))

	authServer := authProcess.GetAuthServer()
	err = authServer.SetAuthPreference(ctx, &types.AuthPreferenceV2{
		Spec: types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.SecondFactorOptional,
			Webauthn: &types.Webauthn{
				RPID: "localhost",
			},
		},
	})
	require.NoError(t, err)

	const origin = "https://localhost"
	device, err := mocku2f.Create()
	require.NoError(t, err)
	device.SetPasswordless()
	setupWebAuthnChallengeSolver(t, device, origin, true)

	setupUserAndRole := func(t *testing.T, name string, roleSpec types.RoleSpecV6) {
		// create role
		role, err := types.NewRole(name, roleSpec)
		require.NoError(t, err)
		err = authServer.CreateRole(ctx, role)
		require.NoError(t, err)

		// create user
		user, err := types.NewUser(name)
		user.SetRoles([]string{name})
		require.NoError(t, err)
		err = authServer.CreateUser(ctx, user)
		require.NoError(t, err)
	}

	setupUserMFA := func(t *testing.T, name string) {
		token, err := authServer.CreateResetPasswordToken(ctx, auth.CreateUserTokenRequest{
			Name: name,
		})
		require.NoError(t, err)

		tokenID := token.GetName()
		res, err := authServer.CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
			TokenID:     tokenID,
			DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
			DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
		})
		require.NoError(t, err)
		cc := wanlib.CredentialCreationFromProto(res.GetWebauthn())

		ccr, err := device.SignCredentialCreation(origin, cc)
		require.NoError(t, err)
		_, err = authServer.ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
			TokenID: tokenID,
			NewMFARegisterResponse: &proto.MFARegisterResponse{
				Response: &proto.MFARegisterResponse_Webauthn{
					Webauthn: wanlib.CredentialCreationResponseToProto(ccr),
				},
			},
		})
		require.NoError(t, err)
	}

	testCases := []struct {
		// the test name, and the user/role name for the test.
		name  string
		setup func(t *testing.T)
	}{
		{
			name: "default",
			setup: func(t *testing.T) {
				userName := "default"
				setupUserAndRole(t, userName, types.RoleSpecV6{
					Allow: types.RoleConditions{
						Logins:     []string{user.Username},
						NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					},
					Options: types.RoleOptions{
						ForwardAgent:   true,
						MaxSessions:    1,
						MaxConnections: 1,
					},
				})
			},
		},
		{
			name: "session_mfa",
			setup: func(t *testing.T) {
				userName := "session_mfa"
				setupUserAndRole(t, userName, types.RoleSpecV6{
					Allow: types.RoleConditions{
						Logins:     []string{user.Username},
						NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					},
					Options: types.RoleOptions{
						RequireMFAType: types.RequireMFAType_SESSION,
						ForwardAgent:   true,
						MaxSessions:    1,
						MaxConnections: 1,
					},
				})
				setupUserMFA(t, userName)
			},
		},
	}

	type testLoginFunc func(t *testing.T, proxyAddr string, user string) CliOption

	loginCases := map[string]testLoginFunc{
		"home login": func(t *testing.T, proxyAddr, user string) CliOption {
			_, _, opt := mustLoginHome(t, authServer, proxyAddr, user, connector.GetName(), func(cf *CLIConf) error {
				cf.AddKeysToAgent = "no"
				return nil
			})
			return opt
		},
		"identity login": func(t *testing.T, proxyAddr, user string) CliOption {
			_, opt := mustLoginIdentity(t, authServer, proxyAddr, user, connector.GetName(), func(cf *CLIConf) error {
				cf.AddKeysToAgent = "no"
				return nil
			})
			return opt
		},
		"headless login": func(t *testing.T, proxyAddr, user string) CliOption {
			return setMockHeadlessLogin(t, authServer, user, proxyAddr)
		},
	}

	hostNameCases := map[string]string{
		"nodename":    "node01",
		"labels":      "env=staging",
		"addr":        "127.0.0.1",
		"public_addr": "localhost",
	}

	testSCP := func(t *testing.T, hostName string, upload bool, opts ...CliOption) {
		testDir := t.TempDir()
		localFilePath := filepath.Join(testDir, "local-test-file")
		remoteFilePath := filepath.Join(testDir, "remote-test-file")
		hostFilePath := fmt.Sprintf("%v:%v", hostName, remoteFilePath)

		dst, src, srcFilePath := localFilePath, hostFilePath, remoteFilePath
		if upload {
			dst, src, srcFilePath = hostFilePath, localFilePath, localFilePath
		}

		file, err := os.Create(srcFilePath)
		require.NoError(t, err)
		defer file.Close()

		testText := "This-is-a-test-file"
		file.WriteString(testText)

		err = Run(context.Background(), []string{
			"scp",
			"--insecure",
			src,
			dst,
		}, opts...)
		require.NoError(t, err)

		copiedText, err := os.ReadFile(localFilePath)
		require.NoError(t, err)
		require.Equal(t, testText, string(copiedText))
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)

			for loginName, loginFunc := range loginCases {
				t.Run(loginName, func(t *testing.T) {
					loginOpt := loginFunc(t, proxyAddr.String(), tc.name)

					for hostType, hostName := range hostNameCases {
						t.Run(hostType, func(t *testing.T) {
							t.Run("SSH", func(t *testing.T) {
								err := Run(ctx, []string{
									"ssh",
									"--insecure",
									hostName,
									"echo", "hello",
								}, loginOpt)
								require.NoError(t, err)

								t.Run("AgentForwarding", func(t *testing.T) {
									stdout := &output{buf: bytes.Buffer{}}
									err := Run(ctx, []string{
										"--add-keys-to-agent=yes",
										"ssh",
										"--insecure",
										"-A",
										hostName,
										"ssh-add", "-l",
									}, loginOpt, func(cf *CLIConf) error {
										cf.OverrideStdout = stdout
										return nil
									})
									require.NoError(t, err)

									agentKeyComment := fmt.Sprintf("teleport:127.0.0.1:localhost:%v", tc.name)
									require.Contains(t, stdout.String(), agentKeyComment, "agent key entry for %q not found during session:\n%v", agentKeyComment, stdout.String())
								})
							})

							// 'tsh scp' does not support matching by node labels.
							if hostType != "labels" {
								t.Run("SCPDownload", func(t *testing.T) {
									testSCP(t, hostName, false, loginOpt)
								})
								t.Run("SCPUpload", func(t *testing.T) {
									testSCP(t, hostName, true, loginOpt)
								})
							}
						})
					}
				})
			}
		})
	}
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
		"--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
		cf.MockSSOLogin = ssoLogin
		cf.AuthConnector = connector.GetName()
		return nil
	})
	require.ErrorIs(t, err, loginFailed)
}

func TestOIDCLogin(t *testing.T) {
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

	mustLoginHome(t, authServer, proxyAddr.String(), alice.GetName(), connector.GetName(), setClusterName("localhost"), setOverrideStderr(buf), func(cf *CLIConf) error {
		cf.Username = "alice" // explicitly use wrong user name, it should be ignored
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
		opts               []CliOption
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
			name: "write identity in kubeconfig format with tls routing enabled",
			opts: []CliOption{
				func(cf *CLIConf) error {
					cf.Format = "kubernetes"
					cf.KubernetesCluster = kubeClusterName
					return nil
				},
			},
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
			name: "write identity in kubeconfig format with tls routing disabled",
			opts: []CliOption{
				func(cf *CLIConf) error {
					cf.Format = "kubernetes"
					cf.KubernetesCluster = kubeClusterName
					return nil
				},
			},
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
			if tt.requiresTLSRouting {
				switchProxyListenerMode(t, authServer, types.ProxyListenerMode_Multiplex)
			}

			identityFile, _ := mustLoginIdentity(t, authServer, proxyAddr.String(), alice.GetName(), connector.GetName(), tt.opts...)

			require.NoError(t, err)
			tt.validationFunc(t, identityFile)
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

	loginOpts := []CliOption{
		setHomePath(t.TempDir()),
		setMockSSOLogin(t, authServer, alice.GetName(), connector.GetName()),
		setOverrideStderr(buf),
	}

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr.String(),
	}, loginOpts...)
	require.NoError(t, err)
	findMOTD(t, sc, motd)

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr.String(),
		"localhost",
	}, loginOpts...)
	require.NoError(t, err)
	findMOTD(t, sc, motd)

	err = Run(context.Background(), []string{"logout"}, loginOpts...)
	require.NoError(t, err)

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr.String(),
		"localhost",
	}, loginOpts...)
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
		"--proxy", proxyAddr1.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(t, authServer1, alice.GetName(), connector.GetName()))
	require.NoError(t, err)

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr2.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(t, authServer2, alice.GetName(), connector.GetName()))

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
		"--auth", connector.GetName(),
		"--proxy", proxyAddr1.String(),
	}, setHomePath(tmpHomePath))

	require.Error(t, err)

	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
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
	tc, err := makeClient(&conf, true)
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

	tc, err = makeClient(&conf, true)
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
	tc, err = makeClient(&conf, true)
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
	tc, err = makeClient(&conf, true)
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
	tc, err = makeClient(&conf, true)
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
		IdentityFileIn:     "../../fixtures/certs/identities/tls.pem",
		Context:            context.Background(),
		InsecureSkipVerify: true,
	}
	tc, err = makeClient(&conf, true)
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

	const origin = "https://localhost"
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

	device, err := mocku2f.Create()
	require.NoError(t, err)
	device.SetPasswordless()

	rootAuth, rootProxy := makeTestServers(t, withBootstrap(connector, alice, noAccessRole, sshLoginRole, perSessionMFARole))

	authAddr, err := rootAuth.AuthAddr()
	require.NoError(t, err)

	proxyAddr, err := rootProxy.ProxyWebAddr()
	require.NoError(t, err)

	stage1Hostname := "test-stage-1"
	node := makeTestSSHNode(t, authAddr, withHostname(stage1Hostname), withSSHLabel("env", "stage"))
	sshHostID := node.Config.HostUUID

	stage2Hostname := "test-stage-2"
	node2 := makeTestSSHNode(t, authAddr, withHostname(stage2Hostname), withSSHLabel("env", "stage"))
	sshHostID2 := node2.Config.HostUUID

	prodHostname := "test-prod-1"
	nodeProd := makeTestSSHNode(t, authAddr, withHostname(prodHostname), withSSHLabel("env", "prod"))
	sshHostID3 := nodeProd.Config.HostUUID

	hasNodes := func(hostIDs ...string) func() bool {
		return func() bool {
			nodes, err := rootAuth.GetAuthServer().GetNodes(ctx, apidefaults.Namespace)
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
	require.Eventually(t, hasNodes(sshHostID, sshHostID2, sshHostID3),
		5*time.Second, 100*time.Millisecond, "nodes never joined cluster")

	defaultPreference, err := rootAuth.GetAuthServer().GetAuthPreference(ctx)
	require.NoError(t, err)

	// set the default auth preference
	webauthnPreference := &types.AuthPreferenceV2{
		Spec: types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.SecondFactorOptional,
			Webauthn: &types.Webauthn{
				RPID: "localhost",
			},
		},
	}
	err = rootAuth.GetAuthServer().SetAuthPreference(ctx, webauthnPreference)
	require.NoError(t, err)

	token, err := rootAuth.GetAuthServer().CreateResetPasswordToken(ctx, auth.CreateUserTokenRequest{
		Name: "alice",
	})
	require.NoError(t, err)
	tokenID := token.GetName()
	res, err := rootAuth.GetAuthServer().CreateRegisterChallenge(ctx, &proto.CreateRegisterChallengeRequest{
		TokenID:     tokenID,
		DeviceType:  proto.DeviceType_DEVICE_TYPE_WEBAUTHN,
		DeviceUsage: proto.DeviceUsage_DEVICE_USAGE_PASSWORDLESS,
	})
	require.NoError(t, err)
	cc := wanlib.CredentialCreationFromProto(res.GetWebauthn())

	ccr, err := device.SignCredentialCreation(origin, cc)
	require.NoError(t, err)
	_, err = rootAuth.GetAuthServer().ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
		TokenID: tokenID,
		NewMFARegisterResponse: &proto.MFARegisterResponse{
			Response: &proto.MFARegisterResponse_Webauthn{
				Webauthn: wanlib.CredentialCreationResponseToProto(ccr),
			},
		},
	})
	require.NoError(t, err)

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
	}{
		{
			name:           "default auth preference runs commands on multiple nodes without mfa",
			authPreference: defaultPreference,
			target:         "env=stage",
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\ntest\n", i, i2...)
			},
			errAssertion: require.NoError,
		},
		{
			name:   "webauthn auth preference runs commands on multiple matches without mfa",
			target: "env=stage",
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\ntest\n", i, i2...)
			},
			errAssertion: require.NoError,
		},
		{
			name:   "webauthn auth preference runs commands on a single match without mfa",
			target: "env=prod",
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			errAssertion: require.NoError,
		},
		{
			name:            "no matching hosts",
			target:          "env=dev",
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
			setup: func(t *testing.T) {
				setupWebAuthnChallengeSolver(t, device, origin, true)
			},
			target: "env=stage",
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
			setup: func(t *testing.T) {
				setupWebAuthnChallengeSolver(t, device, origin, true)
			},
			target: "env=prod",
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
			setup: func(t *testing.T) {
				setupWebAuthnChallengeSolver(t, device, origin, true)
			},
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
			roles: []string{"access", sshLoginRole.GetName(), perSessionMFARole.GetName()},
			setup: func(t *testing.T) {
				setupWebAuthnChallengeSolver(t, device, origin, true)
			},
			target: "env=stage",
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\ntest\n", i, i2...)
			},
			mfaPromptCount: 2,
			errAssertion:   require.NoError,
		},
		{
			name:   "role permits access without mfa",
			target: sshHostID,
			roles:  []string{sshLoginRole.GetName()},
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			errAssertion: require.NoError,
		},
		{
			name:            "role prevents access",
			target:          sshHostID,
			roles:           []string{noAccessRole.GetName()},
			stdoutAssertion: require.Empty,
			errAssertion:    require.Error,
		},
		{
			name:   "command runs on a hostname with mfa set via role",
			target: sshHostID,
			roles:  []string{perSessionMFARole.GetName()},
			setup: func(t *testing.T) {
				setupWebAuthnChallengeSolver(t, device, origin, true)
			},
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
			target: sshHostID,
			roles:  []string{perSessionMFARole.GetName()},
			setup: func(t *testing.T) {
				setupWebAuthnChallengeSolver(t, device, origin, false)
			},
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
			target: sshHostID,
			roles:  []string{perSessionMFARole.GetName()},
			setup: func(t *testing.T) {
				setupWebAuthnChallengeSolver(t, device, origin, false)
			},
			stdoutAssertion: func(t require.TestingT, i interface{}, i2 ...interface{}) {
				require.Equal(t, "test\n", i, i2...)
			},
			errAssertion: require.NoError,
			headless:     true,
		},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if tt.authPreference != nil {
				require.NoError(t, rootAuth.GetAuthServer().SetAuthPreference(ctx, tt.authPreference))
				t.Cleanup(func() {
					require.NoError(t, rootAuth.GetAuthServer().SetAuthPreference(ctx, webauthnPreference))
				})
			}

			if tt.setup != nil {
				tt.setup(t)
			}

			if tt.roles != nil {
				roles := alice.GetRoles()
				t.Cleanup(func() {
					alice.SetRoles(roles)
					require.NoError(t, rootAuth.GetAuthServer().UpsertUser(alice))
				})
				alice.SetRoles(tt.roles)
				require.NoError(t, rootAuth.GetAuthServer().UpsertUser(alice))
			}

			var loginOpt CliOption
			if tt.headless {
				loginOpt = setMockHeadlessLogin(t, rootAuth.GetAuthServer(), alice.GetName(), proxyAddr.String())
			} else {
				_, _, loginOpt = mustLoginHome(t, rootAuth.GetAuthServer(), proxyAddr.String(), alice.GetName(), connector.GetName())
			}

			stdout := &output{buf: bytes.Buffer{}}
			// Clear counter before each ssh command,
			// so we can assert how many times sign was called.
			device.SetCounter(0)

			err = Run(ctx, []string{
				"ssh",
				"--insecure",
				tt.target,
				"echo", "test",
			}, loginOpt,
				func(conf *CLIConf) error {
					conf.overrideStdin = &bytes.Buffer{}
					conf.OverrideStdout = stdout
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

	_, _, loginOpt := mustLoginHome(t, rootAuth.GetAuthServer(), proxyAddr.String(), alice.GetName(), connector.GetName())

	// won't request if can't list node
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--request-reason", "reason here to bypass prompt",
		fmt.Sprintf("%s@%s", user.Username, sshHostnameNoAccess),
		"echo", "test",
	}, loginOpt)
	require.Error(t, err)

	// won't request if can't login with username
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--request-reason", "reason here to bypass prompt",
		fmt.Sprintf("%s@%s", "not-a-username", sshHostname),
		"echo", "test",
	}, loginOpt)
	require.Error(t, err)

	// won't request to non-existent node
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		fmt.Sprintf("%s@unknown", user.Username),
		"echo", "test",
	}, loginOpt)
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
	}, loginOpt)
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
		}, loginOpt)
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
	}, loginOpt)
	require.NoError(t, err)

	// log out and back in with no access request
	err = Run(ctx, []string{
		"logout",
	}, loginOpt)
	require.NoError(t, err)
	err = Run(ctx, []string{
		"login",
		"--insecure",
		"--proxy", proxyAddr.String(),
	}, loginOpt, setMockSSOLogin(t, rootAuth.GetAuthServer(), alice.GetName(), connector.GetName()))
	require.NoError(t, err)

	// ssh with request, by host ID
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--request-reason", "reason here to bypass prompt",
		fmt.Sprintf("%s@%s", user.Username, sshHostID),
		"echo", "test",
	}, loginOpt)
	require.NoError(t, err)

	// fail to ssh to other non-approved node, do not prompt for request
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		fmt.Sprintf("%s@%s", user.Username, sshHostname2),
		"echo", "test",
	}, loginOpt)
	require.Error(t, err)

	// drop the current access request
	err = Run(ctx, []string{
		"--insecure",
		"request",
		"drop",
	}, loginOpt)
	require.NoError(t, err)

	// fail to ssh to other node with no active request
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--disable-access-request",
		fmt.Sprintf("%s@%s", user.Username, sshHostname2),
		"echo", "test",
	}, loginOpt)
	require.Error(t, err)

	// successfully ssh to other node, with new request
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--request-reason", "reason here to bypass prompt",
		fmt.Sprintf("%s@%s", user.Username, sshHostname2),
		"echo", "test",
	}, loginOpt)
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
		"--proxy", rootProxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(t, rootAuthServer, alice.GetName(), connector.GetName()))
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

func TestSSHHeadless(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	user, err := user.Current()
	require.NoError(t, err)

	// Headless ssh should pass session mfa requirements
	sshLoginRole, err := types.NewRole("ssh-login", types.RoleSpecV6{
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
	alice.SetRoles([]string{"ssh-login"})

	rootAuth, rootProxy := makeTestServers(t, withBootstrap(sshLoginRole, alice))

	authAddr, err := rootAuth.AuthAddr()
	require.NoError(t, err)

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

	sshHostname := "test-ssh-server"
	node := makeTestSSHNode(t, authAddr, withHostname(sshHostname), withSSHLabel("access", "true"))
	sshHostID := node.Config.HostUUID

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
	require.Eventually(t, hasNodes(sshHostID), 10*time.Second, 100*time.Millisecond, "nodes never showed up")

	// perform "tsh --headless ssh"
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--headless",
		"--proxy", proxyAddr.String(),
		"--user", "alice",
		fmt.Sprintf("%s@%s", user.Username, sshHostname),
		"echo", "test",
	}, CliOption(func(cf *CLIConf) error {
		cf.MockHeadlessLogin = mockHeadlessLogin(t, rootAuth.GetAuthServer(), alice.GetName())
		return nil
	}))
	require.NoError(t, err)

	// "tsh --auth headless ssh" should also perform headless ssh
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--auth", constants.HeadlessConnector,
		"--proxy", proxyAddr.String(),
		"--user", "alice",
		fmt.Sprintf("%s@%s", user.Username, sshHostname),
		"echo", "test",
	}, CliOption(func(cf *CLIConf) error {
		cf.MockHeadlessLogin = mockHeadlessLogin(t, rootAuth.GetAuthServer(), alice.GetName())
		return nil
	}))
	require.NoError(t, err)

	// headless ssh should fail if user is not set.
	err = Run(ctx, []string{
		"ssh",
		"--insecure",
		"--headless",
		"--proxy", proxyAddr.String(),
		fmt.Sprintf("%s@%s", user.Username, sshHostname),
		"echo", "test",
	}, CliOption(func(cf *CLIConf) error {
		cf.MockHeadlessLogin = mockHeadlessLogin(t, rootAuth.GetAuthServer(), alice.GetName())
		return nil
	}))
	require.Error(t, err)
	require.ErrorIs(t, err, trace.BadParameter("user must be set explicitly for headless login with the --user flag or $TELEPORT_USER env variable"))
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
			setEnvFlags(&tc.inCLIConf, func(envName string) string {
				return tc.envMap[envName]
			})
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

	connector := mockConnector(t)
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	alice.SetRoles([]string{"access"})
	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, alice))
	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)
	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	tshHome, _, _ := mustLoginHome(t, authServer, proxyAddr.String(), alice.GetName(), connector.GetName())

	profile, err := profile.FromDir(tshHome, "")
	require.NoError(t, err)

	mustCreateAuthClientFormUserProfile(t, tshHome, proxyAddr.String())

	// Simulate legacy tsh client behavior where all clusters certs were stored in the certs.pem file.
	require.NoError(t, os.RemoveAll(profile.TLSClusterCASDir()))

	// Verify that authClient created from profile will create a valid client in case where cas dir doesn't exit.
	mustCreateAuthClientFormUserProfile(t, tshHome, proxyAddr.String())
}

func mustCreateAuthClientFormUserProfile(t *testing.T, tshHomePath, addr string) {
	ctx := context.Background()
	credentials := apiclient.LoadProfile(tshHomePath, "")
	c, err := apiclient.New(context.Background(), apiclient.Config{
		Addrs:                    []string{addr},
		Credentials:              []apiclient.Credentials{credentials},
		InsecureAddressDiscovery: true,
	})
	require.NoError(t, err)
	_, err = c.Ping(ctx)
	require.NoError(t, err)
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
      }
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
			expected := fmt.Sprintf(expectedFmt, tt.dbUsersData)
			testSerialization(t, expected, func(f string) (string, error) {
				return serializeDatabases([]types.Database{db}, f, tt.roles)
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
			gotUsers := getDBUsers(tt.database, tt.roles)
			require.Equal(t, tt.wantUsers, gotUsers)

			gotText := formatUsersForDB(tt.database, tt.roles)
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
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
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

			// --trace should have no impact on login, since login is whitelisted
			err = Run(context.Background(), []string{
				"login",
				"--insecure",
				"--proxy", proxyAddr.String(),
				"--trace",
			}, setHomePath(tmpHomePath), setMockSSOLogin(t, authServer, alice.GetName(), connector.GetName()))
			require.NoError(t, err)

			if traceCfg.Enabled {
				collector.WaitForExport()
			}

			// ensure login doesn't generate any spans from tsh if spans are being sampled
			loginAssertion := spanAssertion(false, !traceCfg.Enabled)
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
	t.Parallel()

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
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
			}, setHomePath(tmpHomePath), setMockSSOLogin(t, authServer, alice.GetName(), connector.GetName()))
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
