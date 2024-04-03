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
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"testing"
	"text/template"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/teleagent"
	"github.com/gravitational/teleport/lib/tlsca"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

// TestSSH verifies "tsh ssh" command.
func TestSSH(t *testing.T) {
	if webpkiCACert == nil {
		t.Skip("the current platform doesn't support adding CAs to the system pool")
	}

	t.Setenv("_TELEPORT_TEST_NO_PARALLEL", "1")
	defer lib.SetInsecureDevMode(lib.IsInsecureDevMode())
	lib.SetInsecureDevMode(false)

	d := t.TempDir()
	webCertPath := path.Join(d, "cert.pem")
	webKeyPath := path.Join(d, "key.pem")
	generateWebPKICert(t, webCertPath, webKeyPath)

	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Version = defaults.TeleportConfigVersionV2
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Proxy.KeyPairs = []servicecfg.KeyPairPath{{
				PrivateKey:  webKeyPath,
				Certificate: webCertPath,
			}}
		}),
		withLeafCluster(),
		withLeafConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Version = defaults.TeleportConfigVersionV2
		}),
	)

	// just in case someone changes newTestSuite
	require.False(t, lib.IsInsecureDevMode())

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
			tc.fn(t, s)
		})
	}
}

func testRootClusterSSHAccess(t *testing.T, s *suite) {
	tshHome, _ := mustLogin(t, s)
	err := Run(context.Background(), []string{
		"ssh",
		s.root.Config.Hostname,
		"echo", "hello",
	}, setHomePath(tshHome))
	require.NoError(t, err)

	identityFile := mustLoginIdentity(t, s)
	err = Run(context.Background(), []string{
		"--proxy", s.root.Config.Proxy.WebAddr.String(),
		"-i", identityFile,
		"ssh",
		"localhost",
		"echo", "hello",
	}, setIdentity(identityFile))
	require.NoError(t, err)
}

func testLeafClusterSSHAccess(t *testing.T, s *suite) {
	tshHome, _ := mustLogin(t, s, s.leaf.Config.Auth.ClusterName.GetClusterName())
	require.Eventually(t, func() bool {
		err := Run(context.Background(), []string{
			"ssh",
			"--proxy", s.root.Config.Proxy.WebAddr.String(),
			s.leaf.Config.Hostname,
			"echo", "hello",
		}, setHomePath(tshHome))
		t.Logf("ssh to leaf failed %v", err)
		return err == nil
	}, 5*time.Second, time.Second)

	identityFile := mustLoginIdentity(t, s)
	err := Run(context.Background(), []string{
		"--proxy", s.root.Config.Proxy.WebAddr.String(),
		"-i", identityFile,
		"ssh",
		"--cluster", s.leaf.Config.Auth.ClusterName.GetClusterName(),
		s.leaf.Config.Hostname,
		"echo", "hello",
	}, setIdentity(identityFile))
	require.NoError(t, err)
}

func testJumpHostSSHAccess(t *testing.T, s *suite) {
	// login to root
	tshHome, _ := mustLogin(t, s, s.root.Config.Auth.ClusterName.GetClusterName())

	// Switch to leaf cluster
	err := Run(context.Background(), []string{
		"login",
		s.leaf.Config.Auth.ClusterName.GetClusterName(),
	}, s.setMockSSOLogin(t), setHomePath(tshHome))
	require.NoError(t, err)

	// Connect to leaf node though jump host set to leaf proxy SSH port.
	err = Run(context.Background(), []string{
		"ssh",
		"-J", s.leaf.Config.Proxy.SSHAddr.Addr,
		s.leaf.Config.Hostname,
		"echo", "hello",
	}, s.setMockSSOLogin(t), setHomePath(tshHome))
	require.NoError(t, err)

	t.Run("root cluster online", func(t *testing.T) {
		// Connect to leaf node though jump host set to proxy web port where TLS Routing is enabled.
		err = Run(context.Background(), []string{
			"ssh",
			"-J", s.leaf.Config.Proxy.WebAddr.Addr,
			s.leaf.Config.Hostname,
			"echo", "hello",
		}, s.setMockSSOLogin(t), setHomePath(tshHome))
		require.NoError(t, err)
	})

	t.Run("root cluster offline", func(t *testing.T) {
		// Terminate root cluster.
		err = s.root.Close()
		require.NoError(t, err)

		// Check JumpHost flow when root cluster is offline.
		err = Run(context.Background(), []string{
			"ssh",
			"-J", s.leaf.Config.Proxy.WebAddr.Addr,
			s.leaf.Config.Hostname,
			"echo", "hello",
		}, s.setMockSSOLogin(t), setHomePath(tshHome))
		require.NoError(t, err)
	})
}

func generateWebPKICert(t *testing.T, certPath, keyPath string) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	require.NotNil(t, keyPEM)

	ca := &tlsca.CertAuthority{
		Cert:   webpkiCACert,
		Signer: webpkiCAKey,
	}
	certPEM, err := ca.GenerateCertificate(tlsca.CertificateRequest{
		PublicKey: key.Public(),
		Subject:   pkix.Name{CommonName: "proxy web addr cert"},
		NotAfter:  time.Now().Add(12 * time.Hour),
		DNSNames:  []string{"127.0.0.1"},
		KeyUsage:  x509.KeyUsageDigitalSignature,
	})
	require.NoError(t, err)

	require.NoError(t, os.WriteFile(certPath, certPEM, 0o644))
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0o644))
}

