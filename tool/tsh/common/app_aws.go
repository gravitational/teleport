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
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	awsarn "github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

const (
	awsCLIBinaryName = "aws"
)

func onAWS(cf *CLIConf) error {
	awsApp, err := pickAWSApp(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	if shouldUseAWSEndpointURLMode(cf) {
		log.Debugf("Forcing endpoint URL mode for AWS command %q.", cf.AWSCommandArgs)
		cf.AWSEndpointURLMode = true
	}

	err = awsApp.StartLocalProxies()
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := awsApp.Close(); err != nil {
			log.WithError(err).Error("Failed to close AWS app.")
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

func shouldUseAWSEndpointURLMode(cf *CLIConf) bool {
	inputAWSCommand := strings.Join(removeAWSCommandFlags(cf.AWSCommandArgs), " ")
	switch inputAWSCommand {
	// `aws ssm start-session` first calls ssm.<region>.amazonaws.com to get an
	// stream URL and an token. Then it makes a wss connection with the
	// provided token to the provided stream URL. The wss request currently
	// respects HTTPS_PROXY but does not respect local CA bundle we provided
	// thus causing a failure. Even if this is resolved one day, the wss send
	// the token through websocket data channel for authentication, instead of
	// sigv4, which likely we won't support.
	//
	// When using the endpoint URL mode, only the first request goes through
	// Teleport Proxy. The wss connection does not respect the endpoint URL and
	// goes to AWS directly (thus working fine).
	//
	// Reference:
	// https://github.com/aws/session-manager-plugin/
	//
	// "aws ecs execute-command" also start SSM sessions.
	case "ssm start-session", "ecs execute-command":
		return true
	default:
		return false
	}
}

func removeAWSCommandFlags(args []string) (ret []string) {
	for i := 0; i < len(args); i++ {
		switch {
		case isAWSFlag(args, i):
			// Skip next arg, if next arg is not a flag but a flag value.
			if !isAWSFlag(args, i+1) {
				i++
			}
			continue
		default:
			ret = append(ret, args[i])
		}
	}
	return
}

func isAWSFlag(args []string, i int) bool {
	if i >= len(args) {
		return false
	}
	return strings.HasPrefix(args[i], "--")
}

// awsApp is an AWS app that can start local proxies to serve AWS APIs.
type awsApp struct {
	cf      *CLIConf
	profile *client.ProfileStatus
	appName string

	localALPNProxy    *alpnproxy.LocalProxy
	localForwardProxy *alpnproxy.ForwardProxy
	credentials       *credentials.Credentials
	credentialsOnce   sync.Once
}

// newAWSApp creates a new AWS app.
func newAWSApp(cf *CLIConf, profile *client.ProfileStatus, route tlsca.RouteToApp) (*awsApp, error) {
	return &awsApp{
		cf:      cf,
		profile: profile,
		appName: route.Name,
	}, nil
}

// GetAppName returns the app name.
func (a *awsApp) GetAppName() string {
	return a.appName
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
func (a *awsApp) StartLocalProxies() error {
	// AWS endpoint URL mode
	if a.cf.AWSEndpointURLMode {
		if err := a.startLocalALPNProxy(a.cf.LocalProxyPort); err != nil {
			return trace.Wrap(err)
		}

		return nil
	}

	// HTTPS proxy mode
	if err := a.startLocalALPNProxy(""); err != nil {
		return trace.Wrap(err)
	}
	if err := a.startLocalForwardProxy(a.cf.LocalProxyPort); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// close makes all necessary close calls.
func (a *awsApp) Close() error {
	var errs []error
	if a.localALPNProxy != nil {
		errs = append(errs, a.localALPNProxy.Close())
	}
	if a.localForwardProxy != nil {
		errs = append(errs, a.localForwardProxy.Close())
	}
	return trace.NewAggregate(errs...)
}

// GetAWSCredentials generates fake AWS credentials that are used for
// signing an AWS request during AWS API calls and verified on local AWS proxy
// side.
func (a *awsApp) GetAWSCredentials() (*credentials.Credentials, error) {
	// There is no specific format or value required for access key and secret,
	// as long as the AWS clients and the local proxy are using the same
	// credentials. The only constraint is the access key must have a length
	// between 16 and 128. AWS access key and secret typically have size of 20
	// and 40 respectively. New UUIDs are generated for each tsh command.
	//
	// https://docs.aws.amazon.com/STS/latest/APIReference/API_Credentials.html
	a.credentialsOnce.Do(func() {
		a.credentials = credentials.NewStaticCredentials(
			getEnvOrDefault(awsAccessKeyIDEnvVar, uuid.NewString()),
			getEnvOrDefault(awsSecretAccessKeyEnvVar, uuid.NewString()),
			"",
		)
	})

	if a.credentials == nil {
		return nil, trace.BadParameter("missing credentials")
	}
	return a.credentials, nil
}

// GetEnvVars returns required environment variables to configure the
// clients.
func (a *awsApp) GetEnvVars() (map[string]string, error) {
	if a.localALPNProxy == nil {
		return nil, trace.NotFound("ALPN proxy is not running")
	}

	cred, err := a.GetAWSCredentials()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	credValues, err := cred.Get()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	envVars := map[string]string{
		// AWS CLI and SDKs can load credentials through environment variables.
		//
		// https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html
		"AWS_ACCESS_KEY_ID":     credValues.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY": credValues.SecretAccessKey,
		"AWS_CA_BUNDLE":         a.profile.AppLocalCAPath(a.cf.SiteName, a.appName),
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

	log.Debugf("Running command: %q", cmd)

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

// startLocalALPNProxy starts the local ALPN proxy.
func (a *awsApp) startLocalALPNProxy(port string) error {
	tc, err := makeClient(a.cf)
	if err != nil {
		return trace.Wrap(err)
	}

	appCert, err := loadAppCertificateWithAppLogin(a.cf, tc, a.appName)
	if err != nil {
		return trace.Wrap(err)
	}

	localCA, err := loadAppSelfSignedCA(a.profile, tc, a.appName)
	if err != nil {
		return trace.Wrap(err)
	}

	cred, err := a.GetAWSCredentials()
	if err != nil {
		return trace.Wrap(err)
	}

	listenAddr := "localhost:0"
	if port != "" {
		listenAddr = fmt.Sprintf("localhost:%s", port)
	}

	// Create a listener that is able to sign certificates when receiving AWS
	// requests tunneled from the local forward proxy.
	listener, err := alpnproxy.NewCertGenListener(alpnproxy.CertGenListenerConfig{
		ListenAddr: listenAddr,
		CA:         localCA,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	a.localALPNProxy, err = alpnproxy.NewLocalProxy(
		makeBasicLocalProxyConfig(a.cf, tc, listener),
		alpnproxy.WithClientCert(appCert),
		alpnproxy.WithClusterCAsIfConnUpgrade(a.cf.Context, tc.RootClusterCACertPool),
		alpnproxy.WithHTTPMiddleware(&alpnproxy.AWSAccessMiddleware{
			AWSCredentials: cred,
		}),
	)
	if err != nil {
		if cerr := listener.Close(); cerr != nil {
			return trace.NewAggregate(err, cerr)
		}
		return trace.Wrap(err)
	}

	go func() {
		if err := a.localALPNProxy.StartHTTPAccessProxy(a.cf.Context); err != nil {
			log.WithError(err).Errorf("Failed to start local ALPN proxy.")
		}
	}()
	return nil
}

// startLocalForwardProxy starts the local forward proxy.
func (a *awsApp) startLocalForwardProxy(port string) error {
	listenAddr := "localhost:0"
	if port != "" {
		listenAddr = fmt.Sprintf("localhost:%s", port)
	}

	// Note that the created forward proxy serves HTTP instead of HTTPS, to
	// eliminate the need to install temporary CA for various AWS clients.
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	a.localForwardProxy, err = alpnproxy.NewForwardProxy(alpnproxy.ForwardProxyConfig{
		Listener:     listener,
		CloseContext: a.cf.Context,
		Handlers: []alpnproxy.ConnectRequestHandler{
			// Forward AWS requests to ALPN proxy.
			alpnproxy.NewForwardToHostHandler(alpnproxy.ForwardToHostHandlerConfig{
				MatchFunc: alpnproxy.MatchAWSRequests,
				Host:      a.localALPNProxy.GetAddr(),
			}),

			// Forward non-AWS requests to user's system proxy, if configured.
			alpnproxy.NewForwardToSystemProxyHandler(alpnproxy.ForwardToSystemProxyHandlerConfig{
				InsecureSystemProxy: a.cf.InsecureSkipVerify,
			}),

			// Forward non-AWS requests to their original hosts.
			alpnproxy.NewForwardToOriginalHostHandler(),
		},
	})
	if err != nil {
		if cerr := listener.Close(); cerr != nil {
			return trace.NewAggregate(err, cerr)
		}
		return trace.Wrap(err)
	}

	go func() {
		if err := a.localForwardProxy.Start(); err != nil {
			log.WithError(err).Errorf("Failed to start local forward proxy.")
		}
	}()
	return nil
}

func printAWSRoles(roles awsutils.Roles) {
	if len(roles) == 0 {
		return
	}

	roles.Sort()
	t := asciitable.MakeTable([]string{"Role Name", "Role ARN"})
	for _, role := range roles {
		// Use role.Display for role names to match what AWS web console shows.
		t.AddRow([]string{role.Display, role.ARN})
	}

	fmt.Println("Available AWS roles:")
	fmt.Println(t.AsBuffer().String())
}

func getARNFromFlags(cf *CLIConf, profile *client.ProfileStatus, app types.Application) (string, error) {
	// Filter AWS roles by AWS account ID. If AWS account ID is empty, all
	// roles are returned.
	roles := awsutils.FilterAWSRoles(profile.AWSRolesARNs, app.GetAWSAccountID())

	if cf.AWSRole == "" {
		if len(roles) == 1 {
			log.Infof("AWS Role %v is selected by default as it is the only role configured for this AWS app.", roles[0].Display)
			return roles[0].ARN, nil
		}

		printAWSRoles(roles)
		return "", trace.BadParameter("--aws-role flag is required")
	}

	// Match by role ARN.
	if awsarn.IsARN(cf.AWSRole) {
		if role, found := roles.FindRoleByARN(cf.AWSRole); found {
			return role.ARN, nil
		}

		printAWSRoles(roles)
		return "", trace.NotFound("failed to find the %q role ARN", cf.AWSRole)
	}

	// Match by role name.
	rolesMatched := roles.FindRolesByName(cf.AWSRole)
	switch len(rolesMatched) {
	case 1:
		return rolesMatched[0].ARN, nil
	case 0:
		printAWSRoles(roles)
		return "", trace.NotFound("failed to find the %q role name", cf.AWSRole)
	default:
		// Print roles matched the provided role name.
		printAWSRoles(rolesMatched)
		return "", trace.BadParameter("provided role name %q is ambiguous, please specify full role ARN", cf.AWSRole)
	}
}

func matchAWSApp(app tlsca.RouteToApp) bool {
	return app.AWSRoleARN != ""
}

func pickAWSApp(cf *CLIConf) (*awsApp, error) {
	app, err := pickCloudApp(cf, types.CloudAWS, matchAWSApp, newAWSApp)
	return app, trace.Wrap(err)
}
