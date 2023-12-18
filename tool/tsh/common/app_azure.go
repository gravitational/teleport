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
	"path"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	azureCLIBinaryName = "az"
)

func onAzure(cf *CLIConf) error {
	app, err := pickAzureApp(cf)
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
	cf        *CLIConf
	profile   *client.ProfileStatus
	app       tlsca.RouteToApp
	msiSecret string

	localALPNProxy    *alpnproxy.LocalProxy
	localForwardProxy *alpnproxy.ForwardProxy
}

// newAzureApp creates a new Azure app.
func newAzureApp(cf *CLIConf, profile *client.ProfileStatus, app tlsca.RouteToApp) (*azureApp, error) {
	msiSecret, err := getMSISecret()
	if err != nil {
		return nil, err
	}
	return &azureApp{
		cf:        cf,
		profile:   profile,
		app:       app,
		msiSecret: msiSecret,
	}, nil
}

// getMSISecret will try to find the secret by parsing MSI_ENDPOINT env variable if present; it will return random hex string otherwise.
func getMSISecret() (string, error) {
	endpoint := os.Getenv("MSI_ENDPOINT")
	if endpoint == "" {
		randomHex, err := utils.CryptoRandomHex(10)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return randomHex, nil
	}

	expectedPrefix := "https://" + types.TeleportAzureMSIEndpoint + "/"
	if !strings.HasPrefix(endpoint, expectedPrefix) {
		return "", trace.BadParameter("MSI_ENDPOINT not empty, but doesn't start with %q as expected", expectedPrefix)
	}

	secret := strings.TrimPrefix(endpoint, expectedPrefix)
	if secret == "" {
		return "", trace.BadParameter("MSI secret cannot be empty")
	}
	return secret, nil
}

// StartLocalProxies sets up local proxies for serving Azure clients.
//
// At minimum clients should work with these variables set:
// - HTTPS_PROXY, for routing the traffic through the proxy
// - MSI_ENDPOINT, for informing the client about credential provider endpoint
//
// The request flow to remote server (i.e. Azure APIs) looks like this:
// clients -> local forward proxy -> local ALPN proxy -> remote server
//
// However, with MSI_ENDPOINT variable set, clients will reach out to this address for tokens.
// We intercept calls to https://azure-msi.teleport.dev using alpnproxy.AzureMSIMiddleware.
// These calls are served entirely locally, which helps the overall performance experienced by the user.
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
	envVars := map[string]string{
		// set custom Azure home path; this helps with the scenario in which user runs
		// 1. `tsh az login` in one console
		// 2. `az ...` in another console
		// without custom config dir the second invocation will hang, attempting to connect to (inaccessible without configuration) MSI.
		"AZURE_CONFIG_DIR": path.Join(profile.FullProfilePath(a.cf.HomePath), "azure", a.app.ClusterName, a.app.Name),
		// setting MSI_ENDPOINT instructs Azure CLI to make managed identity calls on this address.
		// the requests will be handled by tsh proxy.
		"MSI_ENDPOINT": "https://" + types.TeleportAzureMSIEndpoint + "/" + a.msiSecret,

		// Needed for az CLI to accept our certs.
		// This isn't portable and applications other than az CLI may have to set different env variables,
		// add the application cert to system root store (not recommended, ultimate fallback)
		// or use equivalent of --insecure flag.
		"REQUESTS_CA_BUNDLE": a.profile.AppLocalCAPath(a.app.Name),
	}

	// Set proxy settings.
	if a.localForwardProxy != nil {
		envVars["HTTPS_PROXY"] = "http://" + a.localForwardProxy.GetAddr()
	}
	return envVars, nil
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
	tc, err := makeClient(a.cf)
	if err != nil {
		return trace.Wrap(err)
	}

	appCerts, err := loadAppCertificateWithAppLogin(a.cf, tc, a.app.Name)
	if err != nil {
		return trace.Wrap(err)
	}

	localCA, err := loadAppSelfSignedCA(a.profile, tc, a.app.Name)
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
		return trace.Wrap(err)
	}

	wsPK, err := utils.ParsePrivateKey(ws.GetPriv())
	if err != nil {
		return trace.Wrap(err)
	}

	a.localALPNProxy, err = alpnproxy.NewLocalProxy(
		makeBasicLocalProxyConfig(a.cf, tc, listener),
		alpnproxy.WithClientCerts(appCerts),
		alpnproxy.WithClusterCAsIfConnUpgrade(a.cf.Context, tc.RootClusterCACertPool),
		alpnproxy.WithHTTPMiddleware(&alpnproxy.AzureMSIMiddleware{
			Key:    wsPK,
			Secret: a.msiSecret,
			// we could, in principle, get the actual TenantID either from live data or from static configuration,
			// but at this moment there is no clear advantage over simply issuing a new random identifier.
			TenantID: uuid.New().String(),
			ClientID: uuid.New().String(),
			Identity: a.app.AzureIdentity,
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
	fmt.Println(formatAzureIdentities(identities))
}

func formatAzureIdentities(identities []string) string {
	if len(identities) == 0 {
		return ""
	}

	t := asciitable.MakeTable([]string{"Available Azure identities"})

	sort.Strings(identities)
	for _, identity := range identities {
		t.AddRow([]string{identity})
	}

	return t.AsBuffer().String()
}

func getAzureIdentityFromFlags(cf *CLIConf, profile *client.ProfileStatus) (string, error) {
	identities := profile.AzureIdentities
	if len(identities) == 0 {
		return "", trace.BadParameter("no Azure identities available, check your permissions")
	}

	reqIdentity := strings.ToLower(cf.AzureIdentity)

	// if flag is missing, try to find singleton identity; failing that, print available options.
	if reqIdentity == "" {
		if len(identities) == 1 {
			log.Infof("Azure identity %v is selected by default as it is the only identity available for this Azure app.", identities[0])
			return identities[0], nil
		}

		// we will never have zero identities here: this is a pre-condition checked above.
		printAzureIdentities(identities)
		return "", trace.BadParameter("multiple Azure identities available, choose one with --azure-identity flag")
	}

	// exact match?
	for _, identity := range identities {
		if strings.ToLower(identity) == reqIdentity {
			return identity, nil
		}
	}

	// suffix match?
	expectedSuffix := strings.ToLower(fmt.Sprintf("/Microsoft.ManagedIdentity/userAssignedIdentities/%v", reqIdentity))
	var matches []string
	for _, identity := range identities {
		if strings.HasSuffix(strings.ToLower(identity), expectedSuffix) {
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

func matchAzureApp(app tlsca.RouteToApp) bool {
	return app.AzureIdentity != ""
}

func pickAzureApp(cf *CLIConf) (*azureApp, error) {
	app, err := pickCloudApp(cf, types.CloudAzure, matchAzureApp, newAzureApp)
	return app, trace.Wrap(err)
}
