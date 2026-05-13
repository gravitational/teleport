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
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	commonclient "github.com/gravitational/teleport/tool/tctl/common/client"
	tctlcfg "github.com/gravitational/teleport/tool/tctl/common/config"
)

// accessGraphSetupDocURL is surfaced in the user-visible error when
// Access Graph is licensed but not yet configured on the cluster.
const accessGraphSetupDocURL = "https://goteleport.com/docs/identity-security/"

// unlicensedAccessGraphMessage is returned when the cluster lacks the
// Identity Security license that gates Access Graph.
const unlicensedAccessGraphMessage = "this Teleport cluster is not licensed for Identity Security, " +
	"which is required to use Access Graph. Contact your Teleport " +
	"account team to enable it."

// unconfiguredAccessGraphMessage is returned when the cluster is
// licensed for Access Graph but the operator has not wired up the
// `access_graph` block in the auth service config.
const unconfiguredAccessGraphMessage = "Access Graph is licensed on this cluster but not configured. " +
	"On self-hosted clusters, add an `access_graph` section to the " +
	"auth_service config in teleport.yaml and restart auth; on " +
	"Teleport Cloud, enable Access Graph from the cluster admin " +
	"settings. See %s for setup instructions."

// accessGraphMinPersistTTL is a preventative cert-lifetime floor for
// disk persistence — `IsMFARequired` (used by `tsh login` `onAppLogin`)
// isn't available for AG. 5m exceeds the 1m single-use MFA clamp.
const accessGraphMinPersistTTL = 5 * time.Minute

// accessGraphCertExpiryBuffer is the pre-expiry guard for in-flight AG calls
// (1m mirrors the issuance NotBefore clock-skew backdate, plus 1m of
// operational margin).
const accessGraphCertExpiryBuffer = 2 * time.Minute

// accessGraphCredentials bundles AG client state. The resolver only
// sets `proxyAddr` for the `tsh login` flow; identity-file mode leaves
// it empty and `ensureAccessGraphCert` fills it from `Ping`.
type accessGraphCredentials struct {
	proxyAddr   string
	clientStore *client.Store
	keyRing     *client.KeyRing
}

// authHostNotSupportedMessage explains why `tctl <accessgraph>` refuses
// to run directly on the auth host (no user to bind the issued cert to)
// and points the operator at the supported flows.
const authHostNotSupportedMessage = `Access Graph credentials cannot be issued directly on the auth host: this command is being run without a tsh profile or identity file, so there is no user to bind the issued certificate to.

The recommended flow is to run ` + "`tsh login`" + ` from a workstation, then re-run this command from that workstation — it will pick up the resulting profile and populate the Access Graph cert in that user's keyring.

If you must run ` + "`tsh login`" + ` on this auth host directly, be aware that it will create a ` + "`~/.tsh`" + ` profile directory for the invoking OS user (if one does not already exist) and write credentials into it. That directory should be cleaned up afterwards. To avoid writing to ` + "`~/.tsh`" + ` at all, either:
  - set ` + "`TELEPORT_HOME`" + ` to an isolated path before running ` + "`tsh login`" + ` and this command (the directory under that path will still need to be cleaned up afterwards), or
  - pass a pre-issued identity file via ` + "`tctl -i <identity-file> --auth-server <proxy-or-auth-addr>`" + ` and skip ` + "`tsh login`" + ` entirely.`

// loadAccessGraphCredentials resolves AG credentials from the active
// profile (or an identity file via `tctl -i`). Running directly on the
// auth host without a client store is not supported;
// see `authHostNotSupportedMessage` for the rationale.
func (c *AccessGraphCommand) loadAccessGraphCredentials(ctx context.Context) (*accessGraphCredentials, error) {
	resolved, err := tctlcfg.ApplyConfig(c.ccf, c.config)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if resolved.ClientStore == nil {
		return nil, trace.BadParameter("%s", authHostNotSupportedMessage)
	}
	return resolveAccessGraphCredentials(ctx, c.ccf, resolved)
}

