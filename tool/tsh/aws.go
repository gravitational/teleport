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
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"

	awsarn "github.com/aws/aws-sdk-go/aws/arn"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
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

	cmd := exec.Command(awsCLIBinaryName, cf.AWSCommandArgs...)
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
	if err := a.startLocalALPNProxy(); err != nil {
		return trace.Wrap(err)
	}
	if err := a.startLocalForwardProxy(); err != nil {
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
	// between 16 and 128. Here access key and secert are generated based on
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

		md5sum := md5.Sum(hashData)
		md5Encoded := hex.EncodeToString(md5sum[:])
		a.credentials = credentials.NewStaticCredentials(md5Encoded[:16], md5Encoded[16:], "")
	})

	if a.credentials == nil {
		return nil, trace.BadParameter("missing credentials")
	}
	return a.credentials, nil
}

// GetEnvVars returns required environment variables to configure the
// clients.
func (a *awsApp) GetEnvVars() (map[string]string, error) {
	if a.localALPNProxy == nil || a.localForwardProxy == nil {
		return nil, trace.NotFound("local proxies are not running")
	}

	cred, err := a.GetAWSCredentials()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	credValues, err := cred.Get()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caPath := a.profile.AppLocalCAPath(a.appName)
	return map[string]string{
		// AWS CLI and SDKs can load credentials through environment variables.
		//
		// https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html
		"AWS_ACCESS_KEY_ID":     credValues.AccessKeyID,
		"AWS_SECRET_ACCESS_KEY": credValues.SecretAccessKey,
		"AWS_CA_BUNDLE":         caPath,

		// Set proxy settings.
		"HTTPS_PROXY": "http://" + a.localForwardProxy.GetAddr(),
		"https_proxy": "http://" + a.localForwardProxy.GetAddr(),
	}, nil
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

	cmd.Stdout = a.cf.Stdout()
	cmd.Stderr = a.cf.Stderr()
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	for key, value := range environmentVariables {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	if err := cmd.Run(); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// startLocalALPNProxy starts the local ALPN proxy.
func (a *awsApp) startLocalALPNProxy() error {
	tc, err := makeClient(a.cf, false)
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
	if a.cf.AWSEndpointURLPort != "" {
		listenAddr = fmt.Sprintf("localhost:%s", a.cf.AWSEndpointURLPort)
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
func (a *awsApp) startLocalForwardProxy() error {
	listenAddr := "localhost:0"
	if a.cf.LocalProxyPort != "" {
		listenAddr = fmt.Sprintf("localhost:%s", a.cf.LocalProxyPort)
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

func printArrayAs(arr []string, columnName string) {
	sort.Strings(arr)
	if len(arr) == 0 {
		return
	}
	t := asciitable.MakeTable([]string{columnName})
	for _, v := range arr {
		t.AddRow([]string{v})
	}
	fmt.Println(t.AsBuffer().String())
}

func getARNFromFlags(cf *CLIConf, profile *client.ProfileStatus) (string, error) {
	if cf.AWSRole == "" {
		printArrayAs(profile.AWSRolesARNs, "Available Role ARNs")
		return "", trace.BadParameter("--aws-role flag is required")
	}
	for _, v := range profile.AWSRolesARNs {
		if v == cf.AWSRole {
			return v, nil
		}
	}

	roleNameToARN := make(map[string]string)
	for _, v := range profile.AWSRolesARNs {
		arn, err := awsarn.Parse(v)
		if err != nil {
			return "", trace.Wrap(err)
		}
		// Example of the ANR Resource: 'role/EC2FullAccess' or 'role/path/to/customrole'
		parts := strings.Split(arn.Resource, "/")
		if len(parts) < 1 || parts[0] != "role" {
			continue
		}
		roleName := strings.Join(parts[1:], "/")

		if val, ok := roleNameToARN[roleName]; ok && cf.AWSRole == roleName {
			return "", trace.BadParameter(
				"provided role name %q is ambiguous between %q and %q ARNs, please specify full role ARN",
				cf.AWSRole, val, arn.String())
		}
		roleNameToARN[roleName] = arn.String()
	}

	roleARN, ok := roleNameToARN[cf.AWSRole]
	if !ok {
		printArrayAs(profile.AWSRolesARNs, "Available Role ARNs")
		printArrayAs(mapKeysToSlice(roleNameToARN), "Available Role Names")
		inputType := "ARN"
		if !awsarn.IsARN(cf.AWSRole) {
			inputType = "name"
		}
		return "", trace.NotFound("failed to find the %q role %s", cf.AWSRole, inputType)
	}
	return roleARN, nil
}

func mapKeysToSlice(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func pickActiveAWSApp(cf *CLIConf) (*awsApp, error) {
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(profile.Apps) == 0 {
		return nil, trace.NotFound("Please login to AWS app using 'tsh app login' first")
	}
	name := cf.AppName
	if name != "" {
		app, err := findApp(profile.Apps, name)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotFound("Please login to AWS app using 'tsh app login' first")
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
		return nil, trace.NotFound("Please login to AWS App using 'tsh app login' first")
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
