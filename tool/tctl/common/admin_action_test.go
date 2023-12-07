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

package common_test

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
	"github.com/stretchr/testify/suite"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	wancli "github.com/gravitational/teleport/lib/auth/webauthncli"
	wantypes "github.com/gravitational/teleport/lib/auth/webauthntypes"
	libclient "github.com/gravitational/teleport/lib/client"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	tctl "github.com/gravitational/teleport/tool/tctl/common"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
	tsh "github.com/gravitational/teleport/tool/tsh/common"
)

func TestAdminActionMFA(t *testing.T) {
	suite.Run(t, &AdminActionTestSuite{})
}

func (s *AdminActionTestSuite) TestUser() {
	t := s.T()
	ctx := context.Background()

	user, err := types.NewUser("teleuser")
	require.NoError(t, err)

	createUser := func() error {
		_, err := s.authServer.CreateUser(ctx, user)
		return trace.Wrap(err)
	}

	deleteUser := func() error {
		return s.authServer.DeleteUser(ctx, "teleuser")
	}

	for name, tc := range map[string]adminActionTestCase{
		"tctl users add": {
			command:    "users add teleuser --roles=access",
			cliCommand: &tctl.UserCommand{},
			cleanup:    deleteUser,
		},
		"tctl users update": {
			command:    "users update teleuser --set-roles=access,auditor",
			cliCommand: &tctl.UserCommand{},
			setup:      createUser,
			cleanup:    deleteUser,
		},
		"tctl users rm": {
			command:    "users rm teleuser",
			cliCommand: &tctl.UserCommand{},
			setup:      createUser,
			cleanup:    deleteUser,
		},
		"tctl users reset": {
			command:    "users reset teleuser",
			cliCommand: &tctl.UserCommand{},
			setup:      createUser,
			cleanup:    deleteUser,
		},
	} {
		t.Run(name, func(t *testing.T) {
			s.testCommand(t, ctx, tc)
		})
	}

	s.testResourceCommand(t, ctx, resourceCommandTestCase{
		resource:       user,
		resourceCreate: createUser,
		resourceDelete: deleteUser,
	})
}

func (s *AdminActionTestSuite) TestBot() {
	t := s.T()
	ctx := context.Background()

	botReq := &proto.CreateBotRequest{
		Name:  "bot",
		Roles: []string{teleport.PresetAccessRoleName},
	}

	createBot := func() error {
		_, err := s.authServer.CreateBot(ctx, botReq)
		return trace.Wrap(err)
	}

	deleteBot := func() error {
		return s.authServer.DeleteBot(ctx, botReq.Name)
	}

	t.Run("BotCommands", func(t *testing.T) {
		for name, tc := range map[string]adminActionTestCase{
			"tctl bots add": {
				command:    fmt.Sprintf("bots add --roles=%v %v", teleport.PresetAccessRoleName, botReq.Name),
				cliCommand: &tctl.BotsCommand{},
				cleanup:    deleteBot,
			},
			"tctl bots rm": {
				command:    fmt.Sprintf("bots rm %v", botReq.Name),
				cliCommand: &tctl.BotsCommand{},
				setup:      createBot,
				cleanup:    deleteBot,
			},
		} {
			t.Run(name, func(t *testing.T) {
				s.testCommand(t, ctx, tc)
			})
		}
	})
}

func (s *AdminActionTestSuite) TestRole() {
	t := s.T()
	ctx := context.Background()

	role, err := types.NewRole("telerole", types.RoleSpecV6{})
	require.NoError(t, err)

	createRole := func() error {
		_, err := s.authServer.CreateRole(ctx, role)
		return trace.Wrap(err)
	}

	getRole := func() (types.Resource, error) {
		return s.authServer.GetRole(ctx, role.GetName())
	}

	deleteRole := func() error {
		return s.authServer.DeleteRole(ctx, role.GetName())
	}

	s.testResourceCommand(t, ctx, resourceCommandTestCase{
		resource:       role,
		resourceCreate: createRole,
		resourceDelete: deleteRole,
	})

	s.testEditCommand(t, ctx, editCommandTestCase{
		resourceRef:    getResourceRef(role),
		resourceCreate: createRole,
		resourceGet:    getRole,
		resourceDelete: deleteRole,
	})
}