// resolveAccessGraphCredentials builds an `accessGraphCredentials` from
// an already-resolved tctl config (profile or identity file).
func resolveAccessGraphCredentials(ctx context.Context, ccf *tctlcfg.GlobalCLIFlags, resolved *tctlcfg.ResolvedConfig) (*accessGraphCredentials, error) {
	if ccf == nil || resolved == nil || resolved.ClientStore == nil || resolved.Profile == nil {
		return nil, trace.BadParameter("missing client store or profile")
	}
	profile := resolved.Profile
	if profile.ProxyURL.Host == "" {
		return nil, trace.NotFound("could not find the proxy public address for the requested profile")
	}

	idx := client.KeyRingIndex{
		ProxyHost:   profile.Name,
		ClusterName: profile.Cluster,
		Username:    profile.Username,
	}
	keyRing, err := resolved.ClientStore.GetKeyRing(idx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Identity-file mode: `profile.ProxyURL.Host` reflects
	// `--auth-server`, which may be an auth gRPC address. Leave
	// `proxyAddr` empty so `ensureAccessGraphCert` resolves it
	// from `Ping`.
	proxyAddr := ""
	if ccf.IdentityFilePath == "" {
		proxyAddr = profile.ProxyURL.Host
	}

	slog.DebugContext(ctx, "Loaded Access Graph credentials",
		"proxy_addr", proxyAddr,
		"profile_name", profile.Name,
		"cluster", profile.Cluster,
		"username", profile.Username,
		"identity_file", ccf.IdentityFilePath != "",
		"has_access_graph_cert", len(keyRing.AccessGraphTLSCert) > 0,
	)

	return &accessGraphCredentials{
		proxyAddr:   proxyAddr,
		clientStore: resolved.ClientStore,
		keyRing:     keyRing,
	}, nil
}

// ensureAccessGraphCert reuses a valid cached cert or re-issues via
// the auth client; on re-issue, an empty `creds.proxyAddr` is filled
// from `Ping`.
func ensureAccessGraphCert(ctx context.Context, creds *accessGraphCredentials, clientFunc commonclient.InitFunc) error {
	if creds == nil || creds.keyRing == nil {
		return trace.BadParameter("missing access graph credentials")
	}

	// Fast path: previously-issued cert is still on an FS-backed keyring
	// and validates. Only the `tsh login` flow exercises this; auth-host
	// and identity-file modes don't carry AG certs and always re-issue.
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

	ping, err := authClient.Ping(ctx)
	if err != nil {
		return trace.Wrap(err, "pinging cluster to check Access Graph support")
	}
	if err := checkAccessGraphSupported(ctx, ping); err != nil {
		return trace.Wrap(err)
	}

	// Backfill `proxyAddr` only when the resolver left it empty so
	// `tsh login` keeps `profile.ProxyURL.Host` even on clusters
	// without `proxy_service.public_addr`.
	if creds.proxyAddr == "" {
		creds.proxyAddr = ping.GetProxyPublicAddr()
		if creds.proxyAddr == "" {
			return trace.NotFound("auth server did not advertise a proxy public address; set proxy_service.public_addr")
		}
		slog.DebugContext(ctx, "Resolved Access Graph proxy address from auth ping",
			"proxy_addr", creds.proxyAddr,
		)
	}
	return trace.Wrap(issueAndStoreAccessGraphCert(ctx, creds, authClient))
}

// checkAccessGraphSupported gates on the `Policy` entitlement.
// Endpoint reachability is checked at AG call time, not here.
func checkAccessGraphSupported(ctx context.Context, ping proto.PingResponse) error {
	features := ping.GetServerFeatures()

	// This gate routes the "not licensed" error message and accepts the legacy
	// `Policy` submessage from older clusters that predate the entitlements map.
	//
	// TODO(ghassan): when #66571's follow-ups split Identity Security out of Policy,
	//  gate on the more specific entitlement here and keep Policy as the legacy fallback.
	policy := features.GetEntitlements()[string(entitlements.Policy)]
	licensed := policy.GetEnabled() || features.GetPolicy().GetEnabled()
	if !licensed {
		return trace.AccessDenied(unlicensedAccessGraphMessage)
	}

	if !features.GetAccessGraph() {
		return trace.AccessDenied(unconfiguredAccessGraphMessage, accessGraphSetupDocURL)
	}

	slog.DebugContext(ctx, "Access Graph is available on this cluster",
		"licensed", licensed,
		"access_graph_flag", features.GetAccessGraph(),
	)
	return nil
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

// shouldPersistAccessGraphCert gates `AddKeyRing`; identity-file uses
// `MemClientStore` so this is a no-op for disk regardless.
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
// keyRing in memory; the caller persists it (when appropriate).
// The cert's NotAfter is bound to the keyring's Teleport TLS cert.
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
