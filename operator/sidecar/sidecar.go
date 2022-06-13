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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/breaker"
	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	libclient "github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
)

const (
	DefaultLocalAddr  = "127.0.0.1:3025"
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

	// User is a user used to access Teleport Auth/Proxy/Tunnel server.
	User string

	// Role is a role allowed to manage Teleport resources.
	Role string
}

func writeIdentityFile(ctx context.Context, clusterAPI auth.ClientI, identityFilePath, userName string) error {
	// generate a keypair:
	key, err := libclient.NewKey()
	if err != nil {
		return trace.Wrap(err)
	}

	reqExpiry := time.Now().UTC().Add(1 * time.Hour)
	// Request signed certs from `auth` server.
	certs, err := clusterAPI.GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: key.Pub,
		Username:  userName,
		Expires:   reqExpiry,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	key.Cert = certs.SSH
	key.TLSCert = certs.TLS

	hostCAs, err := clusterAPI.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return trace.Wrap(err)
	}
	key.TrustedCA = auth.AuthoritiesToTrustedCerts(hostCAs)

	// write the cert+private key to the output:
	filesWritten, err := identityfile.Write(identityfile.WriteConfig{
		OutputPath:           identityFilePath,
		Key:                  key,
		Format:               identityfile.FormatFile,
		OverwriteDestination: true,
	})
	if err != nil {
		return trace.Wrap(err)
	}
	fmt.Printf("\nThe credentials have been written to %s\n", strings.Join(filesWritten, ", "))
	return nil
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

	cfg.AuthServers, err = utils.ParseAddrs([]string{opts.Addr})
	if err != nil {
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
	authConfig.AuthServers = cfg.AuthServers
	authConfig.Log = cfg.Log

	return authConfig, nil
}

// NewSidecarClient returns a connection to the Teleport server running on the same machine or pod.
// It automatically upserts the sidecar role and the user and generates the credentials.
func NewSidecarClient(ctx context.Context, opts Options) (*client.Client, error) {
	var err error
	if err := opts.CheckAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	authClientConfig, err := createAuthClientConfig(opts)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create auth client config")
	}

	authClient, err := authclient.Connect(ctx, authClientConfig)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create auth client")
	}

	role, err := sidecarRole(opts.Role)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create role")
	}

	if err := authClient.UpsertRole(ctx, role); err != nil {
		return nil, trace.Wrap(err, "failed to create operator's role")
	}

	user, err := sidecarUserWithRole(opts.User, opts.Role)
	if err != nil {
		return nil, trace.Wrap(err, "failed to create user")
	}

	if err := authClient.UpsertUser(user); err != nil {
		return nil, trace.Wrap(err, "failed to create operator's role")
	}

	identityfile, err := os.CreateTemp("", "teleport-identity-*")
	if err != nil {
		return nil, trace.Wrap(err, "failed to create temp identity file")
	}
	defer os.Remove(identityfile.Name())

	if err := writeIdentityFile(ctx, authClient, identityfile.Name(), opts.User); err != nil {
		return nil, trace.Wrap(err, "failed to write identity file")
	}

	creds := []client.Credentials{
		client.LoadIdentityFile(identityfile.Name()),
	}

	return client.New(ctx, client.Config{
		Addrs:       []string{opts.Addr},
		Credentials: creds,
	})
}

func sidecarRole(roleName string) (types.Role, error) {
	return types.NewRole(roleName, types.RoleSpecV5{
		Allow: types.RoleConditions{
			Rules: []types.Rule{
				{
					Resources: []string{"*"},
					Verbs:     []string{"*"},
				},
			},
		},
	})
}

func sidecarUserWithRole(userName, roleName string) (types.User, error) {
	user, err := types.NewUser(userName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	user.AddRole(roleName)

	return user, nil
}

func (opts *Options) CheckAndSetDefaults() error {
	if opts.Addr == "" {
		opts.Addr = DefaultLocalAddr
	}
	if opts.ConfigPath == "" {
		opts.ConfigPath = DefaultConfigPath
	}
	if opts.User == "" {
		opts.User = DefaultUser
	}
	if opts.Role == "" {
		opts.Role = DefaultRole
	}
	return nil
}
