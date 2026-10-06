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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/google/uuid"
	"github.com/gravitational/trace"
	"go.yaml.in/yaml/v3"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/scopes"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/aws/awsconfigfile"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	awsutils "github.com/gravitational/teleport/lib/utils/aws"
	logutils "github.com/gravitational/teleport/lib/utils/log"
	"github.com/gravitational/teleport/lib/utils/prompt"
)

const (
	awsCLIBinaryName = "aws"
)

func onAWS(cf *CLIConf) error {
	awsSession, err := newAWSProxySession(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := awsSession.Close(); err != nil {
			logger.ErrorContext(cf.Context, "Failed to close AWS app", "error", err)
		}
	}()

	err = awsSession.Start(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	args := cf.AWSCommandArgs
	if cf.AWSEndpointURLMode {
		return trace.BadParameter("--endpoint-url is no longer supported, use HTTPS proxy instead (default mode)")
	}

	commandToRun := awsCLIBinaryName
	if cf.Exec != "" {
		commandToRun = cf.Exec
	}

	cmd := exec.Command(commandToRun, args...)
	environmentVariables, err := awsSession.GetEnvVars()
	if err != nil {
		return trace.Wrap(err)
	}
	return runAWSCommand(cf, environmentVariables, cmd)
}

type awsProxyApp struct {
	Name    string
	Profile string
	Role    string
}

type awsProxySession interface {
	Start(context.Context) error
	Close() error

	GetEnvVars() (map[string]string, error)
	GetForwardProxyAddr() string
	GetApps() []awsProxyApp
}

// Creates an aws proxy session.
// If 0 or 1 App SQNs are passed in, we use the legacy behavior.
// If > 1 App SQNs are passed in, we use the new multiplexed proxy.
func newAWSProxySession(cf *CLIConf) (awsProxySession, error) {
	if len(cf.AppSQNs) <= 1 {
		awsApp, err := pickAWSApp(cf)
		return awsApp, trace.Wrap(err)
	}

	multiplexedAwsSession, err := newMultiplexedAWSProxySession(cf)
	return multiplexedAwsSession, trace.Wrap(err)
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

// Start starts local proxies for proxying AWS traffic. See StartLocalProxies for
// more detail.
func (a *awsApp) Start(ctx context.Context) error {
	return a.StartLocalProxies(ctx)
}

// StartLocalProxies sets up local proxies for serving AWS clients.
//
// Clients can send AWS requests to our local forward proxy by configuring
// HTTPS_PROXY (or equivalent). This preserves the original hostname. The API
// flow looks like this:
// clients -> local forward proxy -> local ALPN proxy -> remote server
func (a *awsApp) StartLocalProxies(ctx context.Context, opts ...alpnproxy.LocalProxyConfigOpt) error {
	awsMiddleware := &alpnproxy.AWSAccessMiddleware{
		ResolveAWSRoute: func(ctx context.Context, _ string) (*alpnproxy.AWSRequestRoute, error) {
			return &alpnproxy.AWSRequestRoute{
				Credentials: a.GetAWSCredentialsProvider(),
			}, nil
		},
	}

	// AWS endpoint URL mode
	if a.cf.AWSEndpointURLMode {
		return trace.BadParameter("--endpoint-url is no longer supported, use HTTPS proxy instead (default mode)")
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
		"AWS_CA_BUNDLE":         a.profile.AppLocalCAPath(a.cf.SiteName, scopes.QualifiedName{Name: a.routeToApp.Name, Scope: a.routeToApp.Scope}),
		// Proxy settings.
		"HTTPS_PROXY": "http://" + a.localForwardProxy.GetAddr(),
		"https_proxy": "http://" + a.localForwardProxy.GetAddr(),
	}

	return envVars, nil
}

// GetApps returns the single app proxied by this awsApp.
func (a *awsApp) GetApps() []awsProxyApp {
	return []awsProxyApp{
		{Name: a.GetAppName()},
	}
}

// GetForwardProxyAddr returns local forward proxy address.
func (a *awsApp) GetForwardProxyAddr() string {
	return a.localForwardProxy.GetAddr()
}

// runAWSCommand executes cmd with the supplied AWS environment.
func runAWSCommand(cf *CLIConf, environmentVariables map[string]string, cmd *exec.Cmd) error {
	logger.DebugContext(cf.Context, "Running AWS command", "command", logutils.StringerAttr(cmd))

	cmd.Stdout = cf.Stdout()
	cmd.Stderr = cf.Stderr()
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()
	for key, value := range environmentVariables {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", key, value))
	}

	if err := cf.RunCommand(cmd); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

// awsProxyRoute represents a single AWS app a multiplexed session can route to.
type awsProxyRoute struct {
	name       scopes.QualifiedName
	routeToApp proto.RouteToApp

	credentials aws.CredentialsProvider

	// getClientCertificate overrides the normal app certificate loader. It is
	// primarily useful for exercising route selection in testing
	getClientCertificate func(context.Context) (tls.Certificate, error)

	// Profile name to use in AWS config file.
	profile string
}

type multiplexedAWSSession struct {
	cf      *CLIConf
	tc      *client.TeleportClient
	profile *client.ProfileStatus

	// Routes this session provides access to
	routes          []awsProxyRoute
	sharedArtifacts *awsSharedSessionArtifacts

	localHTTPProxy *localHTTPProxy
}

func newAWSProxyRoute(name scopes.QualifiedName, route proto.RouteToApp, profile string) (awsProxyRoute, error) {
	return awsProxyRoute{
		name:        name,
		routeToApp:  route,
		credentials: credentials.NewStaticCredentialsProvider(uuid.NewString(), uuid.NewString(), ""),
		profile:     profile,
	}, nil
}

// awsSharedSessionArtifacts manages the files shared with AWS clients for a
// multiplexed session. It handles locking so multiple AWS proxies don't
// overwrite each other's artifacts and cleans up after itself when it closes.
type awsSharedSessionArtifacts struct {
	credentialsPath    string
	localCAPath        string
	lockfilePath       string
	unlock             func() error
	cleanupArtifacts   func() error
	localCertGenerator *client.EphemeralLocalCertGenerator
}

// newAWSSharedSessionArtifacts creates an awsSharedSessionArtifacts
// initialized in the specified directory.
func newAWSSharedSessionArtifacts(dir string) *awsSharedSessionArtifacts {
	return &awsSharedSessionArtifacts{
		credentialsPath: filepath.Join(dir, "credentials"),
		localCAPath:     filepath.Join(dir, "localca.pem"),
		lockfilePath:    filepath.Join(dir, "proxy.lock"),
	}
}

// WriteWithLock locks the shared AWS artifacts directory, removes any existing
// artifacts, writes profiles, and creates the local certificate generator. The
// lock prevents concurrent multiplexed sessions from replacing artifacts used
// by an active local proxy.
func (a *awsSharedSessionArtifacts) WriteWithLock(profiles []awsconfigfile.AWSProfileCredentials) error {
	if err := a.tryObtainLock(); err != nil {
		return trace.Wrap(err)
	}

	a.cleanupArtifacts = func() error {
		return trace.NewAggregate(
			utils.RemoveFileIfExist(a.credentialsPath),
			utils.RemoveFileIfExist(a.localCAPath),
		)
	}

	// Remove any existing artifacts so we can start from a clean slate.
	if err := a.cleanupArtifacts(); err != nil {
		return trace.Wrap(err)
	}

	if err := a.writeCredentialsFile(profiles); err != nil {
		return trace.Wrap(err)
	}

	localCertGenerator, err := client.NewEphemeralLocalCertGenerator(a.localCAPath)
	if err != nil {
		return trace.Wrap(err)
	}
	a.localCertGenerator = localCertGenerator

	return nil
}

// tryObtainLock obtains the exclusive lock for the generated AWS session
// artifacts. It prevents concurrent sessions from replacing artifacts used by
// an active local proxy.
func (a *awsSharedSessionArtifacts) tryObtainLock() error {
	if _, err := utils.EnsureLocalPath(a.lockfilePath, "", ""); err != nil {
		return trace.Wrap(err)
	}

	exclusiveUnlock, err := utils.FSTryWriteLock(a.lockfilePath)
	if err != nil {
		return trace.Wrap(err)
	}

	a.unlock = exclusiveUnlock

	return nil
}

// writeCredentialsFile writes profiles to the AWS shared credentials file.
func (a *awsSharedSessionArtifacts) writeCredentialsFile(profiles []awsconfigfile.AWSProfileCredentials) error {
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].Name < profiles[j].Name
	})

	return trace.Wrap(awsconfigfile.WriteSharedCredentials(a.credentialsPath, profiles))
}

