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
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	awsCLIBinaryName = "aws"
)

func onAWS(cf *CLIConf) error {
	awsApp, err := pickAWSApp(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = awsApp.StartLocalProxies(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := awsApp.Close(); err != nil {
			logger.ErrorContext(cf.Context, "Failed to close AWS app", "error", err)
		}
	}()

	args := cf.AWSCommandArgs
	if cf.AWSEndpointURLMode {
		args = append(args, "--endpoint-url", awsApp.GetEndpointURL())
	}

	commandToRun := awsCLIBinaryName
	if cf.Exec != "" {
		commandToRun = cf.Exec
	}

	cmd := exec.Command(commandToRun, args...)
	return awsApp.RunCommand(cmd)
}

// awsApp is an AWS app that can start local proxies to serve AWS APIs.
type awsApp struct {
	*localProxyApp

	cf *CLIConf

	credentials     aws.CredentialsProvider
	credentialsOnce sync.Once
}

// newAWSApp creates a new AWS app.
func newAWSApp(tc *client.TeleportClient, cf *CLIConf, appInfo *appInfo) (*awsApp, error) {
	localProxyApp, err := newLocalProxyApp(tc, appInfo.profile, appInfo.RouteToApp, cf.LocalProxyPort, cf.InsecureSkipVerify)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &awsApp{
		localProxyApp: localProxyApp,
		cf:            cf,
	}, nil
}

// GetAppName returns the app name.
func (a *awsApp) GetAppName() string {
	return a.routeToApp.Name
}

// StartLocalProxies sets up local proxies for serving AWS clients.
//
// There are two ways clients can connect to the local proxies.
//
// 1. client can send AWS requests to our local forward proxy by configuring
// HTTPS_PROXY (or equivalent). The API flow looks like this:
// clients -> local forward proxy -> local ALPN proxy -> remote server
//
// 2. client can send AWS requests to our local ALPN proxy directly by
// configuring AWS endpoint URLs. The API flow looks like this.
// clients -> local ALPN proxy -> remote server
//
// The first method is always preferred as the original hostname is preserved
// through forward proxy.
func (a *awsApp) StartLocalProxies(ctx context.Context, opts ...alpnproxy.LocalProxyConfigOpt) error {
	awsMiddleware := &alpnproxy.AWSAccessMiddleware{
		AWSCredentialsProvider: a.GetAWSCredentialsProvider(),
	}

	// AWS endpoint URL mode
	if a.cf.AWSEndpointURLMode {
		err := a.StartLocalProxyWithTLS(ctx, alpnproxy.WithHTTPMiddleware(awsMiddleware))
		return trace.Wrap(err)
	}

	// HTTPS proxy mode
	err := a.StartLocalProxyWithForwarder(ctx, alpnproxy.MatchAWSRequests, alpnproxy.WithHTTPMiddleware(awsMiddleware))
	return trace.Wrap(err)
}

// GetAWSCredentialsProvider returns an [aws.CredentialsProvider] that generates
// fake AWS credentials that are used for signing an AWS request during AWS API
// calls and verified on local AWS proxy side.
func (a *awsApp) GetAWSCredentialsProvider() aws.CredentialsProvider {
	// There is no specific format or value required for access key and secret,
	// as long as the AWS clients and the local proxy are using the same
	// credentials. The only constraint is the access key must have a length
	// between 16 and 128. AWS access key and secret typically have size of 20
	// and 40 respectively. New UUIDs are generated for each tsh command.
	//
	// https://docs.aws.amazon.com/STS/latest/APIReference/API_Credentials.html
	a.credentialsOnce.Do(func() {
		a.credentials = credentials.NewStaticCredentialsProvider(
			getEnvOrDefault(awsAccessKeyIDEnvVar, uuid.NewString()),
			getEnvOrDefault(awsSecretAccessKeyEnvVar, uuid.NewString()),
			"",
		)
	})
	return a.credentials
}

// GetEnvVars returns required environment variables to configure the
// clients.
func (a *awsApp) GetEnvVars() (map[string]string, error) {
	if a.localALPNProxy == nil {
		return nil, trace.NotFound("ALPN proxy is not running")
	}

	cred, err := a.GetAWSCredentialsProvider().Retrieve(context.Background())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	envVars := map[string]string{
		// AWS CLI and SDKs can load credentials through environment variables.
		//
		// https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html
		"AWS_ACCESS_KEY_ID":     cred.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY": cred.SecretAccessKey,
		"AWS_CA_BUNDLE":         a.profile.AppLocalCAPath(a.cf.SiteName, a.routeToApp.Name),
	}

	// Set proxy settings.
	if a.localForwardProxy != nil {
		envVars["HTTPS_PROXY"] = "http://" + a.localForwardProxy.GetAddr()
		envVars["https_proxy"] = "http://" + a.localForwardProxy.GetAddr()
	}
	return envVars, nil
}

// GetForwardProxyAddr returns local forward proxy address.
func (a *awsApp) GetForwardProxyAddr() string {
	if a.localForwardProxy != nil {
		return a.localForwardProxy.GetAddr()
	}
	return ""
}

// GetEndpointURL returns AWS endpoint URL that clients can use.
func (a *awsApp) GetEndpointURL() string {
	if a.localALPNProxy != nil {
		return "https://" + a.localALPNProxy.GetAddr()
	}
	return ""
}

// RunCommand executes provided command.
func (a *awsApp) RunCommand(cmd *exec.Cmd) error {
	environmentVariables, err := a.GetEnvVars()
	if err != nil {
		return trace.Wrap(err)
	}

	logger.DebugContext(a.cf.Context, "Running AWS command", "command", logutils.StringerAttr(cmd))

	cmd.Stdout = a.cf.Stdout()
	cmd.Stderr = a.cf.Stderr()
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	for key, value := range environmentVariables {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	if err := a.cf.RunCommand(cmd); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func printAWSRoles(w io.Writer, roles awsutils.Roles) {
	if len(roles) == 0 {
		return
	}

	roles.Sort()
	t := asciitable.MakeTable([]string{"Role Name", "Role ARN"})
	for _, role := range roles {
		// Use role.Display for role names to match what AWS web console shows.
		t.AddRow([]string{role.Display, role.ARN})
	}

	fmt.Fprintln(w, "Available AWS roles:")
	fmt.Fprintln(w, t.AsBuffer().String())
}

func getARNFromFlags(cf *CLIConf, app types.Application, logins []string) (string, error) {
	// Filter AWS roles by AWS account ID. If AWS account ID is empty, all
	// roles are returned.
	roles := awsutils.FilterAWSRoles(logins, app.GetAWSAccountID())

	if cf.AWSRole == "" {
		if len(roles) == 1 {
			logger.InfoContext(cf.Context, "AWS Role is selected by default as it is the only role configured for this AWS app", "role", roles[0].Display)
			return roles[0].ARN, nil
		}

		printAWSRoles(cf.Stdout(), roles)
		return "", trace.BadParameter("--aws-role flag is required")
	}

	// Match by role ARN.
	if arn.IsARN(cf.AWSRole) {
		if role, found := roles.FindRoleByARN(cf.AWSRole); found {
			return role.ARN, nil
		}

		printAWSRoles(cf.Stdout(), roles)
		return "", trace.NotFound("failed to find the %q role ARN", cf.AWSRole)
	}

	// Match by role name.
	rolesMatched := roles.FindRolesByName(cf.AWSRole)
	switch len(rolesMatched) {
	case 1:
		return rolesMatched[0].ARN, nil
	case 0:
		printAWSRoles(cf.Stdout(), roles)
		return "", trace.NotFound("failed to find the %q role name", cf.AWSRole)
	default:
		// Print roles matched the provided role name.
		printAWSRoles(cf.Stdout(), rolesMatched)
		return "", trace.BadParameter("provided role name %q is ambiguous, please specify full role ARN", cf.AWSRole)
	}
}

// getARNFromRoles fetches the available AWS ARNs logins for given app.
// If any step of fetching the roles ARNs fail, fallback into returning the
// profile ARNs.
//
// TODO(gabrielcorado): DELETE IN V18.0.0
// This is here for backward compatibility in case the auth server
// does not support enriched resources yet.
func getARNFromRoles(cf *CLIConf, roleGetter services.CurrentUserRoleGetter, profile *client.ProfileStatus, siteName string, app types.Application) []string {
	accessChecker, err := services.NewAccessCheckerForRemoteCluster(cf.Context, profile.AccessInfo(), siteName, roleGetter)
	if err != nil {
		logger.DebugContext(cf.Context, "Failed to fetch user roles", "error", err)
		return profile.AWSRolesARNs
	}

	logins, err := accessChecker.GetAllowedLoginsForResource(app)
	if err != nil {
		logger.DebugContext(cf.Context, "Failed to fetch app logins", "error", err)
		return profile.AWSRolesARNs
	}

	return logins
}

func matchAWSApp(app tlsca.RouteToApp) bool {
	return app.AWSRoleARN != ""
}

func pickAWSApp(cf *CLIConf) (*awsApp, error) {
	tc, err := makeClient(cf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var appInfo *appInfo
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		var err error
		profile, err := tc.ProfileStatus()
		if err != nil {
			return trace.Wrap(err)
		}

		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		appInfo, err = getAppInfo(cf, clusterClient.AuthClient, profile, tc.SiteName, matchAWSApp)
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return newAWSApp(tc, cf, appInfo)
}
