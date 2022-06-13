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

package tbot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/authclient"
	"github.com/gravitational/teleport/lib/auth/native"
	"github.com/gravitational/teleport/lib/client"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/destination"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tlsca"
	"github.com/gravitational/teleport/lib/utils"
	libUtils "github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// generateKeys generates TLS and SSH keypairs.
func generateKeys() (private, sshpub, tlspub []byte, err error) {
	privateKey, publicKey, err := native.GenerateKeyPair()
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	sshPrivateKey, err := ssh.ParseRawPrivateKey(privateKey)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	tlsPublicKey, err := tlsca.MarshalPublicKeyFromPrivateKeyPEM(sshPrivateKey)
	if err != nil {
		return nil, nil, nil, trace.Wrap(err)
	}

	return privateKey, publicKey, tlsPublicKey, nil
}

// authenticatedUserClientFromIdentity creates a new auth client from the given
// identity. Note that depending on the connection address given, this may
// attempt to connect via the proxy and therefore requires both SSH and TLS
// credentials.
func (b *Bot) authenticatedUserClientFromIdentity(ctx context.Context, id *identity.Identity, authServer string) (auth.ClientI, error) {
	if id.SSHCert == nil || id.X509Cert == nil {
		return nil, trace.BadParameter("auth client requires a fully formed identity")
	}

	tlsConfig, err := id.TLSConfig(nil /* cipherSuites */)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	sshConfig, err := id.SSHClientConfig()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authAddr, err := utils.ParseAddr(authServer)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	authClientConfig := &authclient.Config{
		TLS:         tlsConfig,
		SSH:         sshConfig,
		AuthServers: []utils.NetAddr{*authAddr},
		Log:         b.log,
	}

	c, err := authclient.Connect(ctx, authClientConfig)
	return c, trace.Wrap(err)
}

// describeTLSIdentity generates an informational message about the given
// TLS identity, appropriate for user-facing log messages.
func describeTLSIdentity(ident *identity.Identity) (string, error) {
	cert := ident.X509Cert
	if cert == nil {
		return "", trace.BadParameter("attempted to describe TLS identity without TLS credentials")
	}

	tlsIdent, err := tlsca.FromSubject(cert.Subject, cert.NotAfter)
	if err != nil {
		return "", trace.Wrap(err, "bot TLS certificate can not be parsed as an identity")
	}

	var principals []string
	for _, principal := range tlsIdent.Principals {
		if !strings.HasPrefix(principal, constants.NoLoginPrefix) {
			principals = append(principals, principal)
		}
	}

	duration := cert.NotAfter.Sub(cert.NotBefore)
	return fmt.Sprintf(
		"valid: after=%v, before=%v, duration=%s | kind=tls, renewable=%v, disallow-reissue=%v, roles=%v, principals=%v, generation=%v",
		cert.NotBefore.Format(time.RFC3339),
		cert.NotAfter.Format(time.RFC3339),
		duration,
		tlsIdent.Renewable,
		tlsIdent.DisallowReissue,
		tlsIdent.Groups,
		principals,
		tlsIdent.Generation,
	), nil
}

// identityConfigurator is a function that alters a cert request
type identityConfigurator = func(req *proto.UserCertsRequest)

// generateIdentity uses an identity to retrieve another possibly impersonated
// identity. The `configurator` function, if not nil, can be used to add
// additional requests to the certificate request, for example to add
// `RouteToDatabase` and similar fields, however in that case it must be
// called with an impersonated identity that already has the relevant
// permissions, much like `tsh (app|db|kube) login` is already used to generate
// an additional set of certs.
func (b *Bot) generateIdentity(
	ctx context.Context,
	currentIdentity *identity.Identity,
	expires time.Time,
	destCfg *config.DestinationConfig,
	defaultRoles []string,
	configurator identityConfigurator,
) (*identity.Identity, error) {
	// TODO: enforce expiration > renewal period (by what margin?)
	//   This should be ignored if a renewal has been triggered manually or
	//   by a CA rotation.

	// Generate a fresh keypair for the impersonated identity. We don't care to
	// reuse keys here: impersonated certs might not be as well-protected so
	// constantly rotating private keys
	privateKey, publicKey, err := native.GenerateKeyPair()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var roleRequests []string
	if len(destCfg.Roles) > 0 {
		roleRequests = destCfg.Roles
	} else {
		b.log.Debugf("Destination specified no roles, defaults will be requested: %v", defaultRoles)
		roleRequests = defaultRoles
	}

	req := proto.UserCertsRequest{
		PublicKey:      publicKey,
		Username:       currentIdentity.X509Cert.Subject.CommonName,
		Expires:        expires,
		RoleRequests:   roleRequests,
		RouteToCluster: currentIdentity.ClusterName,
	}

	if configurator != nil {
		configurator(&req)
	}

	// First, ask the auth server to generate a new set of certs with a new
	// expiration date.
	client := b.client()
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

	newIdentity, err := identity.ReadIdentityFromStore(&identity.LoadIdentityParams{
		PrivateKeyBytes: privateKey,
		PublicKeyBytes:  publicKey,
	}, certs, identity.DestinationKinds()...)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newIdentity, nil
}

