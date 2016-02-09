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
	"fmt"
	log "github.com/Sirupsen/logrus"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"
)

// CLIConfig represents command line flags+args
type CLIConfig struct {
	AuthServerAddr string
	AuthToken      string
	ListenIP       net.IP
	ConfigFile     string
	Roles          string
	Debug          bool
}

// confnigure merges command line arguments with what's in a configuration file
// with CLI commands taking precedence
func configure(ccf *CLIConfig) (cfg service.Config, err error) {
	if err = applyDefaults(&cfg); err != nil {
		return cfg, trace.Wrap(err)
	}

	// use a config file?
	if ccf.ConfigFile != "" || fileExists(defaults.ConfigFilePath) {
		configPath := defaults.ConfigFilePath
		if ccf.ConfigFile != "" {
			configPath = ccf.ConfigFile
		}
		// parse the config file. these values will override defaults:
		utils.ConsoleMessage(os.Stdout, "Using config file: %s", configPath)

		// TODO: replace with simplified config file
		log.Warning("Need to implement simplified config file")
		if err := service.ParseYAMLFile(configPath, &cfg); err != nil {
			return cfg, err
		}
	} else {
		utils.ConsoleMessage(os.Stdout, "Not using a config file")
	}

	// apply --debug flag:
	if ccf.Debug {
		cfg.Console = ioutil.Discard
		utils.InitLoggerDebug()
	}

	// apply --auth-server flag:
	if ccf.AuthServerAddr != "" {
		addr, err := utils.ParseHostPortAddr(ccf.AuthServerAddr, int(defaults.AuthListenPort))
		if err != nil {
			return cfg, err
		}
		log.Infof("Using auth server: %v", addr.FullAddress())
		cfg.AuthServers = append(cfg.AuthServers, *addr)
	}

	// apply --token flag:
	if ccf.AuthToken != "" {
		log.Infof("Using auth token: %s", ccf.AuthToken)
		cfg.SSH.Token = ccf.AuthToken
		cfg.Proxy.Token = ccf.AuthToken
	}

	// apply --roles flag:
	if ccf.Roles != "" {
		if err := validateRoles(ccf.Roles); err != nil {
			log.Error(err.Error())
			return cfg, err
		}
		if strings.Index(ccf.Roles, defaults.RoleNode) == -1 {
			cfg.SSH.Enabled = false
		}
		if strings.Index(ccf.Roles, defaults.RoleAuthService) == -1 {
			cfg.Auth.Enabled = false
		}
		if strings.Index(ccf.Roles, defaults.RoleProxy) == -1 {
			cfg.Proxy.Enabled = false
			cfg.ReverseTunnel.Enabled = false
		}
	}

	// apply --listen-ip flag:
	if ccf.ListenIP != nil {
		if err = applyListenIP(ccf.ListenIP, &cfg); err != nil {
			return cfg, err
		}
	}
	return cfg, nil
}

// applyListenIP replaces all 'listen addr' settings for all services with
// a given IP
func applyListenIP(ip net.IP, cfg *service.Config) error {
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
	return nil
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

// applyDefaults initializes service configuration with default values
func applyDefaults(cfg *service.Config) error {
	hostname, err := os.Hostname()
	if err != nil {
		return trace.Wrap(err)
	}

	// defaults for the auth service:
	cfg.Auth.Enabled = true
	cfg.Auth.HostAuthorityDomain = hostname
	cfg.Auth.SSHAddr = *defaults.AuthListenAddr()
	cfg.Auth.EventsBackend.Type = defaults.BackendType
	cfg.Auth.EventsBackend.Params = boltParams(defaults.DataDir, "events.db")
	cfg.Auth.KeysBackend.Type = defaults.BackendType
	cfg.Auth.KeysBackend.Params = boltParams(defaults.DataDir, "keys.db")
	cfg.Auth.RecordsBackend.Type = defaults.BackendType
	cfg.Auth.RecordsBackend.Params = boltParams(defaults.DataDir, "records.db")
	defaults.ConfigureLimiter(&cfg.Auth.Limiter)

	// defaults for the SSH proxy service:
	cfg.Proxy.Enabled = true
	cfg.Proxy.AssetsDir = defaults.DataDir
	cfg.Proxy.SSHAddr = *defaults.ProxyListenAddr()
	cfg.Proxy.WebAddr = *defaults.ProxyWebListenAddr()
	cfg.ReverseTunnel.Enabled = true
	cfg.Proxy.ReverseTunnelListenAddr = *defaults.ReverseTunnellAddr()
	defaults.ConfigureLimiter(&cfg.Proxy.Limiter)
	defaults.ConfigureLimiter(&cfg.ReverseTunnel.Limiter)

	// defaults for the SSH service:
	cfg.SSH.Enabled = true
	cfg.SSH.Addr = *defaults.SSHServerListenAddr()
	defaults.ConfigureLimiter(&cfg.SSH.Limiter)

	// global defaults
	cfg.Hostname = hostname
	cfg.DataDir = defaults.DataDir
	if cfg.Auth.Enabled {
		cfg.AuthServers = []utils.NetAddr{cfg.Auth.SSHAddr}
	}
	cfg.Console = os.Stdout
	return nil
}

// Generates a string accepted by the BoltDB driver, like this:
// `{"path": "/var/lib/teleport/records.db"}`
func boltParams(storagePath, dbFile string) string {
	return fmt.Sprintf(`{"path": "%s"}`, filepath.Join(storagePath, dbFile))
}

func fileExists(fp string) bool {
	fi, err := os.Stat(fp)
	if err != nil || fi.IsDir() {
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
			return fmt.Errorf("unknown role: '%s'", role)
		}
	}
	return nil
}
