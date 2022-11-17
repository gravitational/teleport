/*
Copyright 2015-2021 Gravitational, Inc.

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

package common

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/config"
	awsconfigurators "github.com/gravitational/teleport/lib/configurators/aws"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/teleport/lib/utils"
)

// Options combines init/start teleport options
type Options struct {
	// Args is a list of command-line args passed from main()
	Args []string
	// InitOnly when set to true, initializes config and aux
	// endpoints but does not start the process
	InitOnly bool
}

// Run inits/starts the process according to the provided options
func Run(options Options) (app *kingpin.Application, executedCommand string, conf *service.Config) {
	var err error

	// configure trace's errors to produce full stack traces
	isDebug, _ := strconv.ParseBool(os.Getenv(teleport.VerboseLogsEnvVar))
	if isDebug {
		trace.SetDebug(true)
	}
	// configure logger for a typical CLI scenario until configuration file is
	// parsed
	utils.InitLogger(utils.LoggingForDaemon, log.ErrorLevel)
	app = utils.InitCLIParser("teleport", "Teleport Access Plane. Learn more at https://goteleport.com")

	// define global flags:
	var (
		ccf                              config.CommandLineFlags
		scpFlags                         scp.Flags
		dumpFlags                        dumpFlags
		configureDatabaseAWSPrintFlags   configureDatabaseAWSPrintFlags
		configureDatabaseAWSCreateFlags  configureDatabaseAWSCreateFlags
		configureDiscoveryBootstrapFlags configureDiscoveryBootstrapFlags
		dbConfigCreateFlags              createDatabaseConfigFlags
		systemdInstallFlags              installSystemdFlags
	)

	// define commands:
	start := app.Command("start", "Starts the Teleport service.")
	status := app.Command("status", "Print the status of the current SSH session.")
	dump := app.Command("configure", "Generate a simple config file to get started.")
	ver := app.Command("version", "Print the version of your teleport binary.")
	scpc := app.Command("scp", "Server-side implementation of SCP.").Hidden()
	sftp := app.Command("sftp", "Server-side implementation of SFTP.").Hidden()
	exec := app.Command(teleport.ExecSubCommand, "Used internally by Teleport to re-exec itself to run a command.").Hidden()
	forward := app.Command(teleport.ForwardSubCommand, "Used internally by Teleport to re-exec itself to port forward.").Hidden()
	checkHomeDir := app.Command(teleport.CheckHomeDirSubCommand, "Used internally by Teleport to re-exec itself to check access to a directory.").Hidden()
	park := app.Command(teleport.ParkSubCommand, "Used internally by Teleport to re-exec itself to do nothing.").Hidden()
	app.HelpFlag.Short('h')

	// define start flags:
	start.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		BoolVar(&ccf.Debug)
	start.Flag("insecure-no-tls", "Disable TLS for the web socket").
		BoolVar(&ccf.DisableTLS)
	start.Flag("roles",
		fmt.Sprintf("Comma-separated list of roles to start with [%s]", strings.Join(defaults.StartRoles, ","))).
		Short('r').
		StringVar(&ccf.Roles)
	start.Flag("pid-file",
		"Full path to the PID file. By default no PID file will be created").StringVar(&ccf.PIDFile)
	start.Flag("advertise-ip",
		"IP to advertise to clients if running behind NAT").
		StringVar(&ccf.AdvertiseIP)
	start.Flag("listen-ip",
		fmt.Sprintf("IP address to bind to [%s]", defaults.BindIP)).
		Short('l').
		IPVar(&ccf.ListenIP)
	start.Flag("auth-server",
		fmt.Sprintf("Address of the auth server [%s]", defaults.AuthConnectAddr().Addr)).
		StringsVar(&ccf.AuthServerAddr)
	start.Flag("token",
		"Invitation token to register with an auth server [none]").
		StringVar(&ccf.AuthToken)
	start.Flag("ca-pin",
		"CA pin to validate the Auth Server (can be repeated for multiple pins)").
		StringsVar(&ccf.CAPins)
	start.Flag("nodename",
		"Name of this node, defaults to hostname").
		StringVar(&ccf.NodeName)
	start.Flag("config",
		fmt.Sprintf("Path to a configuration file [%v]", defaults.ConfigFilePath)).
		Short('c').ExistingFileVar(&ccf.ConfigFile)
	start.Flag("bootstrap",
		"Path to bootstrap file (ignored if already initialized)").ExistingFileVar(&ccf.BootstrapFile)
	start.Flag("config-string",
		"Base64 encoded configuration string").Hidden().Envar(defaults.ConfigEnvar).
		StringVar(&ccf.ConfigString)
	start.Flag("labels", "Comma-separated list of labels for this node, for example env=dev,app=web").StringVar(&ccf.Labels)
	start.Flag("diag-addr",
		"Start diagnostic prometheus and healthz endpoint.").StringVar(&ccf.DiagnosticAddr)
	start.Flag("permit-user-env",
		"Enables reading of ~/.tsh/environment when creating a session").BoolVar(&ccf.PermitUserEnvironment)
	start.Flag("insecure",
		"Insecure mode disables certificate validation").BoolVar(&ccf.InsecureMode)
	start.Flag("fips",
		"Start Teleport in FedRAMP/FIPS 140-2 mode.").
		Default("false").
		BoolVar(&ccf.FIPS)
	start.Flag("skip-version-check",
		"Skip version checking between server and client.").
		Default("false").
		BoolVar(&ccf.SkipVersionCheck)
	// All top-level --app-XXX flags are deprecated in favor of
	// "teleport start app" subcommand.
	start.Flag("app-name",
		"Name of the application to start").Hidden().
		StringVar(&ccf.AppName)
	start.Flag("app-uri",
		"Internal address of the application to proxy.").Hidden().
		StringVar(&ccf.AppURI)
	start.Flag("app-public-addr",
		"Public address of the application to proxy.").Hidden().
		StringVar(&ccf.AppPublicAddr)
	// All top-level --db-XXX flags are deprecated in favor of
	// "teleport start db" subcommand.
	start.Flag("db-name",
		"Name of the proxied database.").Hidden().
		StringVar(&ccf.DatabaseName)
	start.Flag("db-protocol",
		fmt.Sprintf("Proxied database protocol. Supported are: %v.", defaults.DatabaseProtocols)).Hidden().
		StringVar(&ccf.DatabaseProtocol)
	start.Flag("db-uri",
		"Address the proxied database is reachable at.").Hidden().
		StringVar(&ccf.DatabaseURI)
	start.Flag("db-ca-cert",
		"Database CA certificate path.").Hidden().
		StringVar(&ccf.DatabaseCACertFile)
	start.Flag("db-aws-region",
		"AWS region AWS hosted database instance is running in.").Hidden().
		StringVar(&ccf.DatabaseAWSRegion)

	// define start's usage info (we use kingpin's "alias" field for this)
	start.Alias(usageNotes + usageExamples)

	// "teleport app" command and its subcommands
	appCmd := app.Command("app", "Application proxy service commands.")
	appStartCmd := appCmd.Command("start", "Start application proxy service.")
	appStartCmd.Flag("debug", "Enable verbose logging to stderr.").Short('d').BoolVar(&ccf.Debug)
	appStartCmd.Flag("pid-file", "Full path to the PID file. By default no PID file will be created.").StringVar(&ccf.PIDFile)
	appStartCmd.Flag("auth-server", fmt.Sprintf("Address of the auth server [%s].", defaults.AuthConnectAddr().Addr)).StringsVar(&ccf.AuthServerAddr)
	appStartCmd.Flag("token", "Invitation token to register with an auth server [none].").StringVar(&ccf.AuthToken)
	appStartCmd.Flag("ca-pin", "CA pin to validate the auth server (can be repeated for multiple pins).").StringsVar(&ccf.CAPins)
	appStartCmd.Flag("config", fmt.Sprintf("Path to a configuration file [%v].", defaults.ConfigFilePath)).Short('c').ExistingFileVar(&ccf.ConfigFile)
	appStartCmd.Flag("config-string", "Base64 encoded configuration string.").Hidden().Envar(defaults.ConfigEnvar).StringVar(&ccf.ConfigString)
	appStartCmd.Flag("labels", "Comma-separated list of labels for this node, for example env=dev,app=web.").StringVar(&ccf.Labels)
	appStartCmd.Flag("fips", "Start Teleport in FedRAMP/FIPS 140-2 mode.").Default("false").BoolVar(&ccf.FIPS)
	appStartCmd.Flag("name", "Name of the application to start.").StringVar(&ccf.AppName)
	appStartCmd.Flag("uri", "Internal address of the application to proxy.").StringVar(&ccf.AppURI)
	appStartCmd.Flag("public-addr", "Public address of the application to proxy.").StringVar(&ccf.AppPublicAddr)
	appStartCmd.Flag("diag-addr", "Start diagnostic prometheus and healthz endpoint.").StringVar(&ccf.DiagnosticAddr)
	appStartCmd.Flag("insecure", "Insecure mode disables certificate validation").BoolVar(&ccf.InsecureMode)
	appStartCmd.Flag("skip-version-check", "Skip version checking between server and client.").Default("false").BoolVar(&ccf.SkipVersionCheck)
	appStartCmd.Alias(appUsageExamples) // We're using "alias" section to display usage examples.

	// "teleport db" command and its subcommands
	dbCmd := app.Command("db", "Database proxy service commands.")
	dbStartCmd := dbCmd.Command("start", "Start database proxy service.")
	dbStartCmd.Flag("debug", "Enable verbose logging to stderr.").Short('d').BoolVar(&ccf.Debug)
	dbStartCmd.Flag("pid-file", "Full path to the PID file. By default no PID file will be created.").StringVar(&ccf.PIDFile)
	dbStartCmd.Flag("auth-server", fmt.Sprintf("Address of the auth server [%s].", defaults.AuthConnectAddr().Addr)).StringsVar(&ccf.AuthServerAddr)
	dbStartCmd.Flag("token", "Invitation token to register with an auth server [none].").StringVar(&ccf.AuthToken)
	dbStartCmd.Flag("ca-pin", "CA pin to validate the auth server (can be repeated for multiple pins).").StringsVar(&ccf.CAPins)
	dbStartCmd.Flag("config", fmt.Sprintf("Path to a configuration file [%v].", defaults.ConfigFilePath)).Short('c').ExistingFileVar(&ccf.ConfigFile)
	dbStartCmd.Flag("config-string", "Base64 encoded configuration string.").Hidden().Envar(defaults.ConfigEnvar).StringVar(&ccf.ConfigString)
	dbStartCmd.Flag("labels", "Comma-separated list of labels for this node, for example env=dev,app=web.").StringVar(&ccf.Labels)
	dbStartCmd.Flag("fips", "Start Teleport in FedRAMP/FIPS 140-2 mode.").Default("false").BoolVar(&ccf.FIPS)
	dbStartCmd.Flag("name", "Name of the proxied database.").StringVar(&ccf.DatabaseName)
	dbStartCmd.Flag("description", "Description of the proxied database.").StringVar(&ccf.DatabaseDescription)
	dbStartCmd.Flag("protocol", fmt.Sprintf("Proxied database protocol. Supported are: %v.", defaults.DatabaseProtocols)).StringVar(&ccf.DatabaseProtocol)
	dbStartCmd.Flag("uri", "Address the proxied database is reachable at.").StringVar(&ccf.DatabaseURI)
	dbStartCmd.Flag("ca-cert", "Database CA certificate path.").StringVar(&ccf.DatabaseCACertFile)
	dbStartCmd.Flag("aws-region", "(Only for RDS, Aurora, Redshift, ElastiCache or MemoryDB) AWS region AWS hosted database instance is running in.").StringVar(&ccf.DatabaseAWSRegion)
	dbStartCmd.Flag("aws-account-id", "(Only for Keyspaces) AWS Account ID.").StringVar(&ccf.DatabaseAWSAccountID)
	dbStartCmd.Flag("aws-redshift-cluster-id", "(Only for Redshift) Redshift database cluster identifier.").StringVar(&ccf.DatabaseAWSRedshiftClusterID)
	dbStartCmd.Flag("aws-rds-instance-id", "(Only for RDS) RDS instance identifier.").StringVar(&ccf.DatabaseAWSRDSInstanceID)
	dbStartCmd.Flag("aws-rds-cluster-id", "(Only for Aurora) Aurora cluster identifier.").StringVar(&ccf.DatabaseAWSRDSClusterID)
	dbStartCmd.Flag("gcp-project-id", "(Only for Cloud SQL) GCP Cloud SQL project identifier.").StringVar(&ccf.DatabaseGCPProjectID)
	dbStartCmd.Flag("gcp-instance-id", "(Only for Cloud SQL) GCP Cloud SQL instance identifier.").StringVar(&ccf.DatabaseGCPInstanceID)
	dbStartCmd.Flag("ad-keytab-file", "(Only for SQL Server) Kerberos keytab file.").StringVar(&ccf.DatabaseADKeytabFile)
	dbStartCmd.Flag("ad-krb5-file", "(Only for SQL Server) Kerberos krb5.conf file.").Default(defaults.Krb5FilePath).StringVar(&ccf.DatabaseADKrb5File)
	dbStartCmd.Flag("ad-domain", "(Only for SQL Server) Active Directory domain.").StringVar(&ccf.DatabaseADDomain)
	dbStartCmd.Flag("ad-spn", "(Only for SQL Server) Service Principal Name for Active Directory auth.").StringVar(&ccf.DatabaseADSPN)
	dbStartCmd.Flag("diag-addr", "Start diagnostic prometheus and healthz endpoint.").StringVar(&ccf.DiagnosticAddr)
	dbStartCmd.Flag("insecure", "Insecure mode disables certificate validation").BoolVar(&ccf.InsecureMode)
	dbStartCmd.Flag("skip-version-check", "Skip version checking between server and client.").Default("false").BoolVar(&ccf.SkipVersionCheck)
	dbStartCmd.Alias(dbUsageExamples) // We're using "alias" section to display usage examples.

	dbConfigure := dbCmd.Command("configure", "Bootstraps database service configuration and cloud permissions.")
	dbConfigureCreate := dbConfigure.Command("create", "Creates a sample Database Service configuration.")
	dbConfigureCreate.Flag("proxy", fmt.Sprintf("Teleport proxy address to connect to [%s].", defaults.ProxyWebListenAddr().Addr)).
		Default(defaults.ProxyWebListenAddr().Addr).
		StringVar(&dbConfigCreateFlags.ProxyServer)
	dbConfigureCreate.Flag("token", "Invitation token to register with an auth server [none].").Default("/tmp/token").StringVar(&dbConfigCreateFlags.AuthToken)
	dbConfigureCreate.Flag("rds-discovery", "List of AWS regions in which the agent will discover RDS/Aurora instances.").StringsVar(&dbConfigCreateFlags.RDSDiscoveryRegions)
	dbConfigureCreate.Flag("rdsproxy-discovery", "List of AWS regions in which the agent will discover RDS Proxies.").StringsVar(&dbConfigCreateFlags.RDSProxyDiscoveryRegions)
	dbConfigureCreate.Flag("redshift-discovery", "List of AWS regions in which the agent will discover Redshift instances.").StringsVar(&dbConfigCreateFlags.RedshiftDiscoveryRegions)
	dbConfigureCreate.Flag("elasticache-discovery", "List of AWS regions in which the agent will discover ElastiCache Redis clusters.").StringsVar(&dbConfigCreateFlags.ElastiCacheDiscoveryRegions)
	dbConfigureCreate.Flag("memorydb-discovery", "List of AWS regions in which the agent will discover MemoryDB clusters.").StringsVar(&dbConfigCreateFlags.MemoryDBDiscoveryRegions)
	dbConfigureCreate.Flag("azure-mysql-discovery", "List of Azure regions in which the agent will discover MySQL servers.").StringsVar(&dbConfigCreateFlags.AzureMySQLDiscoveryRegions)
	dbConfigureCreate.Flag("azure-postgres-discovery", "List of Azure regions in which the agent will discover Postgres servers.").StringsVar(&dbConfigCreateFlags.AzurePostgresDiscoveryRegions)
	dbConfigureCreate.Flag("azure-redis-discovery", "List of Azure regions in which the agent will discover Azure Cache For Redis servers.").StringsVar(&dbConfigCreateFlags.AzureRedisDiscoveryRegions)
	dbConfigureCreate.Flag("azure-subscription", "List of Azure subscription IDs for Azure discoveries. Default is \"*\".").Default(types.Wildcard).StringsVar(&dbConfigCreateFlags.DatabaseAzureSubscriptions)
	dbConfigureCreate.Flag("azure-resource-group", "List of Azure resource groups for Azure discoveries. Default is \"*\".").Default(types.Wildcard).StringsVar(&dbConfigCreateFlags.DatabaseAzureResourceGroups)
	dbConfigureCreate.Flag("ca-pin", "CA pin to validate the auth server (can be repeated for multiple pins).").StringsVar(&dbConfigCreateFlags.CAPins)
	dbConfigureCreate.Flag("name", "Name of the proxied database.").StringVar(&dbConfigCreateFlags.StaticDatabaseName)
	dbConfigureCreate.Flag("protocol", fmt.Sprintf("Proxied database protocol. Supported are: %v.", defaults.DatabaseProtocols)).StringVar(&dbConfigCreateFlags.StaticDatabaseProtocol)
	dbConfigureCreate.Flag("uri", "Address the proxied database is reachable at.").StringVar(&dbConfigCreateFlags.StaticDatabaseURI)
	dbConfigureCreate.Flag("labels", "Comma-separated list of labels for the database, for example env=dev,dept=it").StringVar(&dbConfigCreateFlags.StaticDatabaseRawLabels)
	dbConfigureCreate.Flag("aws-region", "(Only for RDS, Aurora, Redshift or ElastiCache) AWS region RDS, Aurora, Redshift or ElastiCache database instance is running in.").StringVar(&dbConfigCreateFlags.DatabaseAWSRegion)
	dbConfigureCreate.Flag("aws-redshift-cluster-id", "(Only for Redshift) Redshift database cluster identifier.").StringVar(&dbConfigCreateFlags.DatabaseAWSRedshiftClusterID)
	dbConfigureCreate.Flag("ad-domain", "(Only for SQL Server) Active Directory domain.").StringVar(&dbConfigCreateFlags.DatabaseADDomain)
	dbConfigureCreate.Flag("ad-spn", "(Only for SQL Server) Service Principal Name for Active Directory auth.").StringVar(&dbConfigCreateFlags.DatabaseADSPN)
	dbConfigureCreate.Flag("ad-keytab-file", "(Only for SQL Server) Kerberos keytab file.").StringVar(&dbConfigCreateFlags.DatabaseADKeytabFile)
	dbConfigureCreate.Flag("gcp-project-id", "(Only for Cloud SQL) GCP Cloud SQL project identifier.").StringVar(&dbConfigCreateFlags.DatabaseGCPProjectID)
	dbConfigureCreate.Flag("gcp-instance-id", "(Only for Cloud SQL) GCP Cloud SQL instance identifier.").StringVar(&dbConfigCreateFlags.DatabaseGCPInstanceID)
	dbConfigureCreate.Flag("ca-cert-file", "Database CA certificate path.").StringVar(&dbConfigCreateFlags.DatabaseCACertFile)
	dbConfigureCreate.Flag("output",
		"Write to stdout with -o=stdout, default config file with -o=file or custom path with -o=file:///path").Short('o').Default(
		teleport.SchemeStdout).StringVar(&dbConfigCreateFlags.output)
	dbConfigureCreate.Alias(dbCreateConfigExamples) // We're using "alias" section to display usage examples.

	dbConfigureBootstrap := dbConfigure.Command("bootstrap", "Bootstrap the necessary configuration for the database agent. It reads the provided agent configuration to determine what will be bootstrapped.")
	dbConfigureBootstrap.Flag("config", fmt.Sprintf("Path to a configuration file [%v].", defaults.ConfigFilePath)).Short('c').Default(defaults.ConfigFilePath).ExistingFileVar(&configureDiscoveryBootstrapFlags.config.ConfigPath)
	dbConfigureBootstrap.Flag("manual", "When executed in \"manual\" mode, it will print the instructions to complete the configuration instead of applying them directly.").BoolVar(&configureDiscoveryBootstrapFlags.config.Manual)
	dbConfigureBootstrap.Flag("policy-name", fmt.Sprintf("Name of the Teleport Database agent policy. Default: %q", awsconfigurators.DefaultPolicyName)).Default(awsconfigurators.DefaultPolicyName).StringVar(&configureDiscoveryBootstrapFlags.config.PolicyName)
	dbConfigureBootstrap.Flag("confirm", "Do not prompt user and auto-confirm all actions.").BoolVar(&configureDiscoveryBootstrapFlags.confirm)
	dbConfigureBootstrap.Flag("attach-to-role", "Role name to attach policy to. Mutually exclusive with --attach-to-user. If none of the attach-to flags is provided, the command will try to attach the policy to the current user/role based on the credentials.").StringVar(&configureDiscoveryBootstrapFlags.config.AttachToRole)
	dbConfigureBootstrap.Flag("attach-to-user", "User name to attach policy to. Mutually exclusive with --attach-to-role. If none of the attach-to flags is provided, the command will try to attach the policy to the current user/role based on the credentials.").StringVar(&configureDiscoveryBootstrapFlags.config.AttachToUser)

	dbConfigureAWS := dbConfigure.Command("aws", "Bootstrap for AWS hosted databases.")
	dbConfigureAWSPrintIAM := dbConfigureAWS.Command("print-iam", "Generate and show IAM policies.")
	dbConfigureAWSPrintIAM.Flag("types",
		fmt.Sprintf("Comma-separated list of database types to include in the policy. Any of %s", strings.Join(awsDatabaseTypes, ","))).
		Short('r').
		StringVar(&configureDatabaseAWSPrintFlags.types)
	dbConfigureAWSPrintIAM.Flag("role", "IAM role name to attach policy to. Mutually exclusive with --user").StringVar(&configureDatabaseAWSPrintFlags.role)
	dbConfigureAWSPrintIAM.Flag("user", "IAM user name to attach policy to. Mutually exclusive with --role").StringVar(&configureDatabaseAWSPrintFlags.user)
	dbConfigureAWSPrintIAM.Flag("policy", "Only print IAM policy document.").BoolVar(&configureDatabaseAWSPrintFlags.policyOnly)
	dbConfigureAWSPrintIAM.Flag("boundary", "Only print IAM boundary policy document.").BoolVar(&configureDatabaseAWSPrintFlags.boundaryOnly)
	dbConfigureAWSCreateIAM := dbConfigureAWS.Command("create-iam", "Generate, create and attach IAM policies.")
	dbConfigureAWSCreateIAM.Flag("types",
		fmt.Sprintf("Comma-separated list of database types to include in the policy. Any of %s", strings.Join(awsDatabaseTypes, ","))).
		Short('r').
		StringVar(&configureDatabaseAWSCreateFlags.types)
	dbConfigureAWSCreateIAM.Flag("name", "Created policy name. Defaults to empty. Will be auto-generated if not provided.").Default(awsconfigurators.DefaultPolicyName).StringVar(&configureDatabaseAWSCreateFlags.policyName)
	dbConfigureAWSCreateIAM.Flag("attach", "Try to attach the policy to the IAM identity.").Default("true").BoolVar(&configureDatabaseAWSCreateFlags.attach)
	dbConfigureAWSCreateIAM.Flag("confirm", "Do not prompt user and auto-confirm all actions.").BoolVar(&configureDatabaseAWSCreateFlags.confirm)
	dbConfigureAWSCreateIAM.Flag("role", "IAM role name to attach policy to. Mutually exclusive with --user").StringVar(&configureDatabaseAWSCreateFlags.role)
	dbConfigureAWSCreateIAM.Flag("user", "IAM user name to attach policy to. Mutually exclusive with --role").StringVar(&configureDatabaseAWSCreateFlags.user)

	// "teleport discovery" bootstrap command and subcommnads.
	discoveryCmd := app.Command("discovery", "Teleport discovery service commands")
	discoveryBootstrapCmd := discoveryCmd.Command("bootstrap", "Bootstrap the necessary configuration for the database agent. It reads the provided agent configuration to determine what will be bootstrapped.")
	discoveryBootstrapCmd.Flag("config", fmt.Sprintf("Path to a configuration file [%v].", defaults.ConfigFilePath)).Short('c').Default(defaults.ConfigFilePath).ExistingFileVar(&configureDiscoveryBootstrapFlags.config.ConfigPath)
	discoveryBootstrapCmd.Flag("confirm", "Do not prompt user and auto-confirm all actions.").BoolVar(&configureDatabaseAWSCreateFlags.confirm)
	discoveryBootstrapCmd.Flag("attach-to-role", "Role name to attach policy to. Mutually exclusive with --attach-to-user. If none of the attach-to flags is provided, the command will try to attach the policy to the current user/role based on the credentials.").StringVar(&configureDiscoveryBootstrapFlags.config.AttachToRole)
	discoveryBootstrapCmd.Flag("attach-to-user", "User name to attach policy to. Mutually exclusive with --attach-to-role. If none of the attach-to flags is provided, the command will try to attach the policy to the current user/role based on the credentials.").StringVar(&configureDiscoveryBootstrapFlags.config.AttachToUser)
	discoveryBootstrapCmd.Flag("policy-name", fmt.Sprintf("Name of the Teleport Discovery service policy. Default: %q", awsconfigurators.EC2DiscoveryPolicyName)).Default(awsconfigurators.EC2DiscoveryPolicyName).StringVar(&configureDiscoveryBootstrapFlags.config.PolicyName)

	// "teleport install" command and its subcommands
	installCmd := app.Command("install", "Teleport install commands.")
	systemdInstall := installCmd.Command("systemd", "Creates a systemd unit file configuration.")
	systemdInstall.Flag("env-file", "Full path to the environment file.").Default(config.SystemdDefaultEnvironmentFile).StringVar(&systemdInstallFlags.EnvironmentFile)
	systemdInstall.Flag("pid-file", "Full path to the PID file.").Default(config.SystemdDefaultPIDFile).StringVar(&systemdInstallFlags.PIDFile)
	systemdInstall.Flag("fd-limit", "Maximum number of open file descriptors.").Default(fmt.Sprintf("%v", config.SystemdDefaultFileDescriptorLimit)).IntVar(&systemdInstallFlags.FileDescriptorLimit)
	systemdInstall.Flag("teleport-path", "Full path to the Teleport binary.").StringVar(&systemdInstallFlags.TeleportInstallationFile)
	systemdInstall.Flag("output", "Write to stdout with -o=stdout or custom path with -o=file:///path").Short('o').Default(teleport.SchemeStdout).StringVar(&systemdInstallFlags.output)
	systemdInstall.Alias(systemdInstallExamples) // We're using "alias" section to display usage examples.

	// define a hidden 'scp' command (it implements server-side implementation of handling
	// 'scp' requests)
	scpc.Flag("t", "sink mode (data consumer)").Short('t').Default("false").BoolVar(&scpFlags.Sink)
	scpc.Flag("f", "source mode (data producer)").Short('f').Default("false").BoolVar(&scpFlags.Source)
	scpc.Flag("v", "verbose mode").Default("false").Short('v').BoolVar(&scpFlags.Verbose)
	scpc.Flag("r", "recursive mode").Default("false").Short('r').BoolVar(&scpFlags.Recursive)
	scpc.Flag("d", "directory mode").Short('d').Hidden().BoolVar(&scpFlags.DirectoryMode)
	scpc.Flag("preserve", "preserve access and modification times").Short('p').BoolVar(&scpFlags.PreserveAttrs)
	scpc.Flag("remote-addr", "address of the remote client").StringVar(&scpFlags.RemoteAddr)
	scpc.Flag("local-addr", "local address which accepted the request").StringVar(&scpFlags.LocalAddr)
	scpc.Arg("target", "").StringsVar(&scpFlags.Target)

	// dump flags
	dump.Flag("cluster-name",
		"Unique cluster name, e.g. example.com.").StringVar(&dumpFlags.ClusterName)
	dump.Flag("output",
		"Write to stdout with -o=stdout, default config file with -o=file or custom path with -o=file:///path").Short('o').Default(
		teleport.SchemeStdout).StringVar(&dumpFlags.output)
	dump.Flag("acme",
		"Get automatic certificate from Letsencrypt.org using ACME.").BoolVar(&dumpFlags.ACMEEnabled)
	dump.Flag("acme-email",
		"Email to receive updates from Letsencrypt.org.").StringVar(&dumpFlags.ACMEEmail)
	dump.Flag("test", "Path to a configuration file to test.").ExistingFileVar(&dumpFlags.testConfigFile)
	dump.Flag("version", "Teleport configuration version.").Default(defaults.TeleportConfigVersionV3).StringVar(&dumpFlags.Version)
	dump.Flag("public-addr", "The hostport that the proxy advertises for the HTTP endpoint.").StringVar(&dumpFlags.PublicAddr)
	dump.Flag("cert-file", "Path to a TLS certificate file for the proxy.").ExistingFileVar(&dumpFlags.CertFile)
	dump.Flag("key-file", "Path to a TLS key file for the proxy.").ExistingFileVar(&dumpFlags.KeyFile)
	dump.Flag("data-dir", "Path to a directory where Teleport keep its data.").Default(defaults.DataDir).StringVar(&dumpFlags.DataDir)
	dump.Flag("token", "Invitation token to register with an auth server.").StringVar(&dumpFlags.AuthToken)
	dump.Flag("roles", "Comma-separated list of roles to create config with.").StringVar(&dumpFlags.Roles)
	dump.Flag("auth-server", "Address of the auth server.").StringVar(&dumpFlags.AuthServer)
	dump.Flag("proxy", "Address of the proxy.").StringVar(&dumpFlags.ProxyAddress)
	dump.Flag("app-name", "Name of the application to start when using app role.").StringVar(&dumpFlags.AppName)
	dump.Flag("app-uri", "Internal address of the application to proxy.").StringVar(&dumpFlags.AppURI)
	dump.Flag("node-labels", "Comma-separated list of labels to add to newly created nodes, for example env=staging,cloud=aws.").StringVar(&dumpFlags.NodeLabels)

	dumpNode := app.Command("node", "SSH Node configuration commands")
	dumpNodeConfigure := dumpNode.Command("configure", "Generate a configuration file for an SSH node.")
	dumpNodeConfigure.Flag("cluster-name",
		"Unique cluster name, e.g. example.com.").StringVar(&dumpFlags.ClusterName)
	dumpNodeConfigure.Flag("output",
		"Write to stdout with -o=stdout, default config file with -o=file or custom path with -o=file:///path").Short('o').Default(
		teleport.SchemeStdout).StringVar(&dumpFlags.output)
	dumpNodeConfigure.Flag("version", "Teleport configuration version.").Default(defaults.TeleportConfigVersionV3).StringVar(&dumpFlags.Version)
	dumpNodeConfigure.Flag("public-addr", "The hostport that the node advertises for the SSH endpoint.").StringVar(&dumpFlags.PublicAddr)
	dumpNodeConfigure.Flag("data-dir", "Path to a directory where Teleport keep its data.").Default(defaults.DataDir).StringVar(&dumpFlags.DataDir)
	dumpNodeConfigure.Flag("token", "Invitation token to register with an auth server.").StringVar(&dumpFlags.AuthToken)
	dumpNodeConfigure.Flag("auth-server", "Address of the auth server.").StringVar(&dumpFlags.AuthServer)
	dumpNodeConfigure.Flag("proxy", "Address of the proxy server.").StringVar(&dumpFlags.ProxyAddress)
	dumpNodeConfigure.Flag("labels", "Comma-separated list of labels to add to newly created nodes ex) env=staging,cloud=aws.").StringVar(&dumpFlags.NodeLabels)
	dumpNodeConfigure.Flag("ca-pin", "Comma-separated list of SKPI hashes for the CA used to verify the auth server.").StringVar(&dumpFlags.CAPin)
	dumpNodeConfigure.Flag("join-method", "Method to use to join the cluster (token, iam, ec2)").Default("token").EnumVar(&dumpFlags.JoinMethod, "token", "iam", "ec2")
	dumpNodeConfigure.Flag("node-name", "Name for the teleport node.").StringVar(&dumpFlags.NodeName)

	// parse CLI commands+flags:
	utils.UpdateAppUsageTemplate(app, options.Args)
	command, err := app.Parse(options.Args)
	if err != nil {
		app.Usage(options.Args)
		utils.FatalError(err)
	}

	// Create default configuration.
	conf = service.MakeDefaultConfig()

	// If FIPS mode is specified update defaults to be FIPS appropriate and
	// cross-validate the current config.
	if ccf.FIPS {
		if ccf.InsecureMode {
			utils.FatalError(trace.BadParameter("--insecure not allowed in FIPS mode"))
		}
		service.ApplyFIPSDefaults(conf)
	}

	// execute the selected command unless we're running tests
	switch command {
	case start.FullCommand(), appStartCmd.FullCommand(), dbStartCmd.FullCommand(): // Set appropriate roles for "app" and "db" subcommands.
		switch command {
		case appStartCmd.FullCommand():
			ccf.Roles = defaults.RoleApp
		case dbStartCmd.FullCommand():
			ccf.Roles = defaults.RoleDatabase
		}
		// configuration merge: defaults -> file-based conf -> CLI conf
		if err = config.Configure(&ccf, conf); err != nil {
			utils.FatalError(err)
		}
		if !options.InitOnly {
			err = OnStart(ccf, conf)
		}
	case scpc.FullCommand():
		err = onSCP(&scpFlags)
	case sftp.FullCommand():
		err = onSFTP()
	case status.FullCommand():
		err = onStatus()
	case dump.FullCommand():
		err = onConfigDump(dumpFlags)
	case dumpNodeConfigure.FullCommand():
		dumpFlags.Roles = defaults.RoleNode
		err = onConfigDump(dumpFlags)
	case exec.FullCommand():
		srv.RunAndExit(teleport.ExecSubCommand)
	case forward.FullCommand():
		srv.RunAndExit(teleport.ForwardSubCommand)
	case checkHomeDir.FullCommand():
		srv.RunAndExit(teleport.CheckHomeDirSubCommand)
	case park.FullCommand():
		srv.RunAndExit(teleport.ParkSubCommand)
	case ver.FullCommand():
		utils.PrintVersion()
	case dbConfigureCreate.FullCommand():
		err = onDumpDatabaseConfig(dbConfigCreateFlags)
	case dbConfigureAWSPrintIAM.FullCommand():
		err = onConfigureDatabasesAWSPrint(configureDatabaseAWSPrintFlags)
	case dbConfigureAWSCreateIAM.FullCommand():
		err = onConfigureDatabasesAWSCreate(configureDatabaseAWSCreateFlags)
	case dbConfigureBootstrap.FullCommand():
		configureDiscoveryBootstrapFlags.config.DiscoveryService = false
		err = onConfigureDiscoveryBootstrap(configureDiscoveryBootstrapFlags)
	case systemdInstall.FullCommand():
		err = onDumpSystemdUnitFile(systemdInstallFlags)
	case discoveryBootstrapCmd.FullCommand():
		configureDiscoveryBootstrapFlags.config.DiscoveryService = true
		err = onConfigureDiscoveryBootstrap(configureDiscoveryBootstrapFlags)
	}
	if err != nil {
		utils.FatalError(err)
	}
	return app, command, conf
}

// OnStart is the handler for "start" CLI command
func OnStart(clf config.CommandLineFlags, config *service.Config) error {
	if clf.ConfigFile != "" {
		config.Log.Infof("Starting Teleport v%s", teleport.Version)
	} else {
		config.Log.Infof("Starting Teleport v%s with a config file located at %q", teleport.Version, clf.ConfigFile)
	}
	return service.Run(context.TODO(), *config, nil)
}

// onStatus is the handler for "status" CLI command
func onStatus() error {
	sshClient := os.Getenv("SSH_CLIENT")
	systemUser := os.Getenv("USER")
	teleportUser := os.Getenv(teleport.SSHTeleportUser)
	proxyHost := os.Getenv(teleport.SSHSessionWebproxyAddr)
	clusterName := os.Getenv(teleport.SSHTeleportClusterName)
	hostUUID := os.Getenv(teleport.SSHTeleportHostUUID)
	sid := os.Getenv(teleport.SSHSessionID)

	// For node sessions started by `ssh`, the proxyhost is not
	// set in the session env. Provide a placeholder.
	if proxyHost == "" {
		proxyHost = "<proxyhost>"
	}

	if sid == "" {
		fmt.Println("You are not inside of a Teleport SSH session")
		return nil
	}

	fmt.Printf("User ID     : %s, logged in as %s from %s\n", teleportUser, systemUser, sshClient)
	fmt.Printf("Cluster Name: %s\n", clusterName)
	fmt.Printf("Host UUID   : %s\n", hostUUID)
	fmt.Printf("Session ID  : %s\n", sid)
	fmt.Printf("Session URL : https://%s/web/cluster/%s/console/session/%s\n", proxyHost, clusterName, sid)

	return nil
}

type dumpFlags struct {
	config.SampleFlags
	output         string
	testConfigFile string
}

func (flags *dumpFlags) CheckAndSetDefaults() error {
	if flags.testConfigFile != "" && flags.output != teleport.SchemeStdout {
		return trace.BadParameter("only --output or --test can be set, not both")
	}

	err := checkConfigurationFileVersion(flags.Version)
	if err != nil {
		return trace.Wrap(err)
	}

	flags.output = normalizeOutput(flags.output)
	return nil
}

func normalizeOutput(output string) string {
	switch output {
	case teleport.SchemeFile, "":
		output = teleport.SchemeFile + "://" + defaults.ConfigFilePath
	case teleport.SchemeStdout:
		output = teleport.SchemeStdout + "://"
	}

	return output
}

func checkConfigurationFileVersion(version string) error {
	// allow an empty version as we default to v1
	if version == "" {
		return nil
	}

	return defaults.ValidateConfigVersion(version)
}

// onConfigDump is the handler for "configure" CLI command
func onConfigDump(flags dumpFlags) error {
	var err error
	if err := flags.CheckAndSetDefaults(); err != nil {
		return trace.Wrap(err)
	}

	if flags.testConfigFile != "" {
		// Test an existing config.
		_, err := config.ReadFromFile(flags.testConfigFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "FAIL %s\n", flags.testConfigFile)
			return trace.Wrap(err)
		}
		fmt.Fprintf(os.Stderr, "OK %s\n", flags.testConfigFile)
		return nil
	}

	if modules.GetModules().BuildType() != modules.BuildOSS {
		flags.LicensePath = filepath.Join(flags.DataDir, "license.pem")
	}

	if flags.KeyFile != "" && !filepath.IsAbs(flags.KeyFile) {
		flags.KeyFile, err = filepath.Abs(flags.KeyFile)
		if err != nil {
			return trace.BadParameter("could not find absolute path for --key-file %q", flags.KeyFile)
		}
	}

	if flags.CertFile != "" && !filepath.IsAbs(flags.CertFile) {
		flags.CertFile, err = filepath.Abs(flags.CertFile)
		if err != nil {
			return trace.BadParameter("could not find absolute path for --cert-file %q", flags.CertFile)
		}
	}

	sfc, err := config.MakeSampleFileConfig(flags.SampleFlags)
	if err != nil {
		return trace.Wrap(err)
	}

	configPath, err := dumpConfigFile(flags.output, sfc.DebugDumpToYAML(), sampleConfComment)
	if err != nil {
		return trace.Wrap(err)
	}

	entries, err := os.ReadDir(flags.DataDir)
	if err != nil && !os.IsNotExist(err) {
		fmt.Fprintf(
			os.Stderr, "Could not check the contents of %s: %s\nThe data directory may contain existing cluster state.\n", flags.DataDir, err.Error())
	}

	if err == nil && len(entries) != 0 {
		fmt.Fprintf(
			os.Stderr,
			"The data directory %s is not empty and may contain existing cluster state. Running this configuration is likely a mistake. To join a new cluster, specify an alternate --data-dir or clear the %s directory.\n",
			flags.DataDir, flags.DataDir)
	}

	if strings.Contains(flags.Roles, defaults.RoleDatabase) {
		fmt.Fprintln(os.Stderr, "Role db requires further configuration, db_service will be disabled")
	}

	if strings.Contains(flags.Roles, defaults.RoleWindowsDesktop) {
		fmt.Fprintln(os.Stderr, "Role windowsdesktop requires further configuration, windows_desktop_service will be disabled")
	}

	if configPath != "" {
		canWriteToDataDir, err := utils.CanUserWriteTo(flags.DataDir)
		if err != nil && !trace.IsNotImplemented(err) {
			fmt.Fprintf(os.Stderr, "Failed to check data dir permissions: %+v", err)
		}
		canWriteToConfDir, err := utils.CanUserWriteTo(filepath.Dir(configPath))
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to check config dir permissions: %+v", err)
		}
		requiresRoot := !canWriteToDataDir || !canWriteToConfDir

		fmt.Printf("\nA Teleport configuration file has been created at %q.\n", configPath)
		if modules.GetModules().BuildType() != modules.BuildOSS {
			fmt.Printf("Add your Teleport license file to %q.\n", flags.LicensePath)
		}
		fmt.Printf("To start Teleport with this configuration file, run:\n\n")
		if requiresRoot {
			fmt.Printf("sudo teleport start --config=%q\n\n", configPath)
			fmt.Printf("Note that starting a Teleport server with this configuration will require root access as:\n")
			if !canWriteToConfDir {
				fmt.Printf("- The Teleport configuration is located at %q.\n", configPath)
			}
			if !canWriteToDataDir {
				fmt.Printf("- Teleport will be storing data at %q. To change that, run \"teleport configure\" with the \"--data-dir\" flag.\n", flags.DataDir)
			}
			fmt.Println()
		} else {
			fmt.Printf("teleport start --config=%q\n\n", configPath)
		}

		fmt.Printf("Happy Teleporting!\n")
	}

	return nil
}

func dumpConfigFile(outputURI, contents, comment string) (string, error) {
	// Generate a new config.
	uri, err := url.Parse(outputURI)
	if err != nil {
		return "", trace.BadParameter("could not parse output value %q, use --output=%q",
			outputURI, defaults.ConfigFilePath)
	}

	switch uri.Scheme {
	case teleport.SchemeStdout:
		fmt.Printf("%s\n%s\n", comment, contents)
		return "", nil
	case teleport.SchemeFile, "":
		if uri.Path == "" {
			return "", trace.BadParameter("missing path in --output=%q", uri)
		}
		if !filepath.IsAbs(uri.Path) {
			return "", trace.BadParameter("please use absolute path for file %v", uri.Path)
		}

		configDir := path.Dir(outputURI)
		err := os.MkdirAll(configDir, 0o755)
		err = trace.ConvertSystemError(err)
		if err != nil {
			if trace.IsAccessDenied(err) {
				return "", trace.Wrap(err, "permission denied creating directory %s", configDir)
			}

			return "", trace.Wrap(err, "error creating config file directory %s", configDir)
		}

		f, err := os.OpenFile(uri.Path, os.O_RDWR|os.O_CREATE|os.O_EXCL, teleport.FileMaskOwnerOnly)
		err = trace.ConvertSystemError(err)
		if err != nil {
			if trace.IsAlreadyExists(err) {
				return "", trace.AlreadyExists("will not overwrite existing file %v, rm -f %v and try again", uri.Path, uri.Path)
			}

			if trace.IsAccessDenied(err) {
				return "", trace.Wrap(err, "could not write to config file, missing sudo?")
			}

			return "", trace.Wrap(err)
		}
		if _, err := f.Write([]byte(contents)); err != nil {
			f.Close()
			return "", trace.Wrap(trace.ConvertSystemError(err), "could not write to config file, missing sudo?")
		}
		if err := f.Close(); err != nil {
			return "", trace.Wrap(err, "could not close file %v", uri.Path)
		}

		return uri.Path, nil
	default:
		return "", trace.BadParameter(
			"unsupported --output=%v, use path for example --output=%v", uri.Scheme, defaults.ConfigFilePath)
	}
}

// onSCP implements handling of 'scp' requests on the server side. When the teleport SSH daemon
// receives an SSH "scp" request, it launches itself with 'scp' flag under the requested
// user's privileges
//
// This is the entry point of "teleport scp" call (the parent process is the teleport daemon)
func onSCP(scpFlags *scp.Flags) (err error) {
	// when 'teleport scp' is executed, it cannot write logs to stderr (because
	// they're automatically replayed by the scp client)
	utils.SwitchLoggingtoSyslog()
	if len(scpFlags.Target) == 0 {
		return trace.BadParameter("teleport scp: missing an argument")
	}

	// get user's home dir (it serves as a default destination)
	user, err := user.Current()
	if err != nil {
		return trace.Wrap(err)
	}
	// see if the target is absolute. if not, use user's homedir to make
	// it absolute (and if the user doesn't have a homedir, use "/")
	target := scpFlags.Target[0]
	if !filepath.IsAbs(target) {
		if !utils.IsDir(user.HomeDir) {
			slash := string(filepath.Separator)
			scpFlags.Target[0] = slash + target
		} else {
			scpFlags.Target[0] = filepath.Join(user.HomeDir, target)
		}
	}
	if !scpFlags.Source && !scpFlags.Sink {
		return trace.Errorf("remote mode is not supported")
	}

	scpCfg := scp.Config{
		Flags:       *scpFlags,
		User:        user.Username,
		RunOnServer: true,
	}

	cmd, err := scp.CreateCommand(scpCfg)
	if err != nil {
		return trace.Wrap(err)
	}

	return trace.Wrap(cmd.Execute(&StdReadWriter{}))
}

type StdReadWriter struct{}

func (rw *StdReadWriter) Read(b []byte) (int, error) {
	return os.Stdin.Read(b)
}

func (rw *StdReadWriter) Write(b []byte) (int, error) {
	return os.Stdout.Write(b)
}
