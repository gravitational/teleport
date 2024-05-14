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
	"crypto/x509/pkix"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/db/dbcmd"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	alpncommon "github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
)

// onProxyCommandSSH creates a local ssh proxy, dialing a node and transferring
// data through stdin and stdout, to be used as an OpenSSH and PuTTY proxy
// command.
func onProxyCommandSSH(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(libclient.RetryWithRelogin(cf.Context, tc, func() error {
		clt, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}

		var target string
		switch {
		case tc.Host != "":
			targetHost, targetPort, err := net.SplitHostPort(tc.Host)
			if err != nil {
				targetHost = tc.Host
				targetPort = strconv.Itoa(tc.HostPort)
			}
			targetHost = cleanTargetHost(targetHost, tc.WebProxyHost(), clt.ClusterName())
			target = net.JoinHostPort(targetHost, targetPort)
		case len(tc.SearchKeywords) != 0 || tc.PredicateExpression != "":
			nodes, err := client.GetAllResources[types.Server](cf.Context, clt.AuthClient, tc.ResourceFilter(types.KindNode))
			if err != nil {
				return trace.Wrap(err)
			}

			if len(nodes) == 0 {
				return trace.NotFound("no matching SSH hosts found for search terms or query expression")
			}

			if len(nodes) > 1 {
				return trace.BadParameter("found multiple matching SSH hosts %v", nodes[:2])
			}

			// Dialing is happening by UUID but a port is still required by
			// the Proxy dial request. Zero is an indicator to the Proxy that
			// it may chose the appropriate port based on the target server.
			target = fmt.Sprintf("%s:0", nodes[0].GetName())
		default:
			return trace.BadParameter("no hostname, search terms or query expression provided")
		}

		conn, _, err := clt.DialHostWithResumption(cf.Context, target, clt.ClusterName(), tc.LocalAgent().ExtendedAgent)
		if err != nil {
			return trace.Wrap(err)
		}

		defer conn.Close()

		return trace.Wrap(utils.ProxyConn(cf.Context, utils.CombinedStdio{}, conn))
	}))
}

// cleanTargetHost cleans the targetHost and remote site and proxy suffixes.
// Before the `cut -d` command was used for this purpose but to support multi-platform OpenSSH clients the logic
// it was moved tsh proxy ssh command.
// For more details please look at: Generate Windows-compatible OpenSSH config https://github.com/gravitational/teleport/pull/7848
func cleanTargetHost(targetHost, proxyHost, siteName string) string {
	targetHost = strings.TrimSuffix(targetHost, "."+proxyHost)
	targetHost = strings.TrimSuffix(targetHost, "."+siteName)
	return targetHost
}

// formatCommand formats command making it suitable for the end user to copy the command and paste it into terminal.
func formatCommand(cmd *exec.Cmd) string {
	// environment variables
	env := strings.Join(cmd.Env, " ")

	var args []string
	for _, arg := range cmd.Args {
		// escape the potential quotes within
		arg = strings.Replace(arg, `"`, `\"`, -1)

		// if there is whitespace within, surround with quotes
		if strings.IndexFunc(arg, unicode.IsSpace) != -1 {
			args = append(args, fmt.Sprintf(`"%s"`, arg))
		} else {
			args = append(args, arg)
		}
	}

	argsfmt := strings.Join(args, " ")

	if len(env) > 0 {
		return fmt.Sprintf("%s %s", env, argsfmt)
	}

	return argsfmt
}

