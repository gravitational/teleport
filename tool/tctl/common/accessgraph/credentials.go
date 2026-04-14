/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
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

package accessgraph

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	libhwk "github.com/gravitational/teleport/lib/hardwarekey"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
)

type accessGraphCredentials struct {
	proxyAddr   string
	clientStore *client.Store
	profile     *client.ProfileStatus
	keyRing     *client.KeyRing
}

func (c *AccessGraphCommand) loadAccessGraphCredentials(ctx context.Context) (*accessGraphCredentials, error) {
	proxyAddr := ""
	if len(c.ccf.AuthServerAddr) != 0 {
		proxyAddr = c.ccf.AuthServerAddr[0]
	}

	hwks := libhwk.NewService(ctx, nil /* prompt */)
	clientStore := client.NewFSClientStore(c.config.TeleportHome, client.WithHardwareKeyService(hwks))
	if c.ccf.IdentityFilePath != "" {
		clientStore = client.NewMemClientStore(client.WithHardwareKeyService(hwks))
		if err := identityfile.LoadIdentityFileIntoClientStore(clientStore, c.ccf.IdentityFilePath, proxyAddr, ""); err != nil {
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
				return nil, trace.Errorf("it appears tsh v16 or older was used to log in, make sure to use tsh and tctl on the same major version\n\t%v", profile.GetKeyRingError)
			}
			return nil, trace.Wrap(profile.GetKeyRingError)
		}
		return nil, trace.BadParameter("your credentials have expired, please log in using `tsh login`")
	}

	if proxyAddr == "" {
		proxyAddr = profile.ProxyURL.Host
	}
	if proxyAddr == "" {
		return nil, trace.NotFound("proxy public address is not configured")
	}

	idx := client.KeyRingIndex{
		ProxyHost:   profile.Name,
		ClusterName: profile.Cluster,
		Username:    profile.Username,
	}
	keyRing, err := clientStore.GetKeyRing(idx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	slog.DebugContext(ctx, "Loaded Access Graph credentials",
		"proxy_addr", proxyAddr,
		"profile_name", profile.Name,
		"cluster", profile.Cluster,
		"username", profile.Username,
		"has_access_graph_cert", len(keyRing.AccesssGraphTLSCert) > 0,
	)

	return &accessGraphCredentials{
		proxyAddr:   proxyAddr,
		clientStore: clientStore,
		profile:     profile,
		keyRing:     keyRing,
	}, nil
}

func (c *AccessGraphCommand) ensureAccessGraphCert(ctx context.Context, creds *accessGraphCredentials, clientFunc commonclient.InitFunc) error {
	if creds == nil || creds.keyRing == nil {
		return trace.BadParameter("missing access graph credentials")
	}

	if valid, err := accessGraphCertValid(creds.keyRing); err != nil {
		return trace.Wrap(err)
	} else if valid {
		slog.DebugContext(ctx, "Using cached Access Graph certificate",
			"proxy_addr", creds.proxyAddr,
			"username", creds.keyRing.Username,
		)
		return nil
	}

	slog.DebugContext(ctx, "Access Graph certificate missing or expired, requesting a new certificate",
		"proxy_addr", creds.proxyAddr,
		"username", creds.keyRing.Username,
	)

	authClient, closeFn, err := clientFunc(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeFn(ctx)

	if err := issueAccessGraphCert(ctx, creds.keyRing, authClient); err != nil {
		return trace.Wrap(err)
	}

	if creds.keyRing.ClusterName == "" {
		creds.keyRing.ClusterName = creds.profile.Cluster
	}
	if creds.keyRing.ClusterName == "" {
		clusterName, err := creds.keyRing.RootClusterName()
		if err != nil {
			return trace.Wrap(err)
		}
		creds.keyRing.ClusterName = clusterName
	}

	if err := creds.clientStore.AddKeyRing(creds.keyRing); err != nil {
		return trace.Wrap(err)
	}

	slog.DebugContext(ctx, "Stored Access Graph certificate in keyring",
		"proxy_addr", creds.proxyAddr,
		"cluster", creds.keyRing.ClusterName,
		"username", creds.keyRing.Username,
	)

	return nil
}

func accessGraphCertValid(keyRing *client.KeyRing) (bool, error) {
	if len(keyRing.AccesssGraphTLSCert) == 0 {
		return false, nil
	}

	expires, err := keyRing.AccessGraphTLSCertValidBefore()
	if trace.IsNotFound(err) {
		return false, nil
	}
	if err != nil {
		return false, trace.Wrap(err)
	}

	return expires.After(time.Now()), nil
}

func issueAccessGraphCert(ctx context.Context, keyRing *client.KeyRing, rootAuthClient authclient.ClientI) error {
	tlsPublicKey, err := keys.MarshalPublicKey(keyRing.TLSPrivateKey.Public())
	if err != nil {
		return trace.Wrap(err)
	}

	expires, err := keyRing.TeleportTLSCertValidBefore()
	if err != nil {
		return trace.Wrap(err)
	}

	certs, err := rootAuthClient.GenerateUserCerts(ctx, proto.UserCertsRequest{
		TLSPublicKey: tlsPublicKey,
		Username:     keyRing.Username,
		Expires:      expires,
		Usage:        proto.UserCertsRequest_AccessGraphAPI,
	})
	if err != nil {
		return trace.Wrap(err)
	}

	keyRing.AccesssGraphTLSCert = certs.TLS
	return nil
}
