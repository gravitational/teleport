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
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/srv"
	"github.com/gravitational/teleport/lib/sshutils/scp"
	"github.com/gravitational/teleport/lib/utils"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
	app = utils.InitCLIParser("teleport", "Clustered SSH service. Learn more at https://goteleport.com/teleport")

	// define global flags:
	var ccf config.CommandLineFlags
	var scpFlags scp.Flags
	var dumpFlags dumpFlags

	// define commands:
	start := app.Command("start", "Starts the Teleport service.")
	status := app.Command("status", "Print the status of the current SSH session.")
	dump := app.Command("configure", "Generate a simple config file to get started.")
	ver := app.Command("version", "Print the version.")
	scpc := app.Command("scp", "Server-side implementation of SCP.").Hidden()
	exec := app.Command("exec", "Used internally by Teleport to re-exec itself to run a command.").Hidden()
	forward := app.Command("forward", "Used internally by Teleport to re-exec itself to port forward.").Hidden()
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
		"Start diagnostic prometheus and healthz endpoint.").Hidden().StringVar(&ccf.DiagnosticAddr)
	start.Flag("permit-user-env",
		"Enables reading of ~/.tsh/environment when creating a session").BoolVar(&ccf.PermitUserEnvironment)
	start.Flag("insecure",
		"Insecure mode disables certificate validation").BoolVar(&ccf.InsecureMode)
	start.Flag("fips",
		"Start Teleport in FedRAMP/FIPS 140-2 mode.").
		Default("false").
		BoolVar(&ccf.FIPS)
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
		"AWS region RDS, Aurora or Redshift database instance is running in.").Hidden().
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
	dbStartCmd.Flag("aws-region", "(Only for RDS, Aurora or Redshift) AWS region RDS, Aurora or Redshift database instance is running in.").StringVar(&ccf.DatabaseAWSRegion)
	dbStartCmd.Flag("aws-redshift-cluster-id", "(Only for Redshift) Redshift database cluster identifier.").StringVar(&ccf.DatabaseAWSRedshiftClusterID)
	dbStartCmd.Flag("aws-rds-instance-id", "(Only for RDS) RDS instance identifier.").StringVar(&ccf.DatabaseAWSRDSInstanceID)
	dbStartCmd.Flag("aws-rds-cluster-id", "(Only for Aurora) Aurora cluster identifier.").StringVar(&ccf.DatabaseAWSRDSClusterID)
	dbStartCmd.Flag("gcp-project-id", "(Only for Cloud SQL) GCP Cloud SQL project identifier.").StringVar(&ccf.DatabaseGCPProjectID)
	dbStartCmd.Flag("gcp-instance-id", "(Only for Cloud SQL) GCP Cloud SQL instance identifier.").StringVar(&ccf.DatabaseGCPInstanceID)
	dbStartCmd.Alias(dbUsageExamples) // We're using "alias" section to display usage examples.

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

	// parse CLI commands+flags:
	command, err := app.Parse(options.Args)
	if err != nil {
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
	case start.FullCommand(), appStartCmd.FullCommand(), dbStartCmd.FullCommand():
		// Set appropriate roles for "app" and "db" subcommands.
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
			err = OnStart(conf)
		}
	case scpc.FullCommand():
		err = onSCP(&scpFlags)
	case status.FullCommand():
		err = onStatus()
	case dump.FullCommand():
		err = onConfigDump(dumpFlags)
	case exec.FullCommand():
		err = onExec()
	case forward.FullCommand():
		err = onForward()
	case ver.FullCommand():
		utils.PrintVersion()
	}
	if err != nil {
		utils.FatalError(err)
	}
	return app, command, conf
}

// OnStart is the handler for "start" CLI command
func OnStart(config *service.Config) error {
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

	if sid == "" || proxyHost == "" {
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
	if flags.output == "" || flags.output == teleport.SchemeFile {
		flags.output = teleport.SchemeFile + "://" + defaults.ConfigFilePath
	} else if flags.output == teleport.SchemeStdout {
		flags.output = teleport.SchemeStdout + "://"
	}
	return nil
}

// onConfigDump is the handler for "configure" CLI command
func onConfigDump(flags dumpFlags) error {
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

	// Generate a new config.
	uri, err := url.Parse(flags.output)
	if err != nil {
		return trace.BadParameter("could not parse output value %q, use --output=%q",
			flags.output, defaults.ConfigFilePath)
	}

	if modules.GetModules().BuildType() != modules.BuildOSS {
		flags.LicensePath = filepath.Join(defaults.DataDir, "license.pem")
	}

	sfc, err := config.MakeSampleFileConfig(flags.SampleFlags)
	if err != nil {
		return trace.Wrap(err)
	}
	switch uri.Scheme {
	case teleport.SchemeStdout:
		fmt.Printf("%s\n%s\n", sampleConfComment, sfc.DebugDumpToYAML())
	case teleport.SchemeFile, "":
		if uri.Path == "" {
			return trace.BadParameter("missing path in --output=%q", uri)
		}
		if !filepath.IsAbs(uri.Path) {
			return trace.BadParameter("please use absolute path for file %v", uri.Path)
		}
		f, err := os.OpenFile(uri.Path, os.O_RDWR|os.O_CREATE|os.O_EXCL, teleport.FileMaskOwnerOnly)
		err = trace.ConvertSystemError(err)
		if err != nil {
			if trace.IsAlreadyExists(err) {
				return trace.AlreadyExists("will not overwrite existing file %v, rm -f %v and try again", uri.Path, uri.Path)
			}
			return trace.Wrap(err, "could not write to config file, missing sudo?")
		}
		if _, err := f.Write([]byte(sfc.DebugDumpToYAML())); err != nil {
			f.Close()
			return trace.Wrap(trace.ConvertSystemError(err), "could not write to config file, missing sudo?")
		}
		if err := f.Close(); err != nil {
			return trace.Wrap(err, "could not close file %v", uri.Path)
		}
		if modules.GetModules().BuildType() == modules.BuildOSS {
			fmt.Printf("Wrote config to file %q. Now you can start the server. Happy Teleporting!\n", uri.Path)
		} else {
			fmt.Printf("Wrote config to file %q. Add your license file to %v and start the server. Happy Teleporting!\n", uri.Path, flags.LicensePath)
		}
	default:
		return trace.BadParameter(
			"unsupported --output=%v, use path for example --output=%v", uri.Scheme, defaults.ConfigFilePath)
	}

	return nil
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

// onExec is a subcommand used to re-execute Teleport for execution. Used for
// "exec" or "shell" requests over a "session" channel on Teleport nodes.
func onExec() error {
	srv.RunAndExit(teleport.ExecSubCommand)
	return nil
}

// onForward is a subcommand used to re-execute Teleport for port forwarding.
// Used with "direct-tcpip" channel on Teleport nodes.
func onForward() error {
	srv.RunAndExit(teleport.ForwardSubCommand)
	return nil
}

type StdReadWriter struct {
}

func (rw *StdReadWriter) Read(b []byte) (int, error) {
	return os.Stdin.Read(b)
}

func (rw *StdReadWriter) Write(b []byte) (int, error) {
	return os.Stdout.Write(b)
}