// TestWithRsync tests that Teleport works with rsync.
func TestWithRsync(t *testing.T) {
	disableAgent(t)

	_, err := exec.LookPath("rsync")
	require.NoError(t, err)

	s := newTestSuite(t)

	// login and get host info
	tshHome, _ := mustLogin(t, s)

	testBin, err := os.Executable()
	require.NoError(t, err)

	host, port, err := net.SplitHostPort(s.root.Config.SSH.Addr.String())
	require.NoError(t, err)
	proxyAddr, err := s.root.ProxyWebAddr()
	require.NoError(t, err)

	var mockHeadlessAddr string

	tests := []struct {
		name      string
		setup     func(t *testing.T, dir string)
		createCmd func(ctx context.Context, dir, src, dst string) *exec.Cmd
	}{
		{
			name: "with OpenSSH ssh",
			setup: func(t *testing.T, dir string) {
				// create ssh client config rsync will use
				var buf bytes.Buffer
				err = Run(context.Background(), []string{"config"}, setHomePath(tshHome), setOverrideStdout(&buf))
				require.NoError(t, err)

				configPath := filepath.Join(dir, "ssh_config")
				err = os.WriteFile(configPath, buf.Bytes(), 0o600)
				require.NoError(t, err)
			},
			createCmd: func(ctx context.Context, dir, src, dst string) *exec.Cmd {
				return exec.CommandContext(
					ctx,
					"rsync",
					// ensure ssh will use the client config that was generated
					"-e",
					fmt.Sprintf("ssh -F %s -p %s", filepath.Join(dir, "ssh_config"), port),
					src,
					fmt.Sprintf("%s:%s", host, dst),
				)
			},
		},
		{
			name: "with headless tsh",
			setup: func(t *testing.T, dir string) {
				// setup webauthn for headless auth
				asrv := s.root.GetAuthServer()
				ctx := context.Background()

				_, err = asrv.UpsertAuthPreference(ctx, &types.AuthPreferenceV2{
					Spec: types.AuthPreferenceSpecV2{
						Type:         constants.Local,
						SecondFactor: constants.SecondFactorOptional,
						Webauthn: &types.Webauthn{
							RPID: "root",
						},
					},
				})
				require.NoError(t, err)

				require.EventuallyWithT(t, func(t *assert.CollectT) {
					pref, err := asrv.GetAuthPreference(ctx)
					if !assert.NoError(t, err) {
						return
					}
					w, err := pref.GetWebauthn()
					assert.NoError(t, err)
					assert.NotNil(t, w)
				}, 5*time.Second, 100*time.Millisecond)

				token, err := asrv.CreateResetPasswordToken(ctx, auth.CreateUserTokenRequest{
					Name: s.user.GetName(),
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

				device, err := mocku2f.Create()
				require.NoError(t, err)
				device.SetPasswordless()

				ccr, err := device.SignCredentialCreation("https://root", cc)
				require.NoError(t, err)
				_, err = asrv.ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
					TokenID:     tokenID,
					NewPassword: []byte(mockHeadlessPassword),
					NewMFARegisterResponse: &proto.MFARegisterResponse{
						Response: &proto.MFARegisterResponse_Webauthn{
							Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
						},
					},
				})
				require.NoError(t, err)

				// start a listener to use as a way to implement mock
				// headless auth on the child 'tsh ssh --headless' process
				lis, err := net.Listen("tcp", "127.0.0.1:")
				require.NoError(t, err)
				t.Cleanup(func() {
					lis.Close()
				})
				mockHeadlessAddr = lis.Addr().String()

				go func() {
					conn, err := lis.Accept()
					if !assert.NoError(t, err) {
						return
					}
					defer conn.Close()

					// the child will send the public key
					key := make([]byte, 512)
					_, err = conn.Read(key)
					if !assert.NoError(t, err) {
						return
					}

					// generate certificates for our user
					clusterName, err := asrv.GetClusterName()
					if !assert.NoError(t, err) {
						return
					}
					sshCert, tlsCert, err := asrv.GenerateUserTestCerts(auth.GenerateUserTestCertsRequest{
						Key:            key,
						Username:       s.user.GetName(),
						TTL:            time.Hour,
						Compatibility:  constants.CertificateFormatStandard,
						RouteToCluster: clusterName.GetClusterName(),
						MFAVerified:    "mfa-verified",
					})
					if !assert.NoError(t, err) {
						return
					}

					// load CA cert
					authority, err := asrv.GetCertAuthority(
						ctx, types.CertAuthID{
							Type:       types.HostCA,
							DomainName: clusterName.GetClusterName(),
						}, false)
					if !assert.NoError(t, err) {
						return
					}

					// send login response to the client
					resp := auth.SSHLoginResponse{
						Username:    s.user.GetName(),
						Cert:        sshCert,
						TLSCert:     tlsCert,
						HostSigners: auth.AuthoritiesToTrustedCerts([]types.CertAuthority{authority}),
					}
					encResp, err := json.Marshal(resp)
					if !assert.NoError(t, err) {
						return
					}
					_, err = conn.Write(encResp)
					assert.NoError(t, err)

					assert.NoError(t, conn.Close())
				}()
			},
			createCmd: func(ctx context.Context, dir, src, dst string) *exec.Cmd {
				cmd := exec.CommandContext(
					ctx,
					"rsync",
					// ensure headless tsh will be used to authenticate
					"-e",
					fmt.Sprintf("%s ssh -d --insecure --headless --proxy=%s --user=%s", testBin, proxyAddr, s.user.GetName()),
					src,
					fmt.Sprintf("%s:%s", host, dst),
				)
				// make the re-exec behave as `tsh` instead of test binary.
				cmd.Env = []string{
					fmt.Sprintf("%s=%s", tshBinMockHeadlessAddrEnv, mockHeadlessAddr),
					tshBinMainTestEnv + "=1",
					tshBinMainTestOneshotEnv + "=1",
					fmt.Sprintf("%s=%s", types.HomeEnvVar, tshHome),
				}

				return cmd
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testDir := t.TempDir()
			tt.setup(t, testDir)

			// create a source file with random contents
			srcContents := make([]byte, 1024)
			_, err = rand.Read(srcContents)
			require.NoError(t, err)

			srcPath := filepath.Join(testDir, "src")
			err = os.WriteFile(srcPath, srcContents, 0o644)
			require.NoError(t, err)
			dstPath := filepath.Join(testDir, "dst")

			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			t.Cleanup(cancel)

			cmd := tt.createCmd(ctx, testDir, srcPath, dstPath)
			err := cmd.Run()
			var msg string
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				msg = fmt.Sprintf("exit code: %d", exitErr.ExitCode())
				msg += fmt.Sprintf("stderr: %s", exitErr.Stderr)
			}
			require.NoError(t, err, msg)

			// verify that dst exists and that its contents match src
			dstContents, err := os.ReadFile(dstPath)
			require.NoError(t, err)
			require.Equal(t, srcContents, dstContents)
		})
	}
}

