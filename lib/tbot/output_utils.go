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

package tbot

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/client"
	"github.com/gravitational/teleport/lib/client/identityfile"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tlsca"
)

const renewalRetryLimit = 5

// newBotConfigWriter returns a new BotConfigWriter that writes to the given
// Destination.
func newBotConfigWriter(ctx context.Context, dest bot.Destination, subPath string) *BotConfigWriter {
	return &BotConfigWriter{
		ctx:     ctx,
		dest:    dest,
		subpath: subPath,
	}
}

// BotConfigWriter is a trivial adapter to use the identityfile package with
// bot destinations.
type BotConfigWriter struct {
	ctx context.Context

	// dest is the Destination that will handle writing of files.
	dest bot.Destination

	// subpath is the subdirectory within the Destination to which the files
	// should be written.
	subpath string
}

// WriteFile writes the file to the Destination. Only the basename of the path
// is used. Specified permissions are ignored.
func (b *BotConfigWriter) WriteFile(name string, data []byte, _ os.FileMode) error {
	p := filepath.Base(name)
	if b.subpath != "" {
		p = filepath.Join(b.subpath, p)
	}

	return trace.Wrap(b.dest.Write(b.ctx, p, data))
}

// Remove removes files. This is a dummy implementation that always returns not found.
func (b *BotConfigWriter) Remove(name string) error {
	return &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
}

// Stat checks file status. This implementation always returns not found.
func (b *BotConfigWriter) Stat(name string) (fs.FileInfo, error) {
	return nil, &os.PathError{Op: "stat", Path: name, Err: os.ErrNotExist}
}

// ReadFile reads a given file. This implementation always returns not found.
func (b *BotConfigWriter) ReadFile(name string) ([]byte, error) {
	return nil, &os.PathError{Op: "read", Path: name, Err: os.ErrNotExist}
}

// compile-time assertion that the BotConfigWriter implements the
// identityfile.ConfigWriter interface
var _ identityfile.ConfigWriter = (*BotConfigWriter)(nil)

// NewClientKeyRing returns a sane client.KeyRing for the given bot identity.
func NewClientKeyRing(ident *identity.Identity, hostCAs []types.CertAuthority) (*client.KeyRing, error) {
	pk, err := keys.ParsePrivateKey(ident.PrivateKeyBytes)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &client.KeyRing{
		KeyRingIndex: client.KeyRingIndex{
			ClusterName: ident.ClusterName,
		},
		// tbot identities use a single private key for SSH and TLS.
		SSHPrivateKey: pk,
		TLSPrivateKey: pk,
		Cert:          ident.CertBytes,
		TLSCert:       ident.TLSCertBytes,
		TrustedCerts:  authclient.AuthoritiesToTrustedCerts(hostCAs),

		// Note: these fields are never used or persisted with identity files,
		// so we won't bother to set them. (They may need to be reconstituted
		// on tsh's end based on cert fields, though.)
		KubeTLSCredentials: make(map[string]client.TLSCredential),
		DBTLSCredentials:   make(map[string]client.TLSCredential),
	}, nil
}

