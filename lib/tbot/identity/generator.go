/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
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

package identity

import (
	"cmp"
	"context"
	"crypto"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"go.opentelemetry.io/otel"
	"golang.org/x/crypto/ssh"
	"google.golang.org/protobuf/types/known/durationpb"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	delegationv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/delegation/v1"
	issuancev1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/issuance/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/tlsca"
)

var tracer = otel.Tracer("github.com/gravitational/teleport/lib/tbot/identity")

// GeneratorConfig contains the configuration options for a Generator.
//
// TODO(strideynet): There's some general tidy-up work to be done here with how
// we handle CAs to make it more uniform and clearly sourced
type GeneratorConfig struct {
	// Client that will be used to request identity certificates.
	Client *apiclient.Client

	// Logger to which errors and messages will be written. Can be overridden
	// on a per-call basis by passing WithLogger to Generate.
	Logger *slog.Logger

	// BotIdentity is a Facade containing the bot's internal identity.
	BotIdentity *Facade

	// FIPS controls whether FIPS mode is enabled.
	FIPS bool

	// Insecure controls whether the generated identity TLS config verifies
	// host certificate authenticity, etc.
	Insecure bool

	// Scoped indicates whether the bot is running in scoped mode, and so holds
	// a scoped identity. Certificates for a scoped bot are issued via the
	// scoped issuance RPC rather than by role impersonation.
	Scoped bool
}

// CheckAndSetDefaults checks whether the configuration is valid and sets any
// default values.
func (cfg *GeneratorConfig) CheckAndSetDefaults() error {
	switch {
	case cfg.Client == nil:
		return trace.BadParameter("Client is required")
	case cfg.BotIdentity == nil:
		return trace.BadParameter("BotIdentity is required")
	}

	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}

	return nil
}

// NewGenerator creates a new Generator with the given configuration.
func NewGenerator(cfg GeneratorConfig) (*Generator, error) {
	if err := cfg.CheckAndSetDefaults(); err != nil {
		return nil, err
	}
	return &Generator{
		client:      cfg.Client,
		logger:      cfg.Logger,
		botIdentity: cfg.BotIdentity,
		fips:        cfg.FIPS,
		insecure:    cfg.Insecure,
		scoped:      cfg.Scoped,
	}, nil
}

// Generator can be used by tbot's services to generate non-renewable identities
// scoped to the requested roles, TTL, etc.
type Generator struct {
	client         *apiclient.Client
	logger         *slog.Logger
	botIdentity    *Facade
	fips, insecure bool
	scoped         bool
}

type generateOpts struct {
	delegationSessionID  string
	roles                []string
	ttl, renewalInterval time.Duration
	currentIdentity      *Identity
	logger               *slog.Logger
	privateKey           crypto.Signer
	routeToApp           *proto.RouteToApp
	routeToDatabase      *proto.RouteToDatabase
	kubernetesCluster    *string
	routeToCluster       *string
	reissuable           bool
}

// GenerateOption allows you to customize aspects of the generated identity.
type GenerateOption func(*generateOpts)

// WithDelegation uses the given delegation session ID to generate certificates
// associated with a *human* user and delegation session.
//
// Note: this option is mutually-exclusive with WithRoles.
// Scoped bots do not support delegation.
func WithDelegation(sessionID string) GenerateOption {
	return func(opts *generateOpts) {
		opts.delegationSessionID = sessionID
	}
}

// WithRoles sets the roles the generated identity should include.
//
// Generally, if the user did not specify any roles, it's best to leave this
// empty and rely on the default behavior (of fetching all the bot's available
// roles). If WithCurrentIdentity is provided, we'll default to using the roles
// in its TLS certificate to avoid re-fetching them.
func WithRoles(roles []string) GenerateOption {
	return func(opts *generateOpts) {
		opts.roles = roles
	}
}