func onProxyCommandDB(cf *CLIConf) error {
	tc, err := makeClient(cf)
	if err != nil {
		return trace.Wrap(err)
	}
	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}
	routes, err := profile.DatabasesForCluster(tc.SiteName)
	if err != nil {
		return trace.Wrap(err)
	}
	dbInfo, err := getDatabaseInfo(cf, tc, routes)
	if err != nil {
		return trace.Wrap(err)
	}

	// When proxying without the `--tunnel` flag, we need to:
	// 1. check if --tunnel is required.
	// 2. check if db login is required.
	// These steps are not needed with `--tunnel`, because the local proxy tunnel
	// will manage database certificates itself and reissue them as needed.
	requires := getDBLocalProxyRequirement(tc, dbInfo.RouteToDatabase)
	if requires.tunnel && !cf.LocalProxyTunnel {
		// Some scenarios require a local proxy tunnel, e.g.:
		// - Snowflake, DynamoDB protocol
		// - Hardware-backed private key policy
		return trace.BadParameter(formatDbCmdUnsupported(cf, dbInfo.RouteToDatabase, requires.tunnelReasons...))
	}
	if err := maybeDatabaseLogin(cf, tc, profile, dbInfo, requires); err != nil {
		return trace.Wrap(err)
	}

	rootCluster, err := tc.RootClusterName(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	addr := "localhost:0"
	randomPort := true
	if cf.LocalProxyPort != "" {
		randomPort = false
		addr = fmt.Sprintf("127.0.0.1:%s", cf.LocalProxyPort)
	}

	listener, err := createLocalProxyListener(addr, dbInfo.RouteToDatabase, profile)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := listener.Close(); err != nil {
			log.WithError(err).Warnf("Failed to close listener.")
		}
	}()

	proxyOpts, err := prepareLocalProxyOptions(&localProxyConfig{
		cf:      cf,
		tc:      tc,
		profile: profile,
		dbInfo:  dbInfo,
		tunnel:  cf.LocalProxyTunnel,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	lp, err := alpnproxy.NewLocalProxy(makeBasicLocalProxyConfig(cf, tc, listener), proxyOpts...)
	if err != nil {
		return trace.Wrap(err)
	}
	go func() {
		<-cf.Context.Done()
		lp.Close()
	}()

	if cf.LocalProxyTunnel {
		addr, err := utils.ParseAddr(lp.GetAddr())
		if err != nil {
			return trace.Wrap(err)
		}
		opts := []dbcmd.ConnectCommandFunc{
			dbcmd.WithLocalProxy("localhost", addr.Port(0), ""),
			dbcmd.WithNoTLS(),
			dbcmd.WithLogger(log),
			dbcmd.WithPrintFormat(),
			dbcmd.WithTolerateMissingCLIClient(),
		}
		if opts, err = maybeAddDBUserPassword(cf, tc, dbInfo, opts); err != nil {
			return trace.Wrap(err)
		}
		if opts, err = maybeAddGCPMetadata(cf.Context, tc, dbInfo, opts); err != nil {
			return trace.Wrap(err)
		}

		commands, err := dbcmd.NewCmdBuilder(tc, profile, dbInfo.RouteToDatabase, rootCluster,
			opts...,
		).GetConnectCommandAlternatives()
		if err != nil {
			return trace.Wrap(err)
		}

		// shared template arguments
		templateArgs := map[string]any{
			"database":   dbInfo.ServiceName,
			"type":       defaults.ReadableDatabaseProtocol(dbInfo.Protocol),
			"cluster":    tc.SiteName,
			"address":    listener.Addr().String(),
			"randomPort": randomPort,
		}
		maybeAddGCPMetadataTplArgs(cf.Context, tc, dbInfo, templateArgs)

		tmpl := chooseProxyCommandTemplate(templateArgs, commands, dbInfo)
		err = tmpl.Execute(os.Stdout, templateArgs)
		if err != nil {
			return trace.Wrap(err)
		}

	} else {
		err = dbProxyTpl.Execute(os.Stdout, map[string]any{
			"database":     dbInfo.ServiceName,
			"address":      listener.Addr().String(),
			"ca":           profile.CACertPathForCluster(rootCluster),
			"cert":         profile.DatabaseCertPathForCluster(cf.SiteName, dbInfo.ServiceName),
			"key":          profile.KeyPath(),
			"randomPort":   randomPort,
			"databaseUser": dbInfo.Username,
			"databaseName": dbInfo.Database,
		})
		if err != nil {
			return trace.Wrap(err)
		}
	}

	defer lp.Close()
	if err := lp.Start(cf.Context); err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func maybeAddDBUserPassword(cf *CLIConf, tc *libclient.TeleportClient, dbInfo *databaseInfo, opts []dbcmd.ConnectCommandFunc) ([]dbcmd.ConnectCommandFunc, error) {
	if dbInfo.Protocol == defaults.ProtocolCassandra {
		db, err := dbInfo.GetDatabase(cf.Context, tc)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		if db.IsAWSHosted() {
			// Cassandra client always prompt for password, so we need to provide it
			// Provide an auto generated random password to skip the prompt in case of
			// connection to AWS hosted cassandra.
			password, err := utils.CryptoRandomHex(16)
			if err != nil {
				return nil, trace.Wrap(err)
			}
			return append(opts, dbcmd.WithPassword(password)), nil
		}
	}
	return opts, nil
}

func requiresGCPMetadata(protocol string) bool {
	return protocol == defaults.ProtocolSpanner
}

func maybeAddGCPMetadata(ctx context.Context, tc *libclient.TeleportClient, dbInfo *databaseInfo, opts []dbcmd.ConnectCommandFunc) ([]dbcmd.ConnectCommandFunc, error) {
	if !requiresGCPMetadata(dbInfo.Protocol) {
		return opts, nil
	}
	db, err := dbInfo.GetDatabase(ctx, tc)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	gcp := db.GetGCP()
	return append(opts, dbcmd.WithGCP(gcp)), nil
}

func maybeAddGCPMetadataTplArgs(ctx context.Context, tc *libclient.TeleportClient, dbInfo *databaseInfo, templateArgs map[string]any) {
	if !requiresGCPMetadata(dbInfo.Protocol) {
		return
	}
	templateArgs["gcpProject"] = "<project>"
	templateArgs["gcpInstance"] = "<instance>"
	db, err := dbInfo.GetDatabase(ctx, tc)
	if err == nil {
		gcp := db.GetGCP()
		templateArgs["gcpProject"] = gcp.ProjectID
		templateArgs["gcpInstance"] = gcp.InstanceID
	}
}

type templateCommandItem struct {
	Description string
	Command     string
}

func chooseProxyCommandTemplate(templateArgs map[string]any, commands []dbcmd.CommandAlternative, dbInfo *databaseInfo) *template.Template {
	// there is only one command, use plain template.
	if len(commands) == 1 {
		templateArgs["command"] = formatCommand(commands[0].Command)
		switch dbInfo.Protocol {
		case defaults.ProtocolOracle:
			templateArgs["args"] = commands[0].Command.Args
			return dbProxyOracleAuthTpl
		case defaults.ProtocolSpanner:
			templateArgs["databaseName"] = "<database>"
			if dbInfo.Database != "" {
				templateArgs["databaseName"] = dbInfo.Database
			}
			return dbProxySpannerAuthTpl
		}
		return dbProxyAuthTpl
	}

	// multiple command options, use a different template.

	var commandsArg []templateCommandItem
	for _, cmd := range commands {
		commandsArg = append(commandsArg, templateCommandItem{cmd.Description, formatCommand(cmd.Command)})
	}

	templateArgs["commands"] = commandsArg
	return dbProxyAuthMultiTpl
}

func alpnProtocolForApp(app types.Application) alpncommon.Protocol {
	if app.IsTCP() {
		return alpncommon.ProtocolTCP
	}
	return alpncommon.ProtocolHTTP
}

func onProxyCommandApp(cf *CLIConf) error {
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

	routeToApp, err := getRouteToApp(cf, tc, profile, app)
	if err != nil {
		return trace.Wrap(err)
	}

	opts := []alpnproxy.LocalProxyConfigOpt{
		alpnproxy.WithALPNProtocol(alpnProtocolForApp(app)),
		alpnproxy.WithClusterCAsIfConnUpgrade(cf.Context, tc.RootClusterCACertPool),
		alpnproxy.WithMiddleware(libclient.NewAppCertChecker(tc, routeToApp, nil)),
	}

	addr := "localhost:0"
	if cf.LocalProxyPort != "" {
		addr = fmt.Sprintf("127.0.0.1:%s", cf.LocalProxyPort)
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return trace.Wrap(err)
	}

	lp, err := alpnproxy.NewLocalProxy(makeBasicLocalProxyConfig(cf, tc, listener), opts...)
	if err != nil {
		if cerr := listener.Close(); cerr != nil {
			return trace.NewAggregate(err, cerr)
		}
		return trace.Wrap(err)
	}

	fmt.Printf("Proxying connections to %s on %v\n", cf.AppName, lp.GetAddr())
	if cf.LocalProxyPort == "" {
		fmt.Println("To avoid port randomization, you can choose the listening port using the --port flag.")
	}

	go func() {
		<-cf.Context.Done()
		lp.Close()
	}()

	defer lp.Close()
	if err = lp.Start(cf.Context); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// onProxyCommandAWS creates local proxes for AWS apps.
func onProxyCommandAWS(cf *CLIConf) error {
	if err := checkProxyAWSFormatCompatibility(cf); err != nil {
		return trace.Wrap(err)
	}

	awsApp, err := pickAWSApp(cf)
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

	if err := printProxyAWSTemplate(cf, awsApp); err != nil {
		return trace.Wrap(err)
	}
	<-cf.Context.Done()
	return nil
}

type awsAppInfo interface {
	GetAppName() string
	GetEnvVars() (map[string]string, error)
	GetEndpointURL() string
	GetForwardProxyAddr() string
}

func printProxyAWSTemplate(cf *CLIConf, awsApp awsAppInfo) error {
	envVars, err := awsApp.GetEnvVars()
	if err != nil {
		return trace.Wrap(err)
	}

	templateData := map[string]interface{}{
		"envVars":     envVars,
		"endpointURL": awsApp.GetEndpointURL(),
		"format":      cf.Format,
		"randomPort":  cf.LocalProxyPort == "",
		"appName":     awsApp.GetAppName(),
		"region":      getEnvOrDefault(awsRegionEnvVar, "<region>"),
		"keystore":    getEnvOrDefault(awsKeystoreEnvVar, "<keystore>"),
		"workgroup":   getEnvOrDefault(awsWorkgroupEnvVar, "<workgroup>"),
	}

	if proxyAddr := awsApp.GetForwardProxyAddr(); proxyAddr != "" {
		proxyHost, proxyPort, err := net.SplitHostPort(proxyAddr)
		if err != nil {
			return trace.Wrap(err)
		}
		templateData["proxyScheme"] = "http"
		templateData["proxyHost"] = proxyHost
		templateData["proxyPort"] = proxyPort
	}

	templates := []string{awsProxyHeaderTemplate}
	switch {
	case cf.Format == awsProxyFormatAthenaODBC:
		templates = append(templates, awsProxyAthenaODBCTemplate)
	case cf.Format == awsProxyFormatAthenaJDBC:
		templates = append(templates, awsProxyJDBCHeaderFooterTemplate, awsProxyAthenaJDBCTemplate)
	case cf.AWSEndpointURLMode:
		templates = append(templates, awsEndpointURLProxyTemplate)
	default:
		templates = append(templates, awsHTTPSProxyTemplate)
	}

	combined := template.New("").Funcs(cloudTemplateFuncs)
	for _, text := range templates {
		combined, err = combined.Parse(text)
		if err != nil {
			return trace.Wrap(err)
		}
	}

	return trace.Wrap(combined.Execute(cf.Stdout(), templateData))
}

func checkProxyAWSFormatCompatibility(cf *CLIConf) error {
	switch cf.Format {
	case awsProxyFormatAthenaODBC, awsProxyFormatAthenaJDBC:
		if cf.AWSEndpointURLMode {
			return trace.BadParameter("format %q is not supported in --endpoint-url mode", cf.Format)
		}
	}
	return nil
}

// onProxyCommandAzure creates local proxes for Azure apps.
func onProxyCommandAzure(cf *CLIConf) error {
	azApp, err := pickAzureApp(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = azApp.StartLocalProxies()
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := azApp.Close(); err != nil {
			log.WithError(err).Error("Failed to close Azure app.")
		}
	}()

	envVars, err := azApp.GetEnvVars()
	if err != nil {
		return trace.Wrap(err)
	}

	if err = printCloudTemplate(envVars, cf.Format, cf.LocalProxyPort == "", types.CloudAzure); err != nil {
		return trace.Wrap(err)
	}

	<-cf.Context.Done()
	return nil
}

// onProxyCommandGCloud creates local proxies for GCP apps.
func onProxyCommandGCloud(cf *CLIConf) error {
	gcpApp, err := pickGCPApp(cf)
	if err != nil {
		return trace.Wrap(err)
	}

	err = gcpApp.StartLocalProxies()
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := gcpApp.Close(); err != nil {
			log.WithError(err).Error("Failed to close GCP app.")
		}
	}()

	envVars, err := gcpApp.GetEnvVars()
	if err != nil {
		return trace.Wrap(err)
	}

	if err = printCloudTemplate(envVars, cf.Format, cf.LocalProxyPort == "", types.CloudGCP); err != nil {
		return trace.Wrap(err)
	}

	<-cf.Context.Done()
	return nil
}

// loadAppCertificateWithAppLogin is a wrapper around loadAppCertificate that will attempt to login the user to
// the app of choice at most once, if the return value from loadAppCertificate call indicates that app login
// should fix the problem.
func loadAppCertificateWithAppLogin(cf *CLIConf, tc *libclient.TeleportClient, appName string) (tls.Certificate, error) {
	cert, needLogin, err := loadAppCertificate(tc, appName)
	if err != nil {
		if !needLogin {
			return tls.Certificate{}, trace.Wrap(err)
		}
		log.WithError(err).Debugf("Loading app certificate failed, attempting to login into app %q", appName)
		quiet := cf.Quiet
		cf.Quiet = true
		errLogin := onAppLogin(cf)
		cf.Quiet = quiet
		if errLogin != nil {
			log.WithError(errLogin).Debugf("Login attempt failed")
			// combine errors
			return tls.Certificate{}, trace.NewAggregate(err, errLogin)
		}
		// another attempt
		cert, _, err = loadAppCertificate(tc, appName)
		return cert, trace.Wrap(err)
	}
	return cert, nil
}

// loadAppCertificate loads the app certificate for the provided app.
// Returns tuple (certificate, needLogin, err).
// The boolean `needLogin` will be true if the error returned should go away with successful `tsh app login <appName>`.
func loadAppCertificate(tc *libclient.TeleportClient, appName string) (certificate tls.Certificate, needLogin bool, err error) {
	key, err := tc.LocalAgent().GetKey(tc.SiteName, libclient.WithAppCerts{})
	if err != nil {
		return tls.Certificate{}, false, trace.Wrap(err)
	}

	appCert, err := key.AppTLSCert(appName)
	if trace.IsNotFound(err) {
		return tls.Certificate{}, true, trace.NotFound("please login into the application first: 'tsh apps login %v'", appName)
	} else if err != nil {
		return tls.Certificate{}, false, trace.Wrap(err)
	}

	expiresAt, err := getTLSCertExpireTime(appCert)
	if err != nil {
		return tls.Certificate{}, true, trace.WrapWithMessage(err, "invalid certificate - please login to the application again: 'tsh apps login %v'", appName)
	}
	if time.Until(expiresAt) < 5*time.Second {
		return tls.Certificate{}, true, trace.BadParameter(
			"application %s certificate has expired, please re-login to the app using 'tsh apps login %v'", appName,
			appName)
	}

	return appCert, false, nil
}

func loadDBCertificate(tc *libclient.TeleportClient, dbName string) (tls.Certificate, error) {
	key, err := tc.LocalAgent().GetKey(tc.SiteName, libclient.WithDBCerts{})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	dbCert, err := key.DBTLSCert(dbName)
	if trace.IsNotFound(err) {
		return tls.Certificate{}, trace.NotFound("please login into the database first. 'tsh db login'")
	} else if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	return dbCert, nil
}

// getTLSCertExpireTime returns the certificate NotAfter time.
func getTLSCertExpireTime(cert tls.Certificate) (time.Time, error) {
	x509cert, err := utils.TLSCertLeaf(cert)
	if err != nil {
		return time.Time{}, trace.Wrap(err)
	}
	return x509cert.NotAfter, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func makeBasicLocalProxyConfig(cf *CLIConf, tc *libclient.TeleportClient, listener net.Listener) alpnproxy.LocalProxyConfig {
	return alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:         tc.WebProxyAddr,
		InsecureSkipVerify:      cf.InsecureSkipVerify,
		ParentContext:           cf.Context,
		Listener:                listener,
		ALPNConnUpgradeRequired: tc.TLSRoutingConnUpgradeRequired,
	}
}

func generateDBLocalProxyCert(key *libclient.Key, profile *libclient.ProfileStatus) error {
	path := profile.DatabaseLocalCAPath()
	if utils.FileExists(path) {
		return nil
	}
	certPem, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		Entity: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"Teleport"},
		},
		Signer:      key,
		DNSNames:    []string{"localhost"},
		IPAddresses: []net.IP{net.ParseIP(defaults.Localhost)},
		TTL:         defaults.CATTL,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	if err := os.WriteFile(profile.DatabaseLocalCAPath(), certPem, teleport.FileMaskOwnerOnly); err != nil {
		return trace.ConvertSystemError(err)
	}
	return nil
}