func writeIdentityFile(
	ctx context.Context, log *slog.Logger, keyRing *client.KeyRing, dest bot.Destination,
) error {
	ctx, span := tracer.Start(
		ctx,
		"writeIdentityFile",
	)
	defer span.End()

	cfg := identityfile.WriteConfig{
		OutputPath: config.IdentityFilePath,
		Writer:     newBotConfigWriter(ctx, dest, ""),
		KeyRing:    keyRing,
		Format:     identityfile.FormatFile,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	log.DebugContext(ctx, "Wrote identity file", "files", files)
	return nil
}

// writeIdentityFileTLS writes the identity file in TLS format according to the
// core identityfile.Write method. This isn't usually needed but can be
// useful when writing out TLS certificates with alternative prefix and file
// extensions for application compatibility reasons.
func writeIdentityFileTLS(
	ctx context.Context, log *slog.Logger, keyRing *client.KeyRing, dest bot.Destination,
) error {
	ctx, span := tracer.Start(
		ctx,
		"writeIdentityFileTLS",
	)
	defer span.End()

	cfg := identityfile.WriteConfig{
		OutputPath: config.DefaultTLSPrefix,
		Writer:     newBotConfigWriter(ctx, dest, ""),
		KeyRing:    keyRing,
		Format:     identityfile.FormatTLS,

		// Always overwrite to avoid hitting our no-op Stat() and Remove() functions.
		OverwriteDestination: true,
	}

	files, err := identityfile.Write(ctx, cfg)
	if err != nil {
		return trace.Wrap(err)
	}

	log.DebugContext(ctx, "Wrote TLS identity files", "files", files)
	return nil
}

// concatCACerts borrow's identityfile's CA cert concat method.
func concatCACerts(cas []types.CertAuthority) []byte {
	trusted := authclient.AuthoritiesToTrustedCerts(cas)

	var caCerts []byte
	for _, ca := range trusted {
		for _, cert := range ca.TLSCertificates {
			caCerts = append(caCerts, cert...)
		}
	}

	return caCerts
}

// writeTLSCAs writes the three "main" TLS CAs to disk.
// TODO(noah): This is largely a copy of templateTLSCAs. We should reconsider
// which CAs are actually worth writing for each type of service because
// it seems inefficient to write the "Database" CA for a Kubernetes output.
func writeTLSCAs(ctx context.Context, dest bot.Destination, hostCAs, userCAs, databaseCAs []types.CertAuthority) error {
	ctx, span := tracer.Start(
		ctx,
		"writeTLSCAs",
	)
	defer span.End()

	// Note: This implementation mirrors tctl's current behavior. I've noticed
	// that mariadb at least does not seem to like being passed more than one
	// CA so there may be some compat issues to address in the future for the
	// rare case where a CA rotation is in progress.
	if err := dest.Write(ctx, config.HostCAPath, concatCACerts(hostCAs)); err != nil {
		return trace.Wrap(err)
	}

	if err := dest.Write(ctx, config.UserCAPath, concatCACerts(userCAs)); err != nil {
		return trace.Wrap(err)
	}

	if err := dest.Write(ctx, config.DatabaseCAPath, concatCACerts(databaseCAs)); err != nil {
		return trace.Wrap(err)
	}

	return nil
}

// describeTLSIdentity generates an informational message about the given
// TLS identity, appropriate for user-facing log messages.
func describeTLSIdentity(ctx context.Context, log *slog.Logger, ident *identity.Identity) string {
	failedToDescribe := "failed-to-describe"
	cert := ident.X509Cert
	if cert == nil {
		log.WarnContext(ctx, "Attempted to describe TLS identity without TLS credentials.")
		return failedToDescribe
	}

	tlsIdent, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		log.WarnContext(ctx, "Bot TLS certificate can not be parsed as an identity", "error", err)
		return failedToDescribe
	}

	var principals []string
	for _, principal := range tlsIdent.Principals {
		if !strings.HasPrefix(principal, constants.NoLoginPrefix) {
			principals = append(principals, principal)
		}
	}

	botDesc := ""
	if tlsIdent.BotInstanceID != "" {
		botDesc = fmt.Sprintf(", id=%s", tlsIdent.BotInstanceID)
	}

	duration := cert.NotAfter.Sub(cert.NotBefore)
	return fmt.Sprintf(
		"%s%s | valid: after=%v, before=%v, duration=%s | kind=tls, renewable=%v, disallow-reissue=%v, roles=%v, principals=%v, generation=%v",
		tlsIdent.BotName,
		botDesc,
		cert.NotBefore.Format(time.RFC3339),
		cert.NotAfter.Format(time.RFC3339),
		duration,
		tlsIdent.Renewable,
		tlsIdent.DisallowReissue,
		tlsIdent.Groups,
		principals,
		tlsIdent.Generation,
	)
}