// TestProxySSH verifies "tsh proxy ssh" functionality
func TestProxySSH(t *testing.T) {
	// ssh agent can cause race conditions in parallel tests.
	disableAgent(t)
	ctx := context.Background()

	tests := []struct {
		name string
		opts []testSuiteOptionFunc
	}{
		{
			name: "TLS routing enabled",
			opts: []testSuiteOptionFunc{
				withRootConfigFunc(func(cfg *servicecfg.Config) {
					cfg.Version = defaults.TeleportConfigVersionV2
					cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
				}),
			},
		},
		{
			name: "TLS routing disabled",
			opts: []testSuiteOptionFunc{
				withRootConfigFunc(func(cfg *servicecfg.Config) {
					cfg.Version = defaults.TeleportConfigVersionV2
					cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Separate)
				}),
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := newTestSuite(t, tc.opts...)

			proxyRequest := fmt.Sprintf("%s.%s:%d",
				s.root.Config.Proxy.SSHAddr.Host(),
				s.root.Config.Auth.ClusterName.GetClusterName(),
				s.root.Config.SSH.Addr.Port(defaults.SSHServerListenPort))

			runProxySSH := func(proxyRequest string, opts ...CliOption) error {
				var args []string
				if testing.Verbose() {
					args = append(args, "--debug")
				}
				return Run(ctx, append(args,
					"--insecure",
					"--proxy", s.root.Config.Proxy.WebAddr.Addr,
					"proxy", "ssh", proxyRequest,
				), opts...)
			}

			// login to Teleport
			homePath, kubeConfigPath := mustLogin(t, s)

			require.Eventually(t, func() bool {
				rnodes, _ := s.root.GetAuthServer().GetNodes(context.Background(), "default")
				return len(rnodes) != 0
			}, time.Second*30, time.Millisecond*100)

			t.Run("logged in", func(t *testing.T) {
				t.Parallel()
				err := runProxySSH(proxyRequest, setHomePath(homePath), setKubeConfigPath(kubeConfigPath))
				require.NoError(t, err)
			})

			t.Run("re-login", func(t *testing.T) {
				t.Parallel()
				err := runProxySSH(proxyRequest, setHomePath(t.TempDir()), setKubeConfigPath(filepath.Join(t.TempDir(), teleport.KubeConfigFile)), s.setMockSSOLogin(t))
				require.NoError(t, err)
			})

			t.Run("identity file", func(t *testing.T) {
				t.Parallel()
				err := runProxySSH(proxyRequest, setIdentity(mustLoginIdentity(t, s)))
				require.NoError(t, err)
			})

			t.Run("invalid node login", func(t *testing.T) {
				t.Parallel()

				// it's legal to specify any username before the request
				invalidLoginRequest := fmt.Sprintf("%s@%s", "invalidUser", proxyRequest)
				err := runProxySSH(invalidLoginRequest, setHomePath(homePath), setKubeConfigPath(kubeConfigPath), s.setMockSSOLogin(t))
				require.NoError(t, err)
			})
		})
	}
}