const dbProxyConnectAd = `Teleport Connect is a desktop app that can manage database proxies for you.
Learn more at https://goteleport.com/docs/connect-your-client/teleport-connect/#connecting-to-a-database
`

// dbProxyTpl is the message that gets printed to a user when a database proxy is started.
var dbProxyTpl = template.Must(template.New("").Parse(`Started DB proxy on {{.address}}
{{if .randomPort}}To avoid port randomization, you can choose the listening port using the --port flag.
{{end}}
` + dbProxyConnectAd + `
Use the following credentials to connect to the {{.database}} proxy:
  ca_file={{.ca}}
  cert_file={{.cert}}
  key_file={{.key}}

Your database user is "{{.databaseUser}}".{{if .databaseName}} The target database name is "{{.databaseName}}".{{end}}

`))

var templateFunctions = map[string]any{
	"contains": strings.Contains,
}

// dbProxyAuthTpl is the message that's printed for an authenticated db proxy.
var dbProxyAuthTpl = template.Must(template.New("").Parse(
	`Started authenticated tunnel for the {{.type}} database "{{.database}}" in cluster "{{.cluster}}" on {{.address}}.
{{if .randomPort}}To avoid port randomization, you can choose the listening port using the --port flag.
{{end}}
` + dbProxyConnectAd + `
Use the following command to connect to the database or to the address above using other database GUI/CLI clients:
  $ {{.command}}
`))

