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
	"crypto"
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
	"unicode"

	"github.com/coreos/go-semver/semver"
	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
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

	sshFunc := func() error {
		clt, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}

		targetHost, targetPort, err := net.SplitHostPort(tc.Host)
		if err != nil {
			targetHost = tc.Host
			targetPort = strconv.Itoa(tc.HostPort)
		}
		targetHost = cleanTargetHost(targetHost, tc.WebProxyHost(), clt.ClusterName())
		tc.Host = targetHost
		port, err := strconv.Atoi(targetPort)
		if err != nil {
			return trace.Wrap(err)
		}
		tc.HostPort = port

		target, err := tc.GetTargetNode(cf.Context, clt.AuthClient, nil)
		if err != nil {
			return trace.Wrap(err)
		}

		conn, _, err := clt.DialHostWithResumption(cf.Context, target.Addr, clt.ClusterName(), tc.LocalAgent().ExtendedAgent)
		if err != nil {
			return trace.Wrap(err)
		}

		defer conn.Close()

		return trace.Wrap(utils.ProxyConn(cf.Context, utils.CombinedStdio{}, conn))
	}
	if !cf.Relogin {
		return trace.Wrap(sshFunc())
	}

	return trace.Wrap(libclient.RetryWithRelogin(cf.Context, tc, sshFunc))
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
			logger.WarnContext(cf.Context, "Failed to close listener", "error", err)
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

	lp, err := alpnproxy.NewLocalProxy(makeBasicLocalProxyConfig(cf.Context, tc, listener, cf.InsecureSkipVerify), proxyOpts...)
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
			dbcmd.WithLogger(logger),
			dbcmd.WithPrintFormat(),
			dbcmd.WithTolerateMissingCLIClient(),
			dbcmd.WithGetDatabaseFunc(dbInfo.getDatabaseForDBCmd),
		}
		if opts, err = maybeAddDBUserPassword(cf, tc, dbInfo, opts); err != nil {
			return trace.Wrap(err)
		}
		if opts, err = maybeAddGCPMetadata(cf.Context, tc, dbInfo, opts); err != nil {
			return trace.Wrap(err)
		}
		opts = maybeAddOracleOptions(cf.Context, tc, dbInfo, opts)

		commands, err := dbcmd.NewCmdBuilder(tc, profile, dbInfo.RouteToDatabase, rootCluster,
			opts...,
		).GetConnectCommandAlternatives(cf.Context)
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
			"key":          profile.DatabaseKeyPathForCluster(cf.SiteName, dbInfo.ServiceName),
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

func maybeAddOracleOptions(ctx context.Context, tc *libclient.TeleportClient, dbInfo *databaseInfo, opts []dbcmd.ConnectCommandFunc) []dbcmd.ConnectCommandFunc {
	// Skip for non-Oracle protocols.
	if dbInfo.Protocol != defaults.ProtocolOracle {
		return opts
	}

	// TODO(Tener): DELETE IN 20.0.0 - all agents should now contain improved Oracle engine.
	// minimum version to support TCPS-less connection.
	cutoffVersion := semver.Version{
		Major:      17,
		Minor:      2,
		Patch:      0,
		PreRelease: "",
	}

	devV17Version := semver.Version{
		Major:      17,
		Minor:      0,
		Patch:      0,
		PreRelease: "dev",
	}

	dbServers, err := getDatabaseServers(ctx, tc, dbInfo.ServiceName)
	if err != nil {
		// log, but treat this error as non-fatal.
		logger.WarnContext(ctx, "Error getting database servers", "error", err)
		return opts
	}

	var oldServers, newServers int

	for _, server := range dbServers {
		ver, err := semver.NewVersion(server.GetTeleportVersion())
		if err != nil {
			logger.DebugContext(ctx, "Failed to parse teleport version", "version", server.GetTeleportVersion(), "error", err)
			continue
		}

		if ver.Equal(devV17Version) {
			newServers++
		} else {
			if ver.LessThan(cutoffVersion) {
				oldServers++
			} else {
				newServers++
			}
		}
	}

	logger.DebugContext(ctx, "Retrieved agents for database with Oracle support",
		"database", dbInfo.ServiceName,
		"total", len(dbServers),
		"old_count", oldServers,
		"new_count", newServers,
	)

	if oldServers > 0 {
		logger.WarnContext(ctx, "Detected outdated database agent, for improved client support upgrade all database agents in your cluster to a newer version",
			"lowest_supported_version", cutoffVersion,
		)
	}

	opts = append(opts, dbcmd.WithOracleOpts(oldServers == 0, newServers > 0))
	return opts
}

