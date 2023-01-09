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
	"crypto/tls"
	"crypto/x509/pkix"
	"fmt"
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
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// onAppLogin implements "tsh app login" command.
func onAppLogin(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
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

	var arn string
	if app.IsAWSConsole() {
		var err error
		arn, err = getARNFromFlags(cf, profile, app)
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

	request := types.CreateAppSessionRequest{
		Username:      tc.Username,
		PublicAddr:    app.GetPublicAddr(),
		ClusterName:   tc.SiteName,
		AWSRoleARN:    arn,
		AzureIdentity: azureIdentity,
	}

	ws, err := tc.CreateAppSession(cf.Context, request)
	if err != nil {
		return trace.Wrap(err)
	}

	params := client.ReissueParams{
		RouteToCluster: tc.SiteName,
		RouteToApp: proto.RouteToApp{
			Name:          app.GetName(),
			SessionID:     ws.GetName(),
			PublicAddr:    app.GetPublicAddr(),
			ClusterName:   tc.SiteName,
			AWSRoleARN:    arn,
			AzureIdentity: azureIdentity,
		},
		AccessRequests: profile.ActiveRequests.AccessRequests,
	}

	err = tc.ReissueUserCerts(cf.Context, client.CertCacheKeep, params)
	if err != nil {
		return trace.Wrap(err)
	}

	if err := tc.SaveProfile(true); err != nil {
		return trace.Wrap(err)
	}
	if app.IsAWSConsole() {
		return awsCliTpl.Execute(os.Stdout, map[string]string{
			"awsAppName": app.GetName(),
			"awsCmd":     "s3 ls",
		})
	}
	if app.IsAzureCloud() {
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
		cmd.Stderr = os.Stderr
		cmd.Stdout = os.Stdout

		log.Debugf("Running automatic az login: %v", cmd.String())
		err := cf.RunCommand(cmd)
		if err != nil {
			return trace.Wrap(err, "failed to automatically login with `az login` using identity %q; run with --debug for details", azureIdentity)
		}

		return azureCliTpl.Execute(os.Stdout, map[string]string{
			"appName":  app.GetName(),
			"identity": azureIdentity,
		})
	}
	if app.IsTCP() {
		return appLoginTCPTpl.Execute(os.Stdout, map[string]string{
			"appName": app.GetName(),
		})
	}
	curlCmd, err := formatAppConfig(tc, profile, app.GetName(), app.GetPublicAddr(), appFormatCURL, rootCluster, arn, azureIdentity)
	if err != nil {
		return trace.Wrap(err)
	}
	return appLoginTpl.Execute(os.Stdout, map[string]interface{}{
		"appName":  app.GetName(),
		"curlCmd":  curlCmd,
		"insecure": cf.InsecureSkipVerify,
	})
}

// appLoginTpl is the message that gets printed to a user upon successful login
// into an HTTP application.
var appLoginTpl = template.Must(template.New("").Parse(
	`Logged into app {{.appName}}. Example curl command:

{{.curlCmd}}{{ if .insecure }}

WARNING: tsh was called with --insecure, so this curl command will be unable to validate the certificate presented by Teleport.
{{- end }}
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
	`Logged into AWS app {{.awsAppName}}. Example AWS CLI command:

  tsh aws {{.awsCmd}}
`))

// azureCliTpl is the message that gets printed to a user upon successful login
// into an Azure application.
var azureCliTpl = template.Must(template.New("").Parse(
	`Logged into Azure app "{{.appName}}".
Your identity: {{.identity}}
Example Azure CLI command: tsh az vm list
`))

// getRegisteredApp returns the registered application with the specified name.
func getRegisteredApp(cf *CLIConf, tc *client.TeleportClient) (app types.Application, err error) {
	var apps []types.Application
	err = client.RetryWithRelogin(cf.Context, tc, func() error {
		apps, err = tc.ListApps(cf.Context, &proto.ListResourcesRequest{
			Namespace:           tc.Namespace,
			PredicateExpression: fmt.Sprintf(`name == "%s"`, cf.AppName),
		})
		return trace.Wrap(err)
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(apps) == 0 {
		return nil, trace.NotFound("app %q not found, use `tsh app ls` to see registered apps", cf.AppName)
	}
	return apps[0], nil
}

// onAppLogout implements "tsh app logout" command.
func onAppLogout(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
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

// onAppConfig implements "tsh app config" command.
func onAppConfig(cf *CLIConf) error {
	tc, err := makeClient(cf, false)
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
	conf, err := formatAppConfig(tc, profile, app.Name, app.PublicAddr, cf.Format, "", app.AWSRoleARN, app.AzureIdentity)
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Print(conf)
	return nil
}

func formatAppConfig(tc *client.TeleportClient, profile *client.ProfileStatus, appName, appPublicAddr, format, cluster, awsARN, azureIdentity string) (string, error) {
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
			Name:          appName,
			URI:           uri,
			CA:            profile.CACertPathForCluster(cluster),
			Cert:          profile.AppCertPath(appName),
			Key:           profile.KeyPath(),
			Curl:          curlCmd,
			AWSRoleARN:    awsARN,
			AzureIdentity: azureIdentity,
		}
		out, err := serializeAppConfig(appConfig, format)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return out, nil
	case "", "default":
		cfg := fmt.Sprintf(`Name:      %v
URI:       %v
CA:        %v
Cert:      %v
Key:       %v
`, appName, uri, profile.CACertPathForCluster(cluster),
			profile.AppCertPath(appName), profile.KeyPath())
		if awsARN != "" {
			cfg = cfg + fmt.Sprintf("AWS ARN:   %v\n", awsARN)
		}
		if azureIdentity != "" {
			cfg = cfg + fmt.Sprintf("Azure Id:  %v\n", azureIdentity)
		}
		return cfg, nil
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
	Name          string `json:"name"`
	URI           string `json:"uri"`
	CA            string `json:"ca"`
	Cert          string `json:"cert"`
	Key           string `json:"key"`
	Curl          string `json:"curl"`
	AWSRoleARN    string `json:"aws_role_arn,omitempty"`
	AzureIdentity string `json:"azure_identity,omitempty"`
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
		return nil, trace.NotFound("please login using 'tsh app login' first")
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
	removeFileIfExist(profile.AppLocalCAPath(appName))
}

// removeFileIfExist removes a local file if it exists.
func removeFileIfExist(filePath string) {
	if !utils.FileExists(filePath) {
		return
	}

	if err := os.Remove(filePath); err != nil {
		log.WithError(err).Warnf("Failed to remove %v", filePath)
	}
}

// loadAppSelfSignedCA loads self-signed CA for provided app, or tries to
// generate a new CA if first load fails.
func loadAppSelfSignedCA(profile *client.ProfileStatus, tc *client.TeleportClient, appName string) (tls.Certificate, error) {
	caPath := profile.AppLocalCAPath(appName)
	keyPath := profile.KeyPath()

	caTLSCert, err := keys.LoadX509KeyPair(caPath, keyPath)
	if err == nil {
		return caTLSCert, trace.Wrap(err)
	}

	// Generate and load again.
	log.WithError(err).Debugf("Failed to load certificate from %v. Generating local self signed CA.", caPath)
	if err = generateAppSelfSignedCA(profile, tc, appName); err != nil {
		return tls.Certificate{}, err
	}

	caTLSCert, err = keys.LoadX509KeyPair(caPath, keyPath)
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}
	return caTLSCert, nil
}

// generateAppSelfSignedCA generates a new self-signed CA for provided app and
// saves/overwrites the local CA file in the profile directory.
func generateAppSelfSignedCA(profile *client.ProfileStatus, tc *client.TeleportClient, appName string) error {
	appCerts, err := loadAppCertificate(tc, appName)
	if err != nil {
		return trace.Wrap(err)
	}

	appCertsExpireAt, err := getTLSCertExpireTime(appCerts)
	if err != nil {
		return trace.Wrap(err)
	}

	keyPem, err := utils.ReadPath(profile.KeyPath())
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
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP(defaults.Localhost)},
		TTL:         time.Until(appCertsExpireAt),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	// WriteFile truncates existing file before writing.
	if err = os.WriteFile(profile.AppLocalCAPath(appName), certPem, 0600); err != nil {
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