// dbProxySpannerAuthTpl is the message that's printed for an authenticated spanner db proxy.
var dbProxySpannerAuthTpl = template.Must(template.New("").Parse(
	`Started authenticated tunnel for the {{.type}} database "{{.database}}" in cluster "{{.cluster}}" on {{.address}}.
{{if .randomPort}}To avoid port randomization, you can choose the listening port using the --port flag.
{{end}}
` + dbProxyConnectAd + `
Use the following command to connect to the database or to the address above using other database GUI/CLI clients:
  $ {{.command}}

Or use the following JDBC connection string to connect with other GUI/CLI clients:
jdbc:cloudspanner://{{.address}}/projects/{{.gcpProject}}/instances/{{.gcpInstance}}/databases/{{.databaseName}};usePlainText=true
`))

// dbProxyOracleAuthTpl is the message that's printed for an authenticated db proxy.
var dbProxyOracleAuthTpl = template.Must(template.New("").Funcs(templateFunctions).Parse(
	`Started authenticated tunnel for the {{.type}} database "{{.database}}" in cluster "{{.cluster}}" on {{.address}}.
{{if .randomPort}}To avoid port randomization, you can choose the listening port using the --port flag.
{{end}}
` + dbProxyConnectAd + `
Use the following command to connect to the Oracle database server using CLI:
  $ {{.command}}

or using following Oracle JDBC connection string in order to connect with other GUI/CLI clients:
{{- range $val := .args}}
  {{- if contains $val "jdbc:oracle:"}}
  {{$val}}
  {{- end}}
{{- end}}
`))

