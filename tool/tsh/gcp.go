// Copyright 2022 Gravitational, Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	gcloudCLIBinaryName = "gcloud"
)

func onGcloud(cf *CLIConf) error {
	app, err := pickActiveGCPApp(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = app.StartLocalProxies()
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := app.Close(); err != nil {
			log.WithError(err).Error("Failed to close GCP app.")
		}
	}()

	args := cf.GCPCommandArgs

	cmd := exec.Command(gcloudCLIBinaryName, args...)
	return app.RunCommand(cmd)
}

// gcpApp is an GCP app that can start local proxies to serve GCP APIs.
type gcpApp struct {
	cf      *CLIConf
	profile *client.ProfileStatus
	app     tlsca.RouteToApp
	secret  string

	localALPNProxy    *alpnproxy.LocalProxy
	localForwardProxy *alpnproxy.ForwardProxy
}

// newGCPApp creates a new GCP app.
func newGCPApp(cf *CLIConf, profile *client.ProfileStatus, app tlsca.RouteToApp) (*gcpApp, error) {
	secret, err := getGCPSecret()
	if err != nil {
		return nil, err
	}
	return &gcpApp{
		cf:      cf,
		profile: profile,
		app:     app,
		secret:  secret,
	}, nil
}

// getGCPSecret will return fresh secret to use or read it from environment.
func getGCPSecret() (string, error) {
	secret := os.Getenv(gcloudSecretEnvVar)
	if secret != "" {
		return secret, nil
	}

	return utils.CryptoRandomHex(auth.TokenLenBytes)
}

