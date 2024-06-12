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
	"hash/fnv"
	"net"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/profile"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/gcp"
)

const (
	gcloudCLIBinaryName = "gcloud"
	gsutilCLIBinaryName = "gsutil"
)

func onGcloud(cf *CLIConf) error {
	app, err := pickGCPApp(cf)
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

func onGsutil(cf *CLIConf) error {
	app, err := pickGCPApp(cf)
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

	cmd := exec.Command(gsutilCLIBinaryName, args...)
	return app.RunCommand(cmd)
}

// gcpApp is an GCP app that can start local proxies to serve GCP APIs.
type gcpApp struct {
	cf      *CLIConf
	profile *client.ProfileStatus
	app     tlsca.RouteToApp
	secret  string
	// prefix is a prefix added to the name of configuration files, allowing two instances of gcpApp
	// to run concurrently without overwriting each other files.
	prefix string

	localALPNProxy    *alpnproxy.LocalProxy
	localForwardProxy *alpnproxy.ForwardProxy
}

// newGCPApp creates a new GCP app.
func newGCPApp(cf *CLIConf, profile *client.ProfileStatus, app tlsca.RouteToApp) (*gcpApp, error) {
	secret, err := getGCPSecret()
	if err != nil {
		return nil, err
	}

	h := fnv.New32a()
	_, _ = h.Write([]byte(secret))
	prefix := fmt.Sprintf("%x", h.Sum32())

	return &gcpApp{
		cf:      cf,
		profile: profile,
		app:     app,
		secret:  secret,
		prefix:  prefix,
	}, nil
}

// getGCPSecret will return fresh secret to use or read it from environment.
func getGCPSecret() (string, error) {
	secret := os.Getenv(gcloudSecretEnvVar)
	if secret != "" {
		return secret, nil
	}

	return utils.CryptoRandomHex(defaults.TokenLenBytes)
}

// StartLocalProxies sets up local proxies for serving GCP clients.
//
// At minimum clients should work with these variables set:
// - HTTPS_PROXY, for routing the traffic through the proxy
//
// The request flow to remote server (i.e. GCP APIs) looks like this:
// clients -> local forward proxy -> local ALPN proxy -> remote server
func (a *gcpApp) StartLocalProxies() error {
	// configuration files
	if err := a.writeBotoConfig(); err != nil {
		return trace.Wrap(err)
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

// Close makes all necessary close calls.
func (a *gcpApp) Close() error {
	var errs []error
	// close proxies
	if a.localALPNProxy != nil {
		errs = append(errs, a.localALPNProxy.Close())
	}
	if a.localForwardProxy != nil {
		errs = append(errs, a.localForwardProxy.Close())
	}
	// remove boto config
	errs = append(errs, a.removeBotoConfig()...)
	return trace.NewAggregate(errs...)
}

func (a *gcpApp) getGcloudConfigPath() string {
	return path.Join(profile.FullProfilePath(a.cf.HomePath), "gcp", a.app.ClusterName, a.app.Name, "gcloud")
}

// removeBotoConfig removes config files written by WriteBotoConfig.
func (a *gcpApp) removeBotoConfig() []error {
	// try to remove both files
	return []error{
		trace.Wrap(os.Remove(a.getExternalAccountFilePath())),
		trace.Wrap(os.Remove(a.getBotoConfigPath())),
	}
}

func (a *gcpApp) getBotoConfigDir() string {
	return path.Join(profile.FullProfilePath(a.cf.HomePath), "gcp", a.app.ClusterName, a.app.Name)
}

func (a *gcpApp) getBotoConfigPath() string {
	return path.Join(a.getBotoConfigDir(), a.prefix+"_boto.cfg")
}

func (a *gcpApp) getExternalAccountFilePath() string {
	return path.Join(a.getBotoConfigDir(), a.prefix+"_external.json")
}

// getBotoConfig returns minimal boto configuration, referencing an external account file.
func (a *gcpApp) getBotoConfig() string {
	// gsutil will look for `gs_external_account_authorized_user_file` in `[Credentials]` section as per the source code:
	// https://github.com/GoogleCloudPlatform/gsutil/blob/2fd97591681a51ca0541d04b865e7d67a54efad4/gslib/gcs_json_credentials.py#L290-L294
	// there appears to be no documentation for this config setting otherwise.
	return fmt.Sprintf(`[Credentials]
gs_external_account_authorized_user_file = %v
`, a.getExternalAccountFilePath())
}

// getExternalAccountFile returns the contents of external account file, which depend on a current secret.
func (a *gcpApp) getExternalAccountFile() string {
	return fmt.Sprintf(`{ "type": "external_account_authorized_user","token": %q }`, a.secret)
}

// writeBotoConfig writes app-specific boto configuration file as well as external account file, referenced in boto config.
func (a *gcpApp) writeBotoConfig() error {
	err := os.MkdirAll(a.getBotoConfigDir(), teleport.PrivateDirMode)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.WriteFile(a.getBotoConfigPath(), []byte(a.getBotoConfig()), 0644)
	if err != nil {
		return trace.Wrap(err)
	}

	err = os.WriteFile(a.getExternalAccountFilePath(), []byte(a.getExternalAccountFile()), 0644)
	if err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// GetEnvVars returns required environment variables to configure the
// clients.
func (a *gcpApp) GetEnvVars() (map[string]string, error) {
	projectID, err := gcp.ProjectIDFromServiceAccountName(a.app.GCPServiceAccount)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	envVars := map[string]string{
		// Env var CLOUDSDK_AUTH_ACCESS_TOKEN is one of the available ways of providing access token
		// https://cloud.google.com/sdk/docs/authorizing#:~:text=If%20you%20already,access%20token%20value.
		"CLOUDSDK_AUTH_ACCESS_TOKEN": a.secret,

		// Set core.custom_ca_certs_file via env variable, customizing the path to CA certs file.
		// https://cloud.google.com/sdk/gcloud/reference/config/set#:~:text=custom_ca_certs_file
		"CLOUDSDK_CORE_CUSTOM_CA_CERTS_FILE": a.profile.AppLocalCAPath(a.app.Name),

		// We need to set project ID. This is sourced from the account name.
		// https://cloud.google.com/sdk/gcloud/reference/config#GROUP:~:text=authentication%20to%20gsutil.-,project,-Project%20ID%20of
		"CLOUDSDK_CORE_PROJECT": projectID,

		// Use isolated gcloud config path.
		// https://cloud.google.com/sdk/docs/configurations#:~:text=The%20config%20directory%20can%20be%20changed%20by%20setting%20the%20environment%20variable%20CLOUDSDK_CONFIG
		"CLOUDSDK_CONFIG": a.getGcloudConfigPath(),

		// Set custom path to boto config. Used to provide fixed access token for `gsutil`.
		// More info: https://cloud.google.com/storage/docs/boto-gsutil
		"BOTO_CONFIG": a.getBotoConfigPath(),
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

	// Create a listener that is able to sign certificates when receiving GCP
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
		alpnproxy.WithClientCerts(appCerts),
		alpnproxy.WithClusterCAsIfConnUpgrade(a.cf.Context, tc.RootClusterCACertPool),
		alpnproxy.WithHTTPMiddleware(&alpnproxy.AuthorizationCheckerMiddleware{
			Secret: a.secret,
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

func formatGCPServiceAccounts(accounts []string) string {
	if len(accounts) == 0 {
		return ""
	}

	t := asciitable.MakeTable([]string{"Available GCP service accounts"})

	acc := gcp.SortedGCPServiceAccounts(accounts)
	sort.Sort(acc)

	for _, account := range acc {
		t.AddRow([]string{account})
	}

	return t.AsBuffer().String()
}

func getGCPServiceAccountFromFlags(cf *CLIConf, profile *client.ProfileStatus) (string, error) {
	// helper function to validate correctness of matched service account
	validate := func(account string) (string, error) {
		err := gcp.ValidateGCPServiceAccountName(account)
		if err != nil {
			return "", trace.Wrap(err, "chosen GCP service account %q is invalid", account)
		}
		return account, nil
	}

	accounts := profile.GCPServiceAccounts
	if len(accounts) == 0 {
		return "", trace.BadParameter("no GCP service accounts available, check your permissions")
	}

	reqAccount := cf.GCPServiceAccount

	// if flag is missing, try to find singleton service account; failing that, print available options.
	if reqAccount == "" {
		if len(accounts) == 1 {
			log.Infof("GCP service account %v is selected by default as it is the only one available for this GCP app.", accounts[0])
			return validate(accounts[0])
		}

		// we will never have zero identities here: this is a pre-condition checked above.
		printGCPServiceAccounts(accounts)
		return "", trace.BadParameter("multiple GCP service accounts available, choose one with --gcp-service-account flag")
	}

	// exact match?
	for _, account := range accounts {
		if account == reqAccount {
			return validate(account)
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
		return validate(matches[0])
	case 0:
		printGCPServiceAccounts(accounts)
		return "", trace.NotFound("failed to find the service account matching %q", cf.GCPServiceAccount)
	default:
		printGCPServiceAccounts(matches)
		return "", trace.BadParameter("provided service account %q is ambiguous, please specify full service account name", cf.GCPServiceAccount)
	}
}

func matchGCPApp(app tlsca.RouteToApp) bool {
	return app.GCPServiceAccount != ""
}

func pickGCPApp(cf *CLIConf) (*gcpApp, error) {
	app, err := pickCloudApp(cf, types.CloudGCP, matchGCPApp, newGCPApp)
	return app, trace.Wrap(err)
}
