/*
Copyright 2022 Gravitational, Inc.

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

package sidecar

import (
	"path/filepath"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
)

const (
	DefaultLocalAddr  = "localhost:3025"
	DefaultConfigPath = "/etc/teleport/teleport.yaml"
	DefaultDataDir    = "/var/lib/teleport"
	DefaultUser       = "teleport-operator-sidecar"
	DefaultRole       = "teleport-operator-sidecar"
)

// Options configure the sidecar connection.
type Options struct {
	// ConfigPath is a path to the Teleport configuration file e.g. /etc/teleport/teleport.yaml.
	ConfigPath string

	// DataDir is a path to the Teleport data dir e.g. /var/lib/teleport.
	DataDir string

	// Addr is an endpoint of Teleport e.g. 127.0.0.1:3025.
	Addr string

	// Name is the bot name used to access Teleport Auth/Proxy/Tunnel server.
	Name string

	// Role is a role allowed to manage Teleport resources.
	Role string
}

func createAuthClientConfig(opts Options) (*authclient.Config, error) {
	cfg := service.MakeDefaultConfig()
	cfg.CircuitBreakerConfig = breaker.NoopBreakerConfig()
	cfg.Log = log.StandardLogger()

	// If the config file path provided is not a blank string, load the file and apply its values
	fileConf, err := config.ReadConfigFile(opts.ConfigPath)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err = config.ApplyFileConfig(fileConf, cfg); err != nil {
		return nil, trace.Wrap(err)
	}

	authServers, err := utils.ParseAddrs([]string{opts.Addr})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := cfg.SetAuthServerAddresses(authServers); err != nil {
		return nil, trace.Wrap(err)
	}

	// read the host UUID only in case the identity was not provided,
	// because it will be used for reading local auth server identity
	cfg.HostUUID, err = utils.ReadHostUUID(cfg.DataDir)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	identity, err := auth.ReadLocalIdentity(filepath.Join(cfg.DataDir, teleport.ComponentProcess), auth.IdentityID{Role: types.RoleAdmin, HostUUID: cfg.HostUUID})
	if err != nil {
		// The "admin" identity is not present? This means the tctl is running
		// NOT on the auth server
		if trace.IsNotFound(err) {
			return nil, trace.AccessDenied("tctl must be either used on the auth server or provided with the identity file via --identity flag")
		}
		return nil, trace.Wrap(err)
	}

	authConfig := new(authclient.Config)
	authConfig.TLS, err = identity.TLSConfig(cfg.CipherSuites)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	authConfig.AuthServers = cfg.AuthServerAddresses()
	authConfig.Log = cfg.Log

	return authConfig, nil
}

func sidecarRole(roleName string) (types.Role, error) {
	return types.NewRole(roleName, types.RoleSpecV6{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{"role", "user", "auth_connector", "login_rule"},
					Verbs:     []string{"*"},
				},
			},
		},
	})
}

func (opts *Options) CheckAndSetDefaults() error {
	if opts.Addr == "" {
		opts.Addr = DefaultLocalAddr
	}
	if opts.ConfigPath == "" {
		opts.ConfigPath = DefaultConfigPath
	}
	if opts.Name == "" {
		opts.Name = DefaultUser
	}
	if opts.Role == "" {
		opts.Role = DefaultRole
	}
	if opts.DataDir == "" {
		opts.DataDir = DefaultDataDir
	}
	return nil
}