// TestProxySSHJumpHost verifies "tsh proxy ssh -J" functionality.
func TestProxySSHJumpHost(t *testing.T) {
	ctx := context.Background()

	lib.SetInsecureDevMode(true)
	t.Cleanup(func() { lib.SetInsecureDevMode(false) })

	createAgent(t)

	listenerModes := []types.ProxyListenerMode{
		types.ProxyListenerMode_Multiplex,
		types.ProxyListenerMode_Separate,
	}

	for _, rootListenerMode := range listenerModes {
		for _, leafListenerMode := range listenerModes {
			rootListenerMode := rootListenerMode
			leafListenerMode := leafListenerMode
			t.Run(fmt.Sprintf("Proxy listener mode %s for root and %s for leaf", rootListenerMode, leafListenerMode), func(t *testing.T) {
				t.Parallel()

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
					testserver.WithAuthConfig(
						func(cfg *servicecfg.AuthConfig) {
							cfg.NetworkingConfig.SetProxyListenerMode(rootListenerMode)
							// Disable session recording to prevent writing to disk after the test concludes.
							cfg.SessionRecordingConfig.SetMode(types.RecordOff)
							// Load all CAs on login so that leaf CA is trusted by clients.
							cfg.LoadAllCAs = true
						},
					),
				}
				rootServer := testserver.MakeTestServer(t, rootServerOpts...)

				leafServerOpts := []testserver.TestServerOptFunc{
					testserver.WithBootstrap(accessUser),
					testserver.WithHostname("node02"),
					testserver.WithClusterName(t, "leaf"),
					testserver.WithAuthConfig(
						func(cfg *servicecfg.AuthConfig) {
							cfg.NetworkingConfig.SetProxyListenerMode(leafListenerMode)
							// Disable session recording to prevent writing to disk after the test concludes.
							cfg.SessionRecordingConfig.SetMode(types.RecordOff)
						},
					),
				}
				leafServer := testserver.MakeTestServer(t, leafServerOpts...)
				testserver.SetupTrustedCluster(ctx, t, rootServer, leafServer)

				rootProxyAddr, err := rootServer.ProxyWebAddr()
				require.NoError(t, err)
				leafProxyAddr, err := leafServer.ProxyWebAddr()
				require.NoError(t, err)

				// login to root
				tshHome := t.TempDir()
				err = Run(ctx, []string{
					"--insecure",
					"login",
					"--proxy", rootProxyAddr.String(),
				}, setHomePath(tshHome), setMockSSOLogin(rootServer.GetAuthServer(), accessUser, connector.GetName()))
				require.NoError(t, err)

				// Connect through the leaf proxy jumphost.
				err = Run(ctx, []string{
					"--debug",
					"proxy",
					"ssh",
					"--no-resume",
					"-J", leafProxyAddr.String(),
					"node02",
				}, setHomePath(tshHome))
				require.NoError(t, err)

				// Terminate root cluster.
				err = rootServer.Close()
				require.NoError(t, err)

				// We should be able to connect without root online.
				err = Run(ctx, []string{
					"--debug",
					"--insecure",
					"proxy",
					"ssh",
					"--no-resume",
					"-J", leafProxyAddr.String(),
					"node02",
				}, setHomePath(tshHome))
				require.NoError(t, err)
			})
		}
	}
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

	tshPath, err := os.Executable()
	require.NoError(t, err)

	s := newTestSuite(t)
	tshHome, _ := mustLoginSetEnv(t, s)

	// Create proxy template configuration.
	tshConfigFile := filepath.Join(tshHome, tshConfigPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(tshConfigFile), 0o777))
	require.NoError(t, os.WriteFile(tshConfigFile, []byte(fmt.Sprintf(`
proxy_templates:
- template: '^(\w+)\.(root):(.+)$'
  proxy: "%v"
  host: "$1:$3"
`, s.root.Config.Proxy.WebAddr.Addr)), 0o644))

	// Create SSH config.
	sshConfigFile := filepath.Join(tshHome, "sshconfig")
	err = os.WriteFile(sshConfigFile, []byte(fmt.Sprintf(`
Host *
  HostName %%h
  StrictHostKeyChecking no
  ProxyCommand %v -d --insecure proxy ssh -J {{proxy}} %%r@%%h:%%p
`, tshPath)), 0o644)
	require.NoError(t, err)

	// Connect to "rootnode" with OpenSSH.
	mustRunOpenSSHCommand(t, sshConfigFile, "rootnode.root",
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
				withRootConfigFunc(func(cfg *servicecfg.Config) {
					cfg.Auth.SessionRecordingConfig.SetMode(types.RecordAtNodeSync)
					cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
				}),
			},
		},
		{
			name: "proxy recording mode with TLS routing enabled",
			opts: []testSuiteOptionFunc{
				withRootConfigFunc(func(cfg *servicecfg.Config) {
					cfg.Auth.SessionRecordingConfig.SetMode(types.RecordAtProxySync)
					cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
				}),
			},
		},
		{
			name: "node recording mode with TLS routing disabled",
			opts: []testSuiteOptionFunc{
				withRootConfigFunc(func(cfg *servicecfg.Config) {
					cfg.Auth.SessionRecordingConfig.SetMode(types.RecordAtNodeSync)
					cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Separate)
				}),
			},
		},
		{
			name: "proxy recording mode with TLS routing disabled",
			opts: []testSuiteOptionFunc{
				withRootConfigFunc(func(cfg *servicecfg.Config) {
					cfg.Auth.SessionRecordingConfig.SetMode(types.RecordAtProxySync)
					cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Separate)
				}),
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestSuite(t, tc.opts...)

			// Login to the Teleport proxy.
			mustLoginSetEnv(t, s)

			// Get SSH config file generated by the 'tsh config' command.
			sshConfigFile := mustGetOpenSSHConfigFile(t)

			sshConn := fmt.Sprintf("%s.%s", s.root.Config.Hostname, s.root.Config.Auth.ClusterName.GetClusterName())
			nodePort := s.root.Config.SSH.Addr.Port(defaults.SSHServerListenPort)
			bashCmd := []string{"echo", "hello"}

			// Run ssh command using the OpenSSH client.
			mustRunOpenSSHCommand(t, sshConfigFile, sshConn, nodePort, bashCmd...)

			// Try to run ssh command using the OpenSSH client with invalid node login username.
			// Command should fail because nodeLogin 'invalidUser' is not in valid principals.
			sshConn = fmt.Sprintf("invalidUser@%s", sshConn)
			mustFailToRunOpenSSHCommand(t, sshConfigFile, sshConn, nodePort, bashCmd...)

			// Check if failed login attempt event has proper nodeLogin.
			mustFindFailedNodeLoginAttempt(t, s, "invalidUser")
		})
	}
}

func TestEnvVarCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		inputFormat  string
		inputKey     string
		inputValue   string
		expectOutput string
		expectError  bool
	}{
		{
			inputFormat:  envVarFormatText,
			inputKey:     "key",
			inputValue:   "value",
			expectOutput: "key=value",
		},
		{
			inputFormat:  envVarFormatUnix,
			inputKey:     "key",
			inputValue:   "value",
			expectOutput: "export key=value",
		},
		{
			inputFormat:  envVarFormatWindowsCommandPrompt,
			inputKey:     "key",
			inputValue:   "value",
			expectOutput: "set key=value",
		},
		{
			inputFormat:  envVarFormatWindowsPowershell,
			inputKey:     "key",
			inputValue:   "value",
			expectOutput: "$Env:key=\"value\"",
		},
		{
			inputFormat: "unknown",
			inputKey:    "key",
			inputValue:  "value",
			expectError: true,
		},
	}
	for _, test := range tests {
		t.Run(test.inputFormat, func(t *testing.T) {
			actualOutput, err := envVarCommand(test.inputFormat, test.inputKey, test.inputValue)
			if test.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, test.expectOutput, actualOutput)
			}
		})
	}
}

// TestList verifies "tsh ls" functionality
func TestList(t *testing.T) {
	isInsecure := lib.IsInsecureDevMode()
	lib.SetInsecureDevMode(true)
	t.Cleanup(func() {
		lib.SetInsecureDevMode(isInsecure)
	})

	s := newTestSuite(t,
		withRootConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Version = defaults.TeleportConfigVersionV2
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
		}),
		withLeafCluster(),
		withLeafConfigFunc(func(cfg *servicecfg.Config) {
			cfg.Version = defaults.TeleportConfigVersionV2
		}),
	)
	rootNodeAddress, err := s.root.NodeSSHAddr()
	require.NoError(t, err)
	leafNodeAddress, err := s.leaf.NodeSSHAddr()
	require.NoError(t, err)

	testCases := []struct {
		description string
		command     []string
		assertion   func(t *testing.T, out []byte)
	}{
		{
			description: "List root cluster nodes",
			command:     []string{"ls", "-f", "json"},
			assertion: func(t *testing.T, out []byte) {
				var results []types.ServerV2
				require.NoError(t, json.Unmarshal(out, &results))

				require.Len(t, results, 1)
				require.Equal(t, "rootnode", results[0].Spec.Hostname)
				require.Equal(t, rootNodeAddress.String(), results[0].Spec.Addr)
			},
		},
		{
			description: "List leaf cluster nodes",
			command:     []string{"ls", "-c", "leaf1", "-f", "json"},
			assertion: func(t *testing.T, out []byte) {
				var results []types.ServerV2
				require.NoError(t, json.Unmarshal(out, &results))

				require.Len(t, results, 1)
				require.Equal(t, "leafnode", results[0].Spec.Hostname)
				require.Equal(t, leafNodeAddress.String(), results[0].Spec.Addr)
			},
		},
		{
			description: "List all clusters nodes",
			command:     []string{"ls", "-R", "-f", "json"},
			assertion: func(t *testing.T, out []byte) {
				expected := map[string]string{
					"root":  "rootnode",
					"leaf1": "leafnode",
				}

				type result struct {
					Cluster string         `json:"cluster"`
					Node    types.ServerV2 `json:"node"`
				}
				var results []result
				require.NoError(t, json.Unmarshal(out, &results))

				require.Equal(t, len(expected), len(results))
				for _, res := range results {
					node, ok := expected[res.Cluster]
					require.True(t, ok, "expected node to be present for cluster %s", res.Cluster)
					require.Equal(t, node, res.Node.Spec.Hostname)
					address := leafNodeAddress
					if res.Cluster == s.root.Config.Auth.ClusterName.GetName() {
						address = rootNodeAddress
					}
					require.Equal(t, address.String(), results[0].Node.Spec.Addr)
				}
			},
		},
	}

	tshHome, _ := mustLogin(t, s)
	for _, test := range testCases {
		t.Run(test.description, func(t *testing.T) {
			stdout := &bytes.Buffer{}
			err := Run(context.Background(), test.command, setHomePath(tshHome), setOverrideStdout(stdout))
			require.NoError(t, err)

			test.assertion(t, stdout.Bytes())
		})
	}
}