// dbProxyAuthMultiTpl is the message that's printed for an authenticated db proxy if there are multiple command options.
var dbProxyAuthMultiTpl = template.Must(template.New("").Parse(
	`Started authenticated tunnel for the {{.type}} database "{{.database}}" in cluster "{{.cluster}}" on {{.address}}.
{{if .randomPort}}To avoid port randomization, you can choose the listening port using the --port flag.
{{end}}
` + dbProxyConnectAd + `
Use one of the following commands to connect to the database or to the address above using other database GUI/CLI clients:
{{range $item := .commands}}
  * {{$item.Description}}:

  $ {{$item.Command}}
{{end}}
`))

const (
	envVarFormatText                 = "text"
	envVarFormatUnix                 = "unix"
	envVarFormatWindowsCommandPrompt = "command-prompt"
	envVarFormatWindowsPowershell    = "powershell"

	awsProxyFormatAthenaODBC = "athena-odbc"
	awsProxyFormatAthenaJDBC = "athena-jdbc"
)

var (
	envVarFormats = []string{
		envVarFormatUnix,
		envVarFormatWindowsCommandPrompt,
		envVarFormatWindowsPowershell,
		envVarFormatText,
	}

	awsProxyServiceFormats = []string{
		awsProxyFormatAthenaODBC,
		awsProxyFormatAthenaJDBC,
	}

	awsProxyFormats = append(envVarFormats, awsProxyServiceFormats...)
)

