package main_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/agent"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	authe "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/auth/native"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	"github.com/gravitational/teleport/lib/client"
	dtauthn "github.com/gravitational/teleport/lib/devicetrust/authn"
	dttestenv "github.com/gravitational/teleport/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/teleagent"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
	tshcommon "github.com/gravitational/teleport/tool/tsh/common"
)

func TestMain(m *testing.M) {
	if srv.IsReexec() {
		return
	}
	modules.SetInsecureTestMode(true)

	os.Exit(m.Run())
}

// TestNodeAccess tests 'tsh ssh' and 'tsh scp' functionality with various security features enabled.
func TestNodeAccess(t *testing.T) {
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
				entitlements.DeviceTrust: {Enabled: true},
			},
		},
	})

	ctx := context.Background()
	a := createAgent(t)

	rsaKey, err := native.GenerateRSAPrivateKey()
	require.NoError(t, err)

	testKey := agent.AddedKey{
		PrivateKey: rsaKey,
		Comment:    "test-key",
	}
	require.NoError(t, a.Add(testKey))

	user, err := user.Current()
	require.NoError(t, err)

	connector := mockConnector(t)

	process := testserver.MakeTestServer(t,
		testserver.WithBootstrap(connector),
		testserver.WithHostname("node01"),
		testserver.WithClusterName(t, "root"),
		testserver.WithSSHLabel("env", "staging"),
		testserver.WithSSHPublicAddrs("localhost"),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.PluginRegistry = plugin.NewRegistry()
			authPlugin, err := authe.NewPlugin(authe.Config{
				License:       authe.ValidLicense{},
				HostedPlugins: cfg.Auth.HostedPlugins,
			})
			require.NoError(t, err)

			err = cfg.PluginRegistry.Add(authPlugin)
			require.NoError(t, err)
		}),
	)

	authServer := process.GetAuthServer()
	_, err = authServer.UpsertAuthPreference(ctx, &types.AuthPreferenceV2{
		Spec: types.AuthPreferenceSpecV2{
			Type:         constants.Local,
			SecondFactor: constants.SecondFactorOptional,
			Webauthn: &types.Webauthn{
				RPID: "127.0.0.1",
			},
		},
	})
	require.NoError(t, err)

	proxyAddr, err := process.ProxyWebAddr()
	require.NoError(t, err)

	setupUserAndRole := func(t *testing.T, name string, roleSpec types.RoleSpecV6) {
		// create role
		role, err := auth.CreateRole(ctx, authServer, name, roleSpec)
		require.NoError(t, err)

		// create user
		user, err := types.NewUser(name)
		user.SetRoles([]string{role.GetName()})
		require.NoError(t, err)
		_, err = authServer.CreateUser(ctx, user)
		require.NoError(t, err)
	}

	const origin = "https://127.0.0.1"
	device, err := mocku2f.Create()
	require.NoError(t, err)
	device.SetPasswordless()
	webauthnLoginOpt := setupWebAuthnChallengeSolver(device, true /* success */)
	deviceTrustOpt := setupDeviceTrust(t, process)

	setupUserMFA := func(t *testing.T, name string) {
		token, err := authServer.CreateResetPasswordToken(ctx, authclient.CreateUserTokenRequest{
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
		cc := wantypes.CredentialCreationFromProto(res.GetWebauthn())

		ccr, err := device.SignCredentialCreation(origin, cc)
		require.NoError(t, err)
		_, err = authServer.ChangeUserAuthentication(ctx, &proto.ChangeUserAuthenticationRequest{
			TokenID: tokenID,
			NewMFARegisterResponse: &proto.MFARegisterResponse{
				Response: &proto.MFARegisterResponse_Webauthn{
					Webauthn: wantypes.CredentialCreationResponseToProto(ccr),
				},
			},
		})
		require.NoError(t, err)
	}

	const (
		homeLogin     = "home_login"
		identityLogin = "identity_login"
		headlessLogin = "headless_login"
	)

	loginFuncs := map[string]func(t *testing.T, proxyAddr string, user string) tshcommon.CliOption{
		homeLogin: func(t *testing.T, proxyAddr, user string) tshcommon.CliOption {
			_, _, opt := mustLoginHome(t, authServer, proxyAddr, user, connector.GetName(), func(cf *tshcommon.CLIConf) error {
				cf.AddKeysToAgent = "no"
				return nil
			}, webauthnLoginOpt)
			return opt
		},
		identityLogin: func(t *testing.T, proxyAddr, user string) tshcommon.CliOption {
			_, opt := mustLoginIdentity(t, authServer, proxyAddr, user, connector.GetName(), func(cf *tshcommon.CLIConf) error {
				cf.AddKeysToAgent = "no"
				return nil
			}, webauthnLoginOpt)
			return opt
		},
		headlessLogin: func(t *testing.T, proxyAddr, user string) tshcommon.CliOption {
			return setMockHeadlessLogin(t, authServer, user, proxyAddr)
		},
	}

	testCases := []struct {
		// the test name, and the user/role name for the test.
		name       string
		loginCases []string
		opts       []tshcommon.CliOption
		setup      func(t *testing.T)
	}{
		{
			name:       "default",
			loginCases: []string{homeLogin, identityLogin, headlessLogin},
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
			name:       "device_trust",
			loginCases: []string{homeLogin, headlessLogin}, // device trust does not support identity login
			opts:       []tshcommon.CliOption{deviceTrustOpt},
			setup: func(t *testing.T) {
				userName := "device_trust"
				setupUserAndRole(t, userName, types.RoleSpecV6{
					Allow: types.RoleConditions{
						Logins:     []string{user.Username},
						NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					},
					Options: types.RoleOptions{
						DeviceTrustMode: constants.DeviceTrustModeRequired,
						ForwardAgent:    true,
						MaxSessions:     1,
						MaxConnections:  1,
					},
				})
			},
		},
		{
			name:       "session_mfa",
			loginCases: []string{homeLogin, identityLogin, headlessLogin},
			opts:       []tshcommon.CliOption{webauthnLoginOpt},
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
		{
			name:       "device_trust_session_mfa",
			loginCases: []string{homeLogin, headlessLogin}, // device trust does not support identity login
			opts:       []tshcommon.CliOption{deviceTrustOpt, webauthnLoginOpt},
			setup: func(t *testing.T) {
				userName := "device_trust_session_mfa"
				setupUserAndRole(t, userName, types.RoleSpecV6{
					Allow: types.RoleConditions{
						Logins:     []string{user.Username},
						NodeLabels: types.Labels{types.Wildcard: []string{types.Wildcard}},
					},
					Options: types.RoleOptions{
						DeviceTrustMode: constants.DeviceTrustModeRequired,
						RequireMFAType:  types.RequireMFAType_SESSION,
						ForwardAgent:    true,
						MaxSessions:     1,
						MaxConnections:  1,
					},
				})
				setupUserMFA(t, userName)
			},
		},
	}

	sshHostNameCases := map[string]string{
		"nodename":    "node01",
		"addr":        "127.0.0.1",
		"public_addr": "localhost",
		"labels":      "env=staging",
	}

	scpHostNameCases := map[string]string{
		"nodename":    "node01",
		"addr":        "127.0.0.1",
		"public_addr": "localhost",
		// "tsh scp" does not support matching by node labels
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setup(t)

			for _, loginName := range tc.loginCases {
				t.Run(loginName, func(t *testing.T) {
					loginFunc := loginFuncs[loginName]
					loginOpt := loginFunc(t, proxyAddr.String(), tc.name)
					opts := append(tc.opts, loginOpt)

					opts = append(opts, func(c *tshcommon.CLIConf) error {
						c.DisableSSHResumption = true
						return nil
					})

					for hostType, hostName := range sshHostNameCases {
						t.Run(hostType, func(t *testing.T) {
							t.Run("SSH", func(t *testing.T) {
								err := tshcommon.Run(ctx, []string{
									"ssh",
									"-d",
									"--insecure",
									hostName,
									"echo", "hello",
								}, opts...)
								require.NoError(t, err)
							})
							t.Run("AgentForwarding", func(t *testing.T) {
								stdout := &bytes.Buffer{}
								agentForwardingOpts := append(opts, func(cf *tshcommon.CLIConf) error {
									cf.OverrideStdout = stdout
									return nil
								})
								err := tshcommon.Run(ctx, []string{
									"ssh",
									"-d",
									"--insecure",
									"-A",
									hostName,
									"ssh-add", "-l",
								}, agentForwardingOpts...)
								require.NoError(t, err)
								require.Contains(t, stdout.String(), "test-key", "agent test key entry not found during session:\n%v", stdout.String())
							})
						})
					}

					for hostType, hostName := range scpHostNameCases {
						t.Run(hostType, func(t *testing.T) {
							t.Run("SCPDownload", func(t *testing.T) {
								testDir := t.TempDir()
								localFilePath := filepath.Join(testDir, "local-test-file")
								remoteFilePath := filepath.Join(testDir, "remote-test-file")

								file, err := os.Create(remoteFilePath)
								require.NoError(t, err)
								testText := "This-is-a-test-file"
								_, err = file.WriteString(testText)
								require.NoError(t, err)
								require.NoError(t, file.Close())

								err = tshcommon.Run(context.Background(), []string{
									"scp",
									"-d",
									"--insecure",
									fmt.Sprintf("%v:%v", hostName, remoteFilePath),
									localFilePath,
								}, opts...)
								require.NoError(t, err)

								downloadedText, err := os.ReadFile(localFilePath)
								require.NoError(t, err)
								require.Equal(t, testText, string(downloadedText))
							})

							t.Run("SCPUpload", func(t *testing.T) {
								testDir := t.TempDir()
								localFilePath := filepath.Join(testDir, "local-test-file")
								remoteFilePath := filepath.Join(testDir, "remote-test-file")

								file, err := os.Create(localFilePath)
								require.NoError(t, err)
								testText := "This-is-a-test-file"
								_, err = file.WriteString(testText)
								require.NoError(t, err)
								require.NoError(t, file.Close())

								err = tshcommon.Run(context.Background(), []string{
									"scp",
									"-d",
									"--insecure",
									localFilePath,
									fmt.Sprintf("%v:%v", hostName, remoteFilePath),
								}, opts...)
								require.NoError(t, err)

								uploadedText, err := os.ReadFile(remoteFilePath)
								require.NoError(t, err)
								require.Equal(t, testText, string(uploadedText))
							})
						})
					}
				})
			}
		})
	}
}

