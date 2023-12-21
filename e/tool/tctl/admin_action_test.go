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

package main_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	loginrulepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/loginrule/v1"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/wrappers"
	authe "github.com/gravitational/teleport/e/lib/auth"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	libclient "github.com/gravitational/teleport/lib/client"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/plugin"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	tctl "github.com/gravitational/teleport/tool/tctl/common"
	loginruleresource "github.com/gravitational/teleport/tool/tctl/common/loginrule"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
	tsh "github.com/gravitational/teleport/tool/tsh/common"
)

func TestAdminActionMFA(t *testing.T) {
	s := newAdminActionTestSuite(t)

	t.Run("LoginRules", s.testLoginRules)
}

func (s *adminActionTestSuite) testLoginRules(t *testing.T) {
	ctx := context.Background()

	loginRulePB := &loginrulepb.LoginRule{
		Metadata: &types.Metadata{
			Name:      "loginrule",
			Namespace: "default",
		},
		Version:  "v1",
		Priority: 0,
		TraitsMap: map[string]*wrappers.StringValues{
			"logins": {
				Values: []string{"external.login"},
			},
		},
	}
	loginRule := loginruleresource.ProtoToResource(loginRulePB)

	createLoginRule := func() error {
		_, err := s.authClient.LoginRuleClient().CreateLoginRule(ctx, &loginrulepb.CreateLoginRuleRequest{
			LoginRule: loginRulePB,
		})
		return trace.Wrap(err)
	}

	getLoginRule := func() (types.Resource, error) {
		resp, err := s.authClient.LoginRuleClient().GetLoginRule(ctx, &loginrulepb.GetLoginRuleRequest{
			Name: loginRule.GetName(),
		})
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return loginruleresource.ProtoToResource(resp), nil
	}

	deleteLoginRule := func() error {
		_, err := s.authClient.LoginRuleClient().DeleteLoginRule(ctx, &loginrulepb.DeleteLoginRuleRequest{
			Name: loginRule.GetName(),
		})
		return trace.Wrap(err)
	}

	s.testResourceCommand(t, ctx, resourceCommandTestCase{
		resource:       loginRule,
		resourceCreate: createLoginRule,
		resourceDelete: deleteLoginRule,
	})

	s.testEditCommand(t, ctx, editCommandTestCase{
		resourceRef:    getResourceRef(loginRule),
		resourceCreate: createLoginRule,
		resourceGet:    getLoginRule,
		resourceDelete: deleteLoginRule,
	})
}

type resourceCommandTestCase struct {
	resource       types.Resource
	resourceCreate func() error
	resourceDelete func() error
}

func (s *adminActionTestSuite) testResourceCommand(t *testing.T, ctx context.Context, tc resourceCommandTestCase) {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "resource-*.yaml")
	require.NoError(t, err)
	require.NoError(t, utils.WriteYAML(f, tc.resource))

	t.Run("tctl create", func(t *testing.T) {
		s.testCommand(t, ctx, adminActionTestCase{
			command:    fmt.Sprintf("create %v", f.Name()),
			cliCommand: &tctl.ResourceCommand{},
			cleanup:    tc.resourceDelete,
		})
	})

	t.Run("tctl create -f", func(t *testing.T) {
		s.testCommand(t, ctx, adminActionTestCase{
			command:    fmt.Sprintf("create -f %v", f.Name()),
			cliCommand: &tctl.ResourceCommand{},
			setup:      tc.resourceCreate,
			cleanup:    tc.resourceDelete,
		})
	})

	t.Run("tctl rm", func(t *testing.T) {
		s.testCommand(t, ctx, adminActionTestCase{
			command:    fmt.Sprintf("rm %v", getResourceRef(tc.resource)),
			cliCommand: &tctl.ResourceCommand{},
			setup:      tc.resourceCreate,
			cleanup:    tc.resourceDelete,
		})
	})
}

type editCommandTestCase struct {
	resourceRef    string
	resourceCreate func() error
	resourceGet    func() (types.Resource, error)
	resourceDelete func() error
}

