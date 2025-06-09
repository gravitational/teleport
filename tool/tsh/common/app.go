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
	"strings"
	"sync"
	"text/template"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// onAppLogin implements "tsh apps login" command.
func onAppLogin(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	var (
		clusterClient *client.ClusterClient
		appInfo       *appInfo
		app           types.Application
	)
	if err := client.RetryWithRelogin(cf.Context, tc, func() error {
		var err error
		profile, err := tc.ProfileStatus()
		if err != nil {
			return trace.Wrap(err)
		}

		clusterClient, err = tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}

		appInfo, err = getAppInfo(cf, clusterClient.AuthClient, profile, tc.SiteName, nil /*matchRouteToApp*/)
		if err != nil {
			return trace.Wrap(err)
		}

		app, err = appInfo.GetApp(cf.Context, clusterClient.AuthClient)
		return trace.Wrap(err)
	}); err != nil {
		return trace.Wrap(err)
	}
	defer clusterClient.Close()

	if app.IsMCP() {
		return trace.BadParameter("MCP applications are not supported. Please see 'tsh mcp login --help' for more details.")
	}

	if err := validateTargetPort(app, int(cf.TargetPort)); err != nil {
		return trace.Wrap(err)
	}

	rootClient, err := clusterClient.ConnectToRootCluster(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	routeToApp := appInfo.RouteToApp
	if cf.TargetPort != 0 {
		routeToApp.TargetPort = uint32(cf.TargetPort)
	}

	appCertParams := client.ReissueParams{
		RouteToCluster: tc.SiteName,
		RouteToApp:     routeToApp,
		AccessRequests: appInfo.profile.ActiveRequests,
	}

	key, err := appLogin(cf.Context, clusterClient, rootClient, appCertParams)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := tc.LocalAgent().AddAppKeyRing(key); err != nil {
		return trace.Wrap(err)
	}

	if err := printAppCommand(cf, tc, app, appInfo); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

func appLogin(
	ctx context.Context,
	clusterClient *client.ClusterClient,
	rootClient authclient.ClientI,
	appCertParams client.ReissueParams,
) (*client.KeyRing, error) {
	result, err := clusterClient.IssueUserCertsWithMFA(ctx, appCertParams)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return result.KeyRing, nil
}

func localProxyRequiredForApp(tc *client.TeleportClient) bool {
	return tc.TLSRoutingConnUpgradeRequired
}

func printAppCommand(cf *CLIConf, tc *client.TeleportClient, app types.Application, appInfo *appInfo) error {
	routeToApp := appInfo.RouteToApp
	output := cf.Stdout()
	if cf.Quiet {
		output = io.Discard
	}

	switch {
	case app.IsAWSConsole():
		return awsLoginTemplate.Execute(output, map[string]string{
			"awsAppName": app.GetName(),
			"awsCmd":     "s3 ls",
			"awsRoleARN": routeToApp.AWSRoleARN,
		})

	case app.IsAzureCloud():
		if routeToApp.AzureIdentity == "" {
			return trace.BadParameter("app is Azure Cloud but Azure identity is missing")
		}

		azureApp, err := newAzureApp(tc, cf, appInfo)
		if err != nil {
			return trace.Wrap(err)
		}

		resourceArgumentName := "--username"
		// After the CLI started relying in MSAL by default, the param for the
		// managed identity changed.
		//
		// https://learn.microsoft.com/en-us/cli/azure/release-notes-azure-cli?view=azure-cli-latest#profile
		if azureApp.usingMSAL() {
			resourceArgumentName = "--resource-id"
		}

		args := []string{"az", "login", "--identity", resourceArgumentName, routeToApp.AzureIdentity}
		if cf.Debug {
			args = append(args, "--debug")
		}

		// automatically login with right identity.
		cmd := exec.Command(cf.executablePath, args...)
		cmd.Stdin = os.Stdin
		cmd.Stderr = cf.Stderr()
		cmd.Stdout = output

		logger.DebugContext(cf.Context, "Running automatic az login", "command", logutils.StringerAttr(cmd))
		if err := cf.RunCommand(cmd); err != nil {
			return trace.Wrap(err, "failed to automatically login with `az login` using identity %q; run with --debug for details", routeToApp.AzureIdentity)
		}

		return azureLoginTemplate.Execute(output, map[string]string{
			"appName":  app.GetName(),
			"identity": routeToApp.AzureIdentity,
		})

	case app.IsGCP():
		return gcpLoginTemplate.Execute(output, map[string]string{
			"appName":        app.GetName(),
			"serviceAccount": routeToApp.GCPServiceAccount,
		})

	case app.IsTCP():
		appNameWithOptionalTargetPort := app.GetName()
		if routeToApp.TargetPort != 0 {
			appNameWithOptionalTargetPort = fmt.Sprintf("%s:%d", app.GetName(), routeToApp.TargetPort)
		}

		return tcpAppLoginTemplate.Execute(output, map[string]string{
			"appName":                       app.GetName(),
			"appNameWithOptionalTargetPort": appNameWithOptionalTargetPort,
		})

	case localProxyRequiredForApp(tc):
		return webAppLoginProxyTemplate.Execute(output, map[string]interface{}{
			"appName": app.GetName(),
		})

	default:
		rootCluster, err := tc.RootClusterName(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}

		// for remote apps, override their public address with address pointing at the public proxy address.
		if rootCluster != tc.SiteName {
			routeToApp.PublicAddr = fmt.Sprintf("%v.%v", app.GetName(), tc.WebProxyHost())
		}

		profile, err := tc.ProfileStatus()
		if err != nil {
			return trace.Wrap(err)
		}
		curlCmd, err := formatAppConfig(tc, profile, routeToApp, appFormatCURL)
		if err != nil {
			return trace.Wrap(err)
		}
		return webAppLoginTemplate.Execute(output, map[string]interface{}{
			"appName":  app.GetName(),
			"curlCmd":  curlCmd,
			"insecure": cf.InsecureSkipVerify,
		})
	}
}

// webAppLoginTemplate is the message that gets printed to a user upon successful login
// into an HTTP application.
var webAppLoginTemplate = template.Must(template.New("").Parse(
	`Logged into app {{.appName}}. Example curl command:

{{.curlCmd}}{{ if .insecure }}

WARNING: tsh was called with --insecure, so this curl command will be unable to validate the certificate presented by Teleport.
{{- end }}
`))

// webAppLoginProxyTemplate is the message that gets printed to a user upon successful login
// into an HTTP application and local proxy is required.
var webAppLoginProxyTemplate = template.Must(template.New("").Parse(
	`Logged into app {{.appName}}. Start the local proxy for it:

  tsh proxy app {{.appName}} -p 8080

Then connect to the application through this proxy:

  curl http://127.0.0.1:8080
`))

// tcpAppLoginTemplate is the message that gets printed to a user upon successful
// login into a TCP application.
var tcpAppLoginTemplate = template.Must(template.New("").Parse(
	`Logged into TCP app {{.appNameWithOptionalTargetPort}}. Start the local TCP proxy for it:

  tsh proxy app {{.appName}}

Then connect to the application through this proxy.
`))

// awsLoginTemplate is the message that gets printed to a user upon successful login
// into an AWS Console application.
var awsLoginTemplate = template.Must(template.New("").Parse(
	`Logged into AWS app "{{.awsAppName}}".

Your IAM role:
  {{.awsRoleARN}}

Example AWS CLI command:
  tsh aws {{.awsCmd}}

Or start a local proxy:
  tsh proxy aws --app {{.awsAppName}}
`))

// azureLoginTemplate is the message that gets printed to a user upon successful login
// into an Azure application.
var azureLoginTemplate = template.Must(template.New("").Parse(
	`Logged into Azure app "{{.appName}}".
Your identity: {{.identity}}
Example Azure CLI command: tsh az vm list
`))

// gcpLoginTemplate is the message that gets printed to a user upon successful login
// into a GCP application.
var gcpLoginTemplate = template.Must(template.New("").Parse(
	`Logged into GCP app "{{.appName}}".
Your service account: {{.serviceAccount}}
Example command: tsh gcloud compute instances list
`))

// onAppLogout implements "tsh apps logout" command.
func onAppLogout(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	activeRoutes, err := profile.AppsForCluster(tc.SiteName, tc.ClientStore)
	if err != nil {
		return trace.Wrap(err)
	}

	// If a specific app name was specified, just log out of that app.
	// Otherwise, log out of all apps.
	var logout []tlsca.RouteToApp
	if cf.AppName != "" {
		for _, app := range activeRoutes {
			if app.Name == cf.AppName {
				logout = append(logout, app)
			}
		}

		if len(logout) == 0 {
			// Not logged in but still try to delete any dangling files.
			if err := tc.LogoutApp(cf.AppName); err != nil {
				return trace.Wrap(err)
			}
			return trace.BadParameter("not logged into app %q", cf.AppName)
		}
	} else {
		logout = activeRoutes
	}

	for _, app := range logout {
		err = tc.DeleteAppSession(cf.Context, app.SessionID)
		if err != nil && !trace.IsNotFound(err) {
			return trace.Wrap(err)
		}

		err = tc.LogoutApp(app.Name)
		if err != nil {
			return trace.Wrap(err)
		}

		// remove generated local files for the provided app.
		err := utils.RemoveFileIfExist(profile.AppLocalCAPath(tc.SiteName, app.Name))
		if err != nil {
			logger.WarnContext(cf.Context, "Failed to clean up app session",
				"error", err,
				"profile", profile.AppLocalCAPath(tc.SiteName, app.Name))
		}
	}

	if cf.AppName == "" {
		// Try to delete any dangling files even if the app sessions are expired.
		if err := tc.LogoutAllApps(); err != nil {
			return trace.Wrap(err)
		}
	}

	if len(logout) == 1 {
		fmt.Printf("Logged out of app %q\n", logout[0].Name)
	} else {
		fmt.Println("Logged out of all apps")
	}
	return nil
}

// onAppConfig implements "tsh apps config" command.
func onAppConfig(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	routes, err := profile.AppsForCluster(tc.SiteName, tc.ClientStore)
	if err != nil {
		return trace.Wrap(err)
	}
	app, err := pickActiveApp(cf, routes)
	if err != nil {
		return trace.Wrap(err)
	}
	routeToApp := proto.RouteToApp{
		Name:              app.Name,
		PublicAddr:        app.PublicAddr,
		ClusterName:       app.ClusterName,
		AWSRoleARN:        app.AWSRoleARN,
		AzureIdentity:     app.AzureIdentity,
		GCPServiceAccount: app.GCPServiceAccount,
		URI:               app.GetURI(),
	}
	conf, err := formatAppConfig(tc, profile, routeToApp, cf.Format)
	if err != nil {
		return trace.Wrap(err)
	}
	_, _ = fmt.Fprint(cf.Stdout(), conf)
	return nil
}

const (
	// appFormatURI prints app URI.
	appFormatURI = "uri"
	// appFormatCA prints app CA cert path.
	appFormatCA = "ca"
	// appFormatCert prints app cert path.
	appFormatCert = "cert"
	// appFormatKey prints app key path.
	appFormatKey = "key"
	// appFormatCURL prints app curl command.
	appFormatCURL = "curl"
	// appFormatJSON prints app URI, CA cert path, cert path, key path, and curl command in JSON format.
	appFormatJSON = "json"
	// appFormatYAML prints app URI, CA cert path, cert path, key path, and curl command in YAML format.
	appFormatYAML = "yaml"
)

func formatAppConfig(tc *client.TeleportClient, profile *client.ProfileStatus, routeToApp proto.RouteToApp, format string) (string, error) {
	var uri string
	if port := tc.WebProxyPort(); port == teleport.StandardHTTPSPort {
		uri = fmt.Sprintf("https://%v", routeToApp.PublicAddr)
	} else {
		uri = fmt.Sprintf("https://%v:%v", routeToApp.PublicAddr, port)
	}

	var curlInsecureFlag string
	if tc.InsecureSkipVerify {
		curlInsecureFlag = "--insecure "
	}

	certPath := profile.AppCertPath(tc.SiteName, routeToApp.Name)
	keyPath := profile.AppKeyPath(tc.SiteName, routeToApp.Name)

	curlCmd := fmt.Sprintf(`curl %s\
  --cert %q \
  --key %q \
  %v`,
		curlInsecureFlag,
		certPath,
		keyPath,
		uri)
	format = strings.ToLower(format)
	switch format {
	case appFormatURI:
		return uri, nil
	case appFormatCA:
		return profile.CACertPathForCluster(tc.SiteName), nil
	case appFormatCert:
		return certPath, nil
	case appFormatKey:
		return keyPath, nil
	case appFormatCURL:
		return curlCmd, nil
	case appFormatJSON, appFormatYAML:
		appConfig := &appConfigInfo{
			Name:              routeToApp.Name,
			URI:               uri,
			CA:                profile.CACertPathForCluster(tc.SiteName),
			Cert:              certPath,
			Key:               keyPath,
			Curl:              curlCmd,
			AWSRoleARN:        routeToApp.AWSRoleARN,
			AzureIdentity:     routeToApp.AzureIdentity,
			GCPServiceAccount: routeToApp.GCPServiceAccount,
		}
		out, err := serializeAppConfig(appConfig, format)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return out, nil
	case "", "default":
		t := asciitable.MakeHeadlessTable(2)
		// additional spaces after `Name:` are there to enforce minimum column width,
		// which helps to visually separate the two columns.
		t.AddRow([]string{"Name:     ", routeToApp.Name})
		t.AddRow([]string{"URI:", uri})
		t.AddRow([]string{"CA:", profile.CACertPathForCluster(tc.SiteName)})
		t.AddRow([]string{"Cert:", certPath})
		t.AddRow([]string{"Key:", keyPath})

		if routeToApp.AWSRoleARN != "" {
			t.AddRow([]string{"AWS ARN:", routeToApp.AWSRoleARN})
		}
		if routeToApp.AzureIdentity != "" {
			t.AddRow([]string{"Azure Id:", routeToApp.AzureIdentity})
		}
		if routeToApp.GCPServiceAccount != "" {
			t.AddRow([]string{"GCP Service Account:", routeToApp.GCPServiceAccount})
		}

		return t.AsBuffer().String(), nil
	default:
		acceptedFormats := []string{
			"", "default",
			appFormatCURL,
			appFormatJSON, appFormatYAML,
			appFormatURI, appFormatCA, appFormatCert, appFormatKey,
		}
		return "", trace.BadParameter("invalid format, expected one of %q, got %q", acceptedFormats, format)
	}
}

type appConfigInfo struct {
	Name              string `json:"name"`
	URI               string `json:"uri"`
	CA                string `json:"ca"`
	Cert              string `json:"cert"`
	Key               string `json:"key"`
	Curl              string `json:"curl"`
	AWSRoleARN        string `json:"aws_role_arn,omitempty"`
	AzureIdentity     string `json:"azure_identity,omitempty"`
	GCPServiceAccount string `json:"gcp_service_account,omitempty"`
}

func serializeAppConfig(configInfo *appConfigInfo, format string) (string, error) {
	var out []byte
	var err error
	if format == appFormatJSON {
		out, err = utils.FastMarshalIndent(configInfo, "", "  ")
		// This JSON marshaling returns a string without a newline at the end, which
		// makes display of the string look wonky. Let's append it here.
		out = append(out, '\n')
	} else {
		// The YAML marshaling does return a string with a newline, so no need to append
		// another.
		out, err = yaml.Marshal(configInfo)
	}
	return string(out), trace.Wrap(err)
}

// getAppInfo fetches app information using the user's tsh profile,
// command line args, and the list resources endpoint if necessary. If
// provided, the matcher will be used to filter active apps in the
// tsh profile.
func getAppInfo(cf *CLIConf, clt authclient.ClientI, profile *client.ProfileStatus, siteName string, matchRouteToApp func(tlsca.RouteToApp) bool) (*appInfo, error) {
	activeRoutes := profile.Apps
	if matchRouteToApp != nil {
		var filteredRoutes []tlsca.RouteToApp
		for _, route := range profile.Apps {
			if matchRouteToApp(route) {
				filteredRoutes = append(filteredRoutes, route)
			}
		}
		activeRoutes = filteredRoutes
	}

	if route, err := pickActiveApp(cf, activeRoutes); err == nil {
		return &appInfo{
			profile:    profile,
			RouteToApp: route,
			isActive:   true,
		}, nil
	} else if !trace.IsNotFound(err) {
		// pickActiveApp errors are non-retryable.
		return nil, trace.Wrap(&client.NonRetryableError{Err: err})
	}

	// If we didn't find an active profile for the app, get info from server.
	app, logins, err := getApp(cf.Context, clt, cf.AppName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if len(logins) == 0 && app.IsAWSConsole() {
		logins = getARNFromRoles(cf, clt, profile, siteName, app)
	}

	appInfo := &appInfo{
		profile: profile,
		RouteToApp: proto.RouteToApp{
			Name:        app.GetName(),
			PublicAddr:  app.GetPublicAddr(),
			ClusterName: siteName,
			URI:         app.GetURI(),
		},
		app: app,
	}

	// When getAppInfo gets called inside RetryWithRelogin, it will relogin on
	// trace.BadParameter errors. Wrap errors from pickCloudAppLogin as they
	// are not retryable.
	if err := appInfo.pickCloudAppLogin(cf, logins); err != nil {
		return nil, trace.Wrap(&client.NonRetryableError{Err: err})
	}
	return appInfo, nil
}

// pickCloudAppLogin picks the cloud identity for the app based on provided CLI
// flags and/or available logins of the Teleport user.
func (a *appInfo) pickCloudAppLogin(cf *CLIConf, logins []string) error {
	// If this is a cloud app, set additional applicable fields from CLI flags or roles.
	switch {
	case a.app.IsAWSConsole():
		awsRoleARN, err := getARNFromFlags(cf, a.app, logins)
		if err != nil {
			return trace.Wrap(err)
		}
		a.AWSRoleARN = awsRoleARN

	case a.app.IsAzureCloud():
		azureIdentity, err := getAzureIdentityFromFlags(cf, a.profile)
		if err != nil {
			return trace.Wrap(err)
		}
		logger.DebugContext(cf.Context, "Retrieved azure identity", "azure_identity", azureIdentity)
		a.AzureIdentity = azureIdentity

	case a.app.IsGCP():
		gcpServiceAccount, err := getGCPServiceAccountFromFlags(cf, a.profile)
		if err != nil {
			return trace.Wrap(err)
		}
		logger.DebugContext(cf.Context, "Retrieved GCP service account", "service_account", gcpServiceAccount)
		a.GCPServiceAccount = gcpServiceAccount
	}

	return nil
}

// appInfo wraps a RouteToApp and the corresponding app.
// Its purpose is to prevent repeated fetches of the same app,
// by lazily fetching and caching the app for use as needed.
type appInfo struct {
	proto.RouteToApp
	// app corresponds to the app route and may be nil, so use GetApp
	// instead of accessing it directly.
	app   types.Application
	appMu sync.Mutex
	// isActive indicates an active app matched this app info.
	isActive bool

	// profile is a cached profile status for the current login session.
	profile *client.ProfileStatus
}

// GetApp returns the cached app or fetches it using the app route and
// caches the result.
func (a *appInfo) GetApp(ctx context.Context, clt apiclient.GetResourcesClient) (types.Application, error) {
	a.appMu.Lock()
	defer a.appMu.Unlock()
	if a.app != nil {
		return a.app.Copy(), nil
	}
	// holding mutex across the api call to avoid multiple redundant api calls.
	app, _, err := getApp(ctx, clt, a.Name)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	a.app = app
	return a.app.Copy(), nil
}

// getApp returns the registered application with the specified name.
func getApp(ctx context.Context, clt apiclient.GetResourcesClient, name string) (app types.Application, logins []string, err error) {
	// When listing a single app we only need to grab one page.
	res, err := apiclient.GetEnrichedResourcePage(ctx, clt, &proto.ListResourcesRequest{
		ResourceType:        types.KindAppServer,
		SortBy:              types.SortBy{Field: types.ResourceMetadataName},
		PredicateExpression: fmt.Sprintf(`name == "%s"`, name),
		Limit:               1,
		IncludeLogins:       true,
	})
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	if len(res.Resources) == 0 {
		return nil, nil, trace.NotFound("app %q not found, use `tsh apps ls` to see registered apps", name)
	}

	appServer, ok := res.Resources[0].ResourceWithLabels.(types.AppServer)
	if !ok {
		logger.WarnContext(ctx, "expected types.AppServer but received unexpected type", "resource_type", logutils.TypeAttr(res.Resources[0].ResourceWithLabels))
		return nil, nil, trace.NotFound("app %q not found, use `tsh apps ls` to see registered apps", name)
	}

	return appServer.GetApp(), res.Resources[0].Logins, nil
}

// pickActiveApp returns the app the current profile is logged into.
//
// If logged into multiple apps, returns an error unless one was specified
// explicitly on CLI.
func pickActiveApp(cf *CLIConf, activeRoutes []tlsca.RouteToApp) (proto.RouteToApp, error) {
	if cf.AppName == "" {
		switch len(activeRoutes) {
		case 0:
			return proto.RouteToApp{}, trace.NotFound("please login using 'tsh apps login' first")
		case 1:
			return tlscaRouteToAppToProto(activeRoutes[0]), nil
		default:
			var appNames []string
			for _, r := range activeRoutes {
				appNames = append(appNames, r.Name)
			}
			return proto.RouteToApp{}, trace.BadParameter("multiple apps are available (%v), please specify one via CLI argument",
				strings.Join(appNames, ", "))
		}
	}

	for _, r := range activeRoutes {
		if r.Name == cf.AppName {
			return tlscaRouteToAppToProto(r), nil
		}
	}
	return proto.RouteToApp{}, trace.NotFound("not logged into app %q", cf.AppName)
}

func tlscaRouteToAppToProto(route tlsca.RouteToApp) proto.RouteToApp {
	return proto.RouteToApp{
		Name:              route.Name,
		PublicAddr:        route.PublicAddr,
		ClusterName:       route.ClusterName,
		AWSRoleARN:        route.AWSRoleARN,
		AzureIdentity:     route.AzureIdentity,
		GCPServiceAccount: route.GCPServiceAccount,
		URI:               route.URI,
	}
}
