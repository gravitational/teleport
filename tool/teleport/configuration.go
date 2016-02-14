/*
Copyright 2015-16 Gravitational, Inc.

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
	"io/ioutil"
	"net"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"

	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

// CLIConfig represents command line flags+args
type CLIConfig struct {
	// --name flag
	NodeName string
	// --auth-server flag
	AuthServerAddr string
	// --token flag
	AuthToken string
	// --listen-ip flag
	ListenIP net.IP
	// --config flag
	ConfigFile string
	// --roles flag
	Roles string
	// -d flag
	Debug bool
}

// configure merges command line arguments with what's in a configuration file
// with CLI commands taking precedence
func configure(cliConf *CLIConfig) (cfg *service.Config, err error) {
	// create the default configuration:
	cfg, err = service.MakeDefaultConfig()
	if err != nil {
		return cfg, trace.Wrap(err)
	}

	// use a config file?
	if cliConf.ConfigFile != "" || fileExists(defaults.ConfigFilePath) {
		configPath := defaults.ConfigFilePath
		if cliConf.ConfigFile != "" {
			configPath = cliConf.ConfigFile
		}
		// parse the config file. these values will override defaults:
		utils.Consolef(os.Stdout, "Using config file: %s", configPath)
	} else {
		log.Info("not using a config file")
	}

	// apply --debug flag:
	if cliConf.Debug {
		cfg.Console = ioutil.Discard
		utils.InitLoggerDebug()
	}

	// apply --roles flag:
	if cliConf.Roles != "" {
		if err := validateRoles(cliConf.Roles); err != nil {
			return cfg, trace.Wrap(err)
		}
		if strings.Index(cliConf.Roles, defaults.RoleNode) == -1 {
			cfg.SSH.Enabled = false
		}
		if strings.Index(cliConf.Roles, defaults.RoleAuthService) == -1 {
			cfg.Auth.Enabled = false
		}
		if strings.Index(cliConf.Roles, defaults.RoleProxy) == -1 {
			cfg.Proxy.Enabled = false
			cfg.ReverseTunnel.Enabled = false
		}
	}

	// apply --auth-server flag:
	if cliConf.AuthServerAddr != "" {
		if cliConf.NodeName == "" {
			return cfg, trace.Errorf("Need --name flag")
		}
		if cliConf.AuthToken == "" {
			return cfg, trace.Errorf("Need --token flag")
		}
		if cfg.Auth.Enabled {
			log.Warnf("not starting the local auth service. --auth-server flag tells to connect to another auth server")
			cfg.Auth.Enabled = false
		}
		addr, err := utils.ParseHostPortAddr(cliConf.AuthServerAddr, int(defaults.AuthListenPort))
		if err != nil {
			return cfg, trace.Wrap(err)
		}
		log.Infof("Using auth server: %v", addr.FullAddress())
		cfg.AuthServers = []utils.NetAddr{*addr}
	}

	// apply --token flag:
	if cliConf.NodeName != "" {
		if cliConf.NodeName == "" {
			return cfg, trace.Errorf("Need --name flag")
		}
		cfg.Hostname = cliConf.NodeName
	}

	// apply --token flag:
	if cliConf.AuthToken != "" {
		log.Infof("Using auth token: %s", cliConf.AuthToken)
		cfg.SSH.Token = cliConf.AuthToken
		cfg.Proxy.Token = cliConf.AuthToken
	}

	// apply --listen-ip flag:
	if cliConf.ListenIP != nil {
		applyListenIP(cliConf.ListenIP, cfg)
	}

	log.Info(cfg.DebugDumpToYAML())

	return cfg, nil
}

// applyListenIP replaces all 'listen addr' settings for all services with
// a given IP
func applyListenIP(ip net.IP, cfg *service.Config) {
	listeningAddresses := []*utils.NetAddr{
		&cfg.Auth.SSHAddr,
		&cfg.Auth.SSHAddr,
		&cfg.Proxy.SSHAddr,
		&cfg.Proxy.WebAddr,
		&cfg.SSH.Addr,
		&cfg.Proxy.ReverseTunnelListenAddr,
	}
	for _, addr := range listeningAddresses {
		replaceHost(addr, ip.String())
	}
}

// replaceHost takes utils.NetAddr and replaces the hostname in it, preserving
// the original port
func replaceHost(addr *utils.NetAddr, newHost string) {
	_, port, err := net.SplitHostPort(addr.Addr)
	if err != nil {
		log.Errorf("failed parsing address: '%v'", addr.Addr)
	}
	addr.Addr = net.JoinHostPort(newHost, port)
}

func fileExists(fp string) bool {
	_, err := os.Stat(fp)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}

// validateRoles makes sure that value upassed to --roles flag is valid
func validateRoles(roles string) error {
	for _, role := range strings.Split(roles, ",") {
		switch role {
		case defaults.RoleAuthService,
			defaults.RoleNode,
			defaults.RoleProxy:
			break
		default:
			return trace.Errorf("unknown role: '%s'", role)
		}
	}
	return nil
}
