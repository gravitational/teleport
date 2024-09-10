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
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/webclient"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/mfa"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	libmfa "github.com/gravitational/teleport/lib/client/mfa"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/reversetunnelclient"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/common"
)

const (
	searchHelp = `List of comma separated search keywords or phrases enclosed in quotations (e.g. --search=foo,bar,"some phrase")`
	queryHelp  = `Query by predicate language enclosed in single quotes. Supports ==, !=, &&, and || (e.g. --query='labels["key1"] == "value1" && labels["key2"] != "value2"')`
	labelHelp  = "List of comma separated labels to filter by labels (e.g. key1=value1,key2=value2)"
)

const (
	identityFileEnvVar = "TELEPORT_IDENTITY_FILE"
	authAddrEnvVar     = "TELEPORT_AUTH_SERVER"
)

// GlobalCLIFlags keeps the CLI flags that apply to all tctl commands
type GlobalCLIFlags struct {
	// Debug enables verbose logging mode to the console
	Debug bool
	// ConfigFile is the path to the Teleport configuration file
	ConfigFile string
	// ConfigString is the base64-encoded string with Teleport configuration
	ConfigString string
	// AuthServerAddr lists addresses of auth or proxy servers to connect to,
	AuthServerAddr []string
	// IdentityFilePath is the path to the identity file
	IdentityFilePath string
	// Insecure, when set, skips validation of server TLS certificate when
	// connecting through a proxy (specified in AuthServerAddr).
	Insecure bool
}

// CLICommand interface must be implemented by every CLI command
//
// This allows OSS and Enterprise Teleport editions to plug their own
// implementations of different CLI commands into the common execution
// framework
type CLICommand interface {
	// Initialize allows a caller-defined command to plug itself into CLI
	// argument parsing
	Initialize(*kingpin.Application, *servicecfg.Config)

	// TryRun is executed after the CLI parsing is done. The command must
	// determine if selectedCommand belongs to it and return match=true
	TryRun(ctx context.Context, selectedCommand string, c *authclient.Client) (match bool, err error)
}

// Run is the same as 'make'. It helps to share the code between different
// "distributions" like OSS or Enterprise
//
// distribution: name of the Teleport distribution
func Run(commands []CLICommand) {
	err := TryRun(commands, os.Args[1:])
	if err != nil {
		var exitError *common.ExitCodeError
		if errors.As(err, &exitError) {
			os.Exit(exitError.Code)
		}
		utils.FatalError(err)
	}
}

