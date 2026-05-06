// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package beams

import (
	"cmp"
	"context"
	"crypto"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gravitational/trace"

	"github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	typesvnet "github.com/gravitational/teleport/api/types/vnet"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	tbotclient "github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/readyz"
	"github.com/gravitational/teleport/lib/tbot/services/clientcredentials"
	"github.com/gravitational/teleport/lib/utils"
	"github.com/gravitational/teleport/lib/vnet"
	"github.com/gravitational/teleport/lib/vnet/dns"
)

// VNetServiceBuilder returns a builder for the Beams VNet service.
func VNetServiceBuilder(cfg *VNetServiceConfig, opts ...VNetServiceOpt) bot.ServiceBuilder {
	buildFn := func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(deps.Scoped); err != nil {
			return nil, trace.Wrap(err)
		}

		// Prevent accidental misuse.
		if os.Getenv("TELEPORT_BEAMS_RUNTIME") != "yes" {
			return nil, trace.Errorf("service type %q is not intended for use outside of Teleport Beams, see: https://beams.run for more information", VNetServiceType)
		}

		svc := &VNetService{
			cfg:                       cfg,
			createTUN:                 platformCreateTUN,
			configureHost:             platformConfigureHost,
			defaultCredentialLifetime: bot.DefaultCredentialLifetime,
			statusReporter:            deps.GetStatusReporter(),
			identityGenerator:         deps.IdentityGenerator,
			clientBuilder:             deps.ClientBuilder,
			proxyPinger:               deps.ProxyPinger,
			logger:                    deps.Logger,
			identity: &clientcredentials.UnstableConfig{
				DelegationSessionID: cfg.DelegationSessionID,
			},
		}
		for _, opt := range opts {
			opt(svc)
		}

		identityBuilder := clientcredentials.ServiceBuilder(
			svc.identity,
			cmp.Or(cfg.CredentialLifetime, svc.defaultCredentialLifetime),
		)
		identitySvc, err := identityBuilder.Build(
			deps.WithStatusReporter(readyz.NoopReporter()),
		)
		if err != nil {
			return nil, trace.Wrap(err)
		}
		return bot.NewServicePair(svc, identitySvc), nil
	}

	return bot.NewServiceBuilder(VNetServiceType, cfg.Name, buildFn)
}

// VNetService runs an embedded implementation of VNet to transparently route
// egress traffic from the Beams Runtime through the Teleport proxy. It creates
// a TUN interface and updates the host's routing table. The Beams orchestrator
// is responsible for updating the /etc/resolv.conf to use the VNet nameserver.
type VNetService struct {
	cfg                       *VNetServiceConfig
	createTUN                 func() (vnet.TUNDevice, error)
	configureHost             func(context.Context, vnet.TUNDevice, *vnet.EmbeddedVNetHostConfig) error
	defaultCredentialLifetime bot.CredentialLifetime
	statusReporter            readyz.Reporter
	identityGenerator         *identity.Generator
	clientBuilder             *tbotclient.Builder
	proxyPinger               connection.ProxyPinger
	logger                    *slog.Logger
	// TODO(boxofrad): Wrapping the clientcredentials service is a little awkward
	// consider if we should move its automatic renewal behavior into the identity
	// package.
	identity *clientcredentials.UnstableConfig
	insecure bool
}

// String satisfies the bot.Service interface.
func (svc *VNetService) String() string { return svc.cfg.Name }

// Run the VNet service until the given context is canceled.
func (s *VNetService) Run(ctx context.Context) error {
	device, err := s.createTUN()
	if err != nil {
		return trace.Wrap(err)
	}
	defer device.Close()

	var upstreamNameservers dns.UpstreamNameserverSource
	if len(s.cfg.UpstreamNameservers) == 0 {
		upstreamNameservers, err = platformUpstreamNameserverSource(s.logger)
		if err != nil {
			return trace.Wrap(err)
		}
	} else {
		upstreamNameservers = s.cfg.UpstreamNameservers
	}

	select {
	case <-s.identity.Ready():
	case <-ctx.Done():
		return ctx.Err()
	}
	facade, err := s.identity.Facade()
	if err != nil {
		return trace.Wrap(err)
	}
	client, err := s.clientBuilder.Build(ctx, facade)
	if err != nil {
		return trace.Wrap(err)
	}
	defer client.Close()

	applicationService, err := newVNetApplicationService(
		client,
		s.identityGenerator,
		facade.Get().PrivateKey.Signer,
		cmp.Or(s.cfg.CredentialLifetime, s.defaultCredentialLifetime),
		s.cfg.DelegationSessionID,
		s.insecure,
		s.logger,
	)
	if err != nil {
		return trace.Wrap(err)
	}

	var configureHostOnce sync.Once
	net, err := vnet.NewEmbeddedVNet(vnet.EmbeddedVNetConfig{
		Device:             device,
		ApplicationService: applicationService,
		ConfigureHost: func(ctx context.Context, cfg *vnet.EmbeddedVNetHostConfig) error {
			// ConfigureHost is called with `cfg == nil` when VNet is shutting
			// down to tear down any host configuration, however we do not need
			// to do anything as removing the TUN device (by calling Close) will
			// drop its associated routes.
			if cfg == nil {
				return nil
			}

			// In the Beams environment, we assume DNS zones and CIDR ranges are
			// stable, so we do not need to make our setup function idempotent.
			var err error
			configureHostOnce.Do(func() {
				err = s.configureHost(ctx, device, cfg)
				if err == nil {
					s.statusReporter.Report(readyz.Healthy)
				} else {
					s.statusReporter.ReportReason(readyz.Unhealthy, err.Error())
				}
			})
			return err
		},
		UpstreamNameserverSource: upstreamNameservers,
	})
	if err != nil {
		return trace.Wrap(err, "creating embedded VNet")
	}
	return trace.Wrap(net.Run(ctx))
}