// WithLifetime sets the requested time-to-live of the certificate, along with
// a hint of how frequently it will be renewed - the latter is used for logging
// purposes only.
func WithLifetime(ttl, renewalInterval time.Duration) GenerateOption {
	return func(opts *generateOpts) {
		opts.ttl = ttl
		opts.renewalInterval = renewalInterval
	}
}

// WithCurrentIdentity sets the identity on which the generated identity will be
// based. This largely just affects the default roles and cluster name.
//
// If you do not provide WithCurrentIdentity, the bot's internal identity will
// be used. Note: you should *not* explicitly pass the bot's internal identity.
// Scoped bots do not support generating from another identity.
func WithCurrentIdentity(identity *Identity) GenerateOption {
	return func(opts *generateOpts) {
		opts.currentIdentity = identity
	}
}

// WithCurrentIdentityFacade is a variant of WithCurrentIdentity which allows
// you to pass a Facade for convenience.
func WithCurrentIdentityFacade(facade *Facade) GenerateOption {
	return func(opts *generateOpts) {
		opts.currentIdentity = facade.Get()
	}
}

// WithLogger allows you to override the logger.
func WithLogger(logger *slog.Logger) GenerateOption {
	return func(opts *generateOpts) {
		opts.logger = logger
	}
}

// WithKubernetesCluster sets the KubernetesCluster field on the certificates
// request.
// Scoped bots do not support Kubernetes routing.
func WithKubernetesCluster(name string) GenerateOption {
	return func(opts *generateOpts) {
		opts.kubernetesCluster = &name
	}
}

// WithRouteToApp sets the RouteToApp field on the certificates request.
// Scoped bots support only the app name, public address, scope and local cluster.
func WithRouteToApp(route proto.RouteToApp) GenerateOption {
	return func(opts *generateOpts) {
		opts.routeToApp = &route
	}
}

// WithRouteToDatabase sets the RouteToDatabase field on the certificates
// request.
// Scoped bots do not support database routing.
func WithRouteToDatabase(route proto.RouteToDatabase) GenerateOption {
	return func(opts *generateOpts) {
		opts.routeToDatabase = &route
	}
}

// WithReissuableRoleImpersonation sets the WithReissuableRoleImpersonation
// field on the certificates request.
// Scoped bots do not support reissuable identities.
func WithReissuableRoleImpersonation(allow bool) GenerateOption {
	return func(opts *generateOpts) {
		opts.reissuable = allow
	}
}

// WithRouteToCluster sets the RouteToCluster field on the certificates request.
// Scoped bots do not support routing to another cluster.
func WithRouteToCluster(cluster string) GenerateOption {
	return func(opts *generateOpts) {
		opts.routeToCluster = &cluster
	}
}

// WithPrivateKey overrides the private key that will be used.
//
// In most cases, you should NOT provide this option, and allow the function to
// use a throwaway key for each identity.
func WithPrivateKey(key crypto.Signer) GenerateOption {
	return func(opts *generateOpts) {
		opts.privateKey = key
	}
}

// GenerateFacade calls Generate and wraps the resulting Identity in a Facade
// for easy use in API clients, etc.
func (g *Generator) GenerateFacade(ctx context.Context, opts ...GenerateOption) (*Facade, error) {
	id, err := g.Generate(ctx, opts...)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return NewFacade(g.fips, g.insecure, id), nil
}

