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
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	testserver "github.com/gravitational/teleport/tool/teleport/testenv"
)

func TestAWS(t *testing.T) {
	t.Parallel()

	tmpHomePath := t.TempDir()

	connector := mockConnector(t)
	user, awsRole := makeUserWithAWSRole(t)
	authProcess := testserver.MakeTestServer(
		t,
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
				if strings.HasPrefix(env, key+"=") {
					return strings.TrimPrefix(env, key+"=")
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
	require.NoError(t, err)

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
	require.NoError(t, err)

	validateCmd = func(cmd *exec.Cmd) error {
		// Validate composed AWS CLI command.
		require.Len(t, cmd.Args, 2)
		require.Equal(t, []string{"terraform", "plan"}, cmd.Args[:2])

		return nil
	}
	err = Run(
		context.Background(),
		[]string{"aws", "--app", "aws-app", "--exec", "terraform", "plan"},
		setHomePath(tmpHomePath),
		setCmdRunner(validateCmd),
	)
	require.NoError(t, err)
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
	rootServer := testserver.MakeTestServer(
		t,
		testserver.WithClusterName(t, "root"),
		testserver.WithBootstrap(connector, user, rootAWSRole),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
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

	leafARNs := []string{"arn:aws:iam::999999999999:role/leaf-1", "arn:aws:iam::999999999999:role/leaf-2"}
	leafAWSRole, err := types.NewRole("aws", types.RoleSpecV6{
		Allow: types.RoleConditions{
			AppLabels:   types.Labels{types.Wildcard: apiutils.Strings{types.Wildcard}},
			AWSRoleARNs: leafARNs,
		},
	})
	require.NoError(t, err)
	leafServer := testserver.MakeTestServer(
		t,
		testserver.WithClusterName(t, "leaf"),
		testserver.WithBootstrap(leafAWSRole),
		testserver.WithConfig(func(cfg *servicecfg.Config) {
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
	testserver.SetupTrustedCluster(ctx, t, rootServer, leafServer, types.RoleMapping{Remote: "aws", Local: []string{"aws"}})

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