// LocalCertGenerator returns the certificate generator created by WriteWithLock.
func (a *awsSharedSessionArtifacts) LocalCertGenerator() (*client.EphemeralLocalCertGenerator, error) {
	if a.localCertGenerator == nil {
		return nil, trace.NotFound("local certificate generator is not initialized")
	}
	return a.localCertGenerator, nil
}

// Close removes the generated session artifacts and releases their lock.
func (a *awsSharedSessionArtifacts) Close() error {
	var errs []error
	if a.cleanupArtifacts != nil {
		errs = append(errs, a.cleanupArtifacts())
	}
	if a.unlock != nil {
		errs = append(errs, a.unlock())
	}
	return trace.NewAggregate(errs...)
}

func newMultiplexedAWSProxySession(cf *CLIConf) (*multiplexedAWSSession, error) {
	tc, err := makeClient(cf)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var profile *client.ProfileStatus
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		var err error
		profile, err = tc.ProfileStatus()
		return trace.Wrap(err)
	}); err != nil {
		return nil, trace.Wrap(err)
	}

	routes, err := pickAWSApps(cf, profile)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	awsDir := filepath.Join(profile.Dir, "aws")

	return &multiplexedAWSSession{
		cf:              cf,
		tc:              tc,
		profile:         profile,
		routes:          routes,
		sharedArtifacts: newAWSSharedSessionArtifacts(awsDir),
	}, nil
}

