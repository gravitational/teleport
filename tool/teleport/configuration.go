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
	"net"
	"os"
	"path/filepath"
)

// CLIConfig represents command line flags+args
type CLIConfig struct {
	ProxyAddr   string
	ListenIP    net.IP
	AdvertiseIP net.IP
	ConfigFile  string
	NoSSH       bool
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
		log.Infof("Using config file: %s", configPath)
		if err := service.ParseYAMLFile(configPath, &cfg); err != nil {
			return cfg, err
		}
	} else {
		log.Infof("Not using a config file")
	}

	// apply --nossh flag:
	if ccf.NoSSH {
		cfg.SSH.Enabled = false
		log.Infof("SSH server is disabled via command line flag")
	}
	// apply --listen-ip flag:
	if ccf.ListenIP != nil {
		log.Infof("applying listen-ip flag: '%v'", ccf.ListenIP)
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
	cfg.Proxy.ReverseTunnelListenAddr = *defaults.ReverseTunnellAddr()
	defaults.ConfigureLimiter(&cfg.Proxy.Limiter)

	// defaults for the SSH service:
	cfg.SSH.Enabled = true
	cfg.SSH.Addr = *defaults.SSHServerListenAddr()
	defaults.ConfigureLimiter(&cfg.SSH.Limiter)

	// global defaults
	cfg.Hostname = hostname
	cfg.DataDir = defaults.DataDir
	cfg.AuthServers = []utils.NetAddr{cfg.Auth.SSHAddr}
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