func getDatabase(ctx context.Context, client auth.ClientI, name string) (types.Database, error) {
	res, err := client.ListResources(ctx, proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindDatabaseServer,
		PredicateExpression: fmt.Sprintf(`name == "%s"`, name),
		Limit:               int32(defaults.DefaultChunkSize),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := types.ResourcesWithLabels(res.Resources).AsDatabaseServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var databases []types.Database
	for _, server := range servers {
		databases = append(databases, server.GetDatabase())
	}

	databases = types.DeduplicateDatabases(databases)
	if len(databases) == 0 {
		return nil, trace.NotFound("database %q not found", name)
	}

	return databases[0], nil
}

func (b *Bot) getRouteToDatabase(ctx context.Context, client auth.ClientI, dbCfg *config.DatabaseConfig) (proto.RouteToDatabase, error) {
	if dbCfg.Service == "" {
		return proto.RouteToDatabase{}, nil
	}

	db, err := getDatabase(ctx, client, dbCfg.Service)
	if err != nil {
		return proto.RouteToDatabase{}, trace.Wrap(err)
	}

	username := dbCfg.Username
	if db.GetProtocol() == libdefaults.ProtocolMongoDB && username == "" {
		// This isn't strictly a runtime error so killing the process seems
		// wrong. We'll just loudly warn about it.
		b.log.Errorf("Database `username` field for %q is unset but is required for MongoDB databases.", dbCfg.Service)
	} else if db.GetProtocol() == libdefaults.ProtocolRedis && username == "" {
		// Per tsh's lead, fall back to the default username.
		username = libdefaults.DefaultRedisUsername
	}

	return proto.RouteToDatabase{
		ServiceName: dbCfg.Service,
		Protocol:    db.GetProtocol(),
		Database:    dbCfg.Database,
		Username:    username,
	}, nil
}

func (b *Bot) generateImpersonatedIdentity(
	ctx context.Context,
	expires time.Time,
	destCfg *config.DestinationConfig,
	defaultRoles []string,
) (*identity.Identity, error) {
	ident, err := b.generateIdentity(
		ctx, b.ident(), expires, destCfg, defaultRoles, nil,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Now that we have an initial impersonated identity, we can use it to
	// request any app/db/etc certs
	if destCfg.Database != nil {
		impClient, err := b.authenticatedUserClientFromIdentity(ctx, ident, b.cfg.AuthServer)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		defer impClient.Close()

		route, err := b.getRouteToDatabase(ctx, impClient, destCfg.Database)
		if err != nil {
			return nil, trace.Wrap(err)
		}

		// The impersonated identity is not allowed to reissue certificates,
		// so we'll request the database access identity using the main bot
		// identity (having gathered the necessary info for RouteToDatabase
		// using the correct impersonated ident.)
		newIdent, err := b.generateIdentity(ctx, ident, expires, destCfg, defaultRoles, func(req *proto.UserCertsRequest) {
			req.RouteToDatabase = route
		})

		b.log.Infof("Generated identity for database %q", destCfg.Database.Service)

		return newIdent, trace.Wrap(err)
	}

	return ident, nil
}

func (b *Bot) getIdentityFromToken() (*identity.Identity, error) {
	if b.cfg.Onboarding == nil {
		return nil, trace.BadParameter("onboarding config required via CLI or YAML")
	}
	if b.cfg.Onboarding.Token == "" {
		return nil, trace.BadParameter("unable to start: no token present")
	}
	addr, err := utils.ParseAddr(b.cfg.AuthServer)
	if err != nil {
		return nil, trace.WrapWithMessage(err, "invalid auth server address %+v", b.cfg.AuthServer)
	}

	tlsPrivateKey, sshPublicKey, tlsPublicKey, err := generateKeys()
	if err != nil {
		return nil, trace.WrapWithMessage(err, "unable to generate new keypairs")
	}

	b.log.Info("Attempting to generate new identity from token")
	params := auth.RegisterParams{
		Token: b.cfg.Onboarding.Token,
		ID: auth.IdentityID{
			Role: types.RoleBot,
		},
		Servers:            []utils.NetAddr{*addr},
		PublicTLSKey:       tlsPublicKey,
		PublicSSHKey:       sshPublicKey,
		CAPins:             b.cfg.Onboarding.CAPins,
		CAPath:             b.cfg.Onboarding.CAPath,
		GetHostCredentials: client.HostCredentials,
		JoinMethod:         b.cfg.Onboarding.JoinMethod,
	}
	certs, err := auth.Register(params)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	sha := sha256.Sum256([]byte(params.Token))
	tokenHash := hex.EncodeToString(sha[:])
	ident, err := identity.ReadIdentityFromStore(&identity.LoadIdentityParams{
		PrivateKeyBytes: tlsPrivateKey,
		PublicKeyBytes:  sshPublicKey,
		TokenHashBytes:  []byte(tokenHash),
	}, certs, identity.BotKinds()...)
	return ident, trace.Wrap(err)
}

func (b *Bot) renewIdentityViaAuth(
	ctx context.Context,
) (*identity.Identity, error) {
	// If using the IAM join method we always go through the initial join flow
	// and fetch new nonrenewable certs
	var joinMethod types.JoinMethod
	if b.cfg.Onboarding != nil {
		joinMethod = b.cfg.Onboarding.JoinMethod
	}
	switch joinMethod {
	case types.JoinMethodIAM:
		ident, err := b.getIdentityFromToken()
		return ident, trace.Wrap(err)
	default:
	}

	// Ask the auth server to generate a new set of certs with a new
	// expiration date.
	ident := b.ident()
	certs, err := b.client().GenerateUserCerts(ctx, proto.UserCertsRequest{
		PublicKey: ident.PublicKeyBytes,
		Username:  ident.X509Cert.Subject.CommonName,
		Expires:   time.Now().Add(b.cfg.CertificateTTL),
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newIdentity, err := identity.ReadIdentityFromStore(
		ident.Params(),
		certs,
		identity.BotKinds()...,
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return newIdentity, nil
}

// fetchDefaultRoles requests the bot's own role from the auth server and
// extracts its full list of allowed roles.
func fetchDefaultRoles(ctx context.Context, roleGetter services.RoleGetter, botRole string) ([]string, error) {
	role, err := roleGetter.GetRole(ctx, botRole)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	conditions := role.GetImpersonateConditions(types.Allow)
	return conditions.Roles, nil
}

// renew performs a single renewal
func (b *Bot) renew(
	ctx context.Context, botDestination destination.Destination,
) error {
	// Make sure we can still write to the bot's destination.
	if err := identity.VerifyWrite(botDestination); err != nil {
		return trace.Wrap(err, "Cannot write to destination %s, aborting.", botDestination)
	}

	b.log.Debug("Attempting to renew bot certificates...")
	newIdentity, err := b.renewIdentityViaAuth(ctx)
	if err != nil {
		return trace.Wrap(err)
	}

	identStr, err := describeTLSIdentity(b.ident())
	if err != nil {
		return trace.Wrap(err, "Could not describe bot identity at %s", botDestination)
	}

	b.log.Infof("Successfully renewed bot certificates, %s", identStr)

	// TODO: warn if duration < certTTL? would indicate TTL > server's max renewable cert TTL
	// TODO: error if duration < renewalInterval? next renewal attempt will fail

	// Immediately attempt to reconnect using the new identity (still
	// haven't persisted the known-good certs).
	newClient, err := b.authenticatedUserClientFromIdentity(
		ctx, newIdentity, b.cfg.AuthServer,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	b.setClient(newClient)
	b.setIdent(newIdentity)
	b.log.Debug("Auth client now using renewed credentials.")

	// Now that we're sure the new creds work, persist them.
	if err := identity.SaveIdentity(newIdentity, botDestination, identity.BotKinds()...); err != nil {
		return trace.Wrap(err)
	}

	// Determine the default role list based on the bot role. The role's
	// name should match the certificate's Key ID (user and role names
	// should all match bot-$name)
	botResourceName := newIdentity.X509Cert.Subject.CommonName
	defaultRoles, err := fetchDefaultRoles(ctx, newClient, botResourceName)
	if err != nil {
		b.log.WithError(err).Warnf("Unable to determine default roles, no roles will be requested if unspecified")
		defaultRoles = []string{}
	}

	// Next, generate impersonated certs
	expires := newIdentity.X509Cert.NotAfter
	for _, dest := range b.cfg.Destinations {
		destImpl, err := dest.GetDestination()
		if err != nil {
			return trace.Wrap(err)
		}

		// Check the ACLs. We can't fix them, but we can warn if they're
		// misconfigured. We'll need to precompute a list of keys to check.
		// Note: This may only log a warning, depending on configuration.
		if err := destImpl.Verify(identity.ListKeys(identity.DestinationKinds()...)); err != nil {
			return trace.Wrap(err)
		}

		// Ensure this destination is also writable. This is a hard fail if
		// ACLs are misconfigured, regardless of configuration.
		// TODO: consider not making these a hard error? e.g. write other
		// destinations even if this one is broken?
		if err := identity.VerifyWrite(destImpl); err != nil {
			return trace.Wrap(err, "Could not write to destination %s, aborting.", destImpl)
		}

		impersonatedIdent, err := b.generateImpersonatedIdentity(ctx, expires, dest, defaultRoles)
		if err != nil {
			return trace.Wrap(err, "Failed to generate impersonated certs for %s: %+v", destImpl, err)
		}

		impersonatedIdentStr, err := describeTLSIdentity(impersonatedIdent)
		if err != nil {
			return trace.Wrap(err, "could not describe impersonated certs for destination %s", destImpl)
		}

		b.log.Infof("Successfully renewed impersonated certificates for %s, %s", destImpl, impersonatedIdentStr)

		if err := identity.SaveIdentity(impersonatedIdent, destImpl, identity.DestinationKinds()...); err != nil {
			return trace.Wrap(err, "failed to save impersonated identity to destination %s", destImpl)
		}

		for _, templateConfig := range dest.Configs {
			template, err := templateConfig.GetConfigTemplate()
			if err != nil {
				return trace.Wrap(err)
			}

			if err := template.Render(ctx, b.client(), impersonatedIdent, dest); err != nil {
				b.log.WithError(err).Warnf("Failed to render config template %+v", templateConfig)
			}
		}
	}

	b.log.Infof("Persisted new certificates to disk. Next renewal in approximately %s", b.cfg.RenewalInterval)
	return nil
}

const renewalRetryLimit = 5

func (b *Bot) renewLoop(ctx context.Context) error {
	// TODO: what should this interval be? should it be user configurable?
	// Also, must be < the validity period.
	// TODO: validate that cert is actually renewable.

	b.log.Infof(
		"Beginning renewal loop: ttl=%s interval=%s",
		b.cfg.CertificateTTL,
		b.cfg.RenewalInterval,
	)
	if b.cfg.RenewalInterval > b.cfg.CertificateTTL {
		b.log.Errorf(
			"Certificate TTL (%s) is shorter than the renewal interval (%s). The next renewal is likely to fail.",
			b.cfg.CertificateTTL,
			b.cfg.RenewalInterval,
		)
	}

	// Determine where the bot should write its internal data (renewable cert
	// etc)
	botDestination, err := b.cfg.Storage.GetDestination()
	if err != nil {
		return trace.Wrap(err)
	}

	ticker := time.NewTicker(b.cfg.RenewalInterval)
	jitter := libUtils.NewJitter()
	defer ticker.Stop()
	for {
		var err error
		for attempt := 1; attempt <= renewalRetryLimit; attempt++ {
			err = b.renew(ctx, botDestination)
			if err == nil {
				break
			}

			if attempt != renewalRetryLimit {
				// exponentially back off with jitter, starting at 1 second.
				backoffTime := time.Second * time.Duration(math.Pow(2, float64(attempt-1)))
				backoffTime = jitter(backoffTime)
				b.log.WithError(err).Warnf(
					"Renewal attempt %d of %d failed. Retrying after %s.",
					attempt,
					renewalRetryLimit,
					backoffTime,
				)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(backoffTime):
				}
			}
		}
		if err != nil {
			b.log.Warnf("All %d retry attempts exhausted.", renewalRetryLimit)
			return trace.Wrap(err)
		}

		if b.cfg.Oneshot {
			b.log.Info("Oneshot mode enabled, exiting successfully.")
			break
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			continue
		case <-b.reloadChan:
			continue
		}
	}

	return nil
}
