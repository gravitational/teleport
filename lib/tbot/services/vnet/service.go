package vnet

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"strings"
	"sync"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	typesvnet "github.com/gravitational/teleport/api/types/vnet"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/vnet"
	"github.com/gravitational/trace"
	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/tun"
)

func ServiceBuilder(cfg *Config, alpnUpgradeCache *internal.ALPNUpgradeCache) bot.ServiceBuilder {
	buildFn := func(deps bot.ServiceDependencies) (bot.Service, error) {
		return &Service{
			cfg:                cfg,
			log:                deps.Logger,
			botIdentity:        deps.BotIdentity,
			botIdentityReadyCh: deps.BotIdentityReadyCh,
			botClient:          deps.Client,
			identityGenerator:  deps.IdentityGenerator,
			clientBuilder:      deps.ClientBuilder,
			proxyPinger:        deps.ProxyPinger,
			alpnUpgradeCache:   alpnUpgradeCache,
			credentialLifetime: cfg.GetCredentialLifetime(),
		}, nil
	}
	return bot.NewServiceBuilder(ServiceType, cfg.Name, buildFn)
}

type Service struct {
	cfg                *Config
	log                *slog.Logger
	botIdentity        func() *identity.Identity
	botIdentityReadyCh <-chan struct{}
	botClient          *apiclient.Client
	identityGenerator  *identity.Generator
	clientBuilder      *client.Builder
	proxyPinger        connection.ProxyPinger
	alpnUpgradeCache   *internal.ALPNUpgradeCache
	credentialLifetime bot.CredentialLifetime
}

func (s *Service) String() string { return s.cfg.Name }

func (s *Service) Run(ctx context.Context) error {
	effectiveLifetime := s.credentialLifetime

	impersonatedIdentity, err := s.identityGenerator.GenerateFacade(ctx,
		identity.WithLifetime(effectiveLifetime.TTL, effectiveLifetime.RenewalInterval),
		identity.WithDelegation(s.cfg.DelegationTicket),
	)
	if err != nil {
		return trace.Wrap(err, "generating impersonated identity")
	}
	impersonatedClient, err := s.clientBuilder.Build(ctx, impersonatedIdentity)
	if err != nil {
		return trace.Wrap(err, "building impersonated client")
	}
	defer func() {
		if err := impersonatedClient.Close(); err != nil {
			s.log.ErrorContext(ctx, "Failed to close impersonated client", "error", err)
		}
	}()

	// Create TUN device.
	dev, err := tun.CreateTUN("tbotvnet", 1500)
	if err != nil {
		return trace.Wrap(err, "creating TUN device")
	}
	defer dev.Close()

	devName, err := dev.Name()
	if err != nil {
		return trace.Wrap(err, "getting TUN device name")
	}

	devLink, err := netlink.LinkByName(devName)
	if err != nil {
		return trace.Wrap(err, "resolving link for TUN device")
	}

	// Note: we wrap this function in a sync.Once because we expect the routes
	// and DNS zones to stay the same in beams, and it avoids the complexity of
	// handling any errors. For using vnet outside of beams, we should probably
	// make the function idempotent instead.
	configureHost := func(ctx context.Context, cfg vnet.EmbeddedVNetHostConfig) error {
		// Add IP addresses for the TUN device.
		linkAddrV4 := &netlink.Addr{
			IPNet: netlink.NewIPNet(net.ParseIP(cfg.DeviceIPv4)),
		}
		if err := netlink.AddrAdd(devLink, linkAddrV4); err != nil {
			return trace.Wrap(err, "adding IPv4 address %s to TUN device", cfg.DeviceIPv4)
		}
		linkAddrV6 := &netlink.Addr{
			IPNet: netlink.NewIPNet(net.ParseIP(cfg.DeviceIPv6)),
		}
		if err := netlink.AddrAdd(devLink, linkAddrV6); err != nil {
			return trace.Wrap(err, "adding IPv6 address %s to TUN device", cfg.DeviceIPv6)
		}

		// Bring the TUN device up.
		if err := netlink.LinkSetUp(devLink); err != nil {
			return trace.Wrap(err, "bringing TUN device up")
		}

		// Update the routing table.
		linkIdx := devLink.Attrs().Index
		for _, cidr := range cfg.CIDRRanges {
			dst, err := netlink.ParseIPNet(cidr)
			if err != nil {
				return trace.Wrap(err, "parsing CIDR range %s", cidr)
			}
			route := &netlink.Route{
				LinkIndex: linkIdx,
				Dst:       dst,
			}
			if err := netlink.RouteReplace(route); err != nil {
				return trace.Wrap(err, "adding route from %s to TUN device", cidr)
			}
		}
		return nil
	}

	// Create embedded vnet network stack.
	var once sync.Once
	vn, err := vnet.NewEmbeddedVNet(vnet.EmbeddedVNetConfig{
		Device: dev,
		ApplicationService: &applicationService{
			botIdentity:        impersonatedIdentity,
			client:             impersonatedClient,
			identityGenerator:  s.identityGenerator,
			alpnUpgradeCache:   s.alpnUpgradeCache,
			credentialLifetime: s.credentialLifetime,
			proxyPinger:        s.proxyPinger,
			delegationTicket:   s.cfg.DelegationTicket,
		},
		ConfigureHost: func(ctx context.Context, cfg vnet.EmbeddedVNetHostConfig) error {
			var err error
			once.Do(func() { err = configureHost(ctx, cfg) })
			return err
		},
	})
	if err != nil {
		return trace.Wrap(err, "creating embedded vnet")
	}
	return trace.Wrap(vn.Run(ctx), "running vnet")
}