// TryRun is a helper function for Run to call - it runs a tctl command and returns an error.
// This is useful for testing tctl, because we can capture the returned error in tests.
func TryRun(commands []CLICommand, args []string) error {
	utils.InitLogger(utils.LoggingForCLI, slog.LevelWarn)

	// app is the command line parser
	app := utils.InitCLIParser("tctl", GlobalHelpString)

	// cfg (teleport auth server configuration) is going to be shared by all
	// commands
	cfg := servicecfg.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()

	// each command will add itself to the CLI parser:
	for i := range commands {
		commands[i].Initialize(app, cfg)
	}

	var ccf GlobalCLIFlags

	// If the config file path is being overridden by environment variable, set that.
	// If not, check whether the default config file path exists and set that if so.
	// This preserves tctl's default behavior for backwards compatibility.
	configFileEnvar, isSet := os.LookupEnv(defaults.ConfigFileEnvar)
	if isSet {
		ccf.ConfigFile = configFileEnvar
	} else {
		if utils.FileExists(defaults.ConfigFilePath) {
			ccf.ConfigFile = defaults.ConfigFilePath
		}
	}

	// these global flags apply to all commands
	app.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		BoolVar(&ccf.Debug)
	app.Flag("config", fmt.Sprintf("Path to a configuration file [%v] for an Auth Service instance. Can also be set via the %v environment variable. Ignored if the auth_service is disabled.", defaults.ConfigFilePath, defaults.ConfigFileEnvar)).
		Short('c').
		ExistingFileVar(&ccf.ConfigFile)
	app.Flag("config-string",
		"Base64 encoded configuration string. Ignored if the config auth_service is disabled.").Hidden().Envar(defaults.ConfigEnvar).StringVar(&ccf.ConfigString)
	app.Flag("auth-server",
		fmt.Sprintf("Attempts to connect to specific auth/proxy address(es) instead of local auth [%v]", defaults.AuthConnectAddr().Addr)).
		Envar(authAddrEnvVar).
		StringsVar(&ccf.AuthServerAddr)
	app.Flag("identity",
		"Path to an identity file. Must be provided to make remote connections to auth. An identity file can be exported with 'tctl auth sign'").
		Short('i').
		Envar(identityFileEnvVar).
		StringVar(&ccf.IdentityFilePath)
	app.Flag("insecure", "When specifying a proxy address in --auth-server, do not verify its TLS certificate. Danger: any data you send can be intercepted or modified by an attacker.").
		BoolVar(&ccf.Insecure)

	// "version" command is always available:
	ver := app.Command("version", "Print the version of your tctl binary.")
	app.HelpFlag.Short('h')

	// parse CLI commands+flags:
	utils.UpdateAppUsageTemplate(app, args)
	selectedCmd, err := app.Parse(args)
	if err != nil {
		app.Usage(args)
		return trace.Wrap(err)
	}

	// Identity files do not currently contain a proxy address. When loading an
	// Identity file, an auth server address must be passed on the command line
	// as well.
	if ccf.IdentityFilePath != "" && len(ccf.AuthServerAddr) == 0 {
		return trace.BadParameter("tctl --identity also requires --auth-server")
	}

	// "version" command?
	if selectedCmd == ver.FullCommand() {
		modules.GetModules().PrintVersion()
		return nil
	}

	cfg.TeleportHome = os.Getenv(types.HomeEnvVar)
	if cfg.TeleportHome != "" {
		cfg.TeleportHome = filepath.Clean(cfg.TeleportHome)
	}

	cfg.Debug = ccf.Debug

	// configure all commands with Teleport configuration (they share 'cfg')
	clientConfig, err := ApplyConfig(&ccf, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx := context.Background()

	resolver, err := reversetunnelclient.CachingResolver(
		ctx,
		reversetunnelclient.WebClientResolver(&webclient.Config{
			Context:   ctx,
			ProxyAddr: clientConfig.AuthServers[0].String(),
			Insecure:  clientConfig.Insecure,
			Timeout:   clientConfig.DialTimeout,
		}),
		nil /* clock */)
	if err != nil {
		return trace.Wrap(err)
	}

	dialer, err := reversetunnelclient.NewTunnelAuthDialer(reversetunnelclient.TunnelAuthDialerConfig{
		Resolver:              resolver,
		ClientConfig:          clientConfig.SSH,
		Log:                   clientConfig.Log,
		InsecureSkipTLSVerify: clientConfig.Insecure,
		GetClusterCAs:         apiclient.ClusterCAsFromCertPool(clientConfig.TLS.RootCAs),
	})
	if err != nil {
		return trace.Wrap(err)
	}

	clientConfig.ProxyDialer = dialer

	client, err := authclient.Connect(ctx, clientConfig)
	if err != nil {
		if utils.IsUntrustedCertErr(err) {
			err = trace.WrapWithMessage(err, utils.SelfSignedCertsMsg)
		}
		fmt.Fprintf(os.Stderr,
			"ERROR: Cannot connect to the auth server. Is the auth server running on %q?\n",
			cfg.AuthServerAddresses()[0].Addr)
		return trace.NewAggregate(&common.ExitCodeError{Code: 1}, err)
	}

	// Get the proxy address and set the MFA prompt constructor.
	resp, err := client.Ping(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	proxyAddr := resp.ProxyPublicAddr
	client.SetMFAPromptConstructor(func(opts ...mfa.PromptOpt) mfa.Prompt {
		promptCfg := libmfa.NewPromptConfig(proxyAddr, opts...)
		return libmfa.NewCLIPrompt(promptCfg, os.Stderr)
	})

	// execute whatever is selected:
	var match bool
	for _, c := range commands {
		match, err = c.TryRun(ctx, selectedCmd, client)
		if err != nil {
			return trace.Wrap(err)
		}
		if match {
			break
		}
	}

	ctx, cancel := context.WithTimeout(ctx, constants.TimeoutGetClusterAlerts)
	defer cancel()
	if err := common.ShowClusterAlerts(ctx, client, os.Stderr, nil,
		types.AlertSeverity_HIGH); err != nil {
		log.WithError(err).Warn("Failed to display cluster alerts.")
	}

	return nil
}

// ApplyConfig takes configuration values from the config file and applies them
// to 'servicecfg.Config' object.
//
// The returned authclient.Config has the credentials needed to dial the auth
// server.
func ApplyConfig(ccf *GlobalCLIFlags, cfg *servicecfg.Config) (*authclient.Config, error) {
	// --debug flag
	if ccf.Debug {
		cfg.Debug = ccf.Debug
		utils.InitLogger(utils.LoggingForCLI, slog.LevelDebug)
		log.Debugf("Debug logging has been enabled.")
	}
	cfg.Log = log.StandardLogger()

	if cfg.Version == "" {
		cfg.Version = defaults.TeleportConfigVersionV1
	}

	// If the config file path provided is not a blank string, load the file and apply its values
	var fileConf *config.FileConfig
	var err error
	if ccf.ConfigFile != "" {
		fileConf, err = config.ReadConfigFile(ccf.ConfigFile)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// if configuration is passed as an environment variable,
	// try to decode it and override the config file
	if ccf.ConfigString != "" {
		fileConf, err = config.ReadFromString(ccf.ConfigString)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// It only makes sense to use file config when tctl is run on the same
	// host as the auth server.
	// If this is any other host, then it's remote tctl usage.
	// Remote tctl usage will require ~/.tsh or an identity file.
	// ~/.tsh which will provide credentials AND config to reach auth server.
	// Identity file requires --auth-server flag.
	localAuthSvcConf := fileConf != nil && fileConf.Auth.Enabled()
	if localAuthSvcConf {
		if err = config.ApplyFileConfig(fileConf, cfg); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// --auth-server flag(-s)
	if len(ccf.AuthServerAddr) != 0 {
		authServers, err := utils.ParseAddrs(ccf.AuthServerAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		// Overwrite any existing configuration with flag values.
		if err := cfg.SetAuthServerAddresses(authServers); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Config file (for an auth_service) should take precedence.
	if !localAuthSvcConf {
		// Try profile or identity file.
		if fileConf == nil {
			log.Debug("no config file, loading auth config via extension")
		} else {
			log.Debug("auth_service disabled in config file, loading auth config via extension")
		}
		authConfig, err := LoadConfigFromProfile(ccf, cfg)
		if err == nil {
			return authConfig, nil
		}
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		} else if runtime.GOOS == constants.WindowsOS {
			// On macOS/Linux, a not found error here is okay, as we can attempt
			// to use the local auth identity. The auth server itself doesn't run
			// on Windows though, so exit early with a clear error.
			return nil, trace.BadParameter("tctl requires a tsh profile on Windows. " +
				"Try logging in with tsh first.")
		}
	}

	// If auth server is not provided on the command line or in file
	// configuration, use the default.
	if len(cfg.AuthServerAddresses()) == 0 {
		authServers, err := utils.ParseAddrs([]string{defaults.AuthConnectAddr().Addr})
		if err != nil {
			return nil, trace.Wrap(err)
		}

		if err := cfg.SetAuthServerAddresses(authServers); err != nil {
			return nil, trace.Wrap(err)
		}
	}

	authConfig := new(authclient.Config)
	// read the host UUID only in case the identity was not provided,
	// because it will be used for reading local auth server identity
	cfg.HostUUID, err = utils.ReadHostUUID(cfg.DataDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, trace.Wrap(err, "Could not load Teleport host UUID file at %s. "+
				"Please make sure that a Teleport Auth Service instance is running on this host prior to using tctl or provide credentials by logging in with tsh first.",
				filepath.Join(cfg.DataDir, utils.HostUUIDFile))
		} else if errors.Is(err, fs.ErrPermission) {
			return nil, trace.Wrap(err, "Teleport does not have permission to read Teleport host UUID file at %s. "+
				"Ensure that you are running as a user with appropriate permissions or provide credentials by logging in with tsh first.",
				filepath.Join(cfg.DataDir, utils.HostUUIDFile))
		}
		return nil, trace.Wrap(err)
	}
	identity, err := storage.ReadLocalIdentity(filepath.Join(cfg.DataDir, teleport.ComponentProcess), state.IdentityID{Role: types.RoleAdmin, HostUUID: cfg.HostUUID})
	if err != nil {
		// The "admin" identity is not present? This means the tctl is running
		// NOT on the auth server
		if trace.IsNotFound(err) {
			return nil, trace.AccessDenied("tctl must be used on an Auth Service host or provided with credentials by logging in with tsh first.")
		}
		return nil, trace.Wrap(err)
	}
	authConfig.TLS, err = identity.TLSConfig(cfg.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authConfig.TLS.InsecureSkipVerify = ccf.Insecure
	authConfig.Insecure = ccf.Insecure
	authConfig.AuthServers = cfg.AuthServerAddresses()
	authConfig.Log = cfg.Log
	authConfig.DialOpts = append(authConfig.DialOpts, metadata.WithUserAgentFromTeleportComponent(teleport.ComponentTCTL))

	return authConfig, nil
}

// LoadConfigFromProfile applies config from ~/.tsh/ profile if it's present
func LoadConfigFromProfile(ccf *GlobalCLIFlags, cfg *servicecfg.Config) (*authclient.Config, error) {
	proxyAddr := ""
	if len(ccf.AuthServerAddr) != 0 {
		proxyAddr = ccf.AuthServerAddr[0]
	}

	clientStore := client.NewFSClientStore(cfg.TeleportHome)
	if ccf.IdentityFilePath != "" {
		var err error
		clientStore, err = identityfile.NewClientStoreFromIdentityFile(ccf.IdentityFilePath, proxyAddr, "")
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	profile, err := clientStore.ReadProfileStatus(proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if profile.IsExpired(time.Now()) {
		return nil, trace.BadParameter("your credentials have expired, please login using `tsh login`")
	}

	c := client.MakeDefaultConfig()
	log.WithFields(log.Fields{"proxy": profile.ProxyURL.String(), "user": profile.Username}).Debugf("Found profile.")
	if err := c.LoadProfile(clientStore, proxyAddr); err != nil {
		return nil, trace.Wrap(err)
	}

	webProxyHost, _ := c.WebProxyHostPort()
	idx := client.KeyRingIndex{ProxyHost: webProxyHost, Username: c.Username, ClusterName: profile.Cluster}
	keyRing, err := clientStore.GetKeyRing(idx, client.WithSSHCerts{})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Auth config can be created only using a key associated with the root cluster.
	rootCluster, err := keyRing.RootClusterName()
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if profile.Cluster != rootCluster {
		return nil, trace.BadParameter("your credentials are for cluster %q, please run `tsh login %q` to log in to the root cluster", profile.Cluster, rootCluster)
	}

	authConfig := &authclient.Config{}
	authConfig.TLS, err = keyRing.TeleportClientTLSConfig(cfg.CipherSuites, []string{rootCluster})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authConfig.TLS.InsecureSkipVerify = ccf.Insecure
	authConfig.Insecure = ccf.Insecure
	authConfig.SSH, err = keyRing.ProxyClientSSHConfig(rootCluster)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// Do not override auth servers from command line
	if len(ccf.AuthServerAddr) == 0 {
		webProxyAddr, err := utils.ParseAddr(c.WebProxyAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		log.Debugf("Setting auth server to web proxy %v.", webProxyAddr)
		cfg.SetAuthServerAddress(*webProxyAddr)
	}
	authConfig.AuthServers = cfg.AuthServerAddresses()
	authConfig.Log = cfg.Log
	authConfig.DialOpts = append(authConfig.DialOpts, metadata.WithUserAgentFromTeleportComponent(teleport.ComponentTCTL))

	if c.TLSRoutingEnabled {
		cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
	}

	return authConfig, nil
}
