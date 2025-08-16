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
	"crypto"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/coreos/go-semver/semver"
	"github.com/google/uuid"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

const (
	azureCLIBinaryName = "az"

	// msiEndpointEnvVarName defines the name of environment variable that
	// contains the MSI endpoint value.
	msiEndpointEnvVarName = "MSI_ENDPOINT"
	// identityEndpointEnvVarName defines the name of environment variable that
	// contains the App Service Identity endpoint value.
	identityEndpointEnvVarName = "IDENTITY_ENDPOINT"
	// identityHeaderEnvVarName defines the name of environment variable that
	// contains the App Service Identity secret value.
	identityHeaderEnvVarName = "IDENTITY_HEADER"
)

var (
	// azureCLIVersionMSALRequirement represents the version the login with
	// managed identities started using MSAL by default.
	//
	// https://learn.microsoft.com/en-us/cli/azure/release-notes-azure-cli?view=azure-cli-latest#profile
	azureCLIVersionMSALRequirement = semver.New("2.73.0")
)

func onAzure(cf *CLIConf) error {
	app, err := pickAzureApp(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = app.StartLocalProxies(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := app.Close(); err != nil {
			logger.ErrorContext(cf.Context, "Failed to close Azure app", "error", err)
		}
	}()

	args := cf.AzureCommandArgs

	cmd := exec.Command(azureCLIBinaryName, args...)
	return app.RunCommand(cmd)
}

// azureApp is an Azure app that can start local proxies to serve Azure APIs.
type azureApp struct {
	*localProxyApp

	cf          *CLIConf
	tokenSecret string
	// fetchAzureCLIVersion retrieves the Azure CLI version.
	fetchCLIVersion func() (*semver.Version, error)
}

// newAzureApp creates a new Azure app.
func newAzureApp(tc *client.TeleportClient, cf *CLIConf, appInfo *appInfo) (*azureApp, error) {
	msiSecret, err := getAzureTokenSecret()
	if err != nil {
		return nil, err
	}
	localProxyApp, err := newLocalProxyApp(tc, appInfo.profile, appInfo.RouteToApp, cf.LocalProxyPort, cf.InsecureSkipVerify)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &azureApp{
		localProxyApp: localProxyApp,
		cf:            cf,
		tokenSecret:   msiSecret,
		fetchCLIVersion: sync.OnceValues(func() (*semver.Version, error) {
			// Retrieve the core version as it contains the login-related changes.
			versionInfo := struct {
				CLICoreVersion string `json:"azure-cli-core"`
			}{}

			var buf bytes.Buffer
			cmd := exec.Command(azureCLIBinaryName, "version")
			cmd.Stdout = &buf
			if err := cf.RunCommand(cmd); err != nil {
				return nil, trace.Wrap(err)
			}

			if err := json.Unmarshal(buf.Bytes(), &versionInfo); err != nil {
				return nil, trace.Wrap(err)
			}

			ver, err := semver.NewVersion(versionInfo.CLICoreVersion)
			return ver, trace.Wrap(err)
		}),
	}, nil
}

// getAzureTokenSecret will try to find the secret from the environment.
// If not found it will return random hex string.
func getAzureTokenSecret() (string, error) {
	if secret, err := getAzureIdentitySecretToken(); !trace.IsNotFound(err) {
		if err != nil {
			return "", trace.Wrap(err)
		}

		return secret, nil
	}

	endpoint := os.Getenv(msiEndpointEnvVarName)
	if endpoint == "" {
		randomHex, err := utils.CryptoRandomHex(10)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return randomHex, nil
	}

	expectedPrefix := "https://" + types.TeleportAzureMSIEndpoint + "/"
	if !strings.HasPrefix(endpoint, expectedPrefix) {
		return "", trace.BadParameter("%q environment variable not empty, but doesn't start with %q as expected", msiEndpointEnvVarName, expectedPrefix)
	}

	secret := strings.TrimPrefix(endpoint, expectedPrefix)
	if secret == "" {
		return "", trace.BadParameter("MSI secret cannot be empty")
	}
	return secret, nil
}

// getAzureIdentitySecretToken returns the secret token for App Service Identity.
func getAzureIdentitySecretToken() (string, error) {
	endpoint := os.Getenv(identityEndpointEnvVarName)
	secret := os.Getenv(identityHeaderEnvVarName)
	if endpoint == "" && secret == "" {
		return "", trace.NotFound("App Service Identity environment variables not provided")
	}

	if endpoint == "" || secret == "" {
		return "", trace.BadParameter("%q and %q environment variables should be provided when using App Service Identity", identityEndpointEnvVarName, identityHeaderEnvVarName)
	}

	expectedPrefix := "https://" + types.TeleportAzureIdentityEndpoint
	if !strings.HasPrefix(endpoint, expectedPrefix) {
		return "", trace.BadParameter("%s not empty, but doesn't start with %q as expected", identityEndpointEnvVarName, expectedPrefix)
	}

	return secret, nil
}

// StartLocalProxies sets up local proxies for serving Azure clients.
//
// At minimum clients should work with these variables set:
// - HTTPS_PROXY, for routing the traffic through the proxy
// - MSI_ENDPOINT or IDENTITY_ENDPOINT, for informing the client about credential provider endpoint
//
// The request flow to remote server (i.e. Azure APIs) looks like this:
// clients -> local forward proxy -> local ALPN proxy -> remote server
//
// However, with MSI_ENDPOINT or IDENTITY_ENDPOINT variable set, clients will
// reach out to this address for tokens.
// We intercept calls to those token endpoints using alpnproxy.AzureTokensMiddleware.
// These calls are served entirely locally, which helps the overall performance
// experienced by the user.
func (a *azureApp) StartLocalProxies(ctx context.Context) error {
	azureMiddleware := &alpnproxy.AzureTokenMiddleware{
		Secret: a.tokenSecret,
		// we could, in principle, get the actual TenantID either from live data or from static configuration,
		// but at this moment there is no clear advantage over simply issuing a new random identifier.
		TenantID: uuid.New().String(),
		ClientID: uuid.New().String(),
		Identity: a.routeToApp.AzureIdentity,
	}

	// HTTPS proxy mode
	err := a.StartLocalProxyWithForwarder(ctx,
		alpnproxy.MatchAzureRequests,
		alpnproxy.WithHTTPMiddleware(azureMiddleware),
		alpnproxy.WithOnSetCert(func(cert tls.Certificate) {
			// Note that the PrivateKey is most likely set by api/utils/keys.TLSCertificateForSigner.
			signer, ok := cert.PrivateKey.(crypto.Signer)
			if ok {
				azureMiddleware.SetPrivateKey(signer)
			} else {
				logger.WarnContext(ctx, "Provided tls.Certificate has no valid private key")
			}
		}),
	)
	return trace.Wrap(err)
}

// GetEnvVars returns required environment variables to configure the
// clients.
func (a *azureApp) GetEnvVars() (map[string]string, error) {
	envVars := map[string]string{
		// set custom Azure home path; this helps with the scenario in which user runs
		// 1. `tsh az login` in one console
		// 2. `az ...` in another console
		// without custom config dir the second invocation will hang, attempting to connect to (inaccessible without configuration) MSI.
		"AZURE_CONFIG_DIR": filepath.Join(profile.FullProfilePath(a.cf.HomePath), "azure", a.routeToApp.ClusterName, a.routeToApp.Name),
		// Needed for az CLI to accept our certs.
		// This isn't portable and applications other than az CLI may have to set different env variables,
		// add the application cert to system root store (not recommended, ultimate fallback)
		// or use equivalent of --insecure flag.
		"REQUESTS_CA_BUNDLE": a.profile.AppLocalCAPath(a.cf.SiteName, a.routeToApp.Name),
	}

	if a.usingMSAL() {
		// Setting App service Identity environment variables instructs Azure
		// CLI to make managed identity calls on this address. The requests will
		// be handled by tsh proxy. This is only required when Azure CLI
		// defaults to using MSAL.
		//
		// https://learn.microsoft.com/en-us/azure/app-service/overview-managed-identity?tabs=portal%2Chttp#rest-endpoint-reference
		envVars[identityEndpointEnvVarName] = "https://" + types.TeleportAzureIdentityEndpoint
		envVars[identityHeaderEnvVarName] = a.tokenSecret
	} else {
		// Setting MSI environment variable instructs Azure CLI to make managed
		// identity calls on this address. The requests will be handled by tsh
		// proxy.
		envVars[msiEndpointEnvVarName] = "https://" + types.TeleportAzureMSIEndpoint + "/" + a.tokenSecret
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

	logger.DebugContext(a.cf.Context, "Running azure command", "command", logutils.StringerAttr(cmd))

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

// usingMSAL returns true if the CLI is using Microsoft Authentication
// Library (MSAL).
func (a *azureApp) usingMSAL() bool {
	ver, err := a.fetchCLIVersion()
	if err != nil {
		logger.WarnContext(a.cf.Context, "Unable to determine Azure CLI version. Assuming MSAL will be used.", "error", err)
		return true
	}

	logger.DebugContext(a.cf.Context, "Azure CLI version", "version", ver)
	return ver.Compare(*azureCLIVersionMSALRequirement) >= 0
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
			logger.InfoContext(cf.Context, "Azure identity is selected by default as it is the only identity available for this Azure app", "identity", identities[0])
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

		appInfo, err = getAppInfo(cf, clusterClient.AuthClient, profile, tc.SiteName, matchAzureApp)
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	return newAzureApp(tc, cf, appInfo)
}