// identityConfigurator is a function that alters a cert request
type identityConfigurator = func(req *proto.UserCertsRequest)

// generateIdentity uses an identity to retrieve an impersonated identity.
// The `configurator` function, if not nil, can be used to add additional
// requests to the certificate request, for example to add `RouteToDatabase`
// and similar fields, however in that case it must be called with an
// impersonated identity that already has the relevant permissions, much like
// `tsh (app|db|kube) login` is already used to generate an additional set of
// certs.
func generateIdentity(
	ctx context.Context,
	client *authclient.Client,
	currentIdentity *identity.Identity,
	roles []string,
	ttl time.Duration,
	configurator identityConfigurator,
) (*identity.Identity, error) {
	ctx, span := tracer.Start(ctx, "generateIdentity")
	defer span.End()

	// TODO: enforce expiration > renewal period (by what margin?)
	//   This should be ignored if a renewal has been triggered manually or
	//   by a CA rotation.

	req := proto.UserCertsRequest{
		Username:       currentIdentity.X509Cert.Subject.CommonName,
		Expires:        time.Now().Add(ttl),
		RoleRequests:   roles,
		RouteToCluster: currentIdentity.ClusterName,

		// Make sure to specify this is an impersonated cert request. If unset,
		// auth cannot differentiate renewable vs impersonated requests when
		// len(roleRequests) == 0.
		UseRoleRequests: true,
	}

	if configurator != nil {
		configurator(&req)
	}

	keyPurpose := cryptosuites.BotImpersonatedIdentity
	if req.RouteToDatabase.ServiceName != "" {
		// We still used RSA for all database clients, all other bot
		// impersonated identities can use ECDSA.
		keyPurpose = cryptosuites.DatabaseClient
	}

	// Generate a fresh keypair for the impersonated identity. We don't care to
	// reuse keys here, constantly rotate private keys to limit their effective
	// lifetime.
	key, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(client),
		keyPurpose)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshPub, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.SSHPublicKey = ssh.MarshalAuthorizedKey(sshPub)

	req.TLSPublicKey, err = keys.MarshalPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// First, ask the auth server to generate a new set of certs with a new
	// expiration date.
	certs, err := client.GenerateUserCerts(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The root CA included with the returned user certs will only contain the
	// Teleport User CA. We'll also need the host CA for future API calls.
	localCA, err := client.GetClusterCACert(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	caCerts, err := tlsca.ParseCertificatePEMs(localCA.TLSCA)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Append the host CAs from the auth server.
	for _, cert := range caCerts {
		pemBytes, err := tlsca.MarshalCertificatePEM(cert)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		certs.TLSCACerts = append(certs.TLSCACerts, pemBytes)
	}

	// Do not trust SSH CA certs as returned by GenerateUserCerts() with an
	// impersonated identity. It only returns the SSH UserCA in this context,
	// but we also need the HostCA and can't directly set `includeHostCA` as
	// part of the UserCertsRequest.
	// Instead, copy the SSHCACerts from the primary identity.
	certs.SSHCACerts = currentIdentity.SSHCACertBytes

	privateKeyPEM, err := keys.MarshalPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newIdentity, err := identity.ReadIdentityFromStore(&identity.LoadIdentityParams{
		PrivateKeyBytes: privateKeyPEM,
		PublicKeyBytes:  req.SSHPublicKey,
	}, certs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newIdentity, nil
}

// warnOnEarlyExpiration logs a warning if the given identity is likely to
// expire problematically early. This can happen if either the configured TTL is
// less than the renewal interval, or if the server returns certs valid for a
// shorter-than-expected period of time.
// This assumes the identity was just renewed, for the purposes of calculating
// TTLs, and may log false positive warnings if the time delta is large; the
// time calculations include a 1m buffer to mitigate this.
func warnOnEarlyExpiration(
	ctx context.Context,
	log *slog.Logger,
	ident *identity.Identity,
	lifetime config.CredentialLifetime,
) {
	// Calculate a rough TTL, assuming this was called shortly after the
	// identity was returned. We'll add a minute buffer to compensate and avoid
	// superfluous warning messages.
	effectiveTTL := ident.TLSIdentity.Expires.Sub(time.Now()) + time.Minute

	if effectiveTTL < lifetime.TTL {
		l := log.With(
			"requested_ttl", lifetime.TTL,
			"renewal_interval", lifetime.RenewalInterval,
			"effective_ttl", effectiveTTL,
			"expires", ident.TLSIdentity.Expires,
			"roles", ident.TLSIdentity.Groups,
		)

		// TODO(timothyb89): we can technically fetch our individual roles
		// without explicit permission, and could determine which role in
		// particular limited the TTL.

		if effectiveTTL < lifetime.RenewalInterval {
			l.WarnContext(ctx, "The server returned an identity shorter than "+
				"expected and below the configured renewal interval, probably "+
				"due to a `max_session_ttl` configured on a server-side role. "+
				"Unless corrected, the credentials will be invalid for some "+
				"period until renewal.")
		} else {
			l.WarnContext(ctx, "The server returned an identity shorter than "+
				"the requested TTL, probably due to a `max_session_ttl` "+
				"configured on a server-side role. It may not remain valid as "+
				"long as expected.")
		}
	}
}

// fetchDefaultRoles requests the bot's own role from the auth server and
// extracts its full list of allowed roles.
func fetchDefaultRoles(ctx context.Context, roleGetter services.RoleGetter, identity *identity.Identity) ([]string, error) {
	role, err := roleGetter.GetRole(ctx, identity.X509Cert.Subject.CommonName)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conditions := role.GetImpersonateConditions(types.Allow)
	return conditions.Roles, nil
}

// chooseOneResource chooses one matched resource by name, or tries to choose
// one resource by unambiguous "discovered name".
func chooseOneResource[T types.ResourceWithLabels](resources []T, name, resDesc string) (T, error) {
	for _, r := range resources {
		if r.GetName() == name {
			return r, nil
		}
	}

	// look for an unambiguous "discovered name" match as a fallback.
	var matches []T
	for _, r := range resources {
		discoveredName, ok := r.GetLabel(types.DiscoveredNameLabel)
		if ok && discoveredName == name {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 0:
		var out T
		return out, trace.NotFound("%s %q not found", resDesc, name)
	case 1:
		return matches[0], nil
	default:
		var out T
		errMsg := formatAmbiguousMessage(name, resDesc, matches)
		return out, trace.BadParameter(errMsg)
	}
}

// formatAmbiguousMessage formats a generic error message that describes an ambiguous
// auto-discovered resource name match error.
func formatAmbiguousMessage[T types.ResourceWithLabels](name, resDesc string, matches []T) string {
	matchedNames := make([]string, 0, len(matches))
	for _, match := range matches {
		matchedNames = append(matchedNames, match.GetName())
	}
	slices.Sort(matchedNames)
	return fmt.Sprintf(`%q matches multiple auto-discovered %ss:
%v

Use the full resource name that was generated by the Teleport Discovery service`,
		name, resDesc, strings.Join(matchedNames, "\n"))
}

// makeNameOrDiscoveredNamePredicate returns a predicate that matches resources
// by name or by "discovered name" label.
func makeNameOrDiscoveredNamePredicate(name string) string {
	matchName := fmt.Sprintf("name == %q", name)
	matchDiscoveredName := fmt.Sprintf("labels[%q] == %q",
		types.DiscoveredNameLabel, name,
	)
	return fmt.Sprintf("(%v) || (%v)",
		matchName, matchDiscoveredName,
	)
}
