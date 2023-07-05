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
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	awsarn "github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
)

const (
	awsCLIBinaryName = "aws"
)

func onAWS(cf *CLIConf) error {
	awsApp, err := pickActiveAWSApp(cf)
	if err != nil {
		return trace.Wrap(err)
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

	cmd := exec.Command(awsCLIBinaryName, args...)
	return awsApp.RunCommand(cmd)
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
func newAWSApp(cf *CLIConf, profile *client.ProfileStatus, appName string) (*awsApp, error) {
	return &awsApp{
		cf:      cf,
		profile: profile,
		appName: appName,
	}, nil
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
	// between 16 and 128. Here access key and secret are generated based on
	// current profile and app name so the same values can be recreated.
	//
	// https://docs.aws.amazon.com/STS/latest/APIReference/API_Credentials.html
	a.credentialsOnce.Do(func() {
		keyPem, err := utils.ReadPath(a.profile.KeyPath())
		if err != nil {
			log.WithError(err).Errorf("Failed to read key.")
			return
		}

		hashData := append(
			keyPem,
			[]byte(a.profile.Name+a.profile.Username+a.appName)...,
		)

		// AWS access key and secret typically have size of 20 and 40
		// respectively.
		sum := sha256.Sum256(hashData)
		sumEncoded := hex.EncodeToString(sum[:])
		if len(sumEncoded) > 60 {
			a.credentials = credentials.NewStaticCredentials(sumEncoded[:20], sumEncoded[20:60], "")
		}
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
		"AWS_CA_BUNDLE":         a.profile.AppLocalCAPath(a.appName),
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

	localCA, err := loadAppSelfSignedCA(a.profile, tc, a.appName)
	if err != nil {
		return trace.Wrap(err)
	}

	appCerts, err := loadAppCertificate(tc, a.appName)
	if err != nil {
		return trace.Wrap(err)
	}

	address, err := utils.ParseAddr(tc.WebProxyAddr)
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

	a.localALPNProxy, err = alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		Listener:           listener,
		RemoteProxyAddr:    tc.WebProxyAddr,
		Protocols:          []alpncommon.Protocol{alpncommon.ProtocolHTTP},
		InsecureSkipVerify: a.cf.InsecureSkipVerify,
		ParentContext:      a.cf.Context,
		SNI:                address.Host(),
		AWSCredentials:     cred,
		Certs:              []tls.Certificate{appCerts},
	})
	if err != nil {
		if cerr := listener.Close(); cerr != nil {
			return trace.NewAggregate(err, cerr)
		}
		return trace.Wrap(err)
	}

	go func() {
		if err := a.localALPNProxy.StartAWSAccessProxy(a.cf.Context); err != nil {
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

func pickActiveAWSApp(cf *CLIConf) (*awsApp, error) {
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(profile.Apps) == 0 {
		return nil, trace.NotFound("Please login to AWS app using 'tsh apps login' first")
	}
	name := cf.AppName
	if name != "" {
		app, err := findApp(profile.Apps, name)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotFound("Please login to AWS app using 'tsh apps login' first")
			}
			return nil, trace.Wrap(err)
		}
		if app.AWSRoleARN == "" {
			return nil, trace.BadParameter(
				"Selected app %q is not an AWS application", name,
			)
		}
		return newAWSApp(cf, profile, name)
	}

	awsApps := getAWSAppsName(profile.Apps)
	if len(awsApps) == 0 {
		return nil, trace.NotFound("Please login to AWS App using 'tsh apps login' first")
	}
	if len(awsApps) > 1 {
		names := strings.Join(awsApps, ", ")
		return nil, trace.BadParameter(
			"Multiple AWS apps are available (%v), please specify one using --app CLI argument", names,
		)
	}
	return newAWSApp(cf, profile, awsApps[0])
}

func findApp(apps []tlsca.RouteToApp, name string) (*tlsca.RouteToApp, error) {
	for _, app := range apps {
		if app.Name == name {
			return &app, nil
		}
	}
	return nil, trace.NotFound("failed to find app with %q name", name)
}

func getAWSAppsName(apps []tlsca.RouteToApp) []string {
	var out []string
	for _, app := range apps {
		if app.AWSRoleARN != "" {
			out = append(out, app.Name)
		}
	}
	return out
}