type templateCommandItem struct {
	Description string
	Command     string
}

func chooseProxyCommandTemplate(templateArgs map[string]any, commands []dbcmd.CommandAlternative, dbInfo *databaseInfo) *template.Template {
	templateArgs["command"] = formatCommand(commands[0].Command)

	// protocol-specific templates
	if dbInfo.Protocol == defaults.ProtocolOracle {
		// the JDBC connection string should always be found,
		// but the order of commands is important as only the first command will actually be shown.
		jdbcConnectionString := ""
		ixFound := -1
		for ix, cmd := range commands {
			for _, arg := range cmd.Command.Args {
				if strings.Contains(arg, "jdbc:oracle:") {
					jdbcConnectionString = arg
					ixFound = ix
				}
			}
		}
		templateArgs["jdbcConnectionString"] = jdbcConnectionString
		templateArgs["canUseTCP"] = ixFound > 0
		return dbProxyOracleAuthTpl
	}

	if dbInfo.Protocol == defaults.ProtocolSpanner {
		templateArgs["databaseName"] = "<database>"
		if dbInfo.Database != "" {
			templateArgs["databaseName"] = dbInfo.Database
		}
		return dbProxySpannerAuthTpl
	}

	// there is only one command, use plain template.
	if len(commands) == 1 {
		return dbProxyAuthTpl
	}

	// multiple command options, use a different template.
	var commandsArg []templateCommandItem
	for _, cmd := range commands {
		commandsArg = append(commandsArg, templateCommandItem{cmd.Description, formatCommand(cmd.Command)})
	}

	delete(templateArgs, "command")
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

	portMapping, err := libclient.ParsePortMapping(cf.LocalProxyPortMapping)
	if err != nil {
		return trace.Wrap(err)
	}

	profile, err := tc.ProfileStatus()
	if err != nil {
		return trace.Wrap(err)
	}

	var (
		appInfo *appInfo
		app     types.Application
	)
	if err := libclient.RetryWithRelogin(cf.Context, tc, func() error {
		var err error
		clusterClient, err := tc.ConnectToCluster(cf.Context)
		if err != nil {
			return trace.Wrap(err)
		}
		defer clusterClient.Close()

		appInfo, err = getAppInfo(cf, clusterClient.AuthClient, profile, tc.SiteName, matchGCPApp)
		if err != nil {
			return trace.Wrap(err)
		}

		app, err = appInfo.GetApp(cf.Context, clusterClient.AuthClient)
		return trace.Wrap(err)
	}); err != nil {
		return trace.Wrap(err)
	}

	proxyApp, err := newLocalProxyAppWithPortMapping(cf.Context, tc, profile, appInfo.RouteToApp, app, portMapping, cf.InsecureSkipVerify)
	if err != nil {
		return trace.Wrap(err)
	}
	if err := proxyApp.StartLocalProxy(cf.Context, alpnproxy.WithALPNProtocol(alpnProtocolForApp(app))); err != nil {
		return trace.Wrap(err)
	}

	appName := cf.AppName
	if portMapping.TargetPort != 0 {
		appName = fmt.Sprintf("%s:%d", appName, portMapping.TargetPort)
	}
	fmt.Printf("Proxying connections to %s on %v\n", appName, proxyApp.GetAddr())
	// If target port is not equal to zero, the user must know about the port flag.
	if portMapping.LocalPort == 0 && portMapping.TargetPort == 0 {
		fmt.Println("To avoid port randomization, you can choose the listening port using the --port flag.")
	}

	defer func() {
		if err := proxyApp.Close(); err != nil {
			logger.ErrorContext(cf.Context, "Failed to close app proxy", "error", err)
		}
	}()

	// Proxy connections until the client terminates the command.
	<-cf.Context.Done()
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

	err = awsApp.StartLocalProxies(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := awsApp.Close(); err != nil {
			logger.ErrorContext(cf.Context, "Failed to close AWS app", "error", err)
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

	err = azApp.StartLocalProxies(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := azApp.Close(); err != nil {
			logger.ErrorContext(cf.Context, "Failed to close Azure app", "error", err)
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

	err = gcpApp.StartLocalProxies(cf.Context)
	if err != nil {
		return trace.Wrap(err)
	}

	defer func() {
		if err := gcpApp.Close(); err != nil {
			logger.ErrorContext(cf.Context, "Failed to close GCP app", "error", err)
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

func loadAppCertificate(tc *libclient.TeleportClient, appName string) (tls.Certificate, error) {
	keyRing, err := tc.LocalAgent().GetKeyRing(tc.SiteName, libclient.WithAppCerts{})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	appCert, err := keyRing.AppTLSCert(appName)
	if trace.IsNotFound(err) {
		return tls.Certificate{}, trace.NotFound("please login into the application first: 'tsh apps login %v'", appName)
	} else if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	return appCert, nil
}

func loadDBCertificate(tc *libclient.TeleportClient, dbName string) (tls.Certificate, error) {
	keyRing, err := tc.LocalAgent().GetKeyRing(tc.SiteName, libclient.WithDBCerts{})
	if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	dbCert, err := keyRing.DBTLSCert(dbName)
	if trace.IsNotFound(err) {
		return tls.Certificate{}, trace.NotFound("please login into the database first. 'tsh db login'")
	} else if err != nil {
		return tls.Certificate{}, trace.Wrap(err)
	}

	return dbCert, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func makeBasicLocalProxyConfig(ctx context.Context, tc *libclient.TeleportClient, listener net.Listener, insecure bool) alpnproxy.LocalProxyConfig {
	return alpnproxy.LocalProxyConfig{
		RemoteProxyAddr:         tc.WebProxyAddr,
		InsecureSkipVerify:      insecure,
		ParentContext:           ctx,
		Listener:                listener,
		ALPNConnUpgradeRequired: tc.TLSRoutingConnUpgradeRequired,
	}
}

func generateDBLocalProxyCert(signer crypto.Signer, profile *libclient.ProfileStatus) error {
	path := profile.DatabaseLocalCAPath()
	if utils.FileExists(path) {
		return nil
	}
	certPem, err := tlsca.GenerateSelfSignedCAWithConfig(tlsca.GenerateCAConfig{
		Entity: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"Teleport"},
		},
		Signer:      signer,
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

var dbProxyOracleAuthTpl = template.Must(template.New("").Parse(
	`Started authenticated tunnel for the {{.type}} database "{{.database}}" in cluster "{{.cluster}}" on {{.address}}.
{{if .randomPort}}To avoid port randomization, you can choose the listening port using the --port flag.
{{end}}
Use the following command to connect to the Oracle database server using CLI:
  $ {{.command}}

{{if .canUseTCP }}Other clients can use:
  - a direct connection to {{.address}} without a username and password
  - a custom JDBC connection string: {{.jdbcConnectionString}}

{{else }}You can also connect using Oracle JDBC connection string:
  {{.jdbcConnectionString}}

Note: for improved client compatibility, upgrade your Teleport cluster. For details rerun this command with --debug.
{{- end }}
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
		return fmt.Sprintf(`export %s="%s"`, key, value), nil

	case envVarFormatWindowsCommandPrompt:
		return fmt.Sprintf("set %s=%s", key, value), nil

	case envVarFormatWindowsPowershell:
		return fmt.Sprintf(`$Env:%s="%s"`, key, value), nil

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
