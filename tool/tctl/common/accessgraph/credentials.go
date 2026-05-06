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
	"bytes"
	"context"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/storage"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
)

// accessGraphSetupDocURL is surfaced in the user-visible error when
// Access Graph is licensed but not yet configured on the cluster.
const accessGraphSetupDocURL = "https://goteleport.com/docs/identity-security/"

// unlicensedAccessGraphMessage is returned when the cluster lacks the
// Policy entitlement that gates Access Graph.
const unlicensedAccessGraphMessage = "this Teleport cluster's license does not include the Policy " +
	"entitlement, which is required to use Access Graph. Contact " +
	"your Teleport account team to enable it."

// unconfiguredAccessGraphMessage is returned when the cluster is
// licensed for Access Graph but the operator has not wired up the
// `access_graph` block in the auth service config. Carries a single
// %s for accessGraphSetupDocURL.
const unconfiguredAccessGraphMessage = "Access Graph is licensed on this cluster but not configured. " +
	"On self-hosted clusters, add an `access_graph` section to the " +
	"auth_service config in teleport.yaml and restart auth; on " +
	"Teleport Cloud, enable Access Graph from the cluster admin " +
	"settings. See %s for setup instructions."

// accessGraphMinPersistTTL is the minimum cert lifetime for disk persistence;
// shorter-lived certs (MFA-elevated single-use, role TTL caps, etc.) are
// kept in memory only. tool/tsh/common/app.go onAppLogin handles this with
// a server-side IsMFARequired probe before issuance, but AG has no obvious
// probe target, so this TTL floor is a preventative guard. 5m is well
// above the 1m single-use MFA TTL.
const accessGraphMinPersistTTL = 5 * time.Minute

// accessGraphCertExpiryBuffer is the pre-expiry guard for in-flight AG calls
// (1m mirrors the issuance NotBefore clock-skew backdate, plus 1m of
// operational margin).
const accessGraphCertExpiryBuffer = 2 * time.Minute

// accessGraphCredentials bundles the client-side state needed to talk to
// Access Graph: the proxy address, the optional client store (nil on
// the auth host), and the keyring that holds (or will hold) the AG
// TLS cert.
type accessGraphCredentials struct {
	proxyAddr   string
	clientStore *client.Store
	keyRing     *client.KeyRing
}

// shouldPersistAccessGraphCert reports whether the cert on creds.keyRing
// should be added to the client store. False when there is no store, no
// cert, the cert is unparseable, or the lifetime is below
// accessGraphMinPersistTTL.
func shouldPersistAccessGraphCert(ctx context.Context, creds *accessGraphCredentials) bool {
	if creds.clientStore == nil {
		return false
	}
	if len(creds.keyRing.AccessGraphTLSCert) == 0 {
		return false
	}
	expires, err := creds.keyRing.AccessGraphTLSCertValidBefore()
	if err != nil {
		slog.DebugContext(ctx, "Failed to read Access Graph certificate expiration", "error", err)
		return false
	}
	return expires.After(time.Now().Add(accessGraphMinPersistTTL))
}

