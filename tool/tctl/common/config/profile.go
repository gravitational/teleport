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
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/metadata"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/utils"
	logutils "github.com/gravitational/teleport/lib/utils/log"
)

// LoadConfigFromProfile applies config from ~/.tsh/ profile if it's present
func LoadConfigFromProfile(ccf *GlobalCLIFlags, cfg *servicecfg.Config) (*authclient.Config, error) {
	ctx := context.TODO()

	hwKeyService := keys.NewYubiKeyPIVService(ctx, &keys.CLIPrompt{})

	proxyAddr := ""
	if len(ccf.AuthServerAddr) != 0 {
		proxyAddr = ccf.AuthServerAddr[0]
	}

	clientStore := client.NewFSClientStore(cfg.TeleportHome, hwKeyService)
	if ccf.IdentityFilePath != "" {
		var err error
		clientStore, err = identityfile.NewClientStoreFromIdentityFile(ccf.IdentityFilePath, proxyAddr, "", hwKeyService)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	profile, err := clientStore.ReadProfileStatus(proxyAddr)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if profile.IsExpired(time.Now()) {
		if profile.GetKeyRingError != nil {
			if errors.As(profile.GetKeyRingError, new(*client.LegacyCertPathError)) {
				// Intentionally avoid wrapping the error because the caller
				// ignores NotFound errors.
				return nil, trace.Errorf("it appears tsh v16 or older was used to log in, make sure to use tsh and tctl on the same major version\n\t%v", profile.GetKeyRingError)
			}
			return nil, trace.Wrap(profile.GetKeyRingError)
		}
		return nil, trace.BadParameter("your credentials have expired, please log in using `tsh login`")
	}

	c := client.MakeDefaultConfig()
	slog.DebugContext(ctx, "Found profile",
		"proxy", logutils.StringerAttr(&profile.ProxyURL),
		"user", profile.Username,
	)
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
		slog.DebugContext(ctx, "Setting auth server to web proxy", "web_proxy_addr", webProxyAddr)
		cfg.SetAuthServerAddress(*webProxyAddr)
	}
	authConfig.AuthServers = cfg.AuthServerAddresses()
	authConfig.Log = cfg.Logger
	authConfig.DialOpts = append(authConfig.DialOpts, metadata.WithUserAgentFromTeleportComponent(teleport.ComponentTCTL))

	if c.TLSRoutingEnabled {
		cfg.Auth.NetworkingConfig.SetProxyListenerMode(types.ProxyListenerMode_Multiplex)
	}

	return authConfig, nil
}