func (s *adminActionTestSuite) testEditCommand(t *testing.T, ctx context.Context, tc editCommandTestCase) {
	t.Run("tctl edit", func(t *testing.T) {
		s.testCommand(t, ctx, adminActionTestCase{
			command: fmt.Sprintf("edit %v", tc.resourceRef),
			setup:   tc.resourceCreate,
			cliCommand: &tctl.EditCommand{
				Editor: func(filename string) error {
					// Get the latest version of the resource with the correct revision ID.
					resource, err := tc.resourceGet()
					require.NoError(t, err)

					// Update the expiry so that the edit goes through.
					resource.SetExpiry(time.Now())

					f, err := os.Create(filename)
					require.NoError(t, err)
					require.NoError(t, utils.WriteYAML(f, resource))
					return nil
				},
			},
			cleanup: tc.resourceDelete,
		})
	})
}

type adminActionTestSuite struct {
	authServer *auth.Server
	authClient auth.ClientI
	// userClientWithMFA supports MFA prompt for admin actions.
	userClientWithMFA auth.ClientI
	// userClientWithMFA does not support MFA prompt for admin actions.
	userClientNoMFA auth.ClientI
}

func newAdminActionTestSuite(t *testing.T) *adminActionTestSuite {
	t.Helper()
	ctx := context.Background()

	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "127.0.0.1",
		},
	})
	require.NoError(t, err)
	authPref.SetOrigin(types.OriginDefaults)

	process := testserver.MakeTestServer(t,
		testserver.WithAuthPreference(authPref),
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
	proxyAddr, err := process.ProxyWebAddr()
	require.NoError(t, err)
	authAddr, err := process.AuthAddr()
	require.NoError(t, err)
	authServer := process.GetAuthServer()

	// create admin role and user.
	username := "admin"
	adminRole, err := types.NewRole(username, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{types.Wildcard},
					Verbs:     []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)
	adminRole, err = authServer.CreateRole(ctx, adminRole)
	require.NoError(t, err)

	user, err := types.NewUser(username)
	user.SetRoles([]string{adminRole.GetName()})
	require.NoError(t, err)
	_, err = authServer.CreateUser(ctx, user)
	require.NoError(t, err)

	mockWebauthnLogin := setupWebAuthn(t, authServer, username)
	mockMFAPromptConstructor := func(opts ...mfa.PromptOpt) mfa.Prompt {
		promptCfg := libmfa.NewPromptConfig(proxyAddr.String(), opts...)
		promptCfg.WebauthnLoginFunc = mockWebauthnLogin
		return libmfa.NewCLIPrompt(promptCfg, os.Stderr)
	}

	// Login as the admin user.
	tshHome := t.TempDir()
	err = tsh.Run(context.Background(), []string{
		"login",
		"--insecure",
		"--debug",
		"--user", username,
		"--proxy", proxyAddr.String(),
		"--auth", constants.PasswordlessConnector,
	},
		setHomePath(tshHome),
		setKubeConfigPath(filepath.Join(t.TempDir(), teleport.KubeConfigFile)),
		func(c *tsh.CLIConf) error {
			c.WebauthnLogin = mockWebauthnLogin
			return nil
		},
	)
	require.NoError(t, err)

	userClientNoMFA, err := auth.NewClient(client.Config{
		Addrs: []string{authAddr.String()},
		Credentials: []client.Credentials{
			client.LoadProfile(tshHome, ""),
		},
	})
	require.NoError(t, err)

	userClientWithMFA, err := auth.NewClient(client.Config{
		Addrs: []string{authAddr.String()},
		Credentials: []client.Credentials{
			client.LoadProfile(tshHome, ""),
		},
		MFAPromptConstructor: mockMFAPromptConstructor,
	})
	require.NoError(t, err)

	identity, err := auth.ReadLocalIdentity(filepath.Join(process.Config.DataDir, teleport.ComponentProcess), auth.IdentityID{Role: types.RoleAdmin, HostUUID: process.Config.HostUUID})
	require.NoError(t, err)
	tlsConfig, err := identity.TLSConfig(nil)
	require.NoError(t, err)

	authClient, err := auth.NewClient(client.Config{
		Addrs: []string{authAddr.String()},
		Credentials: []client.Credentials{
			client.LoadTLS(tlsConfig),
		},
		MFAPromptConstructor: mockMFAPromptConstructor,
	})
	require.NoError(t, err)

	return &adminActionTestSuite{
		authServer:        authServer,
		authClient:        authClient,
		userClientNoMFA:   userClientNoMFA,
		userClientWithMFA: userClientWithMFA,
	}
}

