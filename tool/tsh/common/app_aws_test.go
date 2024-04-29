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
	"context"
	"crypto/tls"
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	apiutils "github.com/gravitational/teleport/api/utils"
	"github.com/gravitational/teleport/lib"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
)

func TestAWS(t *testing.T) {
	t.Parallel()

	tmpHomePath := t.TempDir()

	connector := mockConnector(t)
	user, awsRole := makeUserWithAWSRole(t)

	authProcess, proxyProcess := makeTestServers(t, withBootstrap(connector, user, awsRole))
	makeTestApplicationServer(t, proxyProcess, servicecfg.App{
		Name: "aws-app",
		URI:  constants.AWSConsoleURL,
	})

	authServer := authProcess.GetAuthServer()
	require.NotNil(t, authServer)

	proxyAddr, err := proxyProcess.ProxyWebAddr()
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
		[]string{"app", "login", "aws-app"},
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
		[]string{"aws", "--app", "aws-app", "--endpoint-url", "s3", "ls", "--page-size", "100"},
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

	t.Run("aws ssm start-session", func(t *testing.T) {
		// Validate --endpoint-url 127.0.0.1:<port> is added to the command.
		validateCmd := func(cmd *exec.Cmd) error {
			require.Len(t, cmd.Args, 9)
			require.Equal(t, []string{"aws", "ssm", "--region", "us-west-1", "start-session", "--target", "target-id", "--endpoint-url"}, cmd.Args[:8])
			require.Contains(t, cmd.Args[8], "127.0.0.1:")
			return nil
		}
		err = Run(
			context.Background(),
			[]string{"aws", "ssm", "--region", "us-west-1", "start-session", "--target", "target-id"},
			setHomePath(tmpHomePath),
			setCmdRunner(validateCmd),
		)
		require.NoError(t, err)
	})
	t.Run("aws ecs execute-command", func(t *testing.T) {
		// Validate --endpoint-url 127.0.0.1:<port> is added to the command.
		validateCmd := func(cmd *exec.Cmd) error {
			require.Len(t, cmd.Args, 13)
			require.Equal(t, []string{"aws", "ecs", "execute-command", "--debug", "--cluster", "cluster-name", "--task", "task-name", "--command", "/bin/bash", "--interactive", "--endpoint-url"}, cmd.Args[:12])
			require.Contains(t, cmd.Args[12], "127.0.0.1:")
			return nil
		}
		err = Run(
			context.Background(),
			[]string{"aws", "ecs", "execute-command", "--debug", "--cluster", "cluster-name", "--task", "task-name", "--command", "/bin/bash", "--interactive"},
			setHomePath(tmpHomePath),
			setCmdRunner(validateCmd),
		)
		require.NoError(t, err)
	})
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
			},
		},
	})
	require.NoError(t, err)

	alice.SetRoles([]string{"access", awsRole.GetName()})
	return alice, awsRole
}

// deprecated: Use `tools/teleport/testenv.MakeTestServer` instead.
func makeTestApplicationServer(t *testing.T, proxy *service.TeleportProcess, apps ...servicecfg.App) *service.TeleportProcess {
	// Proxy uses self-signed certificates in tests.
	lib.SetInsecureDevMode(true)

	cfg := servicecfg.MakeDefaultConfig()
	cfg.Hostname = "localhost"
	cfg.DataDir = t.TempDir()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	proxyAddr, err := proxy.ProxyWebAddr()
	require.NoError(t, err)

	cfg.SetAuthServerAddress(*proxyAddr)

	token, err := proxy.Config.Token()
	require.NoError(t, err)

	cfg.SetToken(token)
	cfg.SSH.Enabled = false
	cfg.Auth.Enabled = false
	cfg.Proxy.Enabled = false
	cfg.Apps.Enabled = true
	cfg.Apps.Apps = apps
	cfg.Log = utils.NewLoggerForTests()

	return runTeleport(t, cfg)
}