func envVarFormatFlagDescription() string {
	return fmt.Sprintf(
		"Optional format to print the commands for setting environment variables, one of: %s. Default is %s.",
		strings.Join(envVarFormats, ", "),
		envVarDefaultFormat(),
	)
}

func awsProxyFormatFlagDescription() string {
	return fmt.Sprintf(
		"%s Or specify a service format, one of: %s",
		envVarFormatFlagDescription(),
		strings.Join(awsProxyServiceFormats, ", "),
	)
}

func envVarDefaultFormat() string {
	if runtime.GOOS == constants.WindowsOS {
		return envVarFormatWindowsPowershell
	}
	return envVarFormatUnix
}

// envVarCommand returns the command to set environment variables based on the
// format.
//
// https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-envvars.html
func envVarCommand(format, key, value string) (string, error) {
	switch format {
	case envVarFormatUnix:
		return fmt.Sprintf("export %s=%s", key, value), nil

	case envVarFormatWindowsCommandPrompt:
		return fmt.Sprintf("set %s=%s", key, value), nil

	case envVarFormatWindowsPowershell:
		return fmt.Sprintf("$Env:%s=\"%s\"", key, value), nil

	case envVarFormatText:
		return fmt.Sprintf("%s=%s", key, value), nil

	default:
		return "", trace.BadParameter("unsupported format %q", format)
	}
}

