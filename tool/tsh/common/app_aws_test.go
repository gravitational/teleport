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
	"crypto/tls"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/auth/mocku2f"
	"github.com/gravitational/teleport/lib/integrations/awsra/createsession"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
	"github.com/gravitational/trace"
)

func TestAWS(t *testing.T) {
	t.Parallel()

	tmpHomePath := t.TempDir()

	connector := mockConnector(t)
	user, awsRole := makeUserWithAWSRole(t)
	authProcess, err := testserver.NewTeleportProcess(
		t.TempDir(),
		testserver.WithBootstrap(connector, user, awsRole),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Apps.Enabled = true
			cfg.Apps.Apps = []servicecfg.App{
				{
					Name: "aws-app",
					URI:  constants.AWSConsoleURL,
				},
			}
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authProcess.Close())
		require.NoError(t, authProcess.Wait())
	})

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := authProcess.ProxyWebAddr()
	require.NoError(t, err)

	// Log into Teleport cluster.
	err = Run(context.Background(), []string{
		"login", "--insecure", "--debug", "--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, user, connector.GetName()))
	require.NoError(t, err)

	// Run "tsh aws". Use a custom "cmdRunner" instead of executing AWS CLI. We
	// don't want to try a real AWS request as it might get sent to AWS
	// eventually by the App Service.
	validateCmd := func(cmd *exec.Cmd) error {
		// Validate composed AWS CLI command.
		require.Len(t, cmd.Args, 7)
		require.Equal(t, []string{"aws", "s3", "ls", "--page-size", "100", "--endpoint-url"}, cmd.Args[:6])
		endpointURL := cmd.Args[6]

		// Validate AWS credentials are set.
		getEnvValue := func(key string) string {
			for _, env := range cmd.Env {
				if after, ok := strings.CutPrefix(env, key+"="); ok {
					return after
				}
			}
			return ""
		}
		require.NotEmpty(t, getEnvValue("AWS_ACCESS_KEY_ID"))
		require.NotEmpty(t, getEnvValue("AWS_SECRET_ACCESS_KEY"))

		// Validate the local proxy is serving the advertised CA.
		caPool, err := utils.NewCertPoolFromPath(getEnvValue("AWS_CA_BUNDLE"))
		require.NoError(t, err)

		conn, err := tls.Dial("tcp", strings.TrimPrefix(endpointURL, "https://"), &tls.Config{
			ServerName: "localhost",
			RootCAs:    caPool,
		})
		require.NoError(t, err)
		require.NoError(t, conn.Close())
		return nil
	}

	// Log into the "aws-app" app.
	err = Run(
		context.Background(),
		[]string{"app", "login", "--insecure", "--aws-role", "some-aws-role", "aws-app"},
		setHomePath(tmpHomePath),
	)
	require.NoError(t, err)
	err = Run(
		context.Background(),
		[]string{"aws", "--app", "aws-app", "--endpoint-url", "s3", "ls", "--page-size", "100"},
		setHomePath(tmpHomePath),
		setCmdRunner(validateCmd),
	)
	require.Error(t, err)

	// Log out from "aws-app" app. The app should be logged-in automatically as needed.
	err = Run(
		context.Background(),
		[]string{"app", "logout", "aws-app"},
		setHomePath(tmpHomePath),
	)
	require.NoError(t, err)
	err = Run(
		context.Background(),
		[]string{"aws", "--insecure", "--aws-role", "some-aws-role", "--app", "aws-app", "--endpoint-url", "s3", "ls", "--page-size", "100"},
		setHomePath(tmpHomePath),
		setCmdRunner(validateCmd),
	)
	require.Error(t, err)

	validateCmd = func(cmd *exec.Cmd) error {
		// Validate composed AWS CLI command.
		require.Len(t, cmd.Args, 2)
		require.Equal(t, []string{"terraform", "plan"}, cmd.Args[:2])

		return nil
	}
	err = Run(
		context.Background(),
		[]string{"aws", "--insecure", "--aws-role", "some-aws-role", "--app", "aws-app", "--exec", "terraform", "plan"},
		setHomePath(tmpHomePath),
		setCmdRunner(validateCmd),
	)
	require.NoError(t, err)
}

const (
	promptAlphaRoleARN = "arn:aws:iam::123456789012:role/Alpha"
	promptBetaRoleARN  = "arn:aws:iam::123456789012:role/Beta"
)

type interactiveLineReader struct {
	line string
}

func newInteractiveLineReader(line string) *interactiveLineReader {
	return &interactiveLineReader{line: line}
}

func (r *interactiveLineReader) Read(p []byte) (int, error) {
	if r.line == "" {
		return 0, io.EOF
	}

	n := copy(p, r.line)
	r.line = r.line[n:]
	return n, nil
}

func promptTestAWSApp(t *testing.T) types.Application {
	t.Helper()

	app, err := types.NewAppV3(types.Metadata{Name: "aws-test"}, types.AppSpecV3{
		URI: constants.AWSConsoleURL,
	})
	require.NoError(t, err)
	return app
}

func TestPromptRoleInteractive(t *testing.T) {
	t.Parallel()

	testRoleARNs := []string{
		promptAlphaRoleARN,
		promptBetaRoleARN,
	}

	roles := awsutils.FilterAWSRoles(testRoleARNs, "")
	require.Len(t, roles, 2)

	tests := []struct {
		name           string
		input          string
		wantARN        string
		wantOutputText []string
	}{
		{
			name:    "selects role",
			input:   "2\n",
			wantARN: promptBetaRoleARN,
			wantOutputText: []string{
				"Available AWS roles:",
				"Enter role number:",
			},
		},
		{
			name:    "trims whitespace",
			input:   " 1 \n",
			wantARN: promptAlphaRoleARN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout := new(bytes.Buffer)
			role, err := promtRole(newInteractiveLineReader(tt.input), stdout, roles)
			require.NoError(t, err)
			require.Equal(t, tt.wantARN, role.ARN)
			for _, text := range tt.wantOutputText {
				require.Contains(t, stdout.String(), text)
			}
		})
	}
}