func setMockSSOLogin(t *testing.T, authServer *auth.Server, user, connectorName string) tshcommon.CliOption {
	return func(cf *tshcommon.CLIConf) error {
		cf.MockSSOLogin = mockSSOLogin(t, authServer, user)
		cf.AuthConnector = connectorName
		return nil
	}
}

func mockSSOLogin(t *testing.T, authServer *auth.Server, user string) client.SSOLoginFunc {
	return func(ctx context.Context, _ string, priv *keys.PrivateKey, protocol string) (*authclient.SSHLoginResponse, error) {
		// generate certificates for our user
		clusterName, err := authServer.GetClusterName()
		require.NoError(t, err)
		sshCert, tlsCert, err := authServer.GenerateUserTestCerts(auth.GenerateUserTestCertsRequest{
			Key:            priv.MarshalSSHPublicKey(),
			Username:       user,
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
		return &authclient.SSHLoginResponse{
			Username:    user,
			Cert:        sshCert,
			TLSCert:     tlsCert,
			HostSigners: authclient.AuthoritiesToTrustedCerts([]types.CertAuthority{authority}),
		}, nil
	}
}

func setMockHeadlessLogin(t *testing.T, authServer *auth.Server, user, proxy string) tshcommon.CliOption {
	return func(cf *tshcommon.CLIConf) error {
		cf.MockHeadlessLogin = mockHeadlessLogin(t, authServer, user)
		cf.Headless = true
		cf.Username = user
		cf.Proxy = proxy
		cf.ExplicitUsername = true
		return nil
	}
}

func mockHeadlessLogin(t *testing.T, authServer *auth.Server, user string) client.SSHLoginFunc {
	return func(ctx context.Context, priv *keys.PrivateKey) (*authclient.SSHLoginResponse, error) {
		// generate certificates for our user
		clusterName, err := authServer.GetClusterName()
		require.NoError(t, err)
		sshCert, tlsCert, err := authServer.GenerateUserTestCerts(auth.GenerateUserTestCertsRequest{
			Key:            priv.MarshalSSHPublicKey(),
			Username:       user,
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
			Username:    user,
			Cert:        sshCert,
			TLSCert:     tlsCert,
			HostSigners: authclient.AuthoritiesToTrustedCerts([]types.CertAuthority{authority}),
		}, nil
	}
}

func setHomePath(path string) tshcommon.CliOption {
	return func(cf *tshcommon.CLIConf) error {
		cf.HomePath = path
		return nil
	}
}

func setKubeConfigPath(path string) tshcommon.CliOption {
	return func(cf *tshcommon.CLIConf) error {
		cf.KubeConfigPath = path
		return nil
	}
}

func setIdentity(path, proxy string) tshcommon.CliOption {
	return func(cf *tshcommon.CLIConf) error {
		cf.IdentityFileIn = path
		cf.Proxy = proxy
		return nil
	}
}

func setIdentityOut(path string) tshcommon.CliOption {
	return func(cf *tshcommon.CLIConf) error {
		cf.IdentityFileOut = path
		return nil
	}
}

func mustLogin(t *testing.T, proxyAddr string, opts ...tshcommon.CliOption) {
	err := tshcommon.Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--proxy", proxyAddr,
	}, opts...)
	require.NoError(t, err)
}

func mustLoginHome(t *testing.T, authServer *auth.Server, proxyAddr, user, connectorName string, opts ...tshcommon.CliOption) (tshHome string, kubeConfig string, loginOpt tshcommon.CliOption) {
	tshHome = t.TempDir()
	kubeConfig = filepath.Join(t.TempDir(), teleport.KubeConfigFile)

	mustLogin(t, proxyAddr, append(opts, setHomePath(tshHome), setKubeConfigPath(kubeConfig), setMockSSOLogin(t, authServer, user, connectorName))...)

	return tshHome, kubeConfig, func(cf *tshcommon.CLIConf) error {
		cf.HomePath = tshHome
		cf.KubeConfigPath = kubeConfig
		return nil
	}
}

func mustLoginIdentity(t *testing.T, authServer *auth.Server, proxyAddr, user, connectorName string, opts ...tshcommon.CliOption) (identityFilePath string, loginOpt tshcommon.CliOption) {
	identityFilePath = path.Join(t.TempDir(), "identity.pem")

	mustLogin(t, proxyAddr, append(opts, setIdentityOut(identityFilePath), setMockSSOLogin(t, authServer, user, connectorName))...)

	return identityFilePath, setIdentity(identityFilePath, proxyAddr)
}

func setupWebAuthnChallengeSolver(device *mocku2f.Key, success bool) tshcommon.CliOption {
	return func(c *tshcommon.CLIConf) error {
		c.WebauthnLogin = func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
			car, err := device.SignAssertion(origin, assertion)
			if err != nil {
				return nil, "", err
			}

			carProto := wantypes.CredentialAssertionResponseToProto(car)
			if !success {
				carProto.Type = "NOT A VALID TYPE" // set to an invalid type so the ceremony fails
			}

			return &proto.MFAAuthenticateResponse{
				Response: &proto.MFAAuthenticateResponse_Webauthn{
					Webauthn: carProto,
				},
			}, "", nil
		}
		return nil
	}
}