func newVNetApplicationService(
	client *client.Client,
	identityGenerator *identity.Generator,
	privateKey crypto.Signer,
	credentialLifetime bot.CredentialLifetime,
	delegationSessionID string,
	insecure bool,
	logger *slog.Logger,
) (*vnetApplicationService, error) {
	cache, err := utils.NewFnCache(utils.FnCacheConfig{TTL: 1 * time.Minute})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return &vnetApplicationService{
		client:              client,
		identityGenerator:   identityGenerator,
		privateKey:          privateKey,
		credentialLifetime:  credentialLifetime,
		delegationSessionID: delegationSessionID,
		insecure:            insecure,
		cache:               cache,
		logger:              logger,
	}, nil
}

type vnetApplicationService struct {
	vnet.EmbeddedApplicationService

	client              *client.Client
	cache               *utils.FnCache
	identityGenerator   *identity.Generator
	privateKey          crypto.Signer
	credentialLifetime  bot.CredentialLifetime
	delegationSessionID string
	insecure            bool
	logger              *slog.Logger
}

func (v *vnetApplicationService) GetTargetOSConfiguration(ctx context.Context) (*vnetv1.TargetOSConfiguration, error) {
	cfg, err := v.getOSConfiguration(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return cfg.config, nil
}

func (v *vnetApplicationService) getOSConfiguration(ctx context.Context) (*vnetOSConfiguration, error) {
	uncached := func(ctx context.Context) (*vnetOSConfiguration, error) {
		pong, err := v.client.Ping(ctx)
		if err != nil {
			return nil, trace.Wrap(err, "pinging auth server")
		}

		dnsZones := []string{hostname(pong.GetProxyPublicAddr())}

		vnetConfig, err := v.client.GetVnetConfig(ctx)
		switch {
		case trace.IsNotFound(err) || trace.IsNotImplemented(err):
			// Use the defaults, nothing to do here.
		case err != nil:
			return nil, trace.Wrap(err)
		}

		for _, zone := range vnetConfig.GetSpec().GetCustomDnsZones() {
			dnsZones = append(dnsZones, zone.GetSuffix())
		}

		return &vnetOSConfiguration{
			pong: pong,
			config: &vnetv1.TargetOSConfiguration{
				DnsZones: dnsZones,

				// Note: we do not currently honor custom CIDR ranges, because cloud
				// makes assumptions about the IPv4 address of the nameserver.
				Ipv4CidrRanges: []string{typesvnet.DefaultIPv4CIDRRange},
			},
		}, nil
	}
	return utils.FnCacheGet(ctx, v.cache, "", uncached)
}

type vnetOSConfiguration struct {
	pong   proto.PingResponse
	config *vnetv1.TargetOSConfiguration
}

func (v *vnetApplicationService) ResolveFQDN(ctx context.Context, fqdn string) (*vnetv1.ResolveFQDNResponse, error) {
	osConfig, err := v.getOSConfiguration(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	// We never want to serve queries for the proxy address.
	if fqdn == fullyQualify(hostname(osConfig.pong.GetProxyPublicAddr())) {
		return &vnetv1.ResolveFQDNResponse{}, nil
	}

	// If the FQDN is not a subdomain of one of our configured zones, return
	// NotFound and allow the VNet nameserver to recurse to an upstream.
	var inZone bool
	for _, zone := range osConfig.config.GetDnsZones() {
		if isDescendantSubdomain(fqdn, zone) {
			inZone = true
			break
		}
	}
	if !inZone {
		v.logger.DebugContext(ctx,
			"Queried FQDN is not a descendant subdomain of a configured DNS zone",
			"fqdn", fqdn,
			"dns_zones", osConfig.config.GetDnsZones(),
		)
		return &vnetv1.ResolveFQDNResponse{}, nil
	}

	expr := fmt.Sprintf(
		`resource.spec.public_addr == %+q || resource.spec.public_addr == %+q`,
		fqdn,
		strings.TrimSuffix(fqdn, "."),
	)
	rsp, err := client.GetResourcePage[types.AppServer](ctx, v.client, &proto.ListResourcesRequest{
		ResourceType:        types.KindAppServer,
		PredicateExpression: expr,
		Limit:               1,
	})
	if err != nil {
		v.logger.ErrorContext(ctx, "Failed to list application servers",
			"fqdn", fqdn,
			"error", err,
			"expression", expr,
		)
		return nil, trace.Wrap(err, "listing application servers")
	}
	if len(rsp.Resources) == 0 {
		v.logger.DebugContext(ctx, "No matching apps for FQDN", "fqdn", fqdn)
		return &vnetv1.ResolveFQDNResponse{}, nil
	}

	app, ok := rsp.Resources[0].GetApp().(*types.AppV3)
	if !ok {
		return nil, trace.BadParameter("expected *types.AppV3, got %T", rsp.Resources[0].GetApp())
	}
	if !vnet.IsVNetApp(app) {
		v.logger.DebugContext(ctx, "Application protocol not supported by VNet",
			"fqdn", fqdn,
			"app_name", app.GetName(),
			"app_uri", app.GetURI(),
			"app_protocol", app.GetProtocol(),
		)
		return &vnetv1.ResolveFQDNResponse{}, nil
	}

	// VNet intentionally doesn't support HTTP apps for a number of reasons.
	//
	// One such reason is the security risk of untrusted code (e.g. JavaScript
	// in a web browser) being able to access arbitrary local services. Browsers
	// help to some extent here via the same-origin policy, but cannot reliably
	// prevent DNS rebinding attacks for plain HTTP apps.
	//
	// While the underlying issue remains in the beam sandbox, the risk is more
	// acceptable because (1) you can restrict the beam's access to a subset of
	// your application via Delegation Sessions, and (2) allowing untrusted code
	// and agents to access your Teleport-protected resources is the entire point
	// of Beams! by using them you're already accepting a larger security trade-
	// off than the browser sandbox normally would.
	//
	// We make it work by pretending they're actually plain TCP apps:
	//
	// 	- The local ALPN proxy will advertise support for the "teleport-tcp"
	// 	  protocol in the TLS handshake.
	//
	// 	- On the Teleport proxy-side, this protocol is routed to the web server's
	// 	  HandleConnection method.
	//
	// 	- From there, the connection is handed off to the app handler, which
	// 	  determines the protocol from the application *resource* not the ALPN
	// 	  protocol.
	//
	// TODO(boxofrad): Replace this with HTTPS-in-mTLS once RFD 0035e is approved
	// and implemented.
	proxyAddr := osConfig.pong.GetProxyPublicAddr()
	return &vnetv1.ResolveFQDNResponse{
		Match: &vnetv1.ResolveFQDNResponse_MatchedTcpApp{
			MatchedTcpApp: &vnetv1.MatchedTCPApp{
				AppInfo: &vnetv1.AppInfo{
					AppKey: &vnetv1.AppKey{
						Profile: proxyAddr,
						Name:    app.GetName(),
					},
					App:           app,
					Ipv4CidrRange: osConfig.config.GetIpv4CidrRanges()[0],
					Cluster:       osConfig.pong.GetClusterName(),
					DialOptions: &vnetv1.DialOptions{
						WebProxyAddr: proxyAddr,

						// ALPN Upgrade is not required in Teleport Cloud we
						// might need to reevaluate this if we support Beams
						// on-premise (or not? we could just draw a hard line
						// and require sensible proxy configuration).
						AlpnConnUpgradeRequired: false,
						InsecureSkipVerify:      v.insecure,
					},
				},
			},
		},
	}, nil
}

// GetAppCert issues a TLS certificate for the given application.
func (v *vnetApplicationService) GetAppCert(ctx context.Context, key *vnetv1.AppInfo, port uint16) (*tls.Certificate, error) {
	identity, err := v.identityGenerator.Generate(ctx,
		identity.WithPrivateKey(v.privateKey),
		identity.WithRouteToApp(*vnet.RouteToApp(key, port)),
		identity.WithLifetime(v.credentialLifetime.TTL, v.credentialLifetime.RenewalInterval),
		identity.WithDelegation(v.delegationSessionID),
	)
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return identity.TLSCert, nil
}

// GetAppSigner returns the private key for the given application's TLS certificate.
func (v *vnetApplicationService) GetAppSigner(context.Context, *vnetv1.AppKey, uint16) (crypto.Signer, error) {
	return v.privateKey, nil
}

func isDescendantSubdomain(fqdn, zone string) bool {
	return strings.HasSuffix(fqdn, "."+fullyQualify(zone))
}

func fullyQualify(domain string) string {
	if strings.HasSuffix(domain, ".") {
		return domain
	}
	return domain + "."
}

func hostname(hostPort string) string {
	if !strings.Contains(hostPort, ":") {
		return hostPort
	}
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return hostPort
	}
	return host
}

var _ vnet.EmbeddedApplicationService = (*vnetApplicationService)(nil)