func TestPromptRolesInteractive(t *testing.T) {
	t.Parallel()

	testRoleARNs := []string{
		promptAlphaRoleARN,
		promptBetaRoleARN,
	}

	roles := awsutils.FilterAWSRoles(testRoleARNs, "")
	require.Len(t, roles, 2)

	tests := []struct {
		name              string
		input             string
		roles             awsutils.Roles
		wantARN           string
		assertErr         func(*testing.T, error)
		wantOutputText    []string
		wantOutputCounts  map[string]int
		wantOutputIsEmpty bool
	}{
		{
			name:  "invalid input prints error",
			input: "bad\n",
			roles: roles,
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, io.EOF)
			},
			wantOutputText: []string{
				"invalid role number: bad",
			},
			wantOutputCounts: map[string]int{
				"Available AWS roles:": 2,
				"Enter role number:":   2,
			},
		},
		{
			name:  "EOF",
			input: "",
			roles: roles,
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				require.ErrorIs(t, err, io.EOF)
			},
		},
		{
			name:  "no roles",
			input: "1\n",
			roles: []awsutils.Role{},
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
				require.True(t, trace.IsBadParameter(err))
			},
			wantOutputIsEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout := new(bytes.Buffer)
			role, err := promptRoles(newInteractiveLineReader(tt.input), stdout, tt.roles)
			if tt.assertErr != nil {
				tt.assertErr(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.wantARN, role.ARN)
			}

			output := stdout.String()
			if tt.wantOutputIsEmpty {
				require.Empty(t, output)
			}
			for _, text := range tt.wantOutputText {
				require.Contains(t, output, text)
			}
			for text, count := range tt.wantOutputCounts {
				require.Equal(t, count, strings.Count(output, text))
			}
		})
	}
}

