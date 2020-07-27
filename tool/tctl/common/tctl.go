/*
Copyright 2015-2019 Gravitational, Inc.

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
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tsh/common"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	"github.com/sirupsen/logrus"
)

// GlobalCLIFlags keeps the CLI flags that apply to all tctl commands
type GlobalCLIFlags struct {
	// Debug enables verbose logging mode to the console
	Debug bool
	// ConfigFile is the path to the Teleport configuration file
	ConfigFile string
	// ConfigString is the base64-encoded string with Teleport configuration
	ConfigString string
	// AuthServerAddr lists addresses of auth servers to connect to
	AuthServerAddr []string
	// IdentityFilePath is the path to the identity file
	IdentityFilePath string
}

// CLICommand interface must be implemented by every CLI command
//
// This allows OSS and Enterprise Teleport editions to plug their own
// implementations of different CLI commands into the common execution
// framework
//
type CLICommand interface {
	// Initialize allows a caller-defined command to plug itself into CLI
	// argument parsing
	Initialize(*kingpin.Application, *service.Config)

	// TryRun is executed after the CLI parsing is done. The command must
	// determine if selectedCommand belongs to it and return match=true
	TryRun(selectedCommand string, c auth.ClientI) (match bool, err error)
}

// Run() is the same as 'make'. It helps to share the code between different
// "distributions" like OSS or Enterprise
//
// distribution: name of the Teleport distribution
func Run(commands []CLICommand) {
	utils.InitLogger(utils.LoggingForCLI, logrus.WarnLevel)

	// app is the command line parser
	app := utils.InitCLIParser("tctl", GlobalHelpString)

	// cfg (teleport auth server configuration) is going to be shared by all
	// commands
	cfg := service.MakeDefaultConfig()

	// each command will add itself to the CLI parser:
	for i := range commands {
		commands[i].Initialize(app, cfg)
	}

	// these global flags apply to all commands
	var ccf GlobalCLIFlags
	app.Flag("debug", "Enable verbose logging to stderr").
		Short('d').
		BoolVar(&ccf.Debug)
	app.Flag("config", fmt.Sprintf("Path to a configuration file [%v]", defaults.ConfigFilePath)).
		Short('c').
		ExistingFileVar(&ccf.ConfigFile)
	app.Flag("config-string",
		"Base64 encoded configuration string").Hidden().Envar(defaults.ConfigEnvar).StringVar(&ccf.ConfigString)
	app.Flag("auth-server",
		fmt.Sprintf("Address of the auth server [%v]. Can be supplied multiple times", defaults.AuthConnectAddr().Addr)).
		StringsVar(&ccf.AuthServerAddr)
	app.Flag("identity", "Path to the identity file exported with 'tctl auth sign'").
		Short('i').
		StringVar(&ccf.IdentityFilePath)

	// "version" command is always available:
	ver := app.Command("version", "Print cluster version")
	app.HelpFlag.Short('h')

	// parse CLI commands+flags:
	selectedCmd, err := app.Parse(os.Args[1:])
	if err != nil {
		if strings.Contains(err.Error(), "unknown long flag") {
			err = trace.Wrap(err, "make sure role used in flag has a corresponding Teleport role in RBAC, an example RBAC role can be seen here: %q", "https://gravitational.com/teleport/docs/enterprise/ssh_rbac/#roles")
		}
		utils.FatalError(err)
	}

	// "version" command?
	if selectedCmd == ver.FullCommand() {
		utils.PrintVersion()
		return
	}

	// configure all commands with Teleport configuration (they share 'cfg')
	if err := applyConfig(&ccf, cfg); err != nil {
		utils.FatalError(err)
	}

	// connect to the auth sever:
	client, err := connectToAuthService(cfg)
	if err != nil {
		utils.FatalError(err)
	}

	// execute whatever is selected:
	var match bool
	for _, c := range commands {
		match, err = c.TryRun(selectedCmd, client)
		if err != nil {
			utils.FatalError(err)
		}
		if match {
			break
		}
	}
}

// connectToAuthService creates a valid client connection to the auth service
func connectToAuthService(cfg *service.Config) (client auth.ClientI, err error) {
	// connect to the local auth server by default:
	cfg.Auth.Enabled = true
	if len(cfg.AuthServers) == 0 {
		cfg.AuthServers = []utils.NetAddr{
			*defaults.AuthConnectAddr(),
		}
	}

	identity, err := getIdentity(cfg)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	tlsConfig, err := identity.TLSConfig(cfg.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	logrus.Debugf("Connecting to auth servers: %v.", cfg.AuthServers)

	client, err = auth.NewTLSClient(auth.ClientConfig{Addrs: cfg.AuthServers, TLS: tlsConfig})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Check connectivity by calling something on the client.
	_, err = client.GetClusterName()
	if err != nil {
		utils.Consolef(os.Stderr, teleport.ComponentClient,
			"Cannot connect to the auth server: %v.\nIs the auth server running on %v?",
			err, cfg.AuthServers[0].Addr)
		os.Exit(1)
	}
	return client, nil
}

// getIdentity returns auth.Identity to use when connecting to auth server
func getIdentity(cfg *service.Config) (*auth.Identity, error) {
	// If identity was provided in the configuration, use it
	if len(cfg.Identities) != 0 {
		return cfg.Identities[0], nil
	}
	// Otherwise, assume we're running on the auth node and read the host-local
	// identity from Teleport data directory
	identity, err := auth.ReadLocalIdentity(filepath.Join(cfg.DataDir, teleport.ComponentProcess), auth.IdentityID{Role: teleport.RoleAdmin, HostUUID: cfg.HostUUID})
	if err != nil {
		// The "admin" identity is not present? This means the tctl is running
		// NOT on the auth server
		if trace.IsNotFound(err) {
			return nil, trace.AccessDenied("tctl must be either used on the auth " +
				"server or provided with the identity file via --identity flag")
		}
		return nil, trace.Wrap(err)
	}
	return identity, nil
}

// applyConfig takes configuration values from the config file and applies
// them to 'service.Config' object
func applyConfig(ccf *GlobalCLIFlags, cfg *service.Config) error {
	// load /etc/teleport.yaml and apply it's values:
	fileConf, err := config.ReadConfigFile(ccf.ConfigFile)
	if err != nil {
		return trace.Wrap(err)
	}
	// if configuration is passed as an environment variable,
	// try to decode it and override the config file
	if ccf.ConfigString != "" {
		fileConf, err = config.ReadFromString(ccf.ConfigString)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	if err = config.ApplyFileConfig(fileConf, cfg); err != nil {
		return trace.Wrap(err)
	}
	// --debug flag
	if ccf.Debug {
		cfg.Debug = ccf.Debug
		utils.InitLogger(utils.LoggingForCLI, logrus.DebugLevel)
		logrus.Debugf("DEBUG logging enabled")
	}
	// --auth-server flag(-s)
	if len(ccf.AuthServerAddr) != 0 {
		cfg.AuthServers, err = utils.ParseAddrs(ccf.AuthServerAddr)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	// If auth server is not provided on the command line or in file
	// configuration, use the default.
	if len(cfg.AuthServers) == 0 {
		cfg.AuthServers, err = utils.ParseAddrs([]string{defaults.AuthConnectAddr().Addr})
		if err != nil {
			return trace.Wrap(err)
		}
	}
	// --identity flag
	if ccf.IdentityFilePath != "" {
		key, _, err := common.LoadIdentity(ccf.IdentityFilePath)
		if err != nil {
			return trace.Wrap(err)
		}
		identity, err := auth.ReadTLSIdentityFromKeyPair(key.Priv, key.TLSCert, key.TLSCAs())
		if err != nil {
			return trace.Wrap(err)
		}
		cfg.Identities = append(cfg.Identities, identity)
	} else {
		// read the host UUID only in case the identity was not provided,
		// because it will be used for reading local auth server identity
		cfg.HostUUID, err = utils.ReadHostUUID(cfg.DataDir)
		if err != nil {
			return trace.Wrap(err)
		}
	}
	return nil
}