// Start writes generated AWS profile credentials, then starts the local proxy.
func (m *multiplexedAWSSession) Start(ctx context.Context) error {
	profiles, err := m.awsProfileCredentials(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := m.sharedArtifacts.WriteWithLock(profiles); err != nil {
		if errors.Is(err, utils.ErrUnsuccessfulLockTry) {
			return trace.BadParameter("another multiplexed AWS proxy is already running")
		}
		return trace.Wrap(err)
	}

	localCertGenerator, err := m.sharedArtifacts.LocalCertGenerator()
	if err != nil {
		return trace.Wrap(err)
	}

	var localProxyPort int
	if m.cf.LocalProxyPort != "" {
		localProxyPort, err = strconv.Atoi(m.cf.LocalProxyPort)
		if err != nil {
			return trace.Wrap(err, "parsing port")
		}
	}

	awsMiddleware, err := m.buildMultiplexedAWSSessionMiddleware(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	opts := []alpnproxy.LocalProxyConfigOpt{
		alpnproxy.WithClusterCAsIfConnUpgrade(ctx, m.tc.RootClusterCACertPool),
		alpnproxy.WithHTTPMiddleware(awsMiddleware),
		alpnproxy.WithInsecureSkipVerify(m.cf.InsecureSkipVerify),
	}

	m.localHTTPProxy = newLocalHTTPProxy(m.tc, localHTTPProxyConfig{
		LocalPort: localProxyPort,
		ALPNTLSConfig: &tls.Config{
			GetCertificate: localCertGenerator.GetCertificate,
		},
		ALPNProxyConfigOpts: opts,
		ForwardMatcher:      alpnproxy.MatchAWSRequests,
		InsecureSystemProxy: m.cf.InsecureSkipVerify,
	})
	if err := m.localHTTPProxy.Start(ctx); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// buildMultiplexedAWSSessionMiddleware configures request routing and selects
// the appropriate app certificate for the route selected by each request.
func (m *multiplexedAWSSession) buildMultiplexedAWSSessionMiddleware(ctx context.Context) (*alpnproxy.AWSAccessMiddleware, error) {
	router := make(map[string]*alpnproxy.AWSRequestRoute)
	for _, route := range m.routes {
		creds, err := route.credentials.Retrieve(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		getClientCertificate := route.getClientCertificate
		if getClientCertificate == nil {
			// Create a cert checker to check and reissue app certs for this app that this route corresponds to.
			appCertChecker := client.NewAppCertChecker(m.tc, route.routeToApp, nil, client.WithTTL(m.tc.KeyTTL))

			// If a stored cert is found for the app, try using it.
			// Otherwise, let the checker reissue one as needed.
			cert, err := loadAppCertificate(m.tc, route.routeToApp.Name, route.routeToApp.Scope)
			if err == nil {
				appCertChecker.SetCert(cert)
			}

			getClientCertificate = func(ctx context.Context) (tls.Certificate, error) {
				cert, err := appCertChecker.GetOrIssueCert(ctx)
				if err != nil {
					return tls.Certificate{}, trace.Wrap(err)
				}
				return cert, nil
			}
		}

		awsRequestRoute := &alpnproxy.AWSRequestRoute{
			Credentials:          route.credentials,
			GetClientCertificate: getClientCertificate,
		}
		router[creds.AccessKeyID] = awsRequestRoute
	}

	return &alpnproxy.AWSAccessMiddleware{
		ResolveAWSRoute: func(ctx context.Context, accessKeyID string) (*alpnproxy.AWSRequestRoute, error) {
			route, found := router[accessKeyID]
			if !found {
				return nil, trace.NotFound("aws route for access key %s not found", accessKeyID)
			}
			return route, nil
		},
	}, nil
}

// awsProfileCredentials returns credentials for each application routed by the
// multiplexed session.
func (m *multiplexedAWSSession) awsProfileCredentials(ctx context.Context) ([]awsconfigfile.AWSProfileCredentials, error) {
	profiles := make([]awsconfigfile.AWSProfileCredentials, 0, len(m.routes))
	for _, route := range m.routes {
		credentials, err := route.credentials.Retrieve(ctx)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		profiles = append(profiles, awsconfigfile.AWSProfileCredentials{
			Name:            route.profile,
			AccessKeyID:     credentials.AccessKeyID,
			SecretAccessKey: credentials.SecretAccessKey,
		})
	}
	return profiles, nil
}

// Close stops the local proxy and removes the shared AWS session artifacts.
func (m *multiplexedAWSSession) Close() error {
	errs := make([]error, 0)

	if m.localHTTPProxy != nil {
		errs = append(errs, m.localHTTPProxy.Close())
	}

	if m.sharedArtifacts != nil {
		errs = append(errs, m.sharedArtifacts.Close())
	}

	return trace.NewAggregate(errs...)
}

// GetEnvVars returns environment variables that configure AWS clients to use
// this session's generated credentials and local proxies.
func (m *multiplexedAWSSession) GetEnvVars() (map[string]string, error) {
	var forwardProxyAddr string
	if forwardProxyAddr = m.GetForwardProxyAddr(); forwardProxyAddr == "" {
		return nil, trace.NotFound("AWS proxy is not running")
	}

	proxyURL := "http://" + forwardProxyAddr

	return map[string]string{
		"AWS_CA_BUNDLE":               m.sharedArtifacts.localCAPath,
		"AWS_SHARED_CREDENTIALS_FILE": m.sharedArtifacts.credentialsPath,
		"HTTPS_PROXY":                 proxyURL,
		"https_proxy":                 proxyURL,

		// Unset the following variables to prevent any pre-existing values
		// in the environment from overriding the generated credentials.
		"AWS_ACCESS_KEY_ID":           "",
		"AWS_SECRET_ACCESS_KEY":       "",
		"AWS_SESSION_TOKEN":           "",
		"AWS_ROLE_ARN":                "",
		"AWS_ROLE_SESSION_NAME":       "",
		"AWS_WEB_IDENTITY_TOKEN_FILE": "",
	}, nil
}

// GetApps returns all AWS apps proxied by this multiplexed AWS session,
// along with their associated AWS profile
func (m *multiplexedAWSSession) GetApps() []awsProxyApp {
	apps := make([]awsProxyApp, len(m.routes))
	for i, route := range m.routes {
		apps[i] = awsProxyApp{
			Name:    route.name.String(),
			Profile: route.profile,
			Role:    awsRoleDisplayName(route.routeToApp.AWSRoleARN),
		}
	}

	return apps
}

// awsRoleDisplayName returns the short, user-facing name of an AWS IAM role.
func awsRoleDisplayName(roleARN string) string {
	role, err := awsutils.ParseRoleARN(roleARN)
	if err == nil {
		parts := strings.Split(role.Resource, "/")
		return parts[len(parts)-1]
	}

	// An AWS proxy route should always have a valid role ARN. If for some reason
	// we encountered an error when parsing it, we return the raw value
	return roleARN
}

// GetForwardProxyAddr returns the local HTTP forward proxy address.
func (m *multiplexedAWSSession) GetForwardProxyAddr() string {
	if m.localHTTPProxy != nil && m.localHTTPProxy.localForwardProxy != nil {
		return m.localHTTPProxy.localForwardProxy.GetAddr()
	}

	return ""
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

	if len(roles) == 0 {
		return "", trace.NotFound("could not determine any allowed IAM role ARNs for %q: this can indicate lack of permissions or app misconfiguration", app.GetName())
	}

	if cf.AWSRole == "" {
		if len(roles) == 1 {
			logger.InfoContext(cf.Context, "AWS Role is selected by default as it is the only role configured for this AWS app", "role", roles[0].Display)
			return roles[0].ARN, nil
		}

		if cf.Interactive {
			selectPrompt := prompt.NewSelectPrompt(
				"Available AWS roles",
				roles,
				func(role awsutils.Role) string {
					return fmt.Sprintf("%s \x1b[35m[%s]\x1b[0m", role.Name, role.ARN)
				},
			)

			selected, err := selectPrompt.Run(cf.Context)
			if err != nil {
				printAWSRoles(cf.Stdout(), roles)
				return "", trace.Wrap(err)
			}

			cf.AWSRole = selected.ARN
			return selected.ARN, nil
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

// awsRole is a mirror of awsutils.Role for maintaining the schema consistency (snake case for
// JSON and YAML keys).
type awsRole struct {
	Name            string `yaml:"name" json:"name"`
	Display         string `yaml:"display" json:"display"`
	AccountID       string `yaml:"account_id" json:"account_id"`
	ARN             string `yaml:"arn" json:"arn"`
	RequiresRequest bool   `yaml:"requires_request,omitempty" json:"requires_request,omitempty"`
}

type cloudAppLogins struct {
	AWSRoles []awsRole `yaml:"aws_roles,omitempty" json:"aws_roles,omitempty"`
}

// cloudAppInfo is a json/yaml schema for the "apps logins" command.
type cloudAppInfo struct {
	AppName string         `yaml:"app_name" json:"app_name"`
	Logins  cloudAppLogins `yaml:"logins" json:"logins"`
}

func serializeAWSLogins(app types.Application, roles awsutils.Roles) cloudAppInfo {
	roles.Sort()

	awsRoles := make([]awsRole, 0, len(roles))
	for _, role := range roles {
		awsRoles = append(awsRoles, awsRole{
			Name:            role.Name,
			Display:         role.Display,
			AccountID:       role.AccountID,
			ARN:             role.ARN,
			RequiresRequest: role.RequiresRequest,
		})
	}

	return cloudAppInfo{
		AppName: app.GetName(),
		Logins: cloudAppLogins{
			AWSRoles: awsRoles,
		},
	}
}

func printAWSAppLogins(cf *CLIConf, app types.Application, logins []string) error {
	// Filter AWS roles by AWS account ID.
	roles := awsutils.FilterAWSRoles(logins, app.GetAWSAccountID())

	if len(roles) == 0 {
		return trace.NotFound("could not determine any allowed IAM role ARNs for %q: this can indicate lack of permissions or app misconfiguration", app.GetName())
	}

	// Output based on format.
	switch cf.Format {
	case teleport.Text, "":
		// Use existing printAWSRoles function for table output.
		printAWSRoles(cf.Stdout(), roles)
	case teleport.JSON:
		out, err := utils.FastMarshalIndent(serializeAWSLogins(app, roles), "", "  ")
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprintln(cf.Stdout(), string(out))
	case teleport.YAML:
		out, err := yaml.Marshal(serializeAWSLogins(app, roles))
		if err != nil {
			return trace.Wrap(err)
		}
		fmt.Fprint(cf.Stdout(), string(out))
	default:
		return trace.BadParameter("unsupported format %q", cf.Format)
	}

	return nil
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

// Filter activeRoutes, the list of logged in routes, to get only AWS apps and only apps specified by the
// QualifiedNames in requestedRoutes
func selectLoggedInAWSRoutes(activeRoutes []tlsca.RouteToApp, requestedRoutes []scopes.QualifiedName) ([]tlsca.RouteToApp, error) {
	activeAWSRoutes := map[scopes.QualifiedName]tlsca.RouteToApp{}
	for _, route := range activeRoutes {
		if matchAWSApp(route) {
			activeAWSRoutes[scopes.QualifiedName{Name: route.Name, Scope: route.Scope}] = route
		}
	}

	selectedProxyRoutes := make([]tlsca.RouteToApp, 0)
	missing := make([]string, 0)
	for _, requestedRoute := range requestedRoutes {
		name := scopes.QualifiedName{Name: requestedRoute.Name, Scope: requestedRoute.Scope}
		route, ok := activeAWSRoutes[name]

		if !ok {
			missing = append(missing, name.String())
			continue
		}

		selectedProxyRoutes = append(selectedProxyRoutes, route)
	}

	if len(missing) > 0 {
		return nil, trace.BadParameter(
			"not logged into AWS apps %q; log in with `tsh apps login <app>` first", strings.Join(missing, ", "),
		)
	}

	return selectedProxyRoutes, nil
}

func pickAWSApps(cf *CLIConf, profile *client.ProfileStatus) ([]awsProxyRoute, error) {
	if len(cf.AppSQNs) < 1 {
		return nil, trace.BadParameter("you must specify at least one app via CLI argument")
	}

	selectedRoutes, err := selectLoggedInAWSRoutes(profile.Apps, cf.AppSQNs)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	proxyRoutes := make([]awsProxyRoute, len(selectedRoutes))
	for i, route := range selectedRoutes {
		name := scopes.QualifiedName{Name: route.Name, Scope: route.Scope}
		profileName := cf.AWSProfileMappings[name]
		if profileName == "" {
			profileName = name.String()
		}
		proxyRoute, err := newAWSProxyRoute(name, tlscaRouteToAppToProto(route), profileName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		proxyRoutes[i] = proxyRoute
	}

	return proxyRoutes, nil
}

// configureAWSAppSelection parses and validates the AWS application and profile
// mapping flags used by tsh aws and tsh proxy aws.
func configureAWSAppSelection(cf *CLIConf, rawAppNames, rawProfileMappings []string) error {
	appSQNs, err := scopes.ParseSQNs(rawAppNames)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := validateUniqueAWSApps(appSQNs); err != nil {
		return trace.Wrap(err)
	}
	profileMappings, err := parseAWSProfileMappings(rawProfileMappings)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := validateAWSProfileMappings(appSQNs, profileMappings); err != nil {
		return trace.Wrap(err)
	}

	cf.AppSQNs = appSQNs
	cf.AWSProfileMappings = profileMappings
	if len(appSQNs) == 1 {
		cf.AppSQN = appSQNs[0]
	}

	return nil
}

// validateUniqueAWSApps ensures each AWS application is specified at most once.
func validateUniqueAWSApps(apps []scopes.QualifiedName) error {
	seen := make(map[scopes.QualifiedName]struct{}, len(apps))
	for _, app := range apps {
		if _, found := seen[app]; found {
			return trace.BadParameter("AWS application %q was specified more than once with --app", app)
		}
		seen[app] = struct{}{}
	}

	return nil
}

// parseAWSProfileMappings parses repeated --profile <app>=<profile> flags.
func parseAWSProfileMappings(rawMappings []string) (map[scopes.QualifiedName]string, error) {
	mappings := make(map[scopes.QualifiedName]string, len(rawMappings))
	for _, rawMapping := range rawMappings {
		appName, profileName, found := strings.Cut(rawMapping, "=")
		if !found || appName == "" || profileName == "" {
			return nil, trace.BadParameter("invalid AWS profile mapping %q, expected <app>=<profile>", rawMapping)
		}

		var app scopes.QualifiedName
		if err := app.Set(appName); err != nil {
			return nil, trace.Wrap(err, "invalid AWS application in profile mapping %q", rawMapping)
		}
		if _, exists := mappings[app]; exists {
			return nil, trace.BadParameter("AWS application %q has more than one profile mapping", app)
		}

		mappings[app] = profileName
	}

	return mappings, nil
}

// validateAWSProfileMappings ensures every mapped application was selected
// with --app.
func validateAWSProfileMappings(apps []scopes.QualifiedName, mappings map[scopes.QualifiedName]string) error {
	// Profile mappings only apply to multiplexed AWS proxy sessions. The legacy
	// single-app flow does not write an AWS config file.
	if len(mappings) > 0 && len(apps) <= 1 {
		return trace.BadParameter("AWS profile mappings require more than one app selected with --app")
	}

	selectedApps := make(map[scopes.QualifiedName]struct{}, len(apps))
	for _, app := range apps {
		selectedApps[app] = struct{}{}
	}

	for app := range mappings {
		if _, selected := selectedApps[app]; !selected {
			return trace.BadParameter("AWS profile mapping for app %q requires the app to be selected with --app", app)
		}
	}

	// Ensure that after applying profile mappings, there will be no duplicate profile names in the AWS config.
	// Each app's AWS profile name will be the app name by default unless a mapping was specified.
	profileApps := make(map[string]scopes.QualifiedName, len(apps))
	for _, app := range apps {
		profileName := app.String()
		if mappedName, mapped := mappings[app]; mapped {
			profileName = mappedName
		}

		if otherApp, used := profileApps[profileName]; used {
			return trace.BadParameter("AWS applications %q and %q both use profile %q", otherApp, app, profileName)
		}
		profileApps[profileName] = app
	}

	return nil
}