func TestGetARNFromFlagsInteractive(t *testing.T) {
	t.Parallel()

	testRoleARNs := []string{
		promptAlphaRoleARN,
		promptBetaRoleARN,
	}

	tests := []struct {
		name        string
		input       string
		logins      []string
		wantARN     string
		wantAWSRole string
		wantPrompt  bool
	}{
		{
			name:        "selects prompted role",
			input:       "2\n",
			logins:      testRoleARNs,
			wantARN:     promptBetaRoleARN,
			wantAWSRole: promptBetaRoleARN,
			wantPrompt:  true,
		},
		{
			name:    "single role skips prompt",
			input:   "unexpected\n",
			logins:  []string{promptAlphaRoleARN},
			wantARN: promptAlphaRoleARN,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stdout := new(bytes.Buffer)
			cf := &CLIConf{
				Interactive:    true,
				OverrideStdout: stdout,
				overrideStdin:  newInteractiveLineReader(tt.input),
			}

			arn, err := getARNFromFlags(cf, promptTestAWSApp(t), tt.logins)
			require.NoError(t, err)
			require.Equal(t, tt.wantARN, arn)
			require.Equal(t, tt.wantAWSRole, cf.AWSRole)
			if tt.wantPrompt {
				require.Contains(t, stdout.String(), "Available AWS roles:")
				require.Contains(t, stdout.String(), "Enter role number:")
				return
			}
			require.NotContains(t, stdout.String(), "Available AWS roles:")
			require.NotContains(t, stdout.String(), "Enter role number:")
		})
	}
}

