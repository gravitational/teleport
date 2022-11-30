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
	"path"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	azureCLIBinaryName = "az"
)

func onAzure(cf *CLIConf) error {
	app, err := pickActiveAzureApp(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = app.StartLocalProxies()
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := app.Close(); err != nil {
			log.WithError(err).Error("Failed to close Azure app.")
		}
	}()

	args := cf.AzureCommandArgs

	cmd := exec.Command(azureCLIBinaryName, args...)
	return app.RunCommand(cmd)
}

// azureApp is an Azure app that can start local proxies to serve Azure APIs.
type azureApp struct {
	cf      *CLIConf
	profile *client.ProfileStatus
	app     tlsca.RouteToApp

	localALPNProxy    *alpnproxy.LocalProxy
	localForwardProxy *alpnproxy.ForwardProxy
}

// newAzureApp creates a new Azure app.
func newAzureApp(cf *CLIConf, profile *client.ProfileStatus, app tlsca.RouteToApp) (*azureApp, error) {
	return &azureApp{
		cf:      cf,
		profile: profile,
		app:     app,
	}, nil
}

// StartLocalProxies sets up local proxies for serving Azure clients.
//
// There are two ways clients can connect to the local proxies.
//
// 1. client can send Azure requests to our local forward proxy by configuring
// HTTPS_PROXY (or equivalent). The API flow looks like this:
// clients -> local forward proxy -> local ALPN proxy -> remote server
//
// 2. client can send Azure requests to our local ALPN proxy directly by
// configuring Azure endpoint URLs. The API flow looks like this.
// clients -> local ALPN proxy -> remote server
//
// The first method is always preferred as the original hostname is preserved
// through forward proxy.
func (a *azureApp) StartLocalProxies() error {
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
func (a *azureApp) Close() error {
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
func (a *azureApp) GetEnvVars() (map[string]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	envVars := map[string]string{
		// set custom Azure home path; this helps with the scenario in which user runs
		// 1. `tsh az login` in one console
		// 2. `az ...` in another console
		// without custom config dir the second invocation will hang, attempting to connect to (inaccessible without configuration) MSI.
		"AZURE_CONFIG_DIR": path.Join(homeDir, ".azure-teleport-"+a.app.Name),
		// setting MSI_ENDPOINT instructs Azure CLI to make managed identity calls on this address.
		// the requests will be handled by tsh proxy.
		"MSI_ENDPOINT": "https://" + types.TeleportAzureMSIEndpoint,

		// Needed for az cli to accept certs.
		"REQUESTS_CA_BUNDLE": a.profile.AppLocalCAPath(a.app.Name),
	}

	// Set proxy settings.
	if a.localForwardProxy != nil {
		envVars["HTTPS_PROXY"] = "http://" + a.localForwardProxy.GetAddr()
		envVars["HTTP_PROXY"] = "http://" + a.localForwardProxy.GetAddr()
	}
	return envVars, nil
}

// GetForwardProxyAddr returns local forward proxy address.
func (a *azureApp) GetForwardProxyAddr() string {
	if a.localForwardProxy != nil {
		return a.localForwardProxy.GetAddr()
	}
	return ""
}

// GetEndpointURL returns Azure endpoint URL that clients can use.
func (a *azureApp) GetEndpointURL() string {
	if a.localALPNProxy != nil {
		return "https://" + a.localALPNProxy.GetAddr()
	}
	return ""
}

// RunCommand executes provided command.
func (a *azureApp) RunCommand(cmd *exec.Cmd) error {
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
func (a *azureApp) startLocalALPNProxy(port string) error {
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

	// Create a listener that is able to sign certificates when receiving Azure
	// requests tunneled from the local forward proxy.
	listener, err := alpnproxy.NewCertGenListener(alpnproxy.CertGenListenerConfig{
		ListenAddr: listenAddr,
		CA:         localCA,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// backend expects the tokens to be signed with web session private key
	ws, err := tc.GetAppSession(a.cf.Context, types.GetAppSessionRequest{SessionID: a.app.SessionID})
	if err != nil {
		return err
	}

	wsPK, err := utils.ParsePrivateKey(ws.GetPriv())
	if err != nil {
		return err
	}

	a.localALPNProxy, err = alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		Listener:           listener,
		RemoteProxyAddr:    tc.WebProxyAddr,
		Protocols:          []alpncommon.Protocol{alpncommon.ProtocolHTTP},
		InsecureSkipVerify: a.cf.InsecureSkipVerify,
		ParentContext:      a.cf.Context,
		SNI:                address.Host(),
		Certs:              []tls.Certificate{appCerts},
		HTTPMiddleware: &alpnproxy.AzureMSIMiddleware{
			Key: wsPK,
			// we could, in principle, get the actual TenantID either from live data or from static configuration,
			// but at this moment there is no clear advantage over simply issuing a new random identifier.
			TenantID: uuid.New().String(),
			ClientID: uuid.New().String(),
			Identity: a.app.AzureIdentity,
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
func (a *azureApp) startLocalForwardProxy(port string) error {
	listenAddr := "localhost:0"
	if port != "" {
		listenAddr = fmt.Sprintf("localhost:%s", port)
	}

	// Note that the created forward proxy serves HTTP instead of HTTPS, to
	// eliminate the need to install temporary CA for various Azure clients.
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return trace.Wrap(err)
	}

	a.localForwardProxy, err = alpnproxy.NewForwardProxy(alpnproxy.ForwardProxyConfig{
		Listener:     listener,
		CloseContext: a.cf.Context,
		Handlers: []alpnproxy.ConnectRequestHandler{
			// Forward Azure requests to ALPN proxy.
			alpnproxy.NewForwardToHostHandler(alpnproxy.ForwardToHostHandlerConfig{
				MatchFunc: alpnproxy.MatchAzureRequests,
				Host:      a.localALPNProxy.GetAddr(),
			}),

			// Forward non-Azure requests to user's system proxy, if configured.
			alpnproxy.NewForwardToSystemProxyHandler(alpnproxy.ForwardToSystemProxyHandlerConfig{
				InsecureSystemProxy: a.cf.InsecureSkipVerify,
			}),

			// Forward non-Azure requests to their original hosts.
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

func printAzureIdentities(identities []string) {
	if len(identities) == 0 {
		return
	}

	sort.Strings(identities)

	t := asciitable.MakeTable([]string{"Available Azure identities"}, identities)
	fmt.Println(t.AsBuffer().String())
}

func getAzureIdentityFromFlags(cf *CLIConf, profile *client.ProfileStatus) (string, error) {
	identities := profile.AzureIdentities
	if len(identities) == 0 {
		return "", trace.BadParameter("no Azure identities available, check your permissions")
	}

	// if flag is missing, try to find singleton identity; failing that, print available options.
	if cf.AzureIdentity == "" {
		if len(identities) == 1 {
			log.Infof("Azure identity %v is selected by default as it is the only role configured for this azure app.", identities[0])
			return identities[0], nil
		}

		printAzureIdentities(identities)
		return "", trace.BadParameter("--azure-identity flag is required")
	}

	// exact match?
	for _, identity := range identities {
		if identity == cf.AzureIdentity {
			return identity, nil
		}
	}

	// suffix match?
	expectedSuffix := fmt.Sprintf("/Microsoft.ManagedIdentity/userAssignedIdentities/%v", cf.AzureIdentity)
	var matches []string
	for _, identity := range identities {
		if strings.HasSuffix(identity, expectedSuffix) {
			matches = append(matches, identity)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		printAzureIdentities(identities)
		return "", trace.NotFound("failed to find the identity matching %q", cf.AzureIdentity)
	default:
		printAzureIdentities(matches)
		return "", trace.BadParameter("provided identity %q is ambiguous, please specify full identity name", cf.AzureIdentity)
	}
}

func pickActiveAzureApp(cf *CLIConf) (*azureApp, error) {
	profile, err := client.StatusCurrent(cf.HomePath, cf.Proxy, cf.IdentityFileIn)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(profile.Apps) == 0 {
		return nil, trace.NotFound("Please login to Azure app using 'tsh app login' first")
	}
	name := cf.AppName
	if name != "" {
		app, err := findApp(profile.Apps, name)
		if err != nil {
			if trace.IsNotFound(err) {
				return nil, trace.NotFound("Please login to Azure app using 'tsh app login' first")
			}
			return nil, trace.Wrap(err)
		}
		if app.AzureIdentity == "" {
			return nil, trace.BadParameter(
				"Selected app %q is not an Azure application", name,
			)
		}
		return newAzureApp(cf, profile, *app)
	}
	azureApps := getAzureApps(profile.Apps)
	if len(azureApps) == 0 {
		return nil, trace.NotFound("Please login to Azure App using 'tsh app login' first")
	}
	if len(azureApps) > 1 {
		var names []string
		for _, app := range azureApps {
			names = append(names, app.Name)
		}
		return nil, trace.BadParameter(
			"Multiple Azure apps are available (%v), please specify one using --app CLI argument", strings.Join(names, ", "),
		)
	}
	return newAzureApp(cf, profile, azureApps[0])
}

func getAzureApps(apps []tlsca.RouteToApp) []tlsca.RouteToApp {
	var out []tlsca.RouteToApp
	for _, app := range apps {
		if app.AzureIdentity != "" {
			out = append(out, app)
		}
	}
	return out
}