type applicationService struct {
	botIdentity        *identity.Facade
	client             *apiclient.Client
	identityGenerator  *identity.Generator
	alpnUpgradeCache   *internal.ALPNUpgradeCache
	credentialLifetime bot.CredentialLifetime
	proxyPinger        connection.ProxyPinger
	delegationTicket   string
}

func (s *applicationService) ResolveFQDN(ctx context.Context, req *vnetv1.ResolveFQDNRequest) (*vnetv1.ResolveFQDNResponse, error) {
	fqdn := req.GetFqdn()

	// Handle resolving the proxy address.
	proxyAddr, err := s.proxyAddr(ctx)
	if err != nil {
		return nil, trace.BadParameter("getting proxy hostname")
	}
	if fqdn == fullyQualify(hostname(proxyAddr)) {
		return nil, trace.NotFound("proxy address should be resolved upstream")
	}

	// Search for apps with a matching public_addr.
	expr := `resource.spec.public_addr == "` + strings.TrimSuffix(fqdn, ".") + `" || resource.spec.public_addr == "` + fqdn + `"`
	resp, err := apiclient.GetResourcePage[types.AppServer](ctx, s.client, &proto.ListResourcesRequest{
		ResourceType:        types.KindAppServer,
		PredicateExpression: expr,
		Limit:               1,
	})
	if err != nil {
		return nil, trace.Wrap(err, "listing application servers")
	}
	if len(resp.Resources) == 0 {
		return nil, trace.NotFound("no matching app")
	}

	app, ok := resp.Resources[0].GetApp().(*types.AppV3)
	if !ok {
		return nil, trace.BadParameter("expected *types.AppV3, got %T", resp.Resources[0].GetApp())
	}

	// Only TCP apps are supported.
	if !app.IsTCP() {
		return &vnetv1.ResolveFQDNResponse{
			Match: &vnetv1.ResolveFQDNResponse_MatchedWebApp{
				MatchedWebApp: &vnetv1.MatchedWebApp{},
			},
		}, nil
	}

	alpnRequired, err := s.alpnUpgradeCache.IsUpgradeRequired(ctx, proxyAddr, false)
	if err != nil {
		return nil, trace.Wrap(err, "determining if ALPN upgrade is required")
	}

	appInfo := &vnetv1.AppInfo{
		AppKey: &vnetv1.AppKey{
			Profile: proxyAddr,
			Name:    app.GetName(),
		},
		Cluster:       s.botIdentity.Get().ClusterName,
		App:           app,
		Ipv4CidrRange: typesvnet.DefaultIPv4CIDRRange, // TODO: read the CIDR range properly.
		DialOptions: &vnetv1.DialOptions{
			WebProxyAddr:            proxyAddr,
			AlpnConnUpgradeRequired: alpnRequired,
			Sni:                     hostname(proxyAddr),
			InsecureSkipVerify:      true, // TODO: figure out why we're doing this.
		},
	}

	return &vnetv1.ResolveFQDNResponse{
		Match: &vnetv1.ResolveFQDNResponse_MatchedTcpApp{
			MatchedTcpApp: &vnetv1.MatchedTCPApp{AppInfo: appInfo},
		},
	}, nil
}

func (s *applicationService) GetTargetOSConfiguration(ctx context.Context, in *vnetv1.GetTargetOSConfigurationRequest) (*vnetv1.GetTargetOSConfigurationResponse, error) {
	proxyAddr, err := s.proxyAddr(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "getting proxy hostname")
	}

	// TODO: support configuring the CIDR ranges properly.
	return &vnetv1.GetTargetOSConfigurationResponse{
		TargetOsConfiguration: &vnetv1.TargetOSConfiguration{
			DnsZones:       []string{hostname(proxyAddr)},
			Ipv4CidrRanges: []string{typesvnet.DefaultIPv4CIDRRange},
		},
	}, nil
}

func (s *applicationService) GetAppCert(ctx context.Context, req *vnetv1.ReissueAppCertRequest) (*tls.Certificate, error) {
	route := vnet.RouteToApp(req.GetAppInfo(), uint16(req.GetTargetPort()))

	id, err := s.identityGenerator.Generate(ctx,
		identity.WithLifetime(s.credentialLifetime.TTL, s.credentialLifetime.RenewalInterval),
		identity.WithRouteToApp(*route),
		identity.WithDelegation(s.delegationTicket),
	)
	if err != nil {
		return nil, trace.Wrap(err, "issuing app certificate")
	}

	return id.TLSCert, nil
}

func (s *applicationService) GetUserCert(ctx context.Context, req *vnetv1.UserTLSCertRequest) (*tls.Certificate, error) {
	return s.botIdentity.Get().TLSCert, nil
}

func (s *applicationService) proxyAddr(ctx context.Context) (string, error) {
	proxyPong, err := s.proxyPinger.Ping(ctx)
	if err != nil {
		return "", trace.Wrap(err, "pinging proxy")
	}

	proxyAddr, err := proxyPong.ProxyWebAddr()
	if err != nil {
		return "", trace.Wrap(err)
	}
	return proxyAddr, nil
}

var _ vnet.EmbeddedApplicationService = (*applicationService)(nil)

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

func fullyQualify(domain string) string {
	if strings.HasSuffix(domain, ".") {
		return domain
	}
	return domain + "."
}
