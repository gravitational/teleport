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
	"crypto/tls"
	"crypto/x509/pkix"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/ghodss/yaml"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/asciitable"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// onAppLogin implements "tsh apps login" command.
func onAppLogin(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	app, err := getRegisteredApp(cf, tc)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	rootCluster, err := tc.RootClusterName(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	var awsRoleARN string
	if app.IsAWSConsole() {
		var err error
		awsRoleARN, err = getARNFromFlags(cf, profile, app)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	var azureIdentity string
	if app.IsAzureCloud() {
		var err error
		azureIdentity, err = getAzureIdentityFromFlags(cf, profile)
		if err != nil {
			return trace.Wrap(err)
		}
		log.Debugf("Azure identity is %q", azureIdentity)
	}

	var gcpServiceAccount string
	if app.IsGCP() {
		var err error
		gcpServiceAccount, err = getGCPServiceAccountFromFlags(cf, profile)
		if err != nil {
			return trace.Wrap(err)
		}
		log.Debugf("GCP service account is %q", gcpServiceAccount)
	}

	request := types.CreateAppSessionRequest{
		Username:          tc.Username,
		PublicAddr:        app.GetPublicAddr(),
		ClusterName:       tc.SiteName,
		AWSRoleARN:        awsRoleARN,
		AzureIdentity:     azureIdentity,
		GCPServiceAccount: gcpServiceAccount,
	}

	ws, err := tc.CreateAppSession(cf.Context, request)
	if err != nil {
		return trace.Wrap(err)
	}

	params := client.ReissueParams{
		RouteToCluster: profile.Cluster,
		RouteToApp: proto.RouteToApp{
			Name:              app.GetName(),
			SessionID:         ws.GetName(),
			PublicAddr:        app.GetPublicAddr(),
			ClusterName:       tc.SiteName,
			AWSRoleARN:        awsRoleARN,
			AzureIdentity:     azureIdentity,
			GCPServiceAccount: gcpServiceAccount,
		},
		AccessRequests: profile.ActiveRequests.AccessRequests,
	}

	err = tc.ReissueUserCerts(cf.Context, client.CertCacheKeep, params)
	if err != nil {
		return trace.Wrap(err)
	}

	output := cf.Stdout()
	if cf.Quiet {
		output = io.Discard
	}

	switch {
	case app.IsAWSConsole():
		return awsCliTpl.Execute(output, map[string]string{
			"awsAppName": app.GetName(),
			"awsCmd":     "s3 ls",
			"awsRoleARN": awsRoleARN,
		})

	case app.IsAzureCloud():
		if azureIdentity == "" {
			return trace.BadParameter("app is Azure Cloud but Azure identity is missing")
		}

		var args []string
		if cf.Debug {
			args = append(args, "--debug")
		}
		args = append(args, "az", "login", "--identity", "-u", azureIdentity)

		// automatically login with right identity.
		cmd := exec.Command(cf.executablePath, args...)
		cmd.Stdin = os.Stdin
		cmd.Stderr = cf.Stderr()
		cmd.Stdout = output

		log.Debugf("Running automatic az login: %v", cmd.String())
		err := cf.RunCommand(cmd)
		if err != nil {
			return trace.Wrap(err, "failed to automatically login with `az login` using identity %q; run with --debug for details", azureIdentity)
		}

		return azureCliTpl.Execute(output, map[string]string{
			"appName":  app.GetName(),
			"identity": azureIdentity,
		})

	case app.IsGCP():
		return gcpCliTpl.Execute(output, map[string]string{
			"appName":        app.GetName(),
			"serviceAccount": gcpServiceAccount,
		})

	case app.IsTCP():
		return appLoginTCPTpl.Execute(output, map[string]string{
			"appName": app.GetName(),
		})

	case localProxyRequiredForApp(tc):
		return appLoginLocalProxyTpl.Execute(output, map[string]interface{}{
			"appName": app.GetName(),
		})

	default:
		publicAddr := app.GetPublicAddr()
		// for remote apps, override their public address with address pointing at the public proxy address.
		if rootCluster != tc.SiteName {
			publicAddr = fmt.Sprintf("%v.%v", app.GetName(), tc.WebProxyHost())
		}
		curlCmd, err := formatAppConfig(tc, profile, app.GetName(), publicAddr, appFormatCURL, rootCluster, awsRoleARN, azureIdentity, gcpServiceAccount)
		if err != nil {
			return trace.Wrap(err)
		}
		return appLoginTpl.Execute(output, map[string]interface{}{
			"appName":  app.GetName(),
			"curlCmd":  curlCmd,
			"insecure": cf.InsecureSkipVerify,
		})
	}
}

func localProxyRequiredForApp(tc *client.TeleportClient) bool {
	return tc.TLSRoutingConnUpgradeRequired
}

// appLoginTpl is the message that gets printed to a user upon successful login
// into an HTTP application.
var appLoginTpl = template.Must(template.New("").Parse(
	`Logged into app {{.appName}}. Example curl command:

{{.curlCmd}}{{ if .insecure }}

WARNING: tsh was called with --insecure, so this curl command will be unable to validate the certificate presented by Teleport.
{{- end }}
`))

// appLoginLocalProxyTpl is the message that gets printed to a user upon successful login
// into an HTTP application and local proxy is required.
var appLoginLocalProxyTpl = template.Must(template.New("").Parse(
	`Logged into app {{.appName}}. Start the local proxy for it:

  tsh proxy app {{.appName}} -p 8080

Then connect to the application through this proxy:

  curl http://127.0.0.1:8080
`))

// appLoginTCPTpl is the message that gets printed to a user upon successful
// login into a TCP application.
var appLoginTCPTpl = template.Must(template.New("").Parse(
	`Logged into TCP app {{.appName}}. Start the local TCP proxy for it:

  tsh proxy app {{.appName}}

Then connect to the application through this proxy.
`))

// awsCliTpl is the message that gets printed to a user upon successful login
// into an AWS Console application.
var awsCliTpl = template.Must(template.New("").Parse(
	`Logged into AWS app "{{.awsAppName}}".

Your IAM role:
  {{.awsRoleARN}}

Example AWS CLI command:
  tsh aws {{.awsCmd}}

Or start a local proxy:
  tsh proxy aws --app {{.awsAppName}}
`))

// azureCliTpl is the message that gets printed to a user upon successful login
// into an Azure application.
var azureCliTpl = template.Must(template.New("").Parse(
	`Logged into Azure app "{{.appName}}".
Your identity: {{.identity}}
Example Azure CLI command: tsh az vm list
`))

// gcpCliTpl is the message that gets printed to a user upon successful login
// into a GCP application.
var gcpCliTpl = template.Must(template.New("").Parse(
	`Logged into GCP app "{{.appName}}".
Your service account: {{.serviceAccount}}
Example command: tsh gcloud compute instances list
`))

// getRegisteredApp returns the registered application with the specified name.
func getRegisteredApp(cf *CLIConf, tc *client.TeleportClient) (app types.Application, err error) {
	var apps []types.Application
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		apps, err = tc.ListApps(cf.Context, &proto.ListResourcesRequest{
			Namespace:           tc.Namespace,
			ResourceType:        types.KindAppServer,
			PredicateExpression: fmt.Sprintf(`name == "%s"`, cf.AppName),
		})
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(apps) == 0 {
		return nil, trace.NotFound("app %q not found, use `tsh apps ls` to see registered apps", cf.AppName)
	}
	return apps[0], nil
}

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
	var logout []tlsca.RouteToApp
	// If app name wasn't given on the command line, log out of all.
	if cf.AppName == "" {
		logout = profile.Apps
	} else {
		for _, app := range profile.Apps {
			if app.Name == cf.AppName {
				logout = append(logout, app)
			}
		}
		if len(logout) == 0 {
			return trace.BadParameter("not logged into app %q",
				cf.AppName)
		}
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

		removeAppLocalFiles(profile, app.Name)
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
	app, err := pickActiveApp(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	conf, err := formatAppConfig(tc, profile, app.Name, app.PublicAddr, cf.Format, "", app.AWSRoleARN, app.AzureIdentity, app.GCPServiceAccount)
	if err != nil {
		return trace.Wrap(err)
	}
	_, _ = fmt.Fprint(cf.Stdout(), conf)
	return nil
}

func formatAppConfig(tc *client.TeleportClient, profile *client.ProfileStatus, appName, appPublicAddr, format, cluster, awsARN, azureIdentity, gcpServiceAccount string) (string, error) {
	var uri string
	if port := tc.WebProxyPort(); port == teleport.StandardHTTPSPort {
		uri = fmt.Sprintf("https://%v", appPublicAddr)
	} else {
		uri = fmt.Sprintf("https://%v:%v", appPublicAddr, port)
	}

	var curlInsecureFlag string
	if tc.InsecureSkipVerify {
		curlInsecureFlag = "--insecure "
	}

	curlCmd := fmt.Sprintf(`curl %s\
  --cert %v \
  --key %v \
  %v`,
		curlInsecureFlag,
		profile.AppCertPath(appName),
		profile.KeyPath(),
		uri)
	format = strings.ToLower(format)
	switch format {
	case appFormatURI:
		return uri, nil
	case appFormatCA:
		return profile.CACertPathForCluster(cluster), nil
	case appFormatCert:
		return profile.AppCertPath(appName), nil
	case appFormatKey:
		return profile.KeyPath(), nil
	case appFormatCURL:
		return curlCmd, nil
	case appFormatJSON, appFormatYAML:
		appConfig := &appConfigInfo{
			Name:              appName,
			URI:               uri,
			CA:                profile.CACertPathForCluster(cluster),
			Cert:              profile.AppCertPath(appName),
			Key:               profile.KeyPath(),
			Curl:              curlCmd,
			AWSRoleARN:        awsARN,
			AzureIdentity:     azureIdentity,
			GCPServiceAccount: gcpServiceAccount,
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
		t.AddRow([]string{"Name:     ", appName})
		t.AddRow([]string{"URI:", uri})
		t.AddRow([]string{"CA:", profile.CACertPathForCluster(cluster)})
		t.AddRow([]string{"Cert:", profile.AppCertPath(appName)})
		t.AddRow([]string{"Key:", profile.KeyPath()})

		if awsARN != "" {
			t.AddRow([]string{"AWS ARN:", awsARN})
		}
		if azureIdentity != "" {
			t.AddRow([]string{"Azure Id:", azureIdentity})
		}
		if gcpServiceAccount != "" {
			t.AddRow([]string{"GCP Service Account:", gcpServiceAccount})
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

// pickActiveApp returns the app the current profile is logged into.
//
// If logged into multiple apps, returns an error unless one was specified
// explicitly on CLI.
func pickActiveApp(cf *CLIConf) (*tlsca.RouteToApp, error) {
	profile, err := cf.ProfileStatus()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(profile.Apps) == 0 {
		return nil, trace.NotFound("please login using 'tsh apps login' first")
	}
	name := cf.AppName
	if name == "" {
		apps := profile.AppNames()
		if len(apps) > 1 {
			return nil, trace.BadParameter("multiple apps are available (%v), please specify one via CLI argument",
				strings.Join(apps, ", "))
		}
		name = apps[0]
	}
	for _, app := range profile.Apps {
		if app.Name == name {
			return &app, nil
		}
	}
	return nil, trace.NotFound("not logged into app %q", name)
}

// removeAppLocalFiles removes generated local files for the provided app.
func removeAppLocalFiles(profile *client.ProfileStatus, appName string) {
	err := utils.RemoveFileIfExist(profile.AppLocalCAPath(appName))
	if err != nil {
		log.WithError(err).Warnf("Failed to remove %v", profile.AppLocalCAPath(appName))
	}
}

// loadAppSelfSignedCA loads self-signed CA for provided app, or tries to
// generate a new CA if first load fails.
func loadAppSelfSignedCA(profile *client.ProfileStatus, tc *client.TeleportClient, appName string) (tls.Certificate, error) {
	appCerts, _, err := loadAppCertificate(tc, appName)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	appCertsExpireAt, err := getTLSCertExpireTime(appCerts)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	cert, err := loadSelfSignedCA(profile.AppLocalCAPath(appName), profile.KeyPath(), appCertsExpireAt, "localhost")
	return cert, trace.Wrap(err)
}

func loadSelfSignedCA(caPath, keyPath string, validUntil time.Time, dnsNames ...string) (tls.Certificate, error) {
	caTLSCert, err := keys.LoadX509KeyPair(caPath, keyPath)
	if err == nil {
		if expire, err := getTLSCertExpireTime(caTLSCert); err == nil && time.Now().Before(expire) {
			return caTLSCert, nil
		}
	}
	if err != nil && !trace.IsNotFound(err) {
		log.WithError(err).Debugf("Failed to load certificate from %v.", caPath)
	}

	// Generate and load again.
	if err = generateSelfSignedCA(caPath, keyPath, validUntil, dnsNames...); err != nil {
		return tls.Certificate{}, err
	}

	caTLSCert, err = keys.LoadX509KeyPair(caPath, keyPath)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return caTLSCert, nil
}

// generateSelfSignedCA generates a new self-signed CA for provided dnsNames
// and saves/overwrites the local CA file in the profile directory.
func generateSelfSignedCA(caPath, keyPath string, validUntil time.Time, dnsNames ...string) error {
	log.Debugf("Generating local self signed CA at %v", caPath)
	keyPem, err := utils.ReadPath(keyPath)
	if err != nil {
		return trace.Wrap(err)
	}

	key, err := keys.ParsePrivateKey(keyPem)
	if err != nil {
		return trace.Wrap(err)
	}

	certPem, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		Entity: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"Teleport"},
		},
		Signer:      key,
		DNSNames:    dnsNames,
		IPAddresses: []net.IP{net.ParseIP(defaults.Localhost)},
		TTL:         time.Until(validUntil),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if _, err := utils.EnsureLocalPath(caPath, "", ""); err != nil {
		return trace.Wrap(err)
	}

	// WriteFile truncates existing file before writing.
	if err = os.WriteFile(caPath, certPem, 0600); err != nil {
		return trace.ConvertSystemError(err)
	}
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