// create a new local agent key ring and serve it on $SSH_AUTH_SOCK for tests.
func createAgent(t *testing.T) (agent.ExtendedAgent, string) {
	t.Helper()

	currentUser, err := user.Current()
	require.NoError(t, err)

	sockDir := "test"
	sockName := "agent.sock"

	keyring, ok := agent.NewKeyring().(agent.ExtendedAgent)
	require.True(t, ok)

	teleAgent := teleagent.NewServer(func() (teleagent.Agent, error) {
		return teleagent.NopCloser(keyring), nil
	})

	// Start the SSH agent.
	err = teleAgent.ListenUnixSocket(sockDir, sockName, currentUser)
	require.NoError(t, err)
	go teleAgent.Serve()
	t.Cleanup(func() {
		teleAgent.Close()
	})

	t.Setenv(teleport.SSHAuthSock, teleAgent.Path)

	return keyring, teleAgent.Path
}

func disableAgent(t *testing.T) {
	t.Helper()
	t.Setenv(teleport.SSHAuthSock, "")
}

func (s *suite) setMockSSOLogin(t *testing.T) CliOption {
	return setMockSSOLogin(s.root.GetAuthServer(), s.user, s.connector.GetName())
}

func mustLogin(t *testing.T, s *suite, args ...string) (tshHome, kubeConfig string) {
	tshHome = t.TempDir()
	kubeConfig = filepath.Join(t.TempDir(), teleport.KubeConfigFile)
	args = append([]string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", s.root.Config.Proxy.WebAddr.String(),
	}, args...)
	err := Run(context.Background(), args,
		s.setMockSSOLogin(t),
		setHomePath(tshHome),
		setKubeConfigPath(kubeConfig),
	)
	require.NoError(t, err)
	return
}

// login with new temp tshHome and set it in Env. This is useful
// when running "ssh" commands with a tsh "ProxyCommand".
func mustLoginSetEnv(t *testing.T, s *suite, args ...string) (tshHome, kubeConfig string) {
	tshHome, kubeConfig = mustLogin(t, s, args...)
	t.Setenv(types.HomeEnvVar, tshHome)
	return
}

func mustLoginIdentity(t *testing.T, s *suite) string {
	identityFile := path.Join(t.TempDir(), "identity.pem")
	mustLogin(t, s, "--out", identityFile)
	return identityFile
}

func mustGetOpenSSHConfigFile(t *testing.T) string {
	var buff bytes.Buffer
	err := Run(context.Background(), []string{
		"config",
	}, setCopyStdout(&buff))
	require.NoError(t, err)

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "ssh_config")
	err = os.WriteFile(configPath, buff.Bytes(), 0o600)
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

	_, agentPath := createAgent(t)
	cmd := exec.Command(sshPath, ss...)
	cmd.Env = []string{
		fmt.Sprintf("%s=1", tshBinMainTestEnv),
		fmt.Sprintf("SSH_AUTH_SOCK=%s", agentPath),
		fmt.Sprintf("PATH=%s", filepath.Dir(sshPath)),
		fmt.Sprintf("%s=%s", types.HomeEnvVar, os.Getenv(types.HomeEnvVar)),
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	return trace.Wrap(cmd.Run())
}

func mustRunOpenSSHCommand(t *testing.T, configFile string, sshConnString string, port int, args ...string) {
	err := retryutils.RetryStaticFor(time.Second*10, time.Millisecond*500, func() error {
		err := runOpenSSHCommand(t, configFile, sshConnString, port, args...)
		return trace.Wrap(err)
	})
	require.NoError(t, err)
}

func mustFailToRunOpenSSHCommand(t *testing.T, configFile string, sshConnString string, port int, args ...string) {
	err := runOpenSSHCommand(t, configFile, sshConnString, port, args...)
	require.Error(t, err)
}

func mustFindFailedNodeLoginAttempt(t *testing.T, s *suite, nodeLogin string) {
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		now := time.Now()
		ctx := context.Background()
		es, _, err := s.root.GetAuthServer().SearchEvents(ctx, events.SearchEventsRequest{
			From:  now.Add(-time.Hour),
			To:    now.Add(time.Hour),
			Order: types.EventOrderDescending,
		})
		assert.NoError(t, err)

		for _, e := range es {
			if e.GetCode() == events.AuthAttemptFailureCode {
				assert.Equal(t, e.(*apievents.AuthAttempt).Login, nodeLogin)
				return
			}
		}
		t.Errorf("failed to find AuthAttemptFailureCode event (0/%d events matched)", len(es))
	}, 5*time.Second, 500*time.Millisecond)
}