var cloudTemplateFuncs = template.FuncMap{
	"envVarCommand": envVarCommand,
}

// awsProxyHeaderTemplate contains common header used for AWS proxy.
const awsProxyHeaderTemplate = `
{{define "header"}}
{{- if .envVars.HTTPS_PROXY -}}
Started AWS proxy on {{.envVars.HTTPS_PROXY}}.
{{- else -}}
Started AWS proxy which serves as an AWS endpoint URL at {{.endpointURL}}.
{{- end }}
{{if .randomPort}}To avoid port randomization, you can choose the listening port using the --port flag.
{{end}}
{{end}}
`

// awsProxyJDBCHeaderFooterTemplate contains common header and footer for AWS
// proxy in JDBC formats.
const awsProxyJDBCHeaderFooterTemplate = `
{{define "jdbc-header" }}
{{- template "header" . -}}
First, add the following certificate to your keystore:
{{.envVars.AWS_CA_BUNDLE}}

For example, to import the certificate using "keytool":
keytool -noprompt -importcert -alias teleport-{{.appName}} -file {{.envVars.AWS_CA_BUNDLE}} -keystore {{.keystore}}

{{end}}
{{define "jdbc-footer" }}

Note that a new certificate might be generated for a new app session. If you
encounter the "remote error: tls: unknown certificate" error, make sure your
keystore is up-to-date.

{{end}}
`

// awsHTTPSProxyTemplate is the message that gets printed to a user when an
// HTTPS proxy is started.
var awsHTTPSProxyTemplate = `{{- template "header" . -}}
Use the following credentials and HTTPS proxy setting to connect to the proxy:
  {{ envVarCommand .format "AWS_ACCESS_KEY_ID" .envVars.AWS_ACCESS_KEY_ID}}
  {{ envVarCommand .format "AWS_SECRET_ACCESS_KEY" .envVars.AWS_SECRET_ACCESS_KEY}}
  {{ envVarCommand .format "AWS_CA_BUNDLE" .envVars.AWS_CA_BUNDLE}}
  {{ envVarCommand .format "HTTPS_PROXY" .envVars.HTTPS_PROXY}}
`