func TestAWSRolesAnywhereBasedAccess(t *testing.T) {
	ctx := context.Background()

	tmpHomePath := t.TempDir()

	awsConfigFile := filepath.Join(tmpHomePath, "aws_config")
	t.Setenv("AWS_CONFIG_FILE", awsConfigFile)

	connector := mockConnector(t)
	user, awsRole := makeUserWithAWSRole(t)
	authProcess, err := testserver.NewTeleportProcess(
		t.TempDir(),
		testserver.WithBootstrap(connector, user, awsRole),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authProcess.Close())
		require.NoError(t, authProcess.Wait())
	})

	expectedAWSCredentials := `{"Version":1,"AccessKeyId":"aki","SecretAccessKey":"sak","SessionToken":"st","Expiration":"2025-06-25T12:07:02.474135Z"}`
	authProcess.GetAuthServer().AWSRolesAnywhereCreateSessionOverride = func(ctx context.Context, req createsession.CreateSessionRequest) (*createsession.CreateSessionResponse, error) {
		return &createsession.CreateSessionResponse{
			Version:         1,
			AccessKeyID:     "aki",
			SecretAccessKey: "sak",
			SessionToken:    "st",
			Expiration:      "2025-06-25T12:07:02.474135Z",
		}, nil
	}

	integrationName := "aws-app"
	profileName := "aws-profile"
	integration, err := types.NewIntegrationAWSRA(
		types.Metadata{Name: integrationName},
		&types.AWSRAIntegrationSpecV1{
			TrustAnchorARN: "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
		},
	)
	require.NoError(t, err)
	_, err = authProcess.GetAuthServer().CreateIntegration(ctx, integration)
	require.NoError(t, err)

	awsAppUsingRolesAnywhere, err := types.NewAppServerV3(types.Metadata{
		Name: profileName,
	}, types.AppServerSpecV3{
		HostID: authProcess.GetID(),
		App: &types.AppV3{Metadata: types.Metadata{
			Name: profileName,
		}, Spec: types.AppSpecV3{
			URI:         constants.AWSConsoleURL,
			Integration: integrationName,
			AWS: &types.AppAWS{
				RolesAnywhereProfile: &types.AppAWSRolesAnywhereProfile{
					ProfileARN:            "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/12345678-1234-1234-1234-123456789012",
					AcceptRoleSessionName: true,
				},
			},
			PublicAddr: "example.com",
		}},
	})
	require.NoError(t, err)

	_, err = authProcess.GetAuthServer().UpsertApplicationServer(ctx, awsAppUsingRolesAnywhere)
	require.NoError(t, err)

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := authProcess.ProxyWebAddr()
	require.NoError(t, err)

	// Log into Teleport cluster.
	err = Run(ctx, []string{
		"login", "--insecure", "--debug", "--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, user, connector.GetName()))
	require.NoError(t, err)

	// Log into the "aws-profile" app.
	err = Run(
		ctx,
		[]string{"apps", "login", "--insecure", "--aws-role", "some-aws-role", profileName},
		setHomePath(tmpHomePath),
	)
	require.NoError(t, err)

	// check if external files were set correctly
	require.FileExists(t, awsConfigFile)

	// check if certificate file was written
	expectedCredentialFilePath := filepath.Join(tmpHomePath, "keys", proxyAddr.Host(), user.GetName()+"-app", "server01", profileName+".crt")
	require.FileExists(t, expectedCredentialFilePath)

	awsConfigContents, err := os.ReadFile(awsConfigFile)
	require.NoError(t, err)

	expectedProfileConfig := `; Do not edit. Section managed by Teleport.
[profile aws-profile]
credential_process=tsh apps config --format aws-credential-process aws-profile
`
	require.Equal(t, expectedProfileConfig, string(awsConfigContents))

	// Running the tsh apps config command should return the credentials
	appsConfigcommandOutput := &bytes.Buffer{}
	err = Run(
		ctx,
		[]string{"apps", "config", "--format", "aws-credential-process", profileName},
		setHomePath(tmpHomePath),
		setCopyStdout(appsConfigcommandOutput),
	)
	require.NoError(t, err)
	require.JSONEq(t, expectedAWSCredentials, appsConfigcommandOutput.String())

	// Profile is removed after logout.
	err = Run(
		ctx,
		[]string{"apps", "logout", "--insecure"},
		setHomePath(tmpHomePath),
	)
	require.NoError(t, err)

	awsConfigContents, err = os.ReadFile(awsConfigFile)
	require.NoError(t, err)
	require.Empty(t, awsConfigContents)
}

func TestAWSRolesAnywhereBasedAccess_usingMFA(t *testing.T) {
	tmpHomePath := t.TempDir()

	awsConfigFile := filepath.Join(tmpHomePath, "aws_config")
	t.Setenv("AWS_CONFIG_FILE", awsConfigFile)

	connector := mockConnector(t)

	user, awsRole := makeUserWithAWSRole(t)
	awsRoleOptions := awsRole.GetOptions()
	awsRoleOptions.RequireMFAType = types.RequireMFAType_SESSION
	awsRole.SetOptions(awsRoleOptions)

	authProcess, err := testserver.NewTeleportProcess(
		t.TempDir(),
		testserver.WithBootstrap(connector, user, awsRole),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, authProcess.Close())
		require.NoError(t, authProcess.Wait())
	})

	authServer := authProcess.GetAuthServer()
	proxyAddr, err := authProcess.ProxyWebAddr()
	require.NoError(t, err)

	// Set up MFA device for the user.
	origin := "https://127.0.0.1"
	device, err := mocku2f.Create()
	require.NoError(t, err)
	device.SetPasswordless()
	webauthnLoginOpt := setupWebAuthnChallengeSolver(device, true /* success */)

	_, err = authProcess.GetAuthServer().UpsertAuthPreference(t.Context(), &types.AuthPreferenceV2{
		Spec: types.AuthPreferenceSpecV2{
			SecondFactor: constants.SecondFactorOptional,
			Webauthn: &types.Webauthn{
				RPID: "127.0.0.1",
			},
			RequireMFAType: types.RequireMFAType_SESSION,
		},
	})
	require.NoError(t, err)
	registerDeviceForUser(t, authServer, device, user.GetName(), origin)

	authServer.AWSRolesAnywhereCreateSessionOverride = func(ctx context.Context, req createsession.CreateSessionRequest) (*createsession.CreateSessionResponse, error) {
		return &createsession.CreateSessionResponse{
			Version:         1,
			AccessKeyID:     "aki",
			SecretAccessKey: "sak",
			SessionToken:    "st",
			Expiration:      "2025-06-25T12:07:02.474135Z",
		}, nil
	}

	integrationName := "aws-app"
	profileName := "aws-profile"
	integration, err := types.NewIntegrationAWSRA(
		types.Metadata{Name: integrationName},
		&types.AWSRAIntegrationSpecV1{
			TrustAnchorARN: "arn:aws:rolesanywhere:eu-west-2:123456789012:trust-anchor/12345678-1234-1234-1234-123456789012",
		},
	)
	require.NoError(t, err)
	_, err = authProcess.GetAuthServer().CreateIntegration(t.Context(), integration)
	require.NoError(t, err)

	awsAppUsingRolesAnywhere, err := types.NewAppServerV3(types.Metadata{
		Name: profileName,
	}, types.AppServerSpecV3{
		HostID: authProcess.GetID(),
		App: &types.AppV3{Metadata: types.Metadata{
			Name: profileName,
		}, Spec: types.AppSpecV3{
			URI:         constants.AWSConsoleURL,
			Integration: integrationName,
			AWS: &types.AppAWS{
				RolesAnywhereProfile: &types.AppAWSRolesAnywhereProfile{
					ProfileARN:            "arn:aws:rolesanywhere:eu-west-2:123456789012:profile/12345678-1234-1234-1234-123456789012",
					AcceptRoleSessionName: true,
				},
			},
			PublicAddr: "example.com",
		}},
	})
	require.NoError(t, err)

	_, err = authServer.UpsertApplicationServer(t.Context(), awsAppUsingRolesAnywhere)
	require.NoError(t, err)

	// Log into Teleport cluster.
	err = Run(t.Context(), []string{
		"login", "--insecure", "--debug", "--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, user, connector.GetName()))
	require.NoError(t, err)

	// Log into the "aws-profile" app.
	err = Run(
		t.Context(),
		[]string{"apps", "login", "--insecure", "--aws-role", "some-aws-role", profileName},
		setHomePath(tmpHomePath),
		webauthnLoginOpt,
	)
	require.ErrorContains(t, err, "AWS access is configured to use per-session MFA")

	// Log in again but now use the `--env` flag to export the credentials to the shell.
	output := &bytes.Buffer{}
	err = Run(
		t.Context(),
		[]string{"apps", "login", "--insecure", "--aws-role", "some-aws-role", profileName, "--env"},
		setHomePath(tmpHomePath),
		setOverrideStdout(output),
		webauthnLoginOpt,
	)
	require.NoError(t, err)

	require.Equal(t, `export AWS_ACCESS_KEY_ID=aki
export AWS_SECRET_ACCESS_KEY=sak
export AWS_SESSION_TOKEN=st
# Export the above variables in your current shell to start using the AWS credentials.
`, output.String())

	// Verify that the AWS config file was not created.
	require.NoFileExists(t, awsConfigFile)

	// Verify that the certificate file was not created.
	expectedCredentialFilePath := filepath.Join(tmpHomePath, "keys", proxyAddr.Host(), user.GetName()+"-app", "server01", profileName+".crt")
	require.NoFileExists(t, expectedCredentialFilePath)
}

// TestAWSConsoleLogins given a AWS console application, execute a app login
// without proving a role ARN and verify the provided list of available logins
// is correct.
func TestAWSConsoleLogins(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tmpHomePath := t.TempDir()
	connector := mockConnector(t)

	userARNs := []string{"arn:aws:iam::111111111111:role/user-1", "arn:aws:iam::111111111111:role/user-2"}
	rootARNs := []string{"arn:aws:iam::111111111111:role/root-1", "arn:aws:iam::111111111111:role/root-2"}
	rootAWSRole, err := types.NewRole("aws", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels:   types.Labels{types.Wildcard: apiutils.Strings{types.Wildcard}},
			AWSRoleARNs: rootARNs,
		},
	})
	require.NoError(t, err)
	user, err := types.NewUser("alice@example.com")
	require.NoError(t, err)
	user.SetRoles([]string{"access", rootAWSRole.GetName()})
	user.SetAWSRoleARNs(userARNs)
	rootServer, err := testserver.NewTeleportProcess(
		t.TempDir(),
		testserver.WithClusterName("root"),
		testserver.WithBootstrap(connector, user, rootAWSRole),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.InsecureMode = true
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Apps.Enabled = true
			cfg.Apps.Apps = []servicecfg.App{
				{
					Name: "awsconsole",
					URI:  constants.AWSConsoleURL,
				},
			}
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, rootServer.Close())
		require.NoError(t, rootServer.Wait())
	})

	leafARNs := []string{"arn:aws:iam::999999999999:role/leaf-1", "arn:aws:iam::999999999999:role/leaf-2"}
	leafAWSRole, err := types.NewRole("aws", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels:   types.Labels{types.Wildcard: apiutils.Strings{types.Wildcard}},
			AWSRoleARNs: leafARNs,
		},
	})
	require.NoError(t, err)
	leafServer, err := testserver.NewTeleportProcess(
		t.TempDir(),
		testserver.WithClusterName("leaf"),
		testserver.WithBootstrap(leafAWSRole),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
			cfg.InsecureMode = true
			cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
			cfg.Apps.Enabled = true
			cfg.Apps.Apps = []servicecfg.App{
				{
					Name: "awsconsole",
					URI:  constants.AWSConsoleURL,
				},
			}
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, leafServer.Close())
		require.NoError(t, leafServer.Wait())
	})
	SetupTrustedCluster(ctx, t, rootServer, leafServer, types.RoleMapping{Remote: "aws", Local: []string{"aws"}})

	authServer := rootServer.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)

	// Log into Teleport cluster.
	err = Run(context.Background(), []string{
		"login", "--insecure", "--debug", "--proxy", proxyAddr.String(),
	}, setHomePath(tmpHomePath), setMockSSOLogin(authServer, user, connector.GetName()))
	require.NoError(t, err)

	for cluster, expectedARNs := range map[string][]string{
		"root": append(userARNs, rootARNs...),
		"leaf": append(leafARNs, append(userARNs, rootARNs...)...),
	} {
		t.Run(cluster, func(t *testing.T) {
			commandOutput := new(bytes.Buffer)
			// Don't provide the `--aws-role`. We expect a failure since there
			// are multiple ARN roles.
			err := Run(
				context.Background(),
				[]string{"app", "login", "--insecure", "--cluster", cluster, "awsconsole"},
				setCopyStdout(commandOutput), setHomePath(tmpHomePath),
				// TODO(gabrielcorado): Given the `RetryWithRerlLogin` is going
				//   to perform a relogin for BadParameter error, we need to
				//   provide login mock here. Once the function is fixed and
				//   only retry `Retry` errors, this can be removed.
				setMockSSOLogin(authServer, user, connector.GetName()),
			)
			require.ErrorContains(t, err, "--aws-role flag is required")
			require.Regexp(t, strings.Join(expectedARNs, "|"), commandOutput.String(), "mismatch on expected roles")
		})
	}
}

