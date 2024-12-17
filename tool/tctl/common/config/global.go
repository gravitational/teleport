/*
 * Teleport
 * Copyright (C) 2024  Gravitational, Inc.
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

package config

import (
	"errors"
	"io/fs"
	"log/slog"
	"path/filepath"
	"runtime"

	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/state"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/config"
	"github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/utils/hostid"
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
	cfg.HostUUID, err = hostid.ReadFile(cfg.DataDir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, trace.Wrap(err, "Could not load Teleport host UUID file at %s. "+
				"Please make sure that a Teleport Auth Service instance is running on this host prior to using tctl or provide credentials by logging in with tsh first.",
				filepath.Join(cfg.DataDir, hostid.FileName))
		} else if errors.Is(err, fs.ErrPermission) {
			return nil, trace.Wrap(err, "Teleport does not have permission to read Teleport host UUID file at %s. "+
				"Ensure that you are running as a user with appropriate permissions or provide credentials by logging in with tsh first.",
				filepath.Join(cfg.DataDir, hostid.FileName))
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