// awsEndpointURLProxyTemplate is the message that gets printed to a user when an
// AWS endpoint URL proxy is started.
var awsEndpointURLProxyTemplate = `{{- template "header" . -}}
In addition to the endpoint URL, use the following credentials to connect to the proxy:
  {{ envVarCommand .format "AWS_ACCESS_KEY_ID" .envVars.AWS_ACCESS_KEY_ID}}
  {{ envVarCommand .format "AWS_SECRET_ACCESS_KEY" .envVars.AWS_SECRET_ACCESS_KEY}}
  {{ envVarCommand .format "AWS_CA_BUNDLE" .envVars.AWS_CA_BUNDLE}}
`

// awsProxyAthenaODBCTemplate is the message that gets printed to a user when an
// AWS proxy is used for Athena ODBC driver.
var awsProxyAthenaODBCTemplate = `{{- template "header" . -}}
Set the following properties for the Athena ODBC data source:
[Teleport AWS Athena Access]
AuthenticationType = IAM Credentials
UID = {{.envVars.AWS_ACCESS_KEY_ID}}
PWD = {{.envVars.AWS_SECRET_ACCESS_KEY}}
UseProxy = 1;
ProxyScheme = {{.proxyScheme}};
ProxyHost = {{.proxyHost}};
ProxyPort = {{.proxyPort}};
TrustedCerts = {{.envVars.AWS_CA_BUNDLE}}

Here is a sample connection string using the above credentials and proxy settings:
DRIVER=Simba Amazon Athena ODBC Connector;AuthenticationType=IAM Credentials;UID={{.envVars.AWS_ACCESS_KEY_ID}};PWD={{.envVars.AWS_SECRET_ACCESS_KEY}};UseProxy=1;ProxyScheme={{.proxyScheme}};ProxyHost={{.proxyHost}};ProxyPort={{.proxyPort}};TrustedCerts={{.envVars.AWS_CA_BUNDLE}};AWSRegion={{.region}};Workgroup={{.workgroup}}
`

// awsProxyAthenaJDBCTemplate is the message that gets printed to a user when
// an AWS proxy is used for Athena JDBC driver.
var awsProxyAthenaJDBCTemplate = `{{- template "jdbc-header" . -}}
Then, set the following properties in the JDBC connection URL:
User = {{.envVars.AWS_ACCESS_KEY_ID}}
Password = {{.envVars.AWS_SECRET_ACCESS_KEY}}
ProxyHost = {{.proxyHost}};
ProxyPort = {{.proxyPort}};

Here is a sample JDBC connection URL using the above credentials and proxy settings:
jdbc:awsathena://User={{.envVars.AWS_ACCESS_KEY_ID}};Password={{.envVars.AWS_SECRET_ACCESS_KEY}};ProxyHost={{.proxyHost}};ProxyPort={{.proxyPort}};AwsRegion={{.region}};Workgroup={{.workgroup}}

{{- template "jdbc-footer" -}}
`

func printCloudTemplate(envVars map[string]string, format string, randomPort bool, cloudName string) error {
	templateData := map[string]interface{}{
		"envVars":    envVars,
		"format":     format,
		"randomPort": randomPort,
		"cloudName":  cloudName,
	}
	err := cloudHTTPSProxyTemplate.Execute(os.Stdout, templateData)
	return trace.Wrap(err)
}

// cloudHTTPSProxyTemplate is the message that gets printed to a user when a cloud HTTPS proxy is started.
var cloudHTTPSProxyTemplate = template.Must(template.New("").Funcs(cloudTemplateFuncs).Parse(
	`Started {{.cloudName}} proxy on {{.envVars.HTTPS_PROXY}}.
{{if .randomPort}}To avoid port randomization, you can choose the listening port using the --port flag.
{{end}}
Use the following credentials and HTTPS proxy setting to connect to the proxy:

{{- $fmt := .format }}
{{ range $key, $value := .envVars}}
  {{envVarCommand $fmt $key $value}}
{{- end}}
`))
