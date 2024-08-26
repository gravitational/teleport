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
	"log/slog"
	"net/url"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	"golang.org/x/exp/maps"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	debugclient "github.com/gravitational/teleport/lib/client/debug"
	awslib "github.com/gravitational/teleport/lib/cloud/aws"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/configurators"
	awsconfigurators "github.com/gravitational/teleport/lib/configurators/aws"
	"github.com/gravitational/teleport/lib/defaults"
	dtconfig "github.com/gravitational/teleport/lib/devicetrust/config"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/openssh"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/teleport/lib/tpm"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
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
func Run(options Options) (app *kingpin.Application, executedCommand string, conf *servicecfg.Config) {
	var err error
	ctx := context.Background()

	// configure trace's errors to produce full stack traces
	isDebug, _ := strconv.ParseBool(os.Getenv(teleport.VerboseLogsEnvVar))
	if isDebug {
		trace.SetDebug(true)
	}
	// configure logger for a typical CLI scenario until configuration file is
	// parsed
	utils.InitLogger(utils.LoggingForDaemon, slog.LevelError)
	app = utils.InitCLIParser("teleport", "Teleport Access Platform. Learn more at https://goteleport.com")

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
		installAutoDiscoverNodeFlags     installAutoDiscoverNodeFlags
		waitFlags                        waitFlags
		rawVersion                       bool
	)

	// define commands:
	start := app.Command("start", "Starts the Teleport service.")
	status := app.Command("status", "Print the status of the current SSH session.")
	dump := app.Command("configure", "Generate a simple config file to get started.")
	ver := app.Command("version", "Print the version of your teleport binary.")
	join := app.Command("join", "Join a Teleport cluster without running the Teleport daemon.")
	joinOpenSSH := join.Command("openssh", "Join an SSH server to a Teleport cluster.")
	scpc := app.Command("scp", "Server-side implementation of SCP.").Hidden()
	sftp := app.Command(teleport.SFTPSubCommand, "Server-side implementation of SFTP.").Hidden()
	exec := app.Command(teleport.ExecSubCommand, "Used internally by Teleport to re-exec itself to run a command.").Hidden()
	networking := app.Command(teleport.NetworkingSubCommand, "Used internally by Teleport to re-exec itself to handle networking requests.").Hidden()
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
		"Invitation token or path to file with token value. Used to register with an auth server [none]").
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
	start.Flag("apply-on-startup",
		fmt.Sprintf("Path to a non-empty YAML file containing resources to apply on startup. Works on initialized clusters, unlike --bootstrap. Only supports the following kinds: %s.", maps.Keys(auth.ResourceApplyPriority))).
		ExistingFileVar(&ccf.ApplyOnStartupFile)
	start.Flag("bootstrap",
		"Path to a non-empty YAML file containing bootstrap resources (ignored if already initialized)").ExistingFileVar(&ccf.BootstrapFile)
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
	appStartCmd.Flag("token", "Invitation token or path to file with token value to register with an auth server [none].").StringVar(&ccf.AuthToken)
	appStartCmd.Flag("ca-pin", "CA pin to validate the auth server (can be repeated for multiple pins).").StringsVar(&ccf.CAPins)
	appStartCmd.Flag("config", fmt.Sprintf("Path to a configuration file [%v].", defaults.ConfigFilePath)).Short('c').ExistingFileVar(&ccf.ConfigFile)
	appStartCmd.Flag("config-string", "Base64 encoded configuration string.").Hidden().Envar(defaults.ConfigEnvar).StringVar(&ccf.ConfigString)
	appStartCmd.Flag("labels", "Comma-separated list of labels for this node, for example env=dev,app=web.").StringVar(&ccf.Labels)
	appStartCmd.Flag("fips", "Start Teleport in FedRAMP/FIPS 140-2 mode.").Default("false").BoolVar(&ccf.FIPS)
	appStartCmd.Flag("name", "Name of the application to start.").StringVar(&ccf.AppName)
	appStartCmd.Flag("uri", "Internal address of the application to proxy.").StringVar(&ccf.AppURI)
	appStartCmd.Flag("cloud", fmt.Sprintf("Set to one of %v if application should proxy particular cloud API", []string{types.CloudAWS, types.CloudAzure, types.CloudGCP})).StringVar(&ccf.AppCloud)
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
	dbStartCmd.Flag("token", "Invitation token or path to file with token value to register with an auth server [none].").StringVar(&ccf.AuthToken)
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
	dbStartCmd.Flag("aws-account-id", "(Only for Keyspaces or DynamoDB) AWS Account ID.").StringVar(&ccf.DatabaseAWSAccountID)
	dbStartCmd.Flag("aws-assume-role-arn", "Optional AWS IAM role to assume.").StringVar(&ccf.DatabaseAWSAssumeRoleARN)
	dbStartCmd.Flag("aws-external-id", "Optional AWS external ID used when assuming an AWS role.").StringVar(&ccf.DatabaseAWSExternalID)
	dbStartCmd.Flag("aws-redshift-cluster-id", "(Only for Redshift) Redshift database cluster identifier.").StringVar(&ccf.DatabaseAWSRedshiftClusterID)
	dbStartCmd.Flag("aws-rds-instance-id", "(Only for RDS) RDS instance identifier.").StringVar(&ccf.DatabaseAWSRDSInstanceID)
	dbStartCmd.Flag("aws-rds-cluster-id", "(Only for Aurora) Aurora cluster identifier.").StringVar(&ccf.DatabaseAWSRDSClusterID)
	dbStartCmd.Flag("aws-session-tags", "(Only for DynamoDB) List of STS tags.").StringVar(&ccf.DatabaseAWSSessionTags)
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
	dbConfigureCreate.Flag("token", "Invitation token or path to file with token value to register with an auth server [none].").Default("/tmp/token").StringVar(&dbConfigCreateFlags.AuthToken)
	dbConfigureCreate.Flag("rds-discovery", "List of AWS regions in which the agent will discover RDS/Aurora instances.").StringsVar(&dbConfigCreateFlags.RDSDiscoveryRegions)
	dbConfigureCreate.Flag("rdsproxy-discovery", "List of AWS regions in which the agent will discover RDS Proxies.").StringsVar(&dbConfigCreateFlags.RDSProxyDiscoveryRegions)
	dbConfigureCreate.Flag("redshift-discovery", "List of AWS regions in which the agent will discover Redshift instances.").StringsVar(&dbConfigCreateFlags.RedshiftDiscoveryRegions)
	dbConfigureCreate.Flag("redshift-serverless-discovery", "List of AWS regions in which the agent will discover Redshift Serverless instances.").StringsVar(&dbConfigCreateFlags.RedshiftServerlessDiscoveryRegions)
	dbConfigureCreate.Flag("elasticache-discovery", "List of AWS regions in which the agent will discover ElastiCache Redis clusters.").StringsVar(&dbConfigCreateFlags.ElastiCacheDiscoveryRegions)
	dbConfigureCreate.Flag("memorydb-discovery", "List of AWS regions in which the agent will discover MemoryDB clusters.").StringsVar(&dbConfigCreateFlags.MemoryDBDiscoveryRegions)
	dbConfigureCreate.Flag("opensearch-discovery", "List of AWS regions in which the agent will discover OpenSearch domains.").StringsVar(&dbConfigCreateFlags.OpenSearchDiscoveryRegions)
	dbConfigureCreate.Flag("aws-tags", "(Only for AWS discoveries) Comma-separated list of AWS resource tags to match, for example env=dev,dept=it").StringVar(&dbConfigCreateFlags.AWSRawTags)
	dbConfigureCreate.Flag("azure-mysql-discovery", "List of Azure regions in which the agent will discover MySQL servers.").StringsVar(&dbConfigCreateFlags.AzureMySQLDiscoveryRegions)
	dbConfigureCreate.Flag("azure-postgres-discovery", "List of Azure regions in which the agent will discover PostgreSQL servers.").StringsVar(&dbConfigCreateFlags.AzurePostgresDiscoveryRegions)
	dbConfigureCreate.Flag("azure-redis-discovery", "List of Azure regions in which the agent will discover Azure Cache For Redis servers.").StringsVar(&dbConfigCreateFlags.AzureRedisDiscoveryRegions)
	dbConfigureCreate.Flag("azure-sqlserver-discovery", "List of Azure regions in which the agent will discover Azure SQL Databases and Managed Instances.").StringsVar(&dbConfigCreateFlags.AzureSQLServerDiscoveryRegions)
	dbConfigureCreate.Flag("azure-subscription", "List of Azure subscription IDs for Azure discoveries. Default is \"*\".").Default(types.Wildcard).StringsVar(&dbConfigCreateFlags.AzureSubscriptions)
	dbConfigureCreate.Flag("azure-resource-group", "List of Azure resource groups for Azure discoveries. Default is \"*\".").Default(types.Wildcard).StringsVar(&dbConfigCreateFlags.AzureResourceGroups)
	dbConfigureCreate.Flag("azure-tags", "(Only for Azure discoveries) Comma-separated list of Azure resource tags to match, for example env=dev,dept=it").StringVar(&dbConfigCreateFlags.AzureRawTags)
	dbConfigureCreate.Flag("ca-pin", "CA pin to validate the auth server (can be repeated for multiple pins).").StringsVar(&dbConfigCreateFlags.CAPins)
	dbConfigureCreate.Flag("name", "Name of the proxied database.").StringVar(&dbConfigCreateFlags.StaticDatabaseName)
	dbConfigureCreate.Flag("protocol", fmt.Sprintf("Proxied database protocol. Supported are: %v.", defaults.DatabaseProtocols)).StringVar(&dbConfigCreateFlags.StaticDatabaseProtocol)
	dbConfigureCreate.Flag("uri", "Address the proxied database is reachable at.").StringVar(&dbConfigCreateFlags.StaticDatabaseURI)
	dbConfigureCreate.Flag("labels", "Comma-separated list of labels for the database, for example env=dev,dept=it").StringVar(&dbConfigCreateFlags.StaticDatabaseRawLabels)
	dbConfigureCreate.Flag("aws-region", "(Only for AWS-hosted databases) AWS region RDS, Aurora, Redshift, Redshift Serverless, ElastiCache, OpenSearch or MemoryDB database instance is running in.").StringVar(&dbConfigCreateFlags.DatabaseAWSRegion)
	dbConfigureCreate.Flag("aws-account-id", "(Only for Keyspaces or DynamoDB) AWS Account ID.").StringVar(&dbConfigCreateFlags.DatabaseAWSAccountID)
	dbConfigureCreate.Flag("aws-assume-role-arn", "Optional AWS IAM role to assume.").StringVar(&dbConfigCreateFlags.DatabaseAWSAssumeRoleARN)
	dbConfigureCreate.Flag("aws-external-id", "(Only for AWS-hosted databases) Optional AWS external ID to use when assuming AWS roles.").StringVar(&dbConfigCreateFlags.DatabaseAWSExternalID)
	dbConfigureCreate.Flag("aws-redshift-cluster-id", "(Only for Redshift) Redshift database cluster identifier.").StringVar(&dbConfigCreateFlags.DatabaseAWSRedshiftClusterID)
	dbConfigureCreate.Flag("aws-rds-cluster-id", "(Only for RDS Aurora) RDS Aurora database cluster identifier.").StringVar(&dbConfigCreateFlags.DatabaseAWSRDSClusterID)
	dbConfigureCreate.Flag("aws-rds-instance-id", "(Only for RDS) RDS database instance identifier.").StringVar(&dbConfigCreateFlags.DatabaseAWSRDSInstanceID)
	dbConfigureCreate.Flag("aws-elasticache-group-id", "(Only for ElastiCache) ElastiCache replication group identifier.").StringVar(&dbConfigCreateFlags.DatabaseAWSElastiCacheGroupID)
	dbConfigureCreate.Flag("aws-memorydb-cluster-name", "(Only for MemoryDB) MemoryDB cluster name.").StringVar(&dbConfigCreateFlags.DatabaseAWSMemoryDBClusterName)
	dbConfigureCreate.Flag("ad-domain", "(Only for SQL Server) Active Directory domain.").StringVar(&dbConfigCreateFlags.DatabaseADDomain)
	dbConfigureCreate.Flag("ad-spn", "(Only for SQL Server) Service Principal Name for Active Directory auth.").StringVar(&dbConfigCreateFlags.DatabaseADSPN)
	dbConfigureCreate.Flag("ad-keytab-file", "(Only for SQL Server) Kerberos keytab file.").StringVar(&dbConfigCreateFlags.DatabaseADKeytabFile)
	dbConfigureCreate.Flag("gcp-project-id", "(Only for Cloud SQL) GCP Cloud SQL project identifier.").StringVar(&dbConfigCreateFlags.DatabaseGCPProjectID)
	dbConfigureCreate.Flag("gcp-instance-id", "(Only for Cloud SQL) GCP Cloud SQL instance identifier.").StringVar(&dbConfigCreateFlags.DatabaseGCPInstanceID)
	dbConfigureCreate.Flag("ca-cert-file", "Database CA certificate path.").StringVar(&dbConfigCreateFlags.DatabaseCACertFile)
	dbConfigureCreate.Flag("output",
		`Write to stdout with "--output=stdout", default config file with "--output=file" or custom path with --output=file:///path`).Short('o').Default(
		teleport.SchemeStdout).StringVar(&dbConfigCreateFlags.output)
	dbConfigureCreate.Flag("dynamic-resources-labels", "Comma-separated list(s) of labels to match dynamic resources, for example env=dev,dept=it. Required to enable dynamic resources matching.").StringsVar(&dbConfigCreateFlags.DynamicResourcesRawLabels)
	dbConfigureCreate.Flag("trust-system-cert-pool", "Allows Teleport to trust certificate authorities available on the host system for self-hosted databases.").BoolVar(&dbConfigCreateFlags.DatabaseTrustSystemCertPool)
	dbConfigureCreate.Alias(dbCreateConfigExamples) // We're using "alias" section to display usage examples.

	dbConfigureBootstrap := dbConfigure.Command("bootstrap", "Bootstrap the necessary configuration for the database agent. It reads the provided agent configuration to determine what will be bootstrapped.")
	dbConfigureBootstrap.Flag("config", fmt.Sprintf("Path to a configuration file [%v].", defaults.ConfigFilePath)).Short('c').Default(defaults.ConfigFilePath).ExistingFileVar(&configureDiscoveryBootstrapFlags.config.ConfigPath)
	dbConfigureBootstrap.Flag("manual", "When executed in \"manual\" mode, it will print the instructions to complete the configuration instead of applying them directly.").BoolVar(&configureDiscoveryBootstrapFlags.config.Manual)
	dbConfigureBootstrap.Flag("policy-name", fmt.Sprintf("Name of the Teleport Database agent policy. Default: %q.", awsconfigurators.DatabaseAccessPolicyName)).Default(awsconfigurators.DatabaseAccessPolicyName).StringVar(&configureDiscoveryBootstrapFlags.config.PolicyName)
	dbConfigureBootstrap.Flag("confirm", "Do not prompt user and auto-confirm all actions.").BoolVar(&configureDiscoveryBootstrapFlags.confirm)
	dbConfigureBootstrap.Flag("attach-to-role", "Role name to attach policy to. Mutually exclusive with --attach-to-user. If none of the attach-to flags is provided, the command will try to attach the policy to the current user/role based on the credentials.").StringVar(&configureDiscoveryBootstrapFlags.config.AttachToRole)
	dbConfigureBootstrap.Flag("attach-to-user", "User name to attach policy to. Mutually exclusive with --attach-to-role. If none of the attach-to flags is provided, the command will try to attach the policy to the current user/role based on the credentials.").StringVar(&configureDiscoveryBootstrapFlags.config.AttachToUser)
	dbConfigureBootstrap.Flag("assumes-roles",
		"Comma-separated list of additional IAM roles that the IAM identity should be able to assume. Each role can be either an IAM role ARN or the name of a role in the identity's account.").
		StringVar(&configureDiscoveryBootstrapFlags.config.ForceAssumesRoles)

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
	dbConfigureAWSPrintIAM.Flag("assumes-roles",
		"Comma-separated list of additional IAM roles that the IAM identity should be able to assume. Each role can be either an IAM role ARN or the name of a role in the identity's account.").
		StringVar(&configureDatabaseAWSPrintFlags.assumesRoles)
	dbConfigureAWSCreateIAM := dbConfigureAWS.Command("create-iam", "Generate, create and attach IAM policies.")
	dbConfigureAWSCreateIAM.Flag("types",
		fmt.Sprintf("Comma-separated list of database types to include in the policy. Any of %s", strings.Join(awsDatabaseTypes, ","))).
		Short('r').
		StringVar(&configureDatabaseAWSCreateFlags.types)
	dbConfigureAWSCreateIAM.Flag("name", "Created policy name. Defaults to empty. Will be auto-generated if not provided.").Default(awsconfigurators.DatabaseAccessPolicyName).StringVar(&configureDatabaseAWSCreateFlags.policyName)
	// --attach flag is deprecated.
	dbConfigureAWSCreateIAM.Flag("attach", "Try to attach the policy to the IAM identity.").Hidden().Default("true").BoolVar(&configureDatabaseAWSCreateFlags.attach)
	dbConfigureAWSCreateIAM.Flag("confirm", "Do not prompt user and auto-confirm all actions.").BoolVar(&configureDatabaseAWSCreateFlags.confirm)
	dbConfigureAWSCreateIAM.Flag("role", "IAM role name to attach policy to. Mutually exclusive with --user").StringVar(&configureDatabaseAWSCreateFlags.role)
	dbConfigureAWSCreateIAM.Flag("user", "IAM user name to attach policy to. Mutually exclusive with --role").StringVar(&configureDatabaseAWSCreateFlags.user)
	dbConfigureAWSCreateIAM.Flag("assumes-roles",
		"Comma-separated list of additional IAM roles that the IAM identity should be able to assume. Each role can be either an IAM role ARN or the name of a role in the identity's account.").
		StringVar(&configureDatabaseAWSCreateFlags.assumesRoles)

	// "teleport discovery" bootstrap command and subcommands.
	discoveryCmd := app.Command("discovery", "Teleport discovery service commands")
	discoveryBootstrapCmd := discoveryCmd.Command("bootstrap", "Bootstrap the necessary configuration for the discovery agent. It reads the provided agent configuration to determine what will be bootstrapped.")
	discoveryBootstrapCmd.Flag("config", fmt.Sprintf("Path to a configuration file [%v].", defaults.ConfigFilePath)).Short('c').Default(defaults.ConfigFilePath).ExistingFileVar(&configureDiscoveryBootstrapFlags.config.ConfigPath)
	discoveryBootstrapCmd.Flag("confirm", "Do not prompt user and auto-confirm all actions.").BoolVar(&configureDiscoveryBootstrapFlags.confirm)
	discoveryBootstrapCmd.Flag("attach-to-role", "Role name to attach policy to. Mutually exclusive with --attach-to-user. If none of the attach-to flags is provided, the command will try to attach the policy to the current user/role based on the credentials.").StringVar(&configureDiscoveryBootstrapFlags.config.AttachToRole)
	discoveryBootstrapCmd.Flag("attach-to-user", "User name to attach policy to. Mutually exclusive with --attach-to-role. If none of the attach-to flags is provided, the command will try to attach the policy to the current user/role based on the credentials.").StringVar(&configureDiscoveryBootstrapFlags.config.AttachToUser)
	discoveryBootstrapCmd.Flag("policy-name", fmt.Sprintf("Name of the Teleport Discovery service policy. Default: %q.", awslib.EC2DiscoveryPolicyName)).Default(awslib.EC2DiscoveryPolicyName).StringVar(&configureDiscoveryBootstrapFlags.config.PolicyName)
	discoveryBootstrapCmd.Flag("proxy", "Teleport proxy address to connect to").StringVar(&configureDiscoveryBootstrapFlags.config.Proxy)
	discoveryBootstrapCmd.Flag("assumes-roles",
		"Comma-separated list of additional IAM roles that the IAM identity should be able to assume. Each role can be either an IAM role ARN or the name of a role in the identity's account.").
		StringVar(&configureDiscoveryBootstrapFlags.config.ForceAssumesRoles)
	discoveryBootstrapCmd.Flag("manual", "When executed in \"manual\" mode, it will print the instructions to complete the configuration instead of applying them directly.").BoolVar(&configureDiscoveryBootstrapFlags.config.Manual)
	discoveryBootstrapCmd.Flag("database-service-role", "Role name to attach database access policies to. If specified, bootstrap for the database service that accesses the databases discovered by this discovery service.").StringVar(&configureDiscoveryBootstrapFlags.databaseServiceRole)
	discoveryBootstrapCmd.Flag("database-service-policy-name", "Name of the policy for bootstrapping database service when database-service-role is provided. ").Default(awsconfigurators.DatabaseAccessPolicyName).StringVar(&configureDiscoveryBootstrapFlags.databaseServicePolicyName)

	// "teleport install" command and its subcommands
	installCmd := app.Command("install", "Teleport install commands.")
	systemdInstall := installCmd.Command("systemd", "Creates a systemd unit file configuration.")
	systemdInstall.Flag("env-file", "Full path to the environment file.").Default(config.SystemdDefaultEnvironmentFile).StringVar(&systemdInstallFlags.EnvironmentFile)
	systemdInstall.Flag("pid-file", "Full path to the PID file.").Default(config.SystemdDefaultPIDFile).StringVar(&systemdInstallFlags.PIDFile)
	systemdInstall.Flag("fd-limit", "Maximum number of open file descriptors.").Default(fmt.Sprintf("%v", config.SystemdDefaultFileDescriptorLimit)).IntVar(&systemdInstallFlags.FileDescriptorLimit)
	systemdInstall.Flag("teleport-path", "Full path to the Teleport binary.").StringVar(&systemdInstallFlags.TeleportInstallationFile)
	systemdInstall.Flag("output", `Write to stdout with "--output=stdout" or custom path with --output=file:///path`).Short('o').Default(teleport.SchemeStdout).StringVar(&systemdInstallFlags.output)
	systemdInstall.Alias(systemdInstallExamples) // We're using "alias" section to display usage examples.

	// This command is hidden because it is only meant to be used by the AutoDiscover script.
	installAutoDiscoverNode := installCmd.Command("autodiscover-node", "Installs, configures and starts teleport as a Node (used in Server Auto Discovery).").Hidden()
	installAutoDiscoverNode.Flag("public-proxy-addr", "Teleport public proxy address. Eg https://example.teleport.sh").StringVar(&installAutoDiscoverNodeFlags.ProxyPublicAddr)
	installAutoDiscoverNode.Flag("teleport-package", "Package name to install. Allowed: teleport or teleport-ent").StringVar(&installAutoDiscoverNodeFlags.TeleportPackage)
	installAutoDiscoverNode.Flag("repo-channel", "Repository channel to use (eg stable/cloud, stable/rolling or stable/vX).").StringVar(&installAutoDiscoverNodeFlags.RepositoryChannel)
	installAutoDiscoverNode.Flag("auto-upgrade", "Enables auto-upgrades. Allowed: true or false").StringVar(&installAutoDiscoverNodeFlags.AutoUpgradesString)
	installAutoDiscoverNode.Flag("azure-client-id", "Azure Client ID when installing in an Azure VM with multiple assigned identities.").StringVar(&installAutoDiscoverNodeFlags.AzureClientID)
	installAutoDiscoverNode.Arg("token", "Token to use to register with the cluster.").Required().StringVar(&installAutoDiscoverNodeFlags.TokenName)

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
		`Write to stdout with "--output=stdout", default config file with "--output=file" or custom path with --output=file:///path`).Short('o').Default(
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
	dump.Flag("token", "Invitation token or path to file with token value to register with an auth server.").StringVar(&dumpFlags.AuthToken)
	dump.Flag("roles", "Comma-separated list of roles to create config with.").StringVar(&dumpFlags.Roles)
	dump.Flag("auth-server", "Address of the auth server.").StringVar(&dumpFlags.AuthServer)
	dump.Flag("proxy", "Address of the proxy.").StringVar(&dumpFlags.ProxyAddress)
	dump.Flag("app-name", "Name of the application to start when using app role.").StringVar(&dumpFlags.AppName)
	dump.Flag("app-uri", "Internal address of the application to proxy.").StringVar(&dumpFlags.AppURI)
	dump.Flag("node-name", "Name for the Teleport node.").StringVar(&dumpFlags.NodeName)
	dump.Flag("node-labels", "Comma-separated list of labels to add to newly created nodes, for example env=staging,cloud=aws.").StringVar(&dumpFlags.NodeLabels)

	ver.Flag("raw", "Print the raw teleport version string.").BoolVar(&rawVersion)

	dumpNode := app.Command("node", "SSH Node configuration commands")
	dumpNodeConfigure := dumpNode.Command("configure", "Generate a configuration file for an SSH node.")
	dumpNodeConfigure.Flag("cluster-name",
		"Unique cluster name, e.g. example.com.").StringVar(&dumpFlags.ClusterName)
	dumpNodeConfigure.Flag("output",
		`Write to stdout with "--output=stdout", default config file with "--output=file" or custom path with --output=file:///path`).Short('o').Default(
		teleport.SchemeStdout).StringVar(&dumpFlags.output)
	dumpNodeConfigure.Flag("version", "Teleport configuration version.").Default(defaults.TeleportConfigVersionV3).StringVar(&dumpFlags.Version)
	dumpNodeConfigure.Flag("public-addr", "The hostport that the node advertises for the SSH endpoint.").StringVar(&dumpFlags.PublicAddr)
	dumpNodeConfigure.Flag("data-dir", "Path to a directory where Teleport keep its data.").Default(defaults.DataDir).StringVar(&dumpFlags.DataDir)
	dumpNodeConfigure.Flag("token", "Invitation token or path to file with token value to register with an auth server.").StringVar(&dumpFlags.AuthToken)
	dumpNodeConfigure.Flag("auth-server", "Address of the auth server.").StringVar(&dumpFlags.AuthServer)
	dumpNodeConfigure.Flag("proxy", "Address of the proxy server.").StringVar(&dumpFlags.ProxyAddress)
	dumpNodeConfigure.Flag("labels", "Comma-separated list of labels to add to newly created nodes ex) env=staging,cloud=aws.").StringVar(&dumpFlags.NodeLabels)
	dumpNodeConfigure.Flag("ca-pin", "Comma-separated list of SKPI hashes for the CA used to verify the auth server.").StringVar(&dumpFlags.CAPin)
	dumpNodeConfigure.Flag("join-method", "Method to use to join the cluster (token, iam, ec2, kubernetes, azure, gcp)").Default("token").EnumVar(&dumpFlags.JoinMethod, "token", "iam", "ec2", "kubernetes", "azure", "gcp")
	dumpNodeConfigure.Flag("node-name", "Name for the Teleport node.").StringVar(&dumpFlags.NodeName)
	dumpNodeConfigure.Flag("silent", "Suppress user hint message.").BoolVar(&dumpFlags.Silent)
	dumpNodeConfigure.Flag("azure-client-id", "Sets the client ID of the managed identity to join with. Only applies to the 'azure' join method.").StringVar(&dumpFlags.AzureClientID)

	waitCmd := app.Command(teleport.WaitSubCommand, "Used internally by Teleport to onWait until a specific condition is reached.").Hidden()
	waitNoResolveCmd := waitCmd.Command("no-resolve", "Used internally to onWait until a domain stops resolving IP addresses.").Hidden()
	waitNoResolveCmd.Arg("domain", "Domain that is resolved.").Hidden().StringVar(&waitFlags.domain)
	waitNoResolveCmd.Flag("period", "Resolution try period. A jitter is applied.").Default(waitNoResolveDefaultPeriod).Hidden().DurationVar(&waitFlags.period)
	waitNoResolveCmd.Flag("timeout", "Stops waiting after this duration and exits in error.").Default(waitNoResolveDefaultTimeout).Hidden().DurationVar(&waitFlags.timeout)
	waitDurationCmd := waitCmd.Command("duration", "Used internally to onWait a given duration before exiting.").Hidden()
	waitDurationCmd.Arg("duration", "Duration to onWait before exit.").Hidden().DurationVar(&waitFlags.duration)

	kubeState := app.Command("kube-state", "Used internally by Teleport to operate Kubernetes Secrets where Teleport stores its state.").Hidden()
	kubeStateDelete := kubeState.Command("delete", "Used internally to delete Kubernetes states when the helm chart is uninstalled.").Hidden()

	// teleport join --proxy-server=proxy.example.com --token=aws-join-token [--openssh-config=/path/to/sshd.conf] [--restart-sshd=true]
	joinOpenSSH.Flag("proxy-server", "Address of the proxy server.").StringVar(&ccf.ProxyServer)
	joinOpenSSH.Flag("token", "Invitation token or path to file with token value to register with an auth server.").StringVar(&ccf.AuthToken)
	joinOpenSSH.Flag("join-method", "Method to use to join the cluster (token, iam, ec2).").EnumVar(&ccf.JoinMethod, "token", "iam", "ec2")
	joinOpenSSH.Flag("openssh-config", fmt.Sprintf("Path to the OpenSSH config file [%v].", "/etc/ssh/sshd_config")).Default("/etc/ssh/sshd_config").StringVar(&ccf.OpenSSHConfigPath)
	joinOpenSSH.Flag("data-dir", fmt.Sprintf("Path to directory to store teleport data [%v].", defaults.DataDir)).Default(defaults.DataDir).StringVar(&ccf.DataDir)
	joinOpenSSH.Flag("restart-sshd", "Restart OpenSSH.").Default("true").BoolVar(&ccf.RestartOpenSSH)
	joinOpenSSH.Flag("sshd-check-command", "Command to use when checking OpenSSH config for validity. (sshd -t -f <sshd_config>)").Default("sshd -t -f").StringVar(&ccf.CheckCommand)
	joinOpenSSH.Flag("sshd-restart-command", "Command to use when restarting openssh.").Default(openssh.DefaultRestartCommand).StringVar(&ccf.RestartCommand)
	joinOpenSSH.Flag("labels", "Comma-separated list of labels for this OpenSSH node, for example env=dev,app=web.").StringVar(&ccf.Labels)
	joinOpenSSH.Flag("address", "Hostname or IP address of this OpenSSH node.").StringVar(&ccf.Address)
	joinOpenSSH.Flag("additional-principals", "Additional principal to include, can be specified multiple times.").StringVar(&ccf.AdditionalPrincipals)
	joinOpenSSH.Flag("insecure", "Insecure mode disables certificate validation.").BoolVar(&ccf.InsecureMode)

	joinOpenSSH.Flag("debug", "Enable verbose logging to stderr.").Short('d').BoolVar(&ccf.Debug)

	integrationCmd := app.Command("integration", "Integration commands")
	integrationConfigureCmd := integrationCmd.Command("configure", "Configure an integration")
	integrationConfDeployServiceCmd := integrationConfigureCmd.Command("deployservice-iam", "Create the required IAM Roles for the AWS OIDC Deploy Service.")
	integrationConfDeployServiceCmd.Flag("cluster", "Teleport Cluster's name.").Required().StringVar(&ccf.IntegrationConfDeployServiceIAMArguments.Cluster)
	integrationConfDeployServiceCmd.Flag("name", "Integration name.").Required().StringVar(&ccf.IntegrationConfDeployServiceIAMArguments.Name)
	integrationConfDeployServiceCmd.Flag("aws-region", "AWS Region.").Required().StringVar(&ccf.IntegrationConfDeployServiceIAMArguments.Region)
	integrationConfDeployServiceCmd.Flag("role", "The AWS Role used by the AWS OIDC Integration.").Required().StringVar(&ccf.IntegrationConfDeployServiceIAMArguments.Role)
	integrationConfDeployServiceCmd.Flag("task-role", "The AWS Role to be used by the deployed service.").Required().StringVar(&ccf.IntegrationConfDeployServiceIAMArguments.TaskRole)

	integrationConfEICECmd := integrationConfigureCmd.Command("eice-iam", "Adds required IAM permissions to connect to EC2 Instances using EC2 Instance Connect Endpoint.")
	integrationConfEICECmd.Flag("aws-region", "AWS Region.").Required().StringVar(&ccf.IntegrationConfEICEIAMArguments.Region)
	integrationConfEICECmd.Flag("role", "The AWS Role used by the AWS OIDC Integration.").Required().StringVar(&ccf.IntegrationConfEICEIAMArguments.Role)

	integrationConfEC2SSMCmd := integrationConfigureCmd.Command("ec2-ssm-iam", "Adds required IAM permissions and SSM Document to enable EC2 Auto Discover using SSM.")
	integrationConfEC2SSMCmd.Flag("role", "The AWS Role name used by the AWS OIDC Integration.").Required().StringVar(&ccf.IntegrationConfEC2SSMIAMArguments.RoleName)
	integrationConfEC2SSMCmd.Flag("aws-region", "AWS Region.").Required().StringVar(&ccf.IntegrationConfEC2SSMIAMArguments.Region)
	integrationConfEC2SSMCmd.Flag("ssm-document-name", "The AWS SSM Document name to create that will be used to install teleport.").Required().StringVar(&ccf.IntegrationConfEC2SSMIAMArguments.SSMDocumentName)
	integrationConfEC2SSMCmd.Flag("proxy-public-url", "Proxy Public URL (eg https://mytenant.teleport.sh).").StringVar(&ccf.
		IntegrationConfEC2SSMIAMArguments.ProxyPublicURL)
	integrationConfEC2SSMCmd.Flag("cluster", "Teleport Cluster's name.").Required().StringVar(&ccf.IntegrationConfEC2SSMIAMArguments.ClusterName)
	integrationConfEC2SSMCmd.Flag("name", "Integration name.").Required().StringVar(&ccf.IntegrationConfEC2SSMIAMArguments.IntegrationName)

	integrationConfAWSAppAccessCmd := integrationConfigureCmd.Command("aws-app-access-iam", "Adds required IAM permissions to connect to AWS using App Access.")
	integrationConfAWSAppAccessCmd.Flag("role", "The AWS Role name used by the AWS OIDC Integration.").Required().StringVar(&ccf.IntegrationConfAWSAppAccessIAMArguments.RoleName)

	integrationConfEKSCmd := integrationConfigureCmd.Command("eks-iam", "Adds required IAM permissions for enrollment of EKS clusters to Teleport.")
	integrationConfEKSCmd.Flag("aws-region", "AWS Region.").Required().StringVar(&ccf.IntegrationConfEKSIAMArguments.Region)
	integrationConfEKSCmd.Flag("role", "The AWS Role used by the AWS OIDC Integration.").Required().StringVar(&ccf.IntegrationConfEKSIAMArguments.Role)

	integrationConfAccessGraphCmd := integrationConfigureCmd.Command("access-graph", "Manages Access Graph configuration.")
	integrationConfTAGSyncCmd := integrationConfAccessGraphCmd.Command("aws-iam", "Adds required IAM permissions for syncing data into Access Graph service.")
	integrationConfTAGSyncCmd.Flag("role", "The AWS Role used by the AWS OIDC Integration.").Required().StringVar(&ccf.IntegrationConfAccessGraphAWSSyncArguments.Role)

	integrationConfAWSOIDCIdPCmd := integrationConfigureCmd.Command("awsoidc-idp", "Creates an IAM IdP (OIDC) in your AWS account to allow the AWS OIDC Integration to access AWS APIs.")
	integrationConfAWSOIDCIdPCmd.Flag("cluster", "Teleport Cluster name.").Required().StringVar(&ccf.
		IntegrationConfAWSOIDCIdPArguments.Cluster)
	integrationConfAWSOIDCIdPCmd.Flag("name", "Integration name.").Required().StringVar(&ccf.
		IntegrationConfAWSOIDCIdPArguments.Name)
	integrationConfAWSOIDCIdPCmd.Flag("role", "The AWS Role used by the AWS OIDC Integration.").Required().StringVar(&ccf.
		IntegrationConfAWSOIDCIdPArguments.Role)
	integrationConfAWSOIDCIdPCmd.Flag("proxy-public-url", "Proxy Public URL (eg https://mytenant.teleport.sh).").Required().StringVar(&ccf.
		IntegrationConfAWSOIDCIdPArguments.ProxyPublicURL)
	integrationConfAWSOIDCIdPCmd.Flag("insecure", "Insecure mode disables certificate validation.").BoolVar(&ccf.InsecureMode)

	integrationConfListDatabasesCmd := integrationConfigureCmd.Command("listdatabases-iam", "Adds required IAM permissions to List RDS Databases (Instances and Clusters).")
	integrationConfListDatabasesCmd.Flag("aws-region", "AWS Region.").Required().StringVar(&ccf.IntegrationConfListDatabasesIAMArguments.Region)
	integrationConfListDatabasesCmd.Flag("role", "The AWS Role used by the AWS OIDC Integration.").Required().StringVar(&ccf.IntegrationConfListDatabasesIAMArguments.Role)

	integrationConfExternalAuditCmd := integrationConfigureCmd.Command("externalauditstorage", "Bootstraps required infrastructure and adds required IAM permissions for External Audit Storage logs.")
	integrationConfExternalAuditCmd.Flag("bootstrap", "Bootstrap required infrastructure.").Default("false").BoolVar(&ccf.IntegrationConfExternalAuditStorageArguments.Bootstrap)
	integrationConfExternalAuditCmd.Flag("aws-region", "AWS region.").Required().StringVar(&ccf.IntegrationConfExternalAuditStorageArguments.Region)
	integrationConfExternalAuditCmd.Flag("cluster-name", "Teleport Cluster name.").Required().StringVar(&ccf.IntegrationConfExternalAuditStorageArguments.ClusterName)
	integrationConfExternalAuditCmd.Flag("integration", "AWS OIDC Integration name.").Required().StringVar(&ccf.IntegrationConfExternalAuditStorageArguments.IntegrationName)
	integrationConfExternalAuditCmd.Flag("role", "The IAM Role used by the AWS OIDC Integration.").Required().StringVar(&ccf.IntegrationConfExternalAuditStorageArguments.Role)
	integrationConfExternalAuditCmd.Flag("policy", "The name for the Policy to attach to the IAM role.").Required().StringVar(&ccf.IntegrationConfExternalAuditStorageArguments.Policy)
	integrationConfExternalAuditCmd.Flag("session-recordings", "The S3 URI where session recordings are stored.").Required().StringVar(&ccf.IntegrationConfExternalAuditStorageArguments.SessionRecordingsURI)
	integrationConfExternalAuditCmd.Flag("audit-events", "The S3 URI where audit events are stored.").Required().StringVar(&ccf.IntegrationConfExternalAuditStorageArguments.AuditEventsURI)
	integrationConfExternalAuditCmd.Flag("athena-results", "The S3 URI where athena results are stored.").Required().StringVar(&ccf.IntegrationConfExternalAuditStorageArguments.AthenaResultsURI)
	integrationConfExternalAuditCmd.Flag("athena-workgroup", "The name of the Athena workgroup used.").Required().StringVar(&ccf.IntegrationConfExternalAuditStorageArguments.AthenaWorkgroup)
	integrationConfExternalAuditCmd.Flag("glue-database", "The name of the Glue database used.").Required().StringVar(&ccf.IntegrationConfExternalAuditStorageArguments.GlueDatabase)
	integrationConfExternalAuditCmd.Flag("glue-table", "The name of the Glue table used.").Required().StringVar(&ccf.IntegrationConfExternalAuditStorageArguments.GlueTable)
	integrationConfExternalAuditCmd.Flag("aws-partition", "AWS partition (default: aws).").Default("aws").StringVar(&ccf.IntegrationConfExternalAuditStorageArguments.Partition)

	integrationConfAzureOIDCCmd := integrationConfigureCmd.Command("azure-oidc", "Configures Azure / Entra ID OIDC integration")
	integrationConfAzureOIDCCmd.Flag("proxy-public-addr", "The public address of Teleport Proxy Service").Required().StringVar(&ccf.IntegrationConfAzureOIDCArguments.ProxyPublicAddr)
	integrationConfAzureOIDCCmd.Flag("auth-connector-name", "The name of Entra ID SAML Auth connector in Teleport.").Required().StringVar(&ccf.IntegrationConfAzureOIDCArguments.AuthConnectorName)
	integrationConfAzureOIDCCmd.Flag("access-graph", "Enable Access Graph integration.").BoolVar(&ccf.IntegrationConfAzureOIDCArguments.AccessGraphEnabled)

	integrationConfSAMLIdP := integrationConfigureCmd.Command("samlidp", "Manage SAML IdP integrations.")
	integrationSAMLIdPGCPWorkforce := integrationConfSAMLIdP.Command("gcp-workforce", "Configures GCP Workforce Identity Federation pool and SAML provider.")
	integrationSAMLIdPGCPWorkforce.Flag("org-id", "GCP organization ID.").Required().StringVar(&ccf.IntegrationConfSAMLIdPGCPWorkforceArguments.OrganizationID)
	integrationSAMLIdPGCPWorkforce.Flag("pool-name", "Name for the new workforce identity pool.").Required().StringVar(&ccf.IntegrationConfSAMLIdPGCPWorkforceArguments.PoolName)
	integrationSAMLIdPGCPWorkforce.Flag("pool-provider-name", "Name for the new workforce identity pool provider.").Required().StringVar(&ccf.IntegrationConfSAMLIdPGCPWorkforceArguments.PoolProviderName)
	integrationSAMLIdPGCPWorkforce.Flag("idp-metadata-url", "Teleport SAML IdP metadata endpoint.").Required().StringVar(&ccf.IntegrationConfSAMLIdPGCPWorkforceArguments.SAMLIdPMetadataURL)

	tpmCmd := app.Command("tpm", "Commands related to managing TPM joining functionality.")
	tpmIdentifyCmd := tpmCmd.Command("identify", "Output identifying information related to the TPM detected on the system.")

	debugCmd := app.Command("debug", "Debug commands")
	debugCmd.Flag("config", fmt.Sprintf("Path to a configuration file [%v].", defaults.ConfigFilePath)).Short('c').ExistingFileVar(&ccf.ConfigFile)
	setLogLevelCmd := debugCmd.Command("set-log-level", "Changes the log level.")
	setLogLevelCmd.Arg("LEVEL", fmt.Sprintf("Log level (case-insensitive). Any of: %s", strings.Join(logutils.SupportedLevelsText, ","))).Required().StringVar(&ccf.LogLevel)
	getLogLevelCmd := debugCmd.Command("get-log-level", "Fetches current log level.")
	collectProfilesCmd := debugCmd.Command("profile", "Export the application profiles (pprof format). Outputs to stdout .tar.gz file contents.")
	collectProfilesCmd.Alias(collectProfileUsageExamples) // We're using "alias" section to display usage examples.
	collectProfilesCmd.Arg("PROFILES", fmt.Sprintf("Comma-separated profile names to be exported. Supported profiles: %s. Default: %s", strings.Join(maps.Keys(debugclient.SupportedProfiles), ","), strings.Join(defaultCollectProfiles, ","))).StringVar(&ccf.Profiles)
	collectProfilesCmd.Flag("seconds", "For CPU and trace profiles, profile for the given duration (if set to 0, it returns a profile snapshot). For other profiles, return a delta profile. Default: 0").Short('s').Default("0").IntVar(&ccf.ProfileSeconds)

	backendCmd := app.Command("backend", "Commands for managing backend data.")
	backendCmd.Hidden()
	backendCloneCmd := backendCmd.Command("clone", "Clones data from a source to a destination backend.")
	backendCloneCmd.Flag("config", "Path to the clone config file.").
		Required().
		Short('c').
		StringVar(&ccf.ConfigFile)
	backendCloneCmd.Alias(`
Examples:

  When cloning a backend you must specify a clone configuration file:

  > teleport backend clone --config clone.yaml

  The following example configuration will clone Teleport's backend
  data from sqlite to dynamodb:

  # src is the configuration for the backend where data is cloned from.
  src:
    type: sqlite
    path: /var/lib/teleport_data
  # dst is the configuration for the backend where data is cloned to.
  dst:
    type: dynamodb
    region: us-east-1
    table: teleport_backend
  # parallel is the amount of backend data cloned in parallel.
  # If a clone operation is taking too long consider increasing this value.
  parallel: 100
  # force, if set to true, will continue cloning data to a destination
  # regardless of whether data is already present. By default this is false
  # to protect against overwriting the data of an existing Teleport cluster.
  force: false`)

	// parse CLI commands+flags:
	utils.UpdateAppUsageTemplate(app, options.Args)
	command, err := app.Parse(options.Args)
	if err != nil {
		app.Usage(options.Args)
		utils.FatalError(err)
	}

	// Create default configuration.
	conf = servicecfg.MakeDefaultConfig()

	// If FIPS mode is specified update defaults to be FIPS appropriate and
	// cross-validate the current config.
	if ccf.FIPS {
		if ccf.InsecureMode {
			utils.FatalError(trace.BadParameter("--insecure not allowed in FIPS mode"))
		}
		servicecfg.ApplyFIPSDefaults(conf)
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
		if err = config.Configure(&ccf, conf, command != appStartCmd.FullCommand()); err != nil {
			utils.FatalError(err)
		}

		// Validate binary modules against the device trust configuration.
		// Catches errors in file-based configs.
		if conf.Auth.Enabled {
			if err := dtconfig.ValidateConfigAgainstModules(conf.Auth.Preference.GetDeviceTrust()); err != nil {
				utils.FatalError(err)
			}
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
	case networking.FullCommand():
		srv.RunAndExit(teleport.NetworkingSubCommand)
	case checkHomeDir.FullCommand():
		srv.RunAndExit(teleport.CheckHomeDirSubCommand)
	case park.FullCommand():
		srv.RunAndExit(teleport.ParkSubCommand)
	case waitNoResolveCmd.FullCommand():
		err = onWaitNoResolve(waitFlags)
	case waitDurationCmd.FullCommand():
		err = onWaitDuration(waitFlags)
	case kubeStateDelete.FullCommand():
		err = onKubeStateDelete()
	case ver.FullCommand():
		if rawVersion {
			// raw version must print the exact version string (relied upon
			// by the systemd unit upgrader).
			fmt.Printf("%s\n", teleport.Version)
		} else {
			modules.GetModules().PrintVersion()
		}
	case dbConfigureCreate.FullCommand():
		err = onDumpDatabaseConfig(dbConfigCreateFlags)
	case dbConfigureAWSPrintIAM.FullCommand():
		err = onConfigureDatabasesAWSPrint(configureDatabaseAWSPrintFlags)
	case dbConfigureAWSCreateIAM.FullCommand():
		err = onConfigureDatabasesAWSCreate(configureDatabaseAWSCreateFlags)
	case dbConfigureBootstrap.FullCommand():
		configureDiscoveryBootstrapFlags.config.Service = configurators.DatabaseService
		err = onConfigureDiscoveryBootstrap(configureDiscoveryBootstrapFlags)
	case systemdInstall.FullCommand():
		err = onDumpSystemdUnitFile(systemdInstallFlags)
	case installAutoDiscoverNode.FullCommand():
		err = onInstallAutoDiscoverNode(installAutoDiscoverNodeFlags)
	case discoveryBootstrapCmd.FullCommand():
		configureDiscoveryBootstrapFlags.config.Service = configurators.DiscoveryService
		err = onConfigureDiscoveryBootstrap(configureDiscoveryBootstrapFlags)
	case joinOpenSSH.FullCommand():
		err = onJoinOpenSSH(ccf, conf)
	case integrationConfDeployServiceCmd.FullCommand():
		err = onIntegrationConfDeployService(ctx, ccf.IntegrationConfDeployServiceIAMArguments)
	case integrationConfEICECmd.FullCommand():
		err = onIntegrationConfEICEIAM(ctx, ccf.IntegrationConfEICEIAMArguments)
	case integrationConfEC2SSMCmd.FullCommand():
		err = onIntegrationConfEC2SSMIAM(ctx, ccf.IntegrationConfEC2SSMIAMArguments)
	case integrationConfAWSAppAccessCmd.FullCommand():
		err = onIntegrationConfAWSAppAccessIAM(ctx, ccf.IntegrationConfAWSAppAccessIAMArguments)
	case integrationConfEKSCmd.FullCommand():
		err = onIntegrationConfEKSIAM(ctx, ccf.IntegrationConfEKSIAMArguments)
	case integrationConfAWSOIDCIdPCmd.FullCommand():
		err = onIntegrationConfAWSOIDCIdP(ctx, ccf)
	case integrationConfListDatabasesCmd.FullCommand():
		err = onIntegrationConfListDatabasesIAM(ctx, ccf.IntegrationConfListDatabasesIAMArguments)
	case integrationConfExternalAuditCmd.FullCommand():
		err = onIntegrationConfExternalAuditCmd(ctx, ccf.IntegrationConfExternalAuditStorageArguments)
	case integrationConfTAGSyncCmd.FullCommand():
		err = onIntegrationConfAccessGraphAWSSync(ctx, ccf.IntegrationConfAccessGraphAWSSyncArguments)
	case integrationConfAzureOIDCCmd.FullCommand():
		err = onIntegrationConfAzureOIDCCmd(ctx, ccf.IntegrationConfAzureOIDCArguments)
	case integrationSAMLIdPGCPWorkforce.FullCommand():
		err = onIntegrationConfSAMLIdPGCPWorkforce(ctx, ccf.IntegrationConfSAMLIdPGCPWorkforceArguments)
	case tpmIdentifyCmd.FullCommand():
		var query *tpm.QueryRes
		query, err = tpm.Query(context.Background(), slog.Default())
		if err != nil {
			break
		}
		tpm.PrintQuery(query, ccf.Debug, os.Stdout)
	case setLogLevelCmd.FullCommand():
		err = onSetLogLevel(ccf.ConfigFile, ccf.LogLevel)
	case getLogLevelCmd.FullCommand():
		err = onGetLogLevel(ccf.ConfigFile)
	case collectProfilesCmd.FullCommand():
		err = onCollectProfiles(ccf.ConfigFile, ccf.Profiles, ccf.ProfileSeconds)
	case backendCloneCmd.FullCommand():
		err = onClone(context.Background(), ccf.ConfigFile)
	}
	if err != nil {
		utils.FatalError(err)
	}

	return app, command, conf
}

// OnStart is the handler for "start" CLI command
func OnStart(clf config.CommandLineFlags, config *servicecfg.Config) error {
	// check to see if the config file is not passed and if the
	// default config file is available. If available it will be used
	configFileUsed := clf.ConfigFile
	if clf.ConfigFile == "" && utils.FileExists(defaults.ConfigFilePath) {
		configFileUsed = defaults.ConfigFilePath
	}

	ctx := context.Background()
	if configFileUsed == "" {
		config.Logger.InfoContext(ctx, "Starting Teleport", "version", teleport.Version)
	} else {
		config.Logger.InfoContext(ctx, "Starting Teleport with a config file", "version", teleport.Version, "config_file", configFileUsed)
	}
	return service.Run(ctx, *config, nil)
}

// onStatus is the handler for "status" CLI command
func onStatus() error {
	sshClient := os.Getenv("SSH_CLIENT")
	systemUser := os.Getenv("USER")
	teleportUser := os.Getenv(teleport.SSHTeleportUser)
	proxyAddr := os.Getenv(teleport.SSHSessionWebProxyAddr)
	clusterName := os.Getenv(teleport.SSHTeleportClusterName)
	hostUUID := os.Getenv(teleport.SSHTeleportHostUUID)
	sid := os.Getenv(teleport.SSHSessionID)

	// For node sessions started by `ssh`, the proxyhost is not
	// set in the session env. Provide a placeholder.
	if proxyAddr == "" {
		proxyAddr = "<proxyhost>:3080"
	}

	if sid == "" {
		fmt.Println("You are not inside of a Teleport SSH session")
		return nil
	}

	fmt.Printf("User ID     : %s, logged in as %s from %s\n", teleportUser, systemUser, sshClient)
	fmt.Printf("Cluster Name: %s\n", clusterName)
	fmt.Printf("Host UUID   : %s\n", hostUUID)
	fmt.Printf("Session ID  : %s\n", sid)
	fmt.Printf("Session URL : https://%s/web/cluster/%s/console/session/%s\n", proxyAddr, clusterName, sid)

	return nil
}

type dumpFlags struct {
	config.SampleFlags
	output         string
	testConfigFile string
	stdout         io.Writer
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

	if flags.stdout == nil {
		flags.stdout = os.Stdout
	}
	if flags.Silent {
		flags.stdout = io.Discard
	}

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

	if modules.GetModules().IsEnterpriseBuild() {
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
			os.Stderr, "%s Could not check the contents of %s: %s\n         The data directory may contain existing cluster state.\n", utils.Color(utils.Yellow, "WARNING:"), flags.DataDir, err.Error())
	}

	if err == nil && len(entries) != 0 {
		fmt.Fprintf(
			os.Stderr,
			"%s The data directory %s is not empty and may contain existing cluster state. Running this configuration is likely a mistake. To join a new cluster, specify an alternate --data-dir or clear the %s directory.\n",
			utils.Color(utils.Yellow, "WARNING:"), flags.DataDir, flags.DataDir)
	}

	if strings.Contains(flags.Roles, defaults.RoleDatabase) {
		fmt.Fprintf(os.Stderr, "%s Role db requires further configuration, db_service will be disabled. Use 'teleport db configure' command to create Teleport database service configurations.\n", utils.Color(utils.Red, "ERROR:"))
	}

	if strings.Contains(flags.Roles, defaults.RoleWindowsDesktop) {
		fmt.Fprintf(os.Stderr, "%s Role windowsdesktop requires further configuration, windows_desktop_service will be disabled. See https://goteleport.com/docs/desktop-access/ for configuration information.\n", utils.Color(utils.Red, "ERROR:"))
	}

	if configPath != "" {
		canWriteToDataDir, err := utils.CanUserWriteTo(flags.DataDir)
		if err != nil && !trace.IsNotImplemented(err) {
			fmt.Fprintf(os.Stderr, "%s Failed to check data dir permissions: %+v\n", utils.Color(utils.Yellow, "WARNING:"), err)
		}
		canWriteToConfDir, err := utils.CanUserWriteTo(filepath.Dir(configPath))
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s Failed to check config dir permissions: %+v\n", utils.Color(utils.Yellow, "WARNING:"), err)
		}
		requiresRoot := !canWriteToDataDir || !canWriteToConfDir

		fmt.Fprintf(flags.stdout, "\nA Teleport configuration file has been created at %q.\n", configPath)
		if modules.GetModules().IsEnterpriseBuild() {
			fmt.Fprintf(flags.stdout, "Add your Teleport license file to %q.\n", flags.LicensePath)
		}
		fmt.Fprintf(flags.stdout, "To start Teleport with this configuration file, run:\n\n")
		if requiresRoot {
			fmt.Fprintf(flags.stdout, "sudo teleport start --config=%q\n\n", configPath)
			fmt.Fprintf(flags.stdout, "Note that starting a Teleport server with this configuration will require root access as:\n")
			if !canWriteToConfDir {
				fmt.Fprintf(flags.stdout, "- The Teleport configuration is located at %q.\n", configPath)
			}
			if !canWriteToDataDir {
				fmt.Fprintf(flags.stdout, "- Teleport will be storing data at %q. To change that, edit the \"data_dir\" field in %q.", flags.DataDir, configPath)
			}
			fmt.Fprintf(flags.stdout, "\n")
		} else {
			fmt.Fprintf(flags.stdout, "teleport start --config=%q\n\n", configPath)
		}

		fmt.Fprintf(flags.stdout, "Happy Teleporting!\n")
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

		configDir := path.Dir(uri.Path)
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
	utils.SwitchLoggingToSyslog()
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

func onJoinOpenSSH(clf config.CommandLineFlags, conf *servicecfg.Config) error {
	// configuration merge: defaults -> file-based conf -> CLI conf
	conf.OpenSSH.Enabled = true
	if err := config.ConfigureOpenSSH(&clf, conf); err != nil {
		return trace.Wrap(err)
	}
	if err := OnStart(clf, conf); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
