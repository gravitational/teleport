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
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"

	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/constants"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/auth/native"
	libdefaults "github.com/gravitational/teleport/lib/defaults"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tbot/config"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tlsca"
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

// generateIdentity uses an identity to retrieve an impersonated identity.
// The `configurator` function, if not nil, can be used to add additional
// requests to the certificate request, for example to add `RouteToDatabase`
// and similar fields, however in that case it must be called with an
// impersonated identity that already has the relevant permissions, much like
// `tsh (app|db|kube) login` is already used to generate an additional set of
// certs.
func (b *Bot) generateIdentity(
	ctx context.Context,
	currentIdentity *identity.Identity,
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
		Expires:        time.Now().Add(b.cfg.CertificateTTL),
		RoleRequests:   roleRequests,
		RouteToCluster: currentIdentity.ClusterName,

		// Make sure to specify this is an impersonated cert request. If unset,
		// auth cannot differentiate renewable vs impersonated requests when
		// len(roleRequests) == 0.
		UseRoleRequests: true,
	}

	if configurator != nil {
		configurator(&req)
	}

	// First, ask the auth server to generate a new set of certs with a new
	// expiration date.
	client := b.Client()
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

func (b *Bot) getRouteToDatabase(ctx context.Context, client auth.ClientI, dbCfg *config.Database) (proto.RouteToDatabase, error) {
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

func getApp(ctx context.Context, client auth.ClientI, appName string) (types.Application, error) {
	res, err := client.ListResources(ctx, proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindAppServer,
		PredicateExpression: fmt.Sprintf(`name == "%s"`, appName),
		Limit:               1,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	servers, err := types.ResourcesWithLabels(res.Resources).AsAppServers()
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var apps []types.Application
	for _, server := range servers {
		apps = append(apps, server.GetApp())
	}
	apps = types.DeduplicateApps(apps)

	if len(apps) == 0 {
		return nil, trace.BadParameter("app %q not found", appName)
	}

	return apps[0], nil
}

func (b *Bot) getRouteToApp(ctx context.Context, client auth.ClientI, appCfg *config.App) (proto.RouteToApp, error) {
	if appCfg.App == "" {
		return proto.RouteToApp{}, trace.BadParameter("App name must be configured")
	}

	app, err := getApp(ctx, client, appCfg.App)
	if err != nil {
		return proto.RouteToApp{}, trace.Wrap(err)
	}

	// TODO: AWS?
	ident := b.ident()
	ws, err := client.CreateAppSession(ctx, types.CreateAppSessionRequest{
		ClusterName: ident.ClusterName,
		Username:    ident.X509Cert.Subject.CommonName,
		PublicAddr:  app.GetPublicAddr(),
	})
	if err != nil {
		return proto.RouteToApp{}, trace.Wrap(err)
	}

	err = auth.WaitForAppSession(ctx, ws.GetName(), ws.GetUser(), client)
	if err != nil {
		return proto.RouteToApp{}, trace.Wrap(err)
	}

	return proto.RouteToApp{
		Name:        app.GetName(),
		SessionID:   ws.GetName(),
		PublicAddr:  app.GetPublicAddr(),
		ClusterName: ident.ClusterName,
	}, nil
}

// generateImpersonatedIdentity generates an impersonated identity for a given
// destination.
//
// It returns two identities:
// - unroutedIdentity: impersonates the roles of the destination, but does not
// include any routing specified within the destination. This gives an
// identity which can be used to act as the roleset when interacting with the
// Teleport API.
// - routedIdentity: impersonates the roles and routes of the destination.
// This identity should be the one actually written to the destination, but,
// may not behave as expected when used to interact with the Teleport API
func (b *Bot) generateImpersonatedIdentity(
	ctx context.Context,
	destCfg *config.DestinationConfig,
	defaultRoles []string,
) (routedIdentity *identity.Identity, unroutedIdentity *identity.Identity, err error) {
	unroutedIdentity, err = b.generateIdentity(
		ctx, b.ident(), destCfg, defaultRoles, nil,
	)
	if err != nil {
		return nil, nil, trace.Wrap(err)
	}

	// Now that we have an initial impersonated identity, we can use it to
	// request any app/db/etc certs
	if destCfg.Database != nil {
		impClient, err := b.AuthenticatedUserClientFromIdentity(ctx, unroutedIdentity)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		defer impClient.Close()

		route, err := b.getRouteToDatabase(ctx, impClient, destCfg.Database)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		// The impersonated identity is not allowed to reissue certificates,
		// so we'll request the database access identity using the main bot
		// identity (having gathered the necessary info for RouteToDatabase
		// using the correct impersonated unroutedIdentity.)
		routedIdentity, err := b.generateIdentity(ctx, unroutedIdentity, destCfg, defaultRoles, func(req *proto.UserCertsRequest) {
			req.RouteToDatabase = route
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		b.log.Infof("Generated identity for database %q", destCfg.Database.Service)

		return routedIdentity, unroutedIdentity, nil
	} else if destCfg.KubernetesCluster != nil {
		// Note: the Teleport server does attempt to verify k8s cluster names
		// and will fail to generate certs if the cluster doesn't exist or is
		// offline.
		routedIdentity, err := b.generateIdentity(ctx, unroutedIdentity, destCfg, defaultRoles, func(req *proto.UserCertsRequest) {
			req.KubernetesCluster = destCfg.KubernetesCluster.ClusterName
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		b.log.Infof("Generated identity for Kubernetes cluster %q", *destCfg.KubernetesCluster)

		return routedIdentity, unroutedIdentity, nil
	} else if destCfg.App != nil {
		impClient, err := b.AuthenticatedUserClientFromIdentity(ctx, unroutedIdentity)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		defer impClient.Close()

		routeToApp, err := b.getRouteToApp(ctx, impClient, destCfg.App)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		routedIdentity, err := b.generateIdentity(ctx, unroutedIdentity, destCfg, defaultRoles, func(req *proto.UserCertsRequest) {
			req.RouteToApp = routeToApp
		})
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}

		b.log.Infof("Generated identity for app %q", *destCfg.App)

		return routedIdentity, unroutedIdentity, nil
	} else if destCfg.Cluster != "" {
		routedIdentity, err := b.generateIdentity(
			ctx, unroutedIdentity, destCfg, defaultRoles, func(req *proto.UserCertsRequest) {
				req.RouteToCluster = destCfg.Cluster
			},
		)
		if err != nil {
			return nil, nil, trace.Wrap(err)
		}
		return routedIdentity, unroutedIdentity, nil
	}

	return unroutedIdentity, unroutedIdentity, nil
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

// renewDestinations performs a single renewal
func (b *Bot) renewDestinations(
	ctx context.Context,
) error {
	// Determine the default role list based on the bot role. The role's
	// name should match the certificate's Key ID (user and role names
	// should all match bot-$name)
	botResourceName := b.ident().X509Cert.Subject.CommonName
	defaultRoles, err := fetchDefaultRoles(ctx, b.Client(), botResourceName)
	if err != nil {
		b.log.WithError(err).Warnf("Unable to determine default roles, no roles will be requested if unspecified")
		defaultRoles = []string{}
	}

	// Next, generate impersonated certs
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

		routedIdentity, unroutedIdentity, err := b.generateImpersonatedIdentity(ctx, dest, defaultRoles)
		if err != nil {
			return trace.Wrap(err, "Failed to generate impersonated certs for %s: %+v", destImpl, err)
		}

		impersonatedIdentStr, err := describeTLSIdentity(routedIdentity)
		if err != nil {
			return trace.Wrap(err, "could not describe impersonated certs for destination %s", destImpl)
		}

		b.log.Infof("Renewed destination certificates for %s, %s", destImpl, impersonatedIdentStr)

		if err := identity.SaveIdentity(routedIdentity, destImpl, identity.DestinationKinds()...); err != nil {
			return trace.Wrap(err, "failed to save impersonated identity to destination %s", destImpl)
		}

		for _, templateConfig := range dest.Configs {
			template, err := templateConfig.GetConfigTemplate()
			if err != nil {
				return trace.Wrap(err)
			}

			if err := template.Render(ctx, b, routedIdentity, unroutedIdentity, dest); err != nil {
				b.log.WithError(err).Warnf("Failed to render config template %+v", templateConfig)
				return trace.Wrap(err)
			}
		}
	}

	// Purge the CA cache. We could be smarter about this in the future if
	// desired, since generally CAs don't change that often.
	b.clearCertAuthorities()
	return nil
}

const renewalRetryLimit = 5

func (b *Bot) renewDestinationsLoop(
	ctx context.Context, reloadChan <-chan struct{},
) error {
	b.log.Infof(
		"Beginning destination renewal loop: ttl=%s interval=%s",
		b.cfg.CertificateTTL,
		b.cfg.RenewalInterval,
	)

	ticker := time.NewTicker(b.cfg.RenewalInterval)
	jitter := retryutils.NewJitter()
	defer ticker.Stop()
	for {
		var err error
		for attempt := 1; attempt <= renewalRetryLimit; attempt++ {
			b.log.Infof(
				"Renewing destinations. Attempt %d of %d.",
				attempt,
				renewalRetryLimit,
			)
			err = b.renewDestinations(ctx)
			if err == nil {
				break
			}

			if attempt != renewalRetryLimit {
				// exponentially back off with jitter, starting at 1 second.
				backoffTime := time.Second * time.Duration(math.Pow(2, float64(attempt-1)))
				backoffTime = jitter(backoffTime)
				b.log.WithError(err).Warnf(
					"Destination renewal attempt %d of %d failed. Retrying after %s.",
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
			b.log.Warnf("%d retry attempts exhausted. Waiting for next normal renewal cycle in %s.", renewalRetryLimit, b.cfg.RenewalInterval)
			return trace.Wrap(err)
		} else {
			b.log.Infof("Renewed destinations. Next destination renewal in approximately %s.", b.cfg.RenewalInterval)
		}

		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			continue
		case <-reloadChan:
			continue
		}
	}
}