func TestFormatCommand(t *testing.T) {
	t.Parallel()

	setEnv := func(command *exec.Cmd, envs ...string) *exec.Cmd {
		command.Env = append(command.Env, envs...)
		return command
	}

	tests := []struct {
		name string
		cmd  *exec.Cmd
		want string
	}{
		{
			name: "simple command",
			cmd:  exec.Command("echo", "hello"),
			want: "echo hello",
		},
		{
			name: "whitespace arguments",
			cmd: exec.Command("echo", "hello world", "return\n\r", `1
2
3`),
			want: "echo \"hello world\" \"return\n\r\" \"1\n2\n3\"",
		},
		{
			name: "args and env",
			cmd:  setEnv(exec.Command("echo", "hello"), "DEBUG=1", "RUN=YES"),
			want: "DEBUG=1 RUN=YES echo hello",
		},
		{
			name: "args, whitespace and env",
			cmd:  setEnv(exec.Command("echo", "hello\"\nworld"), "DEBUG=1", "RUN=YES"),
			want: "DEBUG=1 RUN=YES echo \"hello\\\"\nworld\"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatCommand(tt.cmd))
		})
	}
}

func Test_chooseProxyCommandTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		commands         []dbcmd.CommandAlternative
		randomPort       bool
		wantTemplate     *template.Template
		wantTemplateArgs map[string]any
		wantOutput       string
	}{
		{
			name: "single command",
			commands: []dbcmd.CommandAlternative{
				{
					Description: "default",
					Command:     exec.Command("echo", "hello world"),
				},
			},
			wantTemplate:     dbProxyAuthTpl,
			wantTemplateArgs: map[string]any{"command": "echo \"hello world\""},
			wantOutput: `Started authenticated tunnel for the MySQL database "mydb" in cluster "mycluster" on 127.0.0.1:64444.

Teleport Connect is a desktop app that can manage database proxies for you.
Learn more at https://goteleport.com/docs/connect-your-client/teleport-connect/#connecting-to-a-database

Use the following command to connect to the database or to the address above using other database GUI/CLI clients:
  $ echo "hello world"
`,
		},
		{
			name: "multiple commands",
			commands: []dbcmd.CommandAlternative{
				{
					Description: "default",
					Command:     exec.Command("echo", "hello world"),
				},
				{
					Description: "alternative",
					Command:     exec.Command("echo", "goodbye world"),
				},
			},
			wantTemplate: dbProxyAuthMultiTpl,
			wantTemplateArgs: map[string]any{
				"commands": []templateCommandItem{
					{Description: "default", Command: "echo \"hello world\""},
					{Description: "alternative", Command: "echo \"goodbye world\""},
				},
			},
			wantOutput: `Started authenticated tunnel for the MySQL database "mydb" in cluster "mycluster" on 127.0.0.1:64444.

Teleport Connect is a desktop app that can manage database proxies for you.
Learn more at https://goteleport.com/docs/connect-your-client/teleport-connect/#connecting-to-a-database

Use one of the following commands to connect to the database or to the address above using other database GUI/CLI clients:

  * default:

  $ echo "hello world"

  * alternative:

  $ echo "goodbye world"

`,
		},
		{
			name: "single command, random port",
			commands: []dbcmd.CommandAlternative{
				{
					Description: "default",
					Command:     exec.Command("echo", "hello world"),
				},
			},
			randomPort:       true,
			wantTemplate:     dbProxyAuthTpl,
			wantTemplateArgs: map[string]any{"command": "echo \"hello world\""},
			wantOutput: `Started authenticated tunnel for the MySQL database "mydb" in cluster "mycluster" on 127.0.0.1:64444.
To avoid port randomization, you can choose the listening port using the --port flag.

Teleport Connect is a desktop app that can manage database proxies for you.
Learn more at https://goteleport.com/docs/connect-your-client/teleport-connect/#connecting-to-a-database

Use the following command to connect to the database or to the address above using other database GUI/CLI clients:
  $ echo "hello world"
`,
		},
		{
			name: "multiple commands, random port",
			commands: []dbcmd.CommandAlternative{
				{
					Description: "default",
					Command:     exec.Command("echo", "hello world"),
				},
				{
					Description: "alternative",
					Command:     exec.Command("echo", "goodbye world"),
				},
			},
			randomPort:   true,
			wantTemplate: dbProxyAuthMultiTpl,
			wantTemplateArgs: map[string]any{
				"commands": []templateCommandItem{
					{Description: "default", Command: "echo \"hello world\""},
					{Description: "alternative", Command: "echo \"goodbye world\""},
				},
			},
			wantOutput: `Started authenticated tunnel for the MySQL database "mydb" in cluster "mycluster" on 127.0.0.1:64444.
To avoid port randomization, you can choose the listening port using the --port flag.

Teleport Connect is a desktop app that can manage database proxies for you.
Learn more at https://goteleport.com/docs/connect-your-client/teleport-connect/#connecting-to-a-database

Use one of the following commands to connect to the database or to the address above using other database GUI/CLI clients:

  * default:

  $ echo "hello world"

  * alternative:

  $ echo "goodbye world"

`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			templateArgs := map[string]any{}
			tpl := chooseProxyCommandTemplate(templateArgs, tt.commands, "")
			require.Equal(t, tt.wantTemplate, tpl)
			require.Equal(t, tt.wantTemplateArgs, templateArgs)

			// test resulting template

			templateArgs["database"] = "mydb"
			templateArgs["cluster"] = "mycluster"
			templateArgs["address"] = "127.0.0.1:64444"
			templateArgs["type"] = defaults.ReadableDatabaseProtocol(defaults.ProtocolMySQL)
			templateArgs["randomPort"] = tt.randomPort

			buf := new(bytes.Buffer)
			err := tpl.Execute(buf, templateArgs)
			require.NoError(t, err)
			require.Equal(t, tt.wantOutput, buf.String())
		})
	}
}

