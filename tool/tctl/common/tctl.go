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
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gravitational/teleport"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/reversetunnel"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/tool/tsh/common"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/kingpin"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
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
//
type CLICommand interface {
	// Initialize allows a caller-defined command to plug itself into CLI
	// argument parsing
	Initialize(*kingpin.Application, *service.Config)

	// TryRun is executed after the CLI parsing is done. The command must
	// determine if selectedCommand belongs to it and return match=true
	TryRun(selectedCommand string, c auth.ClientI) (match bool, err error)
}

// Run is the same as 'make'. It helps to share the code between different
// "distributions" like OSS or Enterprise
//
// distribution: name of the Teleport distribution
func Run(commands []CLICommand, loadConfigExt LoadConfigFn) {
	utils.InitLogger(utils.LoggingForCLI, log.WarnLevel)

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
		fmt.Sprintf("Address of the auth server or the proxy [%v]. Can be supplied multiple times", defaults.AuthConnectAddr().Addr)).
		StringsVar(&ccf.AuthServerAddr)
	app.Flag("identity", "Path to the identity file exported with 'tctl auth sign'").
		Short('i').
		StringVar(&ccf.IdentityFilePath)
	app.Flag("insecure", "When specifying a proxy address in --auth-server, do not verify its TLS certificate. Danger: any data you send can be intercepted or modified by an attacker.").
		BoolVar(&ccf.Insecure)

	// "version" command is always available:
	ver := app.Command("version", "Print cluster version")
	app.HelpFlag.Short('h')

	// parse CLI commands+flags:
	selectedCmd, err := app.Parse(os.Args[1:])
	if err != nil {
		utils.FatalError(err)
	}

	// "version" command?
	if selectedCmd == ver.FullCommand() {
		utils.PrintVersion()
		return
	}

	// configure all commands with Teleport configuration (they share 'cfg')
	clientConfig, err := applyConfig(&ccf, cfg, loadConfigExt)
	if err != nil {
		utils.FatalError(err)
	}

	ctx := context.Background()

	client, err := connectToAuthService(ctx, cfg, clientConfig)
	if err != nil {
		utils.Consolef(os.Stderr, log.WithField(trace.Component, teleport.ComponentClient), teleport.ComponentClient,
			"Cannot connect to the auth server: %v.\nIs the auth server running on %q?",
			err, cfg.AuthServers[0].Addr)
		os.Exit(1)
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

// LoadConfigFn is optional config loading function
type LoadConfigFn func(ccf *GlobalCLIFlags, cfg *service.Config) (*AuthServiceClientConfig, error)

// AuthServiceClientConfig is a client config for auth service
type AuthServiceClientConfig struct {
	// TLS holds credentials for mTLS
	TLS *tls.Config
	// SSH is client SSH config
	SSH *ssh.ClientConfig
}

// connectToAuthService creates a valid client connection to the auth service
func connectToAuthService(ctx context.Context, cfg *service.Config, clientConfig *AuthServiceClientConfig) (auth.ClientI, error) {
	// connect to the local auth server by default:
	cfg.Auth.Enabled = true
	if len(cfg.AuthServers) == 0 {
		cfg.AuthServers = []utils.NetAddr{
			*defaults.AuthConnectAddr(),
		}
	}

	log.Debugf("Connecting to auth servers: %v.", cfg.AuthServers)

	// Try connecting to the auth server directly over TLS.
	client, err := auth.NewClient(apiclient.Config{Addrs: utils.NetAddrsToStrings(cfg.AuthServers), Credentials: apiclient.TLSCreds(clientConfig.TLS)})
	if err != nil {
		return nil, trace.Wrap(err, "failed direct dial to auth server: %v", err)
	}

	// Check connectivity by calling something on the client.
	_, err = client.GetClusterName()
	if err != nil {
		err = trace.Wrap(err, "failed direct dial to auth server: %v", err)
		if clientConfig.SSH == nil {
			// No identity file was provided, don't try dialing via a reverse
			// tunnel on the proxy.
			return nil, trace.Wrap(err)
		}

		// If direct dial failed, we may have a proxy address in
		// cfg.AuthServers. Try connecting to the reverse tunnel endpoint and
		// make a client over that.
		//
		// TODO(awly): this logic should be implemented once, in the auth
		// package, and reused in IoT nodes.

		errs := []error{err}

		// Figure out the reverse tunnel address on the proxy first.
		tunAddr, err := findReverseTunnel(ctx, cfg.AuthServers, clientConfig.TLS.InsecureSkipVerify)
		if err != nil {
			errs = append(errs, trace.Wrap(err, "failed lookup of proxy reverse tunnel address: %v", err))
			return nil, trace.NewAggregate(errs...)
		}
		log.Debugf("Attempting to connect using reverse tunnel address %v.", tunAddr)
		// reversetunnel.TunnelAuthDialer will take care of creating a net.Conn
		// within an SSH tunnel.
		client, err = auth.NewClient(apiclient.Config{
			Dialer: &reversetunnel.TunnelAuthDialer{
				ProxyAddr:    tunAddr,
				ClientConfig: clientConfig.SSH,
			},
			Credentials: apiclient.TLSCreds(clientConfig.TLS),
		})
		if err != nil {
			errs = append(errs, trace.Wrap(err, "failed dial to auth server through reverse tunnel: %v", err))
			return nil, trace.NewAggregate(errs...)
		}
		// Check connectivity by calling something on the client.
		if _, err := client.GetClusterName(); err != nil {
			errs = append(errs, trace.Wrap(err, "failed dial to auth server through reverse tunnel: %v", err))
			return nil, trace.NewAggregate(errs...)
		}
	}
	return client, nil
}

// findReverseTunnel uses the web proxy to discover where the SSH reverse tunnel
// server is running.
func findReverseTunnel(ctx context.Context, addrs []utils.NetAddr, insecureTLS bool) (string, error) {
	var errs []error
	for _, addr := range addrs {
		// In insecure mode, any certificate is accepted. In secure mode the hosts
		// CAs are used to validate the certificate on the proxy.
		resp, err := client.Find(ctx, addr.String(), insecureTLS, nil)
		if err == nil {
			return tunnelAddr(addr, resp.Proxy)
		}
		errs = append(errs, err)
	}
	return "", trace.NewAggregate(errs...)
}

// tunnelAddr returns the tunnel address in the following preference order:
//  1. Reverse Tunnel Public Address.
//  2. SSH Proxy Public Address.
//  3. HTTP Proxy Public Address.
//  4. Tunnel Listen Address.
func tunnelAddr(webAddr utils.NetAddr, settings client.ProxySettings) (string, error) {
	// Extract the port the tunnel server is listening on.
	netAddr, err := utils.ParseHostPortAddr(settings.SSH.TunnelListenAddr, defaults.SSHProxyTunnelListenPort)
	if err != nil {
		return "", trace.Wrap(err)
	}
	tunnelPort := netAddr.Port(defaults.SSHProxyTunnelListenPort)

	// If a tunnel public address is set, nothing else has to be done, return it.
	if settings.SSH.TunnelPublicAddr != "" {
		return settings.SSH.TunnelPublicAddr, nil
	}

	// If a tunnel public address has not been set, but a related HTTP or SSH
	// public address has been set, extract the hostname but use the port from
	// the tunnel listen address.
	if settings.SSH.SSHPublicAddr != "" {
		addr, err := utils.ParseHostPortAddr(settings.SSH.SSHPublicAddr, tunnelPort)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return net.JoinHostPort(addr.Host(), strconv.Itoa(tunnelPort)), nil
	}
	if settings.SSH.PublicAddr != "" {
		addr, err := utils.ParseHostPortAddr(settings.SSH.PublicAddr, tunnelPort)
		if err != nil {
			return "", trace.Wrap(err)
		}
		return net.JoinHostPort(addr.Host(), strconv.Itoa(tunnelPort)), nil
	}

	// If nothing is set, fallback to the address we dialed.
	return net.JoinHostPort(webAddr.Host(), strconv.Itoa(tunnelPort)), nil
}

// applyConfig takes configuration values from the config file and applies
// them to 'service.Config' object.
//
// The returned authServiceClientConfig has the credentials needed to dial the
// auth server.
func applyConfig(ccf *GlobalCLIFlags, cfg *service.Config, loadConfigExt LoadConfigFn) (*AuthServiceClientConfig, error) {
	// --debug flag
	if ccf.Debug {
		cfg.Debug = ccf.Debug
		utils.InitLogger(utils.LoggingForCLI, log.DebugLevel)
		log.Debugf("Debug logging has been enabled.")
	}

	// load /etc/teleport.yaml and apply its values:
	fileConf, err := config.ReadConfigFile(ccf.ConfigFile)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	// if configuration is passed as an environment variable,
	// try to decode it and override the config file
	if ccf.ConfigString != "" {
		fileConf, err = config.ReadFromString(ccf.ConfigString)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Config file should take precedence, if available.
	if fileConf == nil && ccf.IdentityFilePath == "" && loadConfigExt != nil {
		// No config file or identity file.
		// Try the extension loader.
		log.Debug("No config file or identity file, loading auth config via extension.")
		authConfig, err := loadConfigExt(ccf, cfg)
		if err == nil {
			return authConfig, nil
		}
		if !trace.IsNotFound(err) {
			return nil, trace.Wrap(err)
		}
	}
	if err = config.ApplyFileConfig(fileConf, cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	// --auth-server flag(-s)
	if len(ccf.AuthServerAddr) != 0 {
		cfg.AuthServers, err = utils.ParseAddrs(ccf.AuthServerAddr)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	// If auth server is not provided on the command line or in file
	// configuration, use the default.
	if len(cfg.AuthServers) == 0 {
		cfg.AuthServers, err = utils.ParseAddrs([]string{defaults.AuthConnectAddr().Addr})
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	authConfig := new(AuthServiceClientConfig)
	// --identity flag
	if ccf.IdentityFilePath != "" {
		key, err := common.LoadIdentity(ccf.IdentityFilePath)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		authConfig.TLS, err = key.TeleportClientTLSConfig(cfg.CipherSuites)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		authConfig.SSH, err = key.ClientSSHConfig()
		if err != nil {
			return nil, trace.Wrap(err)
		}
	} else {
		// read the host UUID only in case the identity was not provided,
		// because it will be used for reading local auth server identity
		cfg.HostUUID, err = utils.ReadHostUUID(cfg.DataDir)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		identity, err := auth.ReadLocalIdentity(filepath.Join(cfg.DataDir, teleport.ComponentProcess), auth.IdentityID{Role: teleport.RoleAdmin, HostUUID: cfg.HostUUID})
		if err != nil {
			// The "admin" identity is not present? This means the tctl is running
			// NOT on the auth server
			if trace.IsNotFound(err) {
				return nil, trace.AccessDenied("tctl must be either used on the auth server or provided with the identity file via --identity flag")
			}
			return nil, trace.Wrap(err)
		}
		authConfig.TLS, err = identity.TLSConfig(cfg.CipherSuites)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}
	authConfig.TLS.InsecureSkipVerify = ccf.Insecure

	return authConfig, nil
}
