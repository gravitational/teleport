package vnet

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net"
	"strings"
	"sync"
	"time"

	beamsv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/beams/v1"
	"github.com/gravitational/teleport/api/utils/retryutils"
	"github.com/gravitational/trace"
	"github.com/vishvananda/netlink"
	"golang.org/x/sync/errgroup"
	"golang.zx2c4.com/wireguard/tun"

	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/defaults"
	"github.com/gravitational/teleport/api/types"
	typesvnet "github.com/gravitational/teleport/api/types/vnet"
	vnetv1 "github.com/gravitational/teleport/gen/proto/go/teleport/lib/vnet/v1"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/utils/set"
	"github.com/gravitational/teleport/lib/vnet"
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
			allowedDomains:     &domainAllowlist{},
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
	allowedDomains     *domainAllowlist
}

func (s *Service) String() string { return s.cfg.Name }

func (s *Service) Run(ctx context.Context) error {
	effectiveLifetime := s.credentialLifetime

	impersonatedIdentity, err := s.identityGenerator.GenerateFacade(ctx,
		identity.WithLifetime(effectiveLifetime.TTL, effectiveLifetime.RenewalInterval),
		identity.WithDelegation(s.cfg.DelegationSessionID),
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
			botIdentity:         impersonatedIdentity,
			client:              impersonatedClient,
			identityGenerator:   s.identityGenerator,
			alpnUpgradeCache:    s.alpnUpgradeCache,
			credentialLifetime:  s.credentialLifetime,
			proxyPinger:         s.proxyPinger,
			delegationSessionID: s.cfg.DelegationSessionID,
			allowedDomains:      s.allowedDomains,
			logger:              s.log,
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

	group, groupCtx := errgroup.WithContext(ctx)
	group.Go(func() error {
		return trace.Wrap(s.watchBeam(groupCtx, impersonatedClient.BeamsServiceClient()), "watching beam")
	})
	group.Go(func() error {
		return trace.Wrap(vn.Run(groupCtx), "running vnet")
	})
	return trace.Wrap(group.Wait())
}

func (s *Service) watchBeam(ctx context.Context, client beamsv1.BeamsServiceClient) error {
	retry, err := retryutils.NewRetryV2(retryutils.RetryV2Config{
		Driver: retryutils.NewExponentialDriver(1 * time.Second),
		Max:    1 * time.Minute,
		Jitter: retryutils.HalfJitter,
	})
	if err != nil {
		return trace.Wrap(err, "creating retry")
	}

	for {
		err := s.watchBeamStream(ctx, client)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// TODO(boxofrad): This causes a circular dependency where the beam
		// isn't created until the beam service returns, but the beam service
		// doesn't return until the pod is ready, and the pod never becomes
		// ready because we fail to read the beam.
		//
		// if trace.IsNotFound(err) {
		// 	return err
		// }

		s.log.WarnContext(ctx, "Watching beam failed", "error", err)
		retry.Inc()

		select {
		case <-retry.After():
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (s *Service) watchBeamStream(ctx context.Context, client beamsv1.BeamsServiceClient) error {
	stream, err := client.WatchBeam(ctx, &beamsv1.WatchBeamRequest{
		Id: s.cfg.BeamID,
	})
	if err != nil {
		return err
	}

	for {
		beam, err := stream.Recv()
		if err != nil {
			return err
		}

		allowedDomains := beam.GetSpec().GetAllowedDomains()
		s.log.DebugContext(ctx, "Replacing allowed domains",
			"allowed_domains", allowedDomains,
			"beam_revision",
			beam.GetMetadata().GetRevision(),
		)
		s.allowedDomains.replace(allowedDomains)
	}
}

type domainAllowlist struct {
	mu      sync.Mutex
	domains set.Set[string]
}

func (a *domainAllowlist) replace(elems []string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.domains = set.New(elems...)
}

func (a *domainAllowlist) contains(domain string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.domains == nil {
		return false
	}
	return a.domains.Contains(domain)
}

func (a *domainAllowlist) all() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.domains.Elements()
}

type applicationService struct {
	botIdentity         *identity.Facade
	client              *apiclient.Client
	identityGenerator   *identity.Generator
	alpnUpgradeCache    *internal.ALPNUpgradeCache
	credentialLifetime  bot.CredentialLifetime
	proxyPinger         connection.ProxyPinger
	delegationSessionID string
	allowedDomains      *domainAllowlist
	logger              *slog.Logger
}

func (s *applicationService) ResolveFQDN(ctx context.Context, req *vnetv1.ResolveFQDNRequest) (*vnetv1.ResolveFQDNResponse, error) {
	fqdn := req.GetFqdn()

	proxyAddr, err := s.proxyAddr(ctx)
	if err != nil {
		return nil, trace.BadParameter("getting proxy hostname")
	}

	// Handle resolving the proxy address.
	proxyHostname := fullyQualify(hostname(proxyAddr))
	if fqdn == proxyHostname {
		return nil, trace.NotFound("proxy address should be resolved upstream")
	}

	switch {
	case s.allowedDomains.contains(fqdn):
		// FQDN is allowlisted.
	case strings.HasSuffix(fqdn, proxyHostname):
		// TODO: should we explicitly allowlist the "normal" Teleport apps like
		// the LLM proxy?
	default:
		return nil, trace.NotFound("fqdn %q is not in beam allowlist")
	}

	// Check if this looks like a database FQDN: <db-user>.<db-service>.db.<proxy>.
	dbZone := "db." + hostname(proxyAddr)
	if strings.HasSuffix(fqdn, "."+fullyQualify(dbZone)) {
		prefix := strings.TrimSuffix(fqdn, "."+fullyQualify(dbZone))
		parts := strings.SplitN(prefix, ".", 2)
		if len(parts) == 2 {
			dbUser, dbServiceName := parts[0], parts[1]
			db, err := s.getDatabase(ctx, dbServiceName)
			if err != nil {
				return nil, trace.Wrap(err, "looking up database %q", dbServiceName)
			}
			alpnRequired, err := s.alpnUpgradeCache.IsUpgradeRequired(ctx, proxyAddr, false)
			if err != nil {
				return nil, trace.Wrap(err, "determining if ALPN upgrade is required")
			}
			return &vnetv1.ResolveFQDNResponse{
				Match: &vnetv1.ResolveFQDNResponse_MatchedDatabase{
					MatchedDatabase: &vnetv1.MatchedDatabase{
						DbServiceName: db.GetName(),
						DbUser:        dbUser,
						DbProtocol:    db.GetProtocol(),
						Profile:       proxyAddr,
						RootCluster:   s.botIdentity.Get().ClusterName,
						Ipv4CidrRange: typesvnet.DefaultIPv4CIDRRange,
						DialOptions: &vnetv1.DialOptions{
							WebProxyAddr:            proxyAddr,
							AlpnConnUpgradeRequired: alpnRequired,
							Sni:                     hostname(proxyAddr),
							InsecureSkipVerify:      true,
						},
					},
				},
			}, nil
		}
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

	if !vnet.IsVNetApp(app) {
		// TODO(greedy52) properly support ResolveFQDNResponse_MatchedWebApp
		// instead of using TCP response.
		slog.InfoContext(ctx, "App not supported", "url", app.GetURI())
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
	proxyHost := hostname(proxyAddr)
	return &vnetv1.GetTargetOSConfigurationResponse{
		TargetOsConfiguration: &vnetv1.TargetOSConfiguration{
			DnsZones:       []string{proxyHost, "db." + proxyHost},
			Ipv4CidrRanges: []string{typesvnet.DefaultIPv4CIDRRange},
		},
	}, nil
}

func (s *applicationService) GetAppCert(ctx context.Context, req *vnetv1.ReissueAppCertRequest) (*tls.Certificate, error) {
	route := vnet.RouteToApp(req.GetAppInfo(), uint16(req.GetTargetPort()))

	id, err := s.identityGenerator.Generate(ctx,
		identity.WithLifetime(s.credentialLifetime.TTL, s.credentialLifetime.RenewalInterval),
		identity.WithRouteToApp(*route),
		identity.WithDelegation(s.delegationSessionID),
	)
	if err != nil {
		return nil, trace.Wrap(err, "issuing app certificate")
	}

	return id.TLSCert, nil
}

func (s *applicationService) GetUserCert(ctx context.Context, req *vnetv1.UserTLSCertRequest) (*tls.Certificate, error) {
	return s.botIdentity.Get().TLSCert, nil
}

func (s *applicationService) GetDBCert(ctx context.Context, req *vnetv1.ReissueDBCertRequest) (*tls.Certificate, error) {
	dbKey := req.GetDbKey()
	id, err := s.identityGenerator.Generate(ctx,
		identity.WithLifetime(s.credentialLifetime.TTL, s.credentialLifetime.RenewalInterval),
		identity.WithRouteToDatabase(proto.RouteToDatabase{
			ServiceName: dbKey.GetDbServiceName(),
			Protocol:    dbKey.GetDbProtocol(),
			Username:    dbKey.GetDbUser(),
			Database:    req.GetDbName(),
		}),
		identity.WithDelegation(s.delegationSessionID),
	)
	if err != nil {
		return nil, trace.Wrap(err, "issuing database certificate")
	}
	return id.TLSCert, nil
}

func (s *applicationService) getDatabase(ctx context.Context, name string) (types.Database, error) {
	servers, err := apiclient.GetAllResources[types.DatabaseServer](ctx, s.client, &proto.ListResourcesRequest{
		Namespace:           defaults.Namespace,
		ResourceType:        types.KindDatabaseServer,
		PredicateExpression: `resource.metadata.name == "` + name + `"`,
		Limit:               1,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	if len(servers) == 0 {
		return nil, trace.NotFound("database %q not found", name)
	}
	return servers[0].GetDatabase(), nil
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