// StartLocalProxies sets up local proxies for serving GCP clients.
//
// At minimum clients should work with these variables set:
// - HTTPS_PROXY, for routing the traffic through the proxy
//
// The request flow to remote server (i.e. GCP APIs) looks like this:
// clients -> local forward proxy -> local ALPN proxy -> remote server
func (a *gcpApp) StartLocalProxies() error {
	// HTTPS proxy mode
	if err := a.startLocalALPNProxy(""); err != nil {
		return trace.Wrap(err)
	}
	if err := a.startLocalForwardProxy(a.cf.LocalProxyPort); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// Close makes all necessary close calls.
func (a *gcpApp) Close() error {
	var errs []error
	if a.localALPNProxy != nil {
		errs = append(errs, a.localALPNProxy.Close())
	}
	if a.localForwardProxy != nil {
		errs = append(errs, a.localForwardProxy.Close())
	}
	return trace.NewAggregate(errs...)
}

// GetEnvVars returns required environment variables to configure the
// clients.
func (a *gcpApp) GetEnvVars() (map[string]string, error) {
	envVars := map[string]string{
		// Env var CLOUDSDK_AUTH_ACCESS_TOKEN is one of the available ways of providing access token
		// https://cloud.google.com/sdk/docs/authorizing#:~:text=If%20you%20already,access%20token%20value.
		"CLOUDSDK_AUTH_ACCESS_TOKEN": a.secret,

		// Set core.custom_ca_certs_file via env variable, customizing the path to CA certs file.
		// https://cloud.google.com/sdk/gcloud/reference/config/set#:~:text=custom_ca_certs_file
		"CLOUDSDK_CORE_CUSTOM_CA_CERTS_FILE": a.profile.AppLocalCAPath(a.app.Name),
	}

	// Set proxy settings.
	if a.localForwardProxy != nil {
		envVars["HTTPS_PROXY"] = "http://" + a.localForwardProxy.GetAddr()
	}
	return envVars, nil
}

// RunCommand executes provided command.
func (a *gcpApp) RunCommand(cmd *exec.Cmd) error {
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
func (a *gcpApp) startLocalALPNProxy(port string) error {
	tc, err := makeClient(a.cf, false)
	if err != nil {
		return trace.Wrap(err)
	}

	localCA, err := loadAppSelfSignedCA(a.profile, tc, a.app.Name)
	if err != nil {
		return trace.Wrap(err)
	}

	appCerts, err := loadAppCertificate(tc, a.app.Name)
	if err != nil {
		return trace.Wrap(err)
	}

	address, err := utils.ParseAddr(tc.WebProxyAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	listenAddr := "localhost:0"
	if port != "" {
		listenAddr = fmt.Sprintf("localhost:%s", port)
	}

	// Create a listener that is able to sign certificates when receiving GCP
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
		Certs:              []tls.Certificate{appCerts},
		HTTPMiddleware: &alpnproxy.AuthorizationCheckerMiddleware{
			Secret: a.secret,
		},
	})

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
func (a *gcpApp) startLocalForwardProxy(port string) error {
	listenAddr := "localhost:0"
	if port != "" {
		listenAddr = fmt.Sprintf("localhost:%s", port)
	}

	// Note that the created forward proxy serves HTTP instead of HTTPS, to
	// eliminate the need to install temporary CA for various GCP clients.
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	a.localForwardProxy, err = alpnproxy.NewForwardProxy(alpnproxy.ForwardProxyConfig{
		Listener:     listener,
		CloseContext: a.cf.Context,
		Handlers: []alpnproxy.ConnectRequestHandler{
			// Forward GCP requests to ALPN proxy.
			alpnproxy.NewForwardToHostHandler(alpnproxy.ForwardToHostHandlerConfig{
				MatchFunc: alpnproxy.MatchGCPRequests,
				Host:      a.localALPNProxy.GetAddr(),
			}),

			// Forward non-GCP requests to user's system proxy, if configured.
			alpnproxy.NewForwardToSystemProxyHandler(alpnproxy.ForwardToSystemProxyHandlerConfig{
				InsecureSystemProxy: a.cf.InsecureSkipVerify,
			}),

			// Forward non-GCP requests to their original hosts.
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

func printGCPServiceAccounts(accounts []string) {
	fmt.Println(formatGCPServiceAccounts(accounts))
}

// SortedGCPServiceAccounts sorts service accounts by project and service account name.
type SortedGCPServiceAccounts []string

// Len returns the length of a list.
func (s SortedGCPServiceAccounts) Len() int {
	return len(s)
}

// Less compares items. Given two accounts, it first compares the project (i.e. what goes after @)
// and if they are equal proceeds to compare the service account name (what goes before @).
// Example of sorted list:
// - test-0@example-100200.iam.gserviceaccount.com
// - test-1@example-123456.iam.gserviceaccount.com
// - test-2@example-123456.iam.gserviceaccount.com
// - test-3@example-123456.iam.gserviceaccount.com
// - test-0@other-999999.iam.gserviceaccount.com
func (s SortedGCPServiceAccounts) Less(i, j int) bool {
	beforeI, afterI, _ := strings.Cut(s[i], "@")
	beforeJ, afterJ, _ := strings.Cut(s[j], "@")

	if afterI != afterJ {
		return afterI < afterJ
	}

	return beforeI < beforeJ
}

// Swap swaps two items in a list.
func (s SortedGCPServiceAccounts) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func formatGCPServiceAccounts(accounts []string) string {
	if len(accounts) == 0 {
		return ""
	}

	t := asciitable.MakeTable([]string{"Available GCP service accounts"})

	acc := SortedGCPServiceAccounts(accounts)
	sort.Sort(acc)

	for _, account := range acc {
		t.AddRow([]string{account})
	}

	return t.AsBuffer().String()
}

func getGCPServiceAccountFromFlags(cf *CLIConf, profile *client.ProfileStatus) (string, error) {
	accounts := profile.GCPServiceAccounts
	if len(accounts) == 0 {
		return "", trace.BadParameter("no GCP service accounts available, check your permissions")
	}

	reqAccount := cf.GCPServiceAccount

	// if flag is missing, try to find singleton service account; failing that, print available options.
	if reqAccount == "" {
		if len(accounts) == 1 {
			log.Infof("GCP service account %v is selected by default as it is the only one available for this GCP app.", accounts[0])
			return accounts[0], nil
		}

		// we will never have zero identities here: this is a pre-condition checked above.
		printGCPServiceAccounts(accounts)
		return "", trace.BadParameter("multiple GCP service accounts available, choose one with --gcp-service-account flag")
	}

	// exact match?
	for _, identity := range accounts {
		if identity == reqAccount {
			return identity, nil
		}
	}

	// prefix match?
	expectedPrefix := fmt.Sprintf("%v@", reqAccount)
	var matches []string
	for _, account := range accounts {
		if strings.HasPrefix(account, expectedPrefix) {
			matches = append(matches, account)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		printGCPServiceAccounts(accounts)
		return "", trace.NotFound("failed to find the service account matching %q", cf.GCPServiceAccount)
	default:
		printGCPServiceAccounts(matches)
		return "", trace.BadParameter("provided service account %q is ambiguous, please specify full service account name", cf.GCPServiceAccount)
	}
}

func pickActiveGCPApp(cf *CLIConf) (*gcpApp, error) {
	profile, err := cf.ProfileStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(profile.Apps) == 0 {
		return nil, trace.NotFound("Please login to a GCP App using 'tsh app login' first")
	}
	name := cf.AppName
	if name != "" {
		app, err := findApp(profile.Apps, name)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotFound("Please login to a GCP App using 'tsh app login' first")
			}
			return nil, trace.Wrap(err)
		}
		if app.GCPServiceAccount == "" {
			return nil, trace.BadParameter(
				"Selected app %q is not an GCP application", name,
			)
		}
		return newGCPApp(cf, profile, *app)
	}
	gcpApps := getGCPApps(profile.Apps)
	if len(gcpApps) == 0 {
		return nil, trace.NotFound("Please login to a GCP App using 'tsh app login' first")
	}
	if len(gcpApps) > 1 {
		var names []string
		for _, app := range gcpApps {
			names = append(names, app.Name)
		}
		return nil, trace.BadParameter(
			"Multiple GCP apps are available (%v), please specify one using --app CLI argument", strings.Join(names, ", "),
		)
	}
	return newGCPApp(cf, profile, gcpApps[0])
}

func getGCPApps(apps []tlsca.RouteToApp) []tlsca.RouteToApp {
	var out []tlsca.RouteToApp
	for _, app := range apps {
		if app.GCPServiceAccount != "" {
			out = append(out, app)
		}
	}
	return out
}
