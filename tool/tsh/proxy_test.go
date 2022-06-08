/*
Copyright 2021 Gravitational, Inc.

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

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gravitational/teleport"
	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/agent"

	apidefaults "github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/utils"
)

// TestTSHSSH verifies "tsh proxy ssh" command.
func TestTSHSSH(t *testing.T) {
	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *service.Config) {
			cfg.Version = defaults.TeleportConfigVersionV2
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
		withLeafCluster(),
		withLeafConfigFunc(func(cfg *service.Config) {
			cfg.Version = defaults.TeleportConfigVersionV2
			cfg.Proxy.SSHAddr.Addr = localListenerAddr()
		}),
	)

	tests := []struct {
		name string
		fn   func(t *testing.T, s *suite)
	}{
		{"ssh root cluster access", testRootClusterSSHAccess},
		{"ssh leaf cluster access", testLeafClusterSSHAccess},
		{"ssh jump host access", testJumpHostSSHAccess},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(types.HomeEnvVar, t.TempDir())

			tc.fn(t, s)
		})
	}
}

func testRootClusterSSHAccess(t *testing.T, s *suite) {
	err := Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", s.connector.GetName(),
		"--proxy", s.root.Config.Proxy.WebAddr.String(),
	}, func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, s.root.GetAuthServer(), s.user)
		return nil
	})
	require.NoError(t, err)
	err = Run(context.Background(), []string{
		"ssh",
		s.root.Config.Hostname,
		"echo", "hello",
	})
	require.NoError(t, err)

	identityFile := path.Join(t.TempDir(), "identity.pem")
	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", s.connector.GetName(),
		"--proxy", s.root.Config.Proxy.WebAddr.String(),
		"--out", identityFile,
	}, func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, s.root.GetAuthServer(), s.user)
		return nil
	})
	require.NoError(t, err)

	err = Run(context.Background(), []string{
		"--proxy", s.root.Config.Proxy.WebAddr.String(),
		"--insecure",
		"-i", identityFile,
		"ssh",
		"localhost",
		"echo", "hello",
	})
	require.NoError(t, err)
}

func testLeafClusterSSHAccess(t *testing.T, s *suite) {
	err := Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", s.connector.GetName(),
		"--proxy", s.root.Config.Proxy.WebAddr.String(),
		s.leaf.Config.Auth.ClusterName.GetClusterName(),
	}, func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, s.root.GetAuthServer(), s.user)
		return nil
	})
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		err = Run(context.Background(), []string{
			"ssh",
			s.leaf.Config.Hostname,
			"echo", "hello",
		})
		return err == nil
	}, 5*time.Second, time.Second)

	identityFile := path.Join(t.TempDir(), "identity.pem")
	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", s.connector.GetName(),
		"--proxy", s.root.Config.Proxy.WebAddr.String(),
		"--out", identityFile,
	}, func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, s.root.GetAuthServer(), s.user)
		return nil
	})
	require.NoError(t, err)

	err = Run(context.Background(), []string{
		"--proxy", s.root.Config.Proxy.WebAddr.String(),
		"--insecure",
		"-i", identityFile,
		"ssh",
		"--cluster", s.leaf.Config.Auth.ClusterName.GetClusterName(),
		s.leaf.Config.Hostname,
		"echo", "hello",
	})
	require.NoError(t, err)
}

func testJumpHostSSHAccess(t *testing.T, s *suite) {
	err := Run(context.Background(), []string{
		"login",
		"--insecure",
		"--auth", s.connector.GetName(),
		"--proxy", s.root.Config.Proxy.WebAddr.String(),
		s.root.Config.Auth.ClusterName.GetClusterName(),
	}, func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, s.root.GetAuthServer(), s.user)
		return nil
	})
	require.NoError(t, err)

	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		s.leaf.Config.Auth.ClusterName.GetClusterName(),
	}, func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, s.root.GetAuthServer(), s.user)
		return nil
	})
	require.NoError(t, err)

	// Connect to leaf node though jump host set to leaf proxy SSH port.
	err = Run(context.Background(), []string{
		"ssh",
		"--insecure",
		"-J", s.leaf.Config.Proxy.SSHAddr.Addr,
		s.leaf.Config.Hostname,
		"echo", "hello",
	}, func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, s.root.GetAuthServer(), s.user)
		return nil
	})
	require.NoError(t, err)

	// Connect to leaf node though jump host set to proxy web port where TLS Routing is enabled.
	err = Run(context.Background(), []string{
		"ssh",
		"--insecure",
		"-J", s.leaf.Config.Proxy.WebAddr.Addr,
		s.leaf.Config.Hostname,
		"echo", "hello",
	}, func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, s.root.GetAuthServer(), s.user)
		return nil
	})
	require.NoError(t, err)
}

// TestProxySSHDial verifies "tsh proxy ssh" command.
func TestProxySSHDial(t *testing.T) {
	createAgent(t)

	tmpHomePath := t.TempDir()

	connector := mockConnector(t)
	sshLoginRole, err := types.NewRoleV3("ssh-login", types.RoleSpecV5{
		Allow: types.RoleConditions{
			Logins: []string{"alice"},
		},
	})

	require.NoError(t, err)
	alice, err := types.NewUser("alice")
	require.NoError(t, err)
	alice.SetRoles([]string{"access", "ssh-login"})

	authProcess, proxyProcess := makeTestServers(t,
		withBootstrap(connector, alice, sshLoginRole),
		withAuthConfig(func(cfg *service.AuthConfig) {
			cfg.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
	)

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
		cf.mockSSOLogin = mockSSOLogin(t, authServer, alice)
		return nil
	})
	require.NoError(t, err)

	unreachableSubsystem := "alice@unknownhost:22"

	// Check if the tsh proxy ssh command can establish a connection to the Teleport proxy.
	// After connection is established the unknown submodule is requested and the call is expected to fail with the
	// "subsystem request failed" error.
	// For real case scenario the 'tsh proxy ssh' and openssh binary use stdin,stdout,stderr pipes
	// as communication channels but in unit test there is no easy way to mock this behavior.
	err = Run(context.Background(), []string{
		"proxy", "ssh", unreachableSubsystem,
	}, setHomePath(tmpHomePath))
	require.Contains(t, err.Error(), "subsystem request failed")
}

// TestProxySSHDialWithIdentityFile retries
func TestProxySSHDialWithIdentityFile(t *testing.T) {
	createAgent(t)

	tmpHomePath := t.TempDir()

	connector := mockConnector(t)
	sshLoginRole, err := types.NewRoleV3("ssh-login", types.RoleSpecV5{
		Allow: types.RoleConditions{
			Logins: []string{"alice"},
		},
	})

	require.NoError(t, err)
	alice, err := types.NewUser("alice")
	require.NoError(t, err)
	alice.SetRoles([]string{"access", "ssh-login"})

	authProcess, proxyProcess := makeTestServers(t,
		withBootstrap(connector, alice, sshLoginRole),
		withAuthConfig(func(cfg *service.AuthConfig) {
			cfg.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
	)

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := proxyProcess.ProxyWebAddr()
	require.NoError(t, err)

	identityFile := path.Join(t.TempDir(), "identity.pem")
	err = Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", connector.GetName(),
		"--proxy", proxyAddr.String(),
		"--out", identityFile,
	}, setHomePath(tmpHomePath), func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, authServer, alice)
		return nil
	})
	require.NoError(t, err)

	unreachableSubsystem := "alice@unknownhost:22"
	err = Run(context.Background(), []string{
		"-i", identityFile,
		"--insecure",
		"proxy",
		"ssh",
		"--proxy", proxyAddr.String(),
		"--cluster", authProcess.Config.Auth.ClusterName.GetClusterName(),
		unreachableSubsystem,
	}, setHomePath(tmpHomePath))
	require.Contains(t, err.Error(), "subsystem request failed")
}

// TestTSHProxyTemplate verifies connecting with OpenSSH client through the
// local proxy started with "tsh proxy ssh -J" using proxy templates.
func TestTSHProxyTemplate(t *testing.T) {
	_, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("Skipping test, no ssh binary found.")
	}

	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	tshHome := t.TempDir()
	t.Setenv(types.HomeEnvVar, tshHome)

	tshPath, err := os.Executable()
	require.NoError(t, err)

	s := newTestSuite(t)
	mustLogin(t, s)

	// Create proxy template configuration.
	tshConfigFile := filepath.Join(tshHome, tshConfigPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(tshConfigFile), 0777))
	require.NoError(t, os.WriteFile(tshConfigFile, []byte(fmt.Sprintf(`
proxy_templates:
- template: '^(\w+)\.(root):(.+)$'
  proxy: "%v"
  host: "$1:$3"
`, s.root.Config.Proxy.WebAddr.Addr)), 0644))

	// Create SSH config.
	sshConfigFile := filepath.Join(tshHome, "sshconfig")
	os.WriteFile(sshConfigFile, []byte(fmt.Sprintf(`
Host *
  HostName %%h
  StrictHostKeyChecking no
  ProxyCommand %v -d --insecure proxy ssh -J {{proxy}} %%r@%%h:%%p
`, tshPath)), 0644)

	// Connect to "localnode" with OpenSSH.
	mustRunOpenSSHCommand(t, sshConfigFile, "localnode.root",
		s.root.Config.SSH.Addr.Port(defaults.SSHServerListenPort), "echo", "hello")
}

// TestTSHConfigConnectWithOpenSSHClient tests OpenSSH configuration generated by tsh config command and
// connects to ssh node using native OpenSSH client with different session recording  modes and proxy listener modes.
func TestTSHConfigConnectWithOpenSSHClient(t *testing.T) {
	// Only run this test if we have access to the external SSH binary.
	_, err := exec.LookPath("ssh")
	if err != nil {
		t.Skip("Skipping no external SSH binary found.")
	}

	lib.SetInsecureDevMode(true)
	defer lib.SetInsecureDevMode(false)

	tests := []struct {
		name string
		opts []testSuiteOptionFunc
	}{
		{
			name: "node recording mode with TLS routing enabled",
			opts: []testSuiteOptionFunc{
				withRootConfigFunc(func(cfg *service.Config) {
					cfg.Auth.SessionRecordingConfig.SetMode(types.RecordAtNode)
					cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
				}),
			},
		},
		{
			name: "proxy recording mode with TLS routing enabled",
			opts: []testSuiteOptionFunc{
				withRootConfigFunc(func(cfg *service.Config) {
					cfg.Auth.SessionRecordingConfig.SetMode(types.RecordAtProxy)
					cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
				}),
			},
		},
		{
			name: "node recording mode with TLS routing disabled",
			opts: []testSuiteOptionFunc{
				withRootConfigFunc(func(cfg *service.Config) {
					cfg.Auth.SessionRecordingConfig.SetMode(types.RecordAtNode)
					cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Separate)
				}),
			},
		},
		{
			name: "proxy recording mode with TLS routing disabled",
			opts: []testSuiteOptionFunc{
				withRootConfigFunc(func(cfg *service.Config) {
					cfg.Auth.SessionRecordingConfig.SetMode(types.RecordAtProxy)
					cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Separate)
				}),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(types.HomeEnvVar, t.TempDir())

			s := newTestSuite(t, tc.opts...)
			// Login to the Teleport proxy.
			mustLogin(t, s)

			// Get SSH config file generated by the 'tsh config' command.
			sshConfigFile := mustGetOpenSSHConfigFile(t)

			sshConn := fmt.Sprintf("%s.%s", s.root.Config.Hostname, s.root.Config.Auth.ClusterName.GetClusterName())
			nodePort := s.root.Config.SSH.Addr.Port(defaults.SSHServerListenPort)
			bashCmd := []string{"echo", "hello"}

			// Run ssh command using the OpenSSH client.
			mustRunOpenSSHCommand(t, sshConfigFile, sshConn, nodePort, bashCmd...)

			// Try to run ssh command using the OpenSSH client with invalid node login username.
			// Command should fail because nodeLogin 'invalidUser' is not in valid principals.
			sshConn = fmt.Sprintf("invalidUser@%s.%s",
				s.root.Config.Hostname, s.root.Config.Auth.ClusterName.GetClusterName())
			mustFailToRunOpenSSHCommand(t, sshConfigFile, sshConn, nodePort, bashCmd...)

			events := mustSearchEvents(t, s.root.GetAuthServer())
			// Check if failed login attempt event has proper nodeLogin.
			mustFindFailedNodeLoginAttempt(t, events, "invalidUser")
		})
	}
}

func createAgent(t *testing.T) string {
	t.Helper()

	user, err := user.Current()
	require.NoError(t, err)

	sockDir := "test"
	sockName := "agent.sock"

	keyring := agent.NewKeyring()
	teleAgent := teleagent.NewServer(func() (teleagent.Agent, error) {
		return teleagent.NopCloser(keyring), nil
	})

	// Start the SSH agent.
	err = teleAgent.ListenUnixSocket(sockDir, sockName, user)
	require.NoError(t, err)
	go teleAgent.Serve()
	t.Cleanup(func() {
		teleAgent.Close()
	})

	t.Setenv(teleport.SSHAuthSock, teleAgent.Path)

	return teleAgent.Path
}

func mustLogin(t *testing.T, s *suite) {
	err := Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--auth", s.connector.GetName(),
		"--proxy", s.root.Config.Proxy.WebAddr.String(),
	}, func(cf *CLIConf) error {
		cf.mockSSOLogin = mockSSOLogin(t, s.root.GetAuthServer(), s.user)
		return nil
	})
	require.NoError(t, err)
}

func mustGetOpenSSHConfigFile(t *testing.T) string {
	var buff bytes.Buffer
	err := Run(context.Background(), []string{
		"config",
	}, func(cf *CLIConf) error {
		cf.overrideStdout = &buff
		return nil
	})
	require.NoError(t, err)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ssh_config")
	err = os.WriteFile(configPath, buff.Bytes(), 0600)
	require.NoError(t, err)

	return configPath
}

func runOpenSSHCommand(t *testing.T, configFile string, sshConnString string, port int, args ...string) error {
	ss := []string{
		"-F", configFile,
		"-y",
		"-p", strconv.Itoa(port),
		"-o", "StrictHostKeyChecking=no",
		sshConnString,
	}

	ss = append(ss, args...)

	sshPath, err := exec.LookPath("ssh")
	require.NoError(t, err)

	cmd := exec.Command(sshPath, ss...)
	cmd.Env = []string{
		fmt.Sprintf("%s=1", tshBinMainTestEnv),
		fmt.Sprintf("SSH_AUTH_SOCK=%s", createAgent(t)),
		fmt.Sprintf("PATH=%s", filepath.Dir(sshPath)),
		fmt.Sprintf("%s=%s", types.HomeEnvVar, os.Getenv(types.HomeEnvVar)),
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	return trace.Wrap(cmd.Run())
}

func mustRunOpenSSHCommand(t *testing.T, configFile string, sshConnString string, port int, args ...string) {
	err := utils.RetryStaticFor(time.Second*10, time.Millisecond*500, func() error {
		err := runOpenSSHCommand(t, configFile, sshConnString, port, args...)
		return trace.Wrap(err)
	})
	require.NoError(t, err)
}

func mustFailToRunOpenSSHCommand(t *testing.T, configFile string, sshConnString string, port int, args ...string) {
	err := runOpenSSHCommand(t, configFile, sshConnString, port, args...)
	require.Error(t, err)
}

func mustSearchEvents(t *testing.T, auth *auth.Server) []apievents.AuditEvent {
	now := time.Now()
	events, _, err := auth.SearchEvents(
		now.Add(-time.Hour),
		now.Add(time.Hour),
		apidefaults.Namespace,
		nil,
		0,
		types.EventOrderDescending,
		"")

	require.NoError(t, err)
	return events
}

func mustFindFailedNodeLoginAttempt(t *testing.T, av []apievents.AuditEvent, nodeLogin string) {
	for _, e := range av {
		if e.GetCode() == events.AuthAttemptFailureCode {
			require.Equal(t, e.(*apievents.AuthAttempt).Login, nodeLogin)
			return
		}
	}
	t.Error("failed to find AuthAttemptFailureCode event")
}