// Generate a non-renewable identity with the given roles, TTL, etc.
func (g *Generator) Generate(ctx context.Context, opts ...GenerateOption) (*Identity, error) {
	ctx, span := tracer.Start(ctx, "Generator/Generate")
	defer span.End()

	o := &generateOpts{}
	for _, fn := range opts {
		fn(o)
	}

	log := cmp.Or(o.logger, g.logger)

	if len(o.roles) != 0 && o.delegationSessionID != "" {
		return nil, trace.BadParameter("delegation sessions and explicit roles are mutually-exclusive")
	}

	// Validate scoped options before resolving roles, substituting the bot
	// identity, or generating keys.
	var scopedReq *issuancev1pb.IssueScopedBotCertsRequest
	if g.scoped {
		var err error
		scopedReq, err = o.scopedCertsRequest(g.botIdentity.Get().ClusterName)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	// Scoped certificates are not issued by role impersonation.
	if !g.scoped && len(o.roles) == 0 {
		if o.currentIdentity != nil {
			// If the caller provided an impersonated identity, take its roles.
			o.roles = o.currentIdentity.TLSIdentity.Groups
		} else {
			// Otherwise, fetch the bot identity's default roles.
			var err error
			if o.roles, err = g.botDefaultRoles(ctx); err != nil {
				return nil, trace.Wrap(err, "fetching default roles")
			}
			log.DebugContext(ctx, "No roles configured, using all roles available.", "roles", o.roles)
		}
	}

	if o.currentIdentity == nil {
		o.currentIdentity = g.botIdentity.Get()
	}

	req := proto.UserCertsRequest{
		Username:       o.currentIdentity.X509Cert.Subject.CommonName,
		Expires:        time.Now().Add(o.ttl),
		RoleRequests:   o.roles,
		RouteToCluster: o.currentIdentity.ClusterName,

		// Make sure to specify this is an impersonated cert request. If unset,
		// auth cannot differentiate renewable vs impersonated requests when
		// len(roleRequests) == 0.
		UseRoleRequests:             true,
		ReissuableRoleImpersonation: o.reissuable,
	}

	if o.routeToApp != nil {
		req.RouteToApp = *o.routeToApp
	}
	if o.routeToDatabase != nil {
		req.RouteToDatabase = *o.routeToDatabase
	}
	if o.kubernetesCluster != nil {
		req.KubernetesCluster = *o.kubernetesCluster
	}
	if o.routeToCluster != nil {
		req.RouteToCluster = *o.routeToCluster
	}

	keyPurpose := cryptosuites.BotImpersonatedIdentity
	if req.RouteToDatabase.ServiceName != "" {
		// We still used RSA for all database clients, all other bot
		// impersonated identities can use ECDSA.
		keyPurpose = cryptosuites.DatabaseClient
	}

	if o.privateKey == nil {
		// Generate a fresh keypair for the impersonated identity. We generally
		// don't care to reuse keys here, and instead constantly rotate private
		// keys to limit their effective lifetime.
		//
		// In the beams/vnet service, the private key remains in-memory only and
		// the benefit of rotating it is minimal. It's also convenient to reuse
		// the key because VNet splits client certificate issuance and signing
		// into separate RPCs, so we'd need to store and correlate keys to their
		// respective applications.
		var err error
		o.privateKey, err = cryptosuites.GenerateKey(ctx,
			cryptosuites.GetCurrentSuiteFromAuthPreference(g.client),
			keyPurpose)
		if err != nil {
			return nil, trace.Wrap(err)
		}
	}

	sshPub, err := ssh.NewPublicKey(o.privateKey.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}
	req.SSHPublicKey = ssh.MarshalAuthorizedKey(sshPub)

	req.TLSPublicKey, err = keys.MarshalPublicKey(o.privateKey.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	var certs *proto.Certs
	switch {
	case g.scoped:
		// Scoped bot certificates, issued via the scoped issuance service.
		scopedReq.SetSshPublicKey(req.SSHPublicKey)
		scopedReq.SetTlsPublicKey(req.TLSPublicKey)
		certs, err = g.generateScopedCertificates(ctx, scopedReq)
	case o.delegationSessionID != "":
		// Delegation certificates, tied to a human user identity.
		certs, err = g.generateDelegationCertificates(ctx, req, o)
	default:
		// Traditional bot certificates.
		certs, err = g.client.GenerateUserCerts(ctx, req)
	}
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// The root CA included with the returned certs will only contain the
	// Teleport User CA (and, for scoped certificates, nothing at all - see
	// generateScopedCertificates). We'll also need the host CA for future API
	// calls.
	localCA, err := g.client.GetClusterCACert(ctx)
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
	certs.SSHCACerts = o.currentIdentity.SSHCACertBytes

	privateKeyPEM, err := keys.MarshalPrivateKey(o.privateKey)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	newIdentity, err := ReadIdentityFromStore(&LoadIdentityParams{
		PrivateKeyBytes: privateKeyPEM,
		PublicKeyBytes:  req.SSHPublicKey,
	}, certs)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	warnOnEarlyExpiration(
		ctx,
		log,
		newIdentity,
		o.ttl,
		o.renewalInterval,
	)

	return newIdentity, nil
}

func (g *Generator) botDefaultRoles(ctx context.Context) ([]string, error) {
	role, err := g.client.GetRole(ctx, g.botIdentity.Get().X509Cert.Subject.CommonName)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	conditions := role.GetImpersonateConditions(types.Allow)
	return conditions.Roles, nil
}

// scopedCertsRequest validates options supported by scoped issuance and builds
// the request without public keys, which are added after key generation.
func (o *generateOpts) scopedCertsRequest(clusterName string) (*issuancev1pb.IssueScopedBotCertsRequest, error) {
	switch {
	case o.delegationSessionID != "":
		return nil, trace.BadParameter("delegation sessions are not supported for scoped bots")
	case o.currentIdentity != nil:
		return nil, trace.BadParameter("current identity is not supported for scoped bots")
	case o.reissuable:
		return nil, trace.BadParameter("reissuable role impersonation is not supported for scoped bots")
	case o.routeToCluster != nil && *o.routeToCluster != "" && *o.routeToCluster != clusterName:
		return nil, trace.BadParameter("leaf cluster routing is not supported for scoped bots")
	case o.routeToDatabase != nil:
		return nil, trace.BadParameter("database routing is not supported for scoped bots")
	case o.kubernetesCluster != nil:
		return nil, trace.BadParameter("kubernetes routing is not supported for scoped bots")
	}

	req := issuancev1pb.IssueScopedBotCertsRequest_builder{
		Ttl: durationpb.New(o.ttl),
	}.Build()
	if o.routeToApp == nil {
		req.SetIdentity(&issuancev1pb.UsageIdentity{})
		return req, nil
	}

	app := o.routeToApp
	if app.ClusterName != "" && app.ClusterName != clusterName {
		return nil, trace.BadParameter("routing to an app in another cluster is not supported for scoped bots")
	}
	// These fields cannot be expressed by the scoped issuance RPC and must not
	// be silently discarded. Update this check when RouteToApp gains fields.
	if app.AWSRoleARN != "" ||
		app.AzureIdentity != "" ||
		app.GCPServiceAccount != "" ||
		app.AWSCredentialProcessCredentials != "" ||
		app.URI != "" ||
		app.TargetPort != 0 {
		return nil, trace.BadParameter("unsupported app routing fields for scoped bots")
	}
	req.SetApp(issuancev1pb.UsageApp_builder{
		Name:       app.Name,
		PublicAddr: app.PublicAddr,
		Scope:      app.Scope,
	}.Build())
	return req, nil
}

// generateScopedCertificates issues certificates via the scoped issuance
// service. The bot must already hold a scoped identity.
func (g *Generator) generateScopedCertificates(
	ctx context.Context, req *issuancev1pb.IssueScopedBotCertsRequest,
) (*proto.Certs, error) {
	res, err := g.client.IssuanceClient().IssueScopedBotCerts(ctx, req)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// Scoped issuance returns no CAs. Generate adds the host TLS CAs and
	// copies the SSH CAs from the current identity.
	return &proto.Certs{
		SSH: res.GetCerts().GetSsh(),
		TLS: res.GetCerts().GetTls(),
	}, nil
}

func (g *Generator) generateDelegationCertificates(ctx context.Context, req proto.UserCertsRequest, o *generateOpts) (*proto.Certs, error) {
	certReq := &delegationv1.GenerateCertsRequest{
		DelegationSessionId: o.delegationSessionID,
		SshPublicKey:        req.SSHPublicKey,
		TlsPublicKey:        req.TLSPublicKey,
		Ttl:                 durationpb.New(o.ttl),
	}
	if req.GetRouteToCluster() != o.currentIdentity.ClusterName {
		return nil, trace.BadParameter("delegation sessions cannot be used with leaf clusters")
	}
	switch {
	case req.GetRouteToApp().Name != "":
		route := req.GetRouteToApp()
		certReq.Routing = &delegationv1.GenerateCertsRequest_RouteToApp{
			RouteToApp: &delegationv1.RouteToApp{
				Name:              route.GetName(),
				PublicAddr:        route.GetPublicAddr(),
				ClusterName:       route.GetClusterName(),
				Uri:               route.GetURI(),
				TargetPort:        route.GetTargetPort(),
				AwsRoleArn:        route.GetAWSRoleARN(),
				AzureIdentity:     route.GetAzureIdentity(),
				GcpServiceAccount: route.GetGCPServiceAccount(),
			},
		}
	case req.GetRouteToDatabase().ServiceName != "":
		route := req.GetRouteToDatabase()
		certReq.Routing = &delegationv1.GenerateCertsRequest_RouteToDatabase{
			RouteToDatabase: &delegationv1.RouteToDatabase{
				ServiceName: route.GetServiceName(),
				Protocol:    route.GetProtocol(),
				Username:    route.GetUsername(),
				Database:    route.GetDatabase(),
				Roles:       route.GetRoles(),
			},
		}
	case req.GetKubernetesCluster() != "":
		certReq.Routing = &delegationv1.GenerateCertsRequest_RouteToKubernetes{
			RouteToKubernetes: &delegationv1.RouteToKubernetes{
				ClusterName: req.GetKubernetesCluster(),
			},
		}
	}
	certsRsp, err := g.client.DelegationSessionServiceClient().
		GenerateCerts(ctx, certReq)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	return &proto.Certs{
		SSH:        certsRsp.GetSsh(),
		TLS:        certsRsp.GetTls(),
		SSHCACerts: o.currentIdentity.SSHCACertBytes,
		TLSCACerts: o.currentIdentity.TLSCACertsBytes,
	}, nil
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
	ident *Identity,
	ttl, renewalInterval time.Duration,
) {
	// Calculate a rough TTL, assuming this was called shortly after the
	// identity was returned. We'll add a minute buffer to compensate and avoid
	// superfluous warning messages.
	effectiveTTL := time.Until(ident.TLSIdentity.Expires) + time.Minute

	if effectiveTTL < ttl {
		l := log.With(
			"requested_ttl", ttl,
			"renewal_interval", renewalInterval,
			"effective_ttl", effectiveTTL,
			"expires", ident.TLSIdentity.Expires,
			"roles", ident.TLSIdentity.Groups,
		)

		// TODO(timothyb89): we can technically fetch our individual roles
		// without explicit permission, and could determine which role in
		// particular limited the TTL.

		if effectiveTTL < renewalInterval {
			//nolint:sloglint // multiline string is actually constant
			l.WarnContext(ctx, "The server returned an identity shorter than "+
				"expected and below the configured renewal interval, probably "+
				"due to a `max_session_ttl` configured on a server-side role. "+
				"Unless corrected, the credentials will be invalid for some "+
				"period until renewal.")
		} else {
			//nolint:sloglint // multiline string is actually constant
			l.WarnContext(ctx, "The server returned an identity shorter than "+
				"the requested TTL, probably due to a `max_session_ttl` "+
				"configured on a server-side role. It may not remain valid as "+
				"long as expected.")
		}
	}
}