type adminActionTestCase struct {
	command    string
	cliCommand tctl.CLICommand
	setup      func() error
	cleanup    func() error
}

func (s *adminActionTestSuite) testCommand(t *testing.T, ctx context.Context, tc adminActionTestCase) {
	t.Helper()

	t.Run("OK with MFA", func(t *testing.T) {
		err := runTestCase(t, ctx, s.userClientWithMFA, tc)
		require.NoError(t, err)
	})

	t.Run("NOK without MFA", func(t *testing.T) {
		err := runTestCase(t, ctx, s.userClientNoMFA, tc)
		require.ErrorIs(t, err, &mfa.ErrAdminActionMFARequired)
	})

	t.Run("OK mfa off", func(t *testing.T) {
		// turn MFA off, admin actions should not require MFA now.
		authPref := types.DefaultAuthPreference()
		authPref.SetSecondFactor(constants.SecondFactorOff)
		originalAuthPref, err := s.authServer.GetAuthPreference(ctx)
		require.NoError(t, err)

		require.NoError(t, s.authServer.SetAuthPreference(ctx, authPref))
		t.Cleanup(func() {
			require.NoError(t, s.authServer.SetAuthPreference(ctx, originalAuthPref))
		})

		err = runTestCase(t, ctx, s.userClientNoMFA, tc)
		require.NoError(t, err)
	})
}

func runTestCase(t *testing.T, ctx context.Context, client auth.ClientI, tc adminActionTestCase) error {
	t.Helper()

	if tc.setup != nil {
		require.NoError(t, tc.setup(), "unexpected error during setup")
	}
	if tc.cleanup != nil {
		t.Cleanup(func() {
			if err := tc.cleanup(); err != nil && !trace.IsNotFound(err) {
				t.Errorf("unexpected error during cleanup: %v", err)
			}
		})
	}

	app := utils.InitCLIParser("tctl", tctl.GlobalHelpString)
	cfg := servicecfg.MakeDefaultConfig()
	tc.cliCommand.Initialize(app, cfg)

	args := strings.Split(tc.command, " ")
	commandName, err := app.Parse(args)
	require.NoError(t, err)

	match, err := tc.cliCommand.TryRun(ctx, commandName, client)
	require.True(t, match)
	return err
}

func getResourceRef(r types.Resource) string {
	switch kind := r.GetKind(); kind {
	case types.KindClusterAuthPreference:
		// single resources are referred to by kind alone.
		return kind
	default:
		return fmt.Sprintf("%v/%v", r.GetKind(), r.GetName())
	}
}

func setupWebAuthn(t *testing.T, authServer *auth.Server, username string) libclient.WebauthnLoginFunc {
	t.Helper()
	ctx := context.Background()

	const origin = "https://127.0.0.1"
	device, err := mocku2f.Create()
	require.NoError(t, err)
	device.SetPasswordless()

	token, err := authServer.CreateResetPasswordToken(ctx, auth.CreateUserTokenRequest{
		Name: username,
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

	userWebID := res.GetWebauthn().PublicKey.User.Id

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

	return func(ctx context.Context, origin string, assertion *wantypes.CredentialAssertion, prompt wancli.LoginPrompt, opts *wancli.LoginOpts) (*proto.MFAAuthenticateResponse, string, error) {
		car, err := device.SignAssertion(origin, assertion)
		if err != nil {
			return nil, "", err
		}
		car.AssertionResponse.UserHandle = userWebID

		return &proto.MFAAuthenticateResponse{
			Response: &proto.MFAAuthenticateResponse_Webauthn{
				Webauthn: wantypes.CredentialAssertionResponseToProto(car),
			},
		}, "", nil
	}
}

func setHomePath(path string) tsh.CliOption {
	return func(cf *tsh.CLIConf) error {
		cf.HomePath = path
		return nil
	}
}

func setKubeConfigPath(path string) tsh.CliOption {
	return func(cf *tsh.CLIConf) error {
		cf.KubeConfigPath = path
		return nil
	}
}