func setupDeviceTrust(t *testing.T, process *service.TeleportProcess) tshcommon.CliOption {
	ctx := context.Background()

	authAddr, err := process.AuthAddr()
	require.NoError(t, err)
	authConn, err := process.WaitForConnector(service.AuthIdentityEvent, nil)
	require.NotNil(t, authConn, err)
	adminTLS, err := authConn.ClientTLSConfig(nil)
	require.NoError(t, err)
	require.NotNil(t, adminTLS)

	clt, err := apiclient.New(ctx, apiclient.Config{
		Addrs: []string{authAddr.String()},
		Credentials: []apiclient.Credentials{
			apiclient.LoadTLS(adminTLS),
		},
		DialTimeout:              time.Second,
		InsecureAddressDiscovery: true,
	})
	require.NoError(t, err)
	defer clt.Close()

	// TODO (codingllama): Use end-user client and tsh enrollment logic
	devices := clt.DevicesClient()

	// Create a fake device and enroll it, so the fake server has the necessary
	// data to verify challenge signatures.
	macOSDev1, err := dttestenv.NewFakeMacOSDevice()
	require.NoError(t, err, "NewFakeMacOSDevice failed")

	// Create device and enroll token
	device, err := devices.CreateDevice(ctx, &devicepb.CreateDeviceRequest{
		Device: &devicepb.Device{
			OsType:   macOSDev1.GetDeviceOSType(),
			AssetTag: macOSDev1.SerialNumber,
		},
		CreateEnrollToken: true,
	})
	require.NoError(t, err)

	// Enroll device
	stream, err := devices.EnrollDevice(ctx)
	require.NoError(t, err)

	// 1. Init.
	initReq, err := macOSDev1.EnrollDeviceInit()
	require.NoError(t, err)
	initReq.Token = device.EnrollToken.Token
	err = stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_Init{
			Init: initReq,
		},
	})
	require.NoError(t, err)

	// 2. Challenge.
	resp, err := stream.Recv()
	require.NoError(t, err)

	sig, err := macOSDev1.SignChallenge(resp.GetMacosChallenge().Challenge)
	require.NoError(t, err)

	err = stream.Send(&devicepb.EnrollDeviceRequest{
		Payload: &devicepb.EnrollDeviceRequest_MacosChallengeResponse{
			MacosChallengeResponse: &devicepb.MacOSEnrollChallengeResponse{
				Signature: sig,
			},
		},
	})
	require.NoError(t, err)

	// 3. Success.
	resp, err = stream.Recv()
	require.NoError(t, err)
	require.NotNil(t, resp.GetSuccess(), "success response is nil, got %T instead", resp.Payload)

	return func(c *tshcommon.CLIConf) error {
		c.DTAuthnRunCeremony = (&dtauthn.Ceremony{
			GetDeviceCredential: func() (*devicepb.DeviceCredential, error) {
				return macOSDev1.GetDeviceCredential(), nil
			},
			CollectDeviceData:            macOSDev1.CollectDeviceData,
			SignChallenge:                macOSDev1.SignChallenge,
			SolveTPMAuthnDeviceChallenge: macOSDev1.SolveTPMAuthnDeviceChallenge,
			GetDeviceOSType:              macOSDev1.GetDeviceOSType,
		}).Run
		return nil
	}
}

// create a new local agent key ring and serve it on $SSH_AUTH_SOCK for tests.
func createAgent(t *testing.T) agent.ExtendedAgent {
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

	return keyring
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