type fakeAWSAppInfo struct {
	forwardProxyAddr string
}

func (f fakeAWSAppInfo) GetAppName() string {
	return "fake-aws-app"
}

func (f fakeAWSAppInfo) GetEnvVars() (map[string]string, error) {
	envVars := map[string]string{
		"AWS_ACCESS_KEY_ID":     "FAKE_ID",
		"AWS_SECRET_ACCESS_KEY": "FAKE_KEY",
		"AWS_CA_BUNDLE":         "FAKE_CA_BUNDLE_PATH",
	}
	if f.forwardProxyAddr != "" {
		envVars["HTTPS_PROXY"] = "http://" + f.forwardProxyAddr
	}
	return envVars, nil
}

func (f fakeAWSAppInfo) GetEndpointURL() string {
	return "https://127.0.0.1:12345"
}

func (f fakeAWSAppInfo) GetForwardProxyAddr() string {
	return f.forwardProxyAddr
}

func TestPrintProxyAWSTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		inputCLIConf *CLIConf
		inputAWSApp  awsAppInfo
		wantSnippets []string
	}{
		{
			name: "HTTPS_PROXY mode",
			inputCLIConf: &CLIConf{
				Format: envVarDefaultFormat(),
			},
			inputAWSApp: fakeAWSAppInfo{
				forwardProxyAddr: "127.0.0.1:8888",
			},
			wantSnippets: []string{
				"127.0.0.1:8888",
				"Use the following credentials and HTTPS proxy setting to connect to the proxy",
			},
		},
		{
			name: "endpoint URL mode",
			inputCLIConf: &CLIConf{
				Format:             envVarDefaultFormat(),
				AWSEndpointURLMode: true,
			},
			inputAWSApp: fakeAWSAppInfo{},
			wantSnippets: []string{
				"AWS endpoint URL at https://127.0.0.1:12345",
			},
		},
		{
			name: "athena-odbc",
			inputCLIConf: &CLIConf{
				Format: awsProxyFormatAthenaODBC,
			},
			inputAWSApp: fakeAWSAppInfo{
				forwardProxyAddr: "127.0.0.1:8888",
			},
			wantSnippets: []string{
				"DRIVER=Simba Amazon Athena ODBC Connector;",
				"UseProxy=1;ProxyScheme=http;ProxyHost=127.0.0.1;ProxyPort=8888;",
			},
		},
		{
			name: "athena-jdbc",
			inputCLIConf: &CLIConf{
				Format: awsProxyFormatAthenaJDBC,
			},
			inputAWSApp: fakeAWSAppInfo{
				forwardProxyAddr: "127.0.0.1:8888",
			},
			wantSnippets: []string{
				"jdbc:awsathena:",
				"ProxyHost=127.0.0.1;ProxyPort=8888;",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buf := new(bytes.Buffer)
			test.inputCLIConf.OverrideStdout = buf
			require.NoError(t, printProxyAWSTemplate(test.inputCLIConf, test.inputAWSApp))
			for _, wantSnippet := range test.wantSnippets {
				require.Contains(t, buf.String(), wantSnippet)
			}
		})
	}
}

func TestCheckProxyAWSFormatCompatibility(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		input      *CLIConf
		checkError require.ErrorAssertionFunc
	}{
		{
			name: "default format is supported in HTTPS_PROXY mode",
			input: &CLIConf{
				Format: envVarDefaultFormat(),
			},
			checkError: require.NoError,
		},
		{
			name: "default format is supported in endpoint URL mode",
			input: &CLIConf{
				Format:             envVarDefaultFormat(),
				AWSEndpointURLMode: true,
			},
			checkError: require.NoError,
		},
		{
			name: "athena-odbc is supported in HTTPS_PROXY mode",
			input: &CLIConf{
				Format: awsProxyFormatAthenaODBC,
			},
			checkError: require.NoError,
		},
		{
			name: "athena-odbc is not supported in endpoint URL mode",
			input: &CLIConf{
				Format:             awsProxyFormatAthenaODBC,
				AWSEndpointURLMode: true,
			},
			checkError: require.Error,
		},
		{
			name: "athena-jdbc is supported in HTTPS_PROXY mode",
			input: &CLIConf{
				Format: awsProxyFormatAthenaJDBC,
			},
			checkError: require.NoError,
		},
		{
			name: "athena-jdbc is not supported in endpoint URL mode",
			input: &CLIConf{
				Format:             awsProxyFormatAthenaJDBC,
				AWSEndpointURLMode: true,
			},
			checkError: require.Error,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.checkError(t, checkProxyAWSFormatCompatibility(test.input))
		})
	}
}