func makeUserWithAWSRole(t *testing.T) (types.User, types.Role) {
	alice, err := types.NewUser("alice@example.com")
	require.NoError(t, err)

	awsRole, err := types.NewRole("aws", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels: types.Labels{
				types.Wildcard: apiutils.Strings{types.Wildcard},
			},
			AWSRoleARNs: []string{
				"arn:aws:iam::123456789012:role/some-aws-role",
				"arn:aws:iam::123456789012:role/some-other-aws-role",
			},
		},
	})
	require.NoError(t, err)

	alice.SetRoles([]string{"access", awsRole.GetName()})
	return alice, awsRole
}

func SetupTrustedCluster(ctx context.Context, t *testing.T, rootServer, leafServer *service.TeleportProcess, additionalRoleMappings ...types.RoleMapping) {
	rootProxyAddr, err := rootServer.ProxyWebAddr()
	require.NoError(t, err)
	rootProxyTunnelAddr, err := rootServer.ProxyTunnelAddr()
	require.NoError(t, err)

	tc, err := types.NewTrustedCluster(rootServer.Config.Auth.ClusterName.GetClusterName(), types.TrustedClusterSpecV2{
		Enabled:              true,
		Token:                testserver.StaticToken,
		ProxyAddress:         rootProxyAddr.String(),
		ReverseTunnelAddress: rootProxyTunnelAddr.String(),
		RoleMap: append(additionalRoleMappings,
			types.RoleMapping{
				Remote: "access",
				Local:  []string{"access"},
			},
		),
	})
	require.NoError(t, err)

	_, err = leafServer.GetAuthServer().UpsertTrustedClusterV2(ctx, tc)
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		rt, err := rootServer.GetAuthServer().GetTunnelConnections(ctx, leafServer.Config.Auth.ClusterName.GetClusterName())
		assert.NoError(t, err)
		assert.Len(t, rt, 1)
	}, time.Second*10, time.Second)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		rts, err := rootServer.GetAuthServer().GetRemoteClusters(ctx)
		require.NoError(t, err)
		require.Len(t, rts, 1)
	}, time.Second*10, time.Second)

	tsrv, err := rootServer.GetReverseTunnelServer()
	require.NoError(t, err)
	require.EventuallyWithT(t, func(t *assert.CollectT) {
		rts, err := tsrv.Cluster(ctx, leafServer.Config.Auth.ClusterName.GetClusterName())
		require.NoError(t, err)
		require.NotNil(t, rts)

		require.Equal(t, 1, rts.GetTunnelsCount())
	}, time.Second*10, time.Second)
}