// resolveAuthHostAccessGraphCredentials builds creds from the local
// admin identity. proxyAddr may be empty for ensureAccessGraphCert
// to backfill via the auth Ping.
func resolveAuthHostAccessGraphCredentials(ctx context.Context, cfg *servicecfg.Config, username string) (*accessGraphCredentials, error) {
	if cfg == nil {
		return nil, trace.BadParameter("missing service config")
	}
	if username == "" {
		return nil, trace.BadParameter("--cert-user is required when running on the auth host")
	}
	ident, err := storage.ReadLocalIdentityForRole(ctx, filepath.Join(cfg.DataDir, teleport.ComponentProcess), types.RoleAdmin)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	tlsPriv, err := keys.ParsePrivateKey(ident.KeyBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	keyRing := &client.KeyRing{
		KeyRingIndex:  client.KeyRingIndex{Username: username},
		TLSPrivateKey: tlsPriv,
		TLSCert:       ident.TLSCertBytes,
	}
	var proxyAddr string
	if addrs := cfg.Proxy.PublicAddrs; len(addrs) > 0 {
		proxyAddr = addrs[0].String()
	}
	return &accessGraphCredentials{
		proxyAddr:   proxyAddr,
		clientStore: nil,
		keyRing:     keyRing,
	}, nil
}

// resolveAccessGraphCredentials builds an accessGraphCredentials from an
// already-loaded client store and profile, so the lookup logic can be
// tested without an on-disk profile.
func resolveAccessGraphCredentials(ctx context.Context, clientStore *client.Store, profile *client.ProfileStatus) (*accessGraphCredentials, error) {
	if profile.ProxyURL.Host == "" {
		return nil, trace.NotFound("could not find the proxy public address for the requested profile")
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
		"proxy_addr", profile.ProxyURL.Host,
		"profile_name", profile.Name,
		"cluster", profile.Cluster,
		"username", profile.Username,
		"has_access_graph_cert", len(keyRing.AccessGraphTLSCert) > 0,
	)

	return &accessGraphCredentials{
		proxyAddr:   profile.ProxyURL.Host,
		clientStore: clientStore,
		keyRing:     keyRing,
	}, nil
}

// ensureAccessGraphCert reuses the keyring's existing Access Graph cert
// when it's still valid, otherwise initializes the auth client, checks
// the license/feature gate, and re-issues. May populate creds.proxyAddr
// from the auth Ping when the auth-host branch left it empty.
func ensureAccessGraphCert(ctx context.Context, creds *accessGraphCredentials, clientFunc commonclient.InitFunc) error {
	if creds == nil || creds.keyRing == nil {
		return trace.BadParameter("missing access graph credentials")
	}

	// Fast path requires both a valid cert AND a known proxy addr — the
	// auth-host branch may have left proxyAddr empty.
	if creds.proxyAddr != "" && validateAccessGraphCert(ctx, creds.keyRing) {
		slog.DebugContext(ctx, "Reusing existing Access Graph certificate from keyring on disk",
			"proxy_addr", creds.proxyAddr,
			"username", creds.keyRing.Username,
		)
		return nil
	}

	slog.DebugContext(ctx, "Re-issuing Access Graph certificate",
		"proxy_addr", creds.proxyAddr,
		"username", creds.keyRing.Username,
	)

	authClient, closeFn, err := clientFunc(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	defer closeFn(ctx)

	ping, err := checkAccessGraphSupported(ctx, authClient)
	if err != nil {
		return trace.Wrap(err)
	}

	// Backfill proxy addr from auth when config didn't supply one.
	if creds.proxyAddr == "" {
		creds.proxyAddr = ping.GetProxyPublicAddr()
		if creds.proxyAddr == "" {
			return trace.NotFound("auth did not advertise a proxy public address; set proxy_service.public_addr")
		}
		slog.DebugContext(ctx, "Resolved Access Graph proxy address from auth ping",
			"proxy_addr", creds.proxyAddr,
		)
	}

	return trace.Wrap(issueAndStoreAccessGraphCert(ctx, creds, authClient))
}

// checkAccessGraphSupported pings auth and refuses if Access Graph isn't
// licensed or isn't enabled on the cluster. The actual license gate is
// the `Policy` entitlement: the `AccessGraph` and `AccessGraphDemoMode`
// entitlement keys are never populated by either OSS or Enterprise
// modules code, and `Features.AccessGraph` is derived server-side from
// `entitlements.Policy.Enabled` (see `e/tool/modules/modules.go`).
// Endpoint reachability is checked at AG call time, not here.
func checkAccessGraphSupported(ctx context.Context, authClient authclient.ClientI) (proto.PingResponse, error) {
	ping, err := authClient.Ping(ctx)
	if err != nil {
		return proto.PingResponse{}, trace.Wrap(err, "pinging cluster to check Access Graph support")
	}
	features := ping.GetServerFeatures()

	// Distinct from the AccessGraph/DemoMode check below: this gate
	// routes the "not licensed" error message and accepts the legacy
	// `Policy` submessage from older clusters that predate the
	// entitlements map.
	policy := features.GetEntitlements()[string(entitlements.Policy)]
	licensed := policy.GetEnabled() || features.GetPolicy().GetEnabled()
	if !licensed {
		return proto.PingResponse{}, trace.AccessDenied(unlicensedAccessGraphMessage)
	}

	if !features.GetAccessGraph() && !features.GetAccessGraphDemoMode() {
		return proto.PingResponse{}, trace.AccessDenied(unconfiguredAccessGraphMessage, accessGraphSetupDocURL)
	}

	slog.DebugContext(ctx, "Access Graph is available on this cluster",
		"licensed", licensed,
		"access_graph_flag", features.GetAccessGraph(),
		"access_graph_demo_mode_flag", features.GetAccessGraphDemoMode(),
	)
	return ping, nil
}

// issueAndStoreAccessGraphCert mints a new Access Graph cert and, when
// shouldPersistAccessGraphCert allows, persists the updated keyring.
func issueAndStoreAccessGraphCert(ctx context.Context, creds *accessGraphCredentials, authClient authclient.ClientI) error {
	if err := issueAccessGraphCert(ctx, creds.keyRing, authClient); err != nil {
		return trace.Wrap(err)
	}

	if !shouldPersistAccessGraphCert(ctx, creds) {
		slog.DebugContext(ctx, "Skipping Access Graph cert persistence",
			"has_client_store", creds.clientStore != nil,
			"proxy_addr", creds.proxyAddr,
			"username", creds.keyRing.Username,
		)
		return nil
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

// validateAccessGraphCert reports whether the keyring's Access Graph
// cert is present, unexpired, and bound to the keyring's TLS private key.
func validateAccessGraphCert(ctx context.Context, keyRing *client.KeyRing) bool {
	if len(keyRing.AccessGraphTLSCert) == 0 {
		slog.DebugContext(ctx, "Access Graph certificate not present in keyring")
		return false
	}
	if !validateAccessGraphCertExpiration(ctx, keyRing) {
		return false
	}
	return validateAccessGraphPrivateKey(ctx, keyRing)
}

// validateAccessGraphCertExpiration reports whether the cert is valid
// for at least accessGraphCertExpiryBuffer past now.
func validateAccessGraphCertExpiration(ctx context.Context, keyRing *client.KeyRing) bool {
	expires, err := keyRing.AccessGraphTLSCertValidBefore()
	if err != nil {
		slog.DebugContext(ctx, "Failed to read Access Graph certificate expiration", "error", err)
		return false
	}
	if !expires.After(time.Now().Add(accessGraphCertExpiryBuffer)) {
		slog.DebugContext(ctx, "Access Graph certificate is expired or below buffer", "expires", expires)
		return false
	}
	return true
}

// validateAccessGraphPrivateKey checks that the cert's subject public key
// matches the keyring's TLS private key.
func validateAccessGraphPrivateKey(ctx context.Context, keyRing *client.KeyRing) bool {
	cert, err := keyRing.AccessGraphTLSCertificate()
	if err != nil {
		slog.DebugContext(ctx, "Failed to parse Access Graph certificate", "error", err)
		return false
	}

	certPub, err := keys.MarshalPublicKey(cert.PublicKey)
	if err != nil {
		slog.DebugContext(ctx, "Failed to marshal Access Graph certificate public key", "error", err)
		return false
	}
	keyPub, err := keyRing.TLSPrivateKey.MarshalTLSPublicKey()
	if err != nil {
		slog.DebugContext(ctx, "Failed to marshal keyring TLS public key", "error", err)
		return false
	}
	if !bytes.Equal(certPub, keyPub) {
		slog.DebugContext(ctx, "Access Graph certificate public key does not match the keyring's TLS private key")
		return false
	}
	return true
}

// issueAccessGraphCert mints a new Access Graph TLS cert and sets it on
// keyRing in memory; the caller persists it. The cert's NotAfter is
// bound to the keyring's Teleport TLS cert.
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

	keyRing.AccessGraphTLSCert = certs.TLS
	slog.DebugContext(ctx, "Issued new Access Graph certificate",
		"username", keyRing.Username,
		"expires", expires,
	)
	return nil
}