func (s *AdminActionTestSuite) TestUserGroup() {
	t := s.T()
	ctx := context.Background()

	userGroup, err := types.NewUserGroup(types.Metadata{
		Name:   "teleusergroup",
		Labels: map[string]string{"label": "value"},
	}, types.UserGroupSpecV1{})
	require.NoError(t, err)

	// Only deletion is permitted through tctl.
	t.Run("tctl rm", func(t *testing.T) {
		s.testCommand(t, ctx, adminActionTestCase{
			command:    fmt.Sprintf("rm %v", getResourceRef(userGroup)),
			cliCommand: &tctl.ResourceCommand{},
			setup: func() error {
				return s.authServer.CreateUserGroup(ctx, userGroup)
			},
			cleanup: func() error {
				return s.authServer.DeleteUserGroup(ctx, userGroup.GetName())
			},
		})
	})
}

type resourceCommandTestCase struct {
	resource       types.Resource
	resourceCreate func() error
	resourceDelete func() error
}

func (s *AdminActionTestSuite) testResourceCommand(t *testing.T, ctx context.Context, tc resourceCommandTestCase) {
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

func (s *AdminActionTestSuite) testEditCommand(t *testing.T, ctx context.Context, tc editCommandTestCase) {
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

type AdminActionTestSuite struct {
	suite.Suite
	authServer *auth.Server
	// userClientWithMFA supports MFA prompt for admin actions.
	userClientWithMFA auth.ClientI
	// userClientWithMFA does not support MFA prompt for admin actions.
	userClientNoMFA auth.ClientI
}

func (s *AdminActionTestSuite) SetupSuite() {
	t := s.T()
	ctx := context.Background()

	authPref, err := types.NewAuthPreference(types.AuthPreferenceSpecV2{
		Type:         constants.Local,
		SecondFactor: constants.SecondFactorOptional,
		Webauthn: &types.Webauthn{
			RPID: "localhost",
		},
	})
	require.NoError(t, err)
	authPref.SetOrigin(types.OriginDefaults)

	var proxyPublicAddr utils.NetAddr
	process := testserver.MakeTestServer(t,
		testserver.WithAuthPreference(authPref),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			proxyPublicAddr = cfg.Proxy.WebAddr
			proxyPublicAddr.Addr = fmt.Sprintf("localhost:%v", proxyPublicAddr.Port(0))
			cfg.Proxy.PublicAddrs = []utils.NetAddr{proxyPublicAddr}
		}),
	)
	authAddr, err := process.AuthAddr()
	require.NoError(t, err)
	s.authServer = process.GetAuthServer()

	// create admin role and user.
	username := "admin"
	adminRole, err := types.NewRole(username, types.RoleSpecV6{
		Allow: types.RoleConditions{
			GroupLabels: types.Labels{types.Wildcard: apiutils.Strings{types.Wildcard}},
			Rules: []types.Rule{
				{
					Resources: []string{types.Wildcard},
					Verbs:     []string{types.Wildcard},
				},
			},
		},
	})
	require.NoError(t, err)
	adminRole, err = s.authServer.CreateRole(ctx, adminRole)
	require.NoError(t, err)

	user, err := types.NewUser(username)
	user.SetRoles([]string{adminRole.GetName()})
	require.NoError(t, err)
	_, err = s.authServer.CreateUser(ctx, user)
	require.NoError(t, err)

	mockWebauthnLogin := setupWebAuthn(t, s.authServer, username)
	mockMFAPromptConstructor := func(opts ...mfa.PromptOpt) mfa.Prompt {
		promptCfg := libmfa.NewPromptConfig(proxyPublicAddr.String(), opts...)
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
		"--proxy", proxyPublicAddr.String(),
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

	s.userClientNoMFA, err = auth.NewClient(client.Config{
		Addrs: []string{authAddr.String()},
		Credentials: []client.Credentials{
			client.LoadProfile(tshHome, ""),
		},
	})
	require.NoError(t, err)

	s.userClientWithMFA, err = auth.NewClient(client.Config{
		Addrs: []string{authAddr.String()},
		Credentials: []client.Credentials{
			client.LoadProfile(tshHome, ""),
		},
		MFAPromptConstructor: mockMFAPromptConstructor,
	})
	require.NoError(t, err)
}

type adminActionTestCase struct {
	command    string
	cliCommand tctl.CLICommand
	setup      func() error
	cleanup    func() error
}

func (s *AdminActionTestSuite) testCommand(t *testing.T, ctx context.Context, tc adminActionTestCase) {
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

	const origin = "https://localhost"
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
