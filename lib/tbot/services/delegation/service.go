package delegation

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v3/jwt"
	"github.com/google/uuid"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/client/proto"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/utils/keys"
	autoupdate "github.com/gravitational/teleport/lib/autoupdate/agent"
	"github.com/gravitational/teleport/lib/cryptosuites"
	"github.com/gravitational/teleport/lib/srv/alpnproxy"
	"github.com/gravitational/teleport/lib/srv/alpnproxy/common"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	"github.com/gravitational/teleport/lib/tbot/services/application"
	identitysvc "github.com/gravitational/teleport/lib/tbot/services/identity"
	tbotSSH "github.com/gravitational/teleport/lib/tbot/ssh"
	"github.com/gravitational/trace"
	"golang.org/x/crypto/ssh"
)

// TODO: pre-fetch an identity to get the bot's default roles
func ServiceBuilder(cfg *Config, opts ...ServiceOpt) bot.ServiceBuilder {
	return func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &Service{
			cfg:                cfg,
			proxyPinger:        deps.ProxyPinger,
			identityGenerator:  deps.IdentityGenerator,
			clientBuilder:      deps.ClientBuilder,
			botAuthClient:      deps.Client,
			applicationTunnels: make(map[string]*alpnproxy.LocalProxy),
		}
		for _, fn := range opts {
			fn(svc)
		}
		svc.log = deps.LoggerForService(svc)
		return svc, nil
	}
}

func WithALPNUpgradeCache(cache *internal.ALPNUpgradeCache) ServiceOpt {
	return func(s *Service) {
		s.alpnUpgradeCache = cache
	}
}

func WithInsecure(insecure bool) ServiceOpt {
	return func(s *Service) {
		s.insecure = insecure
	}
}

func WithFIPS(fips bool) ServiceOpt {
	return func(s *Service) {
		s.fips = fips
	}
}

type ServiceOpt func(s *Service)

type Service struct {
	cfg *Config
	log *slog.Logger

	proxyPinger       connection.ProxyPinger
	identityGenerator *identity.Generator
	clientBuilder     *client.Builder
	botAuthClient     *apiclient.Client
	alpnUpgradeCache  *internal.ALPNUpgradeCache
	insecure, fips    bool

	mu                 sync.Mutex
	applicationTunnels map[string]*alpnproxy.LocalProxy
}

func (s *Service) String() string {
	return s.cfg.GetName()
}

func (s *Service) Run(ctx context.Context) error {
	lis, err := internal.CreateListener(ctx, s.log, s.cfg.Listen)
	if err != nil {
		return trace.Wrap(err, "creating listener")
	}
	defer lis.Close()

	mux := http.NewServeMux()
	mux.Handle("POST /ssh-config", http.HandlerFunc(s.handleSSHConfig))
	mux.Handle("POST /application-tunnel", http.HandlerFunc(s.handleApplicationTunnel))

	srv := &http.Server{Handler: mux}
	go func() {
		<-ctx.Done()
		srv.Shutdown(ctx)
	}()
	return srv.Serve(lis)
}

func (s *Service) handleSSHConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	jwt, err := s.getUserJWT(r)
	if err != nil {
		http.Error(w, "Authorization header must be provided in the form `Bearer <token>`", http.StatusUnauthorized)
		return
	}

	var params struct {
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, "Failed to decode request body as JSON", http.StatusBadRequest)
		return
	}

	roles, err := s.getRoles(params.Roles, jwt)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get roles", "error", err)
		http.Error(w, "Failed to get roles", http.StatusInternalServerError)
		return
	}

	path, err := s.generateUserSSHConfig(ctx, jwt, roles)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to generate user SSH configuration", "error", err)
		http.Error(w, "Failed to generate user SSH configuration", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(struct {
		Path string `json:"path"`
	}{path})
}

func (s *Service) handleApplicationTunnel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	jwt, err := s.getUserJWT(r)
	if err != nil {
		http.Error(w, "Authorization header must be provided in the form `Bearer <token>`", http.StatusUnauthorized)
		return
	}

	var params struct {
		Name  string   `json:"application"`
		Roles []string `json:"roles"`
	}
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		http.Error(w, "Failed to decode request body as JSON", http.StatusBadRequest)
		return
	}

	roles, err := s.getRoles(params.Roles, jwt)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to get roles", "error", err)
		http.Error(w, "Failed to get roles", http.StatusInternalServerError)
		return
	}

	tunnel, err := s.startApplicationTunnel(ctx, jwt, params.Name, roles)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to start application tunnel", "error", err)
		http.Error(w, "Failed to start application tunnel", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(tunnel)
}

func (s *Service) generateUserSSHConfig(ctx context.Context, userJWT string, roles []string) (string, error) {
	baseID, err := s.identityGenerator.GenerateFacade(ctx,
		identity.WithLifetime(s.cfg.CredentialLifetime.TTL, s.cfg.CredentialLifetime.RenewalInterval),
		identity.WithLogger(s.log),
	)
	if err != nil {
		return "", trace.Wrap(err, "generating identity")
	}

	baseClient, err := s.clientBuilder.Build(ctx, baseID)
	if err != nil {
		return "", trace.Wrap(err, "building client")
	}
	defer baseClient.Close()

	userID, err := s.getUserIdentity(ctx, baseClient, userJWT, roles, nil)
	if err != nil {
		return "", trace.Wrap(err, "getting user identity")
	}

	userClient, err := s.clientBuilder.Build(ctx, identity.NewFacade(s.fips, s.insecure, userID))
	if err != nil {
		return "", trace.Wrap(err, "building user client")
	}
	defer userClient.Close()

	clusterNames, err := internal.GetClusterNames(ctx, baseClient, userID.ClusterName)
	if err != nil {
		return "", trace.Wrap(err, "getting cluster names")
	}

	proxyPong, err := s.proxyPinger.Ping(ctx)
	if err != nil {
		return "", trace.Wrap(err, "pinging proxy")
	}

	// TODO: This is bad. We're not setting any of the other options.
	dest := &destination.Directory{
		Path: filepath.Join(s.cfg.Path, uuid.NewString()),
	}
	if err := dest.CheckAndSetDefaults(); err != nil {
		return "", trace.Wrap(err, "check and set defaults")
	}

	// TODO: Also bad.
	if err := os.MkdirAll(dest.Path, os.ModePerm); err != nil {
		return "", trace.Wrap(err, "creating directory")
	}

	hostCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return "", trace.Wrap(err, "getting host CAs")
	}

	keyRing, err := internal.NewClientKeyRing(userID, hostCAs)
	if err != nil {
		return "", trace.Wrap(err, "creating keyring")
	}

	if err := identity.SaveIdentity(
		ctx, userID, dest, identity.DestinationKinds()...,
	); err != nil {
		return "", trace.Wrap(err, "persisting identity")
	}

	if err := internal.WriteIdentityFile(ctx, s.log, keyRing, dest); err != nil {
		return "", trace.Wrap(err, "writing identity file")
	}

	err = identitysvc.RenderSSHConfig(
		ctx,
		s.log,
		proxyPong,
		clusterNames,
		dest,
		s.botAuthClient,
		autoupdate.StableExecutable,
		s.alpnUpgradeCache,
		s.insecure,
		s.fips,
	)
	if err != nil {
		return "", trace.Wrap(err, "rendering SSH config")
	}

	return filepath.Join(dest.Path, tbotSSH.ConfigName), nil
}

func (s *Service) startApplicationTunnel(ctx context.Context, jwt string, applicationName string, roles []string) (*ephemeralApplicationTunnel, error) {
	identityOpts := []identity.GenerateOption{
		identity.WithLifetime(s.cfg.CredentialLifetime.TTL, s.cfg.CredentialLifetime.RenewalInterval),
		identity.WithLogger(s.log),
	}
	id, err := s.identityGenerator.GenerateFacade(ctx, identityOpts...)
	if err != nil {
		return nil, trace.Wrap(err, "generating identity")
	}

	client, err := s.clientBuilder.Build(ctx, id)
	if err != nil {
		return nil, trace.Wrap(err, "building client")
	}
	defer client.Close()

	routeToApp, app, err := application.GetRouteToApp(
		ctx,
		id.Get(),
		client,
		applicationName,
	)
	if err != nil {
		return nil, trace.Wrap(err, "getting route to app")
	}

	userID, err := s.getUserIdentity(ctx, client, jwt, roles, &routeToApp)
	if err != nil {
		return nil, trace.Wrap(err, "getting user identity")
	}

	proxyPong, err := s.proxyPinger.Ping(ctx)
	if err != nil {
		return nil, trace.Wrap(err, "pinging proxy")
	}
	proxyAddr, err := proxyPong.ProxyWebAddr()
	if err != nil {
		return nil, trace.Wrap(err, "determining proxy address")
	}

	tunnelID := uuid.NewString()
	socketPath, err := filepath.Abs(filepath.Join(s.cfg.Path, tunnelID+".sock"))
	if err != nil {
		return nil, trace.Wrap(err, "getting socket path")
	}

	lis, err := internal.CreateListener(ctx, s.log, "unix://"+socketPath)
	if err != nil {
		return nil, trace.Wrap(err, "creating listener")
	}

	// TODO: ALPN upgrade stuff.
	proxy, err := alpnproxy.NewLocalProxy(alpnproxy.LocalProxyConfig{
		RemoteProxyAddr: proxyAddr,
		ParentContext:   ctx,
		Protocols: []common.Protocol{
			application.ALPNProtocolForApp(app),
		},
		Cert:               *userID.TLSCert,
		InsecureSkipVerify: s.insecure,
		Listener:           lis,
	})
	if err != nil {
		return nil, trace.Wrap(err, "creating local proxy")
	}

	// TODO: clean up the proxy and this goroutine.
	s.mu.Lock()
	s.applicationTunnels[tunnelID] = proxy
	s.mu.Unlock()

	go func() { proxy.Start(context.Background()) }()

	expiry, _ := id.Expiry()

	return &ephemeralApplicationTunnel{
		ID:      tunnelID,
		Address: "unix://" + socketPath,
		Expires: expiry,
	}, nil
}

type ephemeralApplicationTunnel struct {
	ID      string    `json:"id"`
	Address string    `json:"address"`
	Expires time.Time `json:"expires"`
}

func (s *Service) getUserJWT(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", trace.Errorf("missing Authorization header")
	}
	token, isBearer := strings.CutPrefix(authHeader, "Bearer ")
	if !isBearer {
		return "", trace.Errorf("malformed Authorization header")
	}
	return token, nil
}

func (s *Service) getUserIdentity(
	ctx context.Context,
	client *apiclient.Client,
	jwt string,
	roles []string,
	routeToApp *proto.RouteToApp,
) (*identity.Identity, error) {
	key, err := cryptosuites.GenerateKey(ctx,
		cryptosuites.GetCurrentSuiteFromAuthPreference(client),
		cryptosuites.BotImpersonatedIdentity,
	)
	if err != nil {
		return nil, trace.Wrap(err, "generating keypair")
	}
	pubKey, err := keys.MarshalPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err, "marshaling public key")
	}
	sshPub, err := ssh.NewPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err)
	}

	userCerts, err := client.GenerateDelegatedCerts(ctx, proto.DelegatedCertsRequest{
		SSHPublicKey: ssh.MarshalAuthorizedKey(sshPub),
		TLSPublicKey: pubKey,
		Assertion:    jwt,
		RouteToApp:   routeToApp,
		Roles:        roles,
	})
	if err != nil {
		return nil, trace.Wrap(err, "getting delegated certs")
	}

	privateKeyPEM, err := keys.MarshalPrivateKey(key)
	if err != nil {
		return nil, trace.Wrap(err, "marshaling private key")
	}
	publicKeyPEM, err := keys.MarshalPublicKey(key.Public())
	if err != nil {
		return nil, trace.Wrap(err, "marshaling public key")
	}

	id, err := identity.ReadIdentityFromStore(&identity.LoadIdentityParams{
		PrivateKeyBytes: privateKeyPEM,
		PublicKeyBytes:  publicKeyPEM,
	}, userCerts)
	if err != nil {
		return nil, trace.Wrap(err, "reading identity from store")
	}

	return id, nil
}

func (s *Service) getRoles(roles []string, userJWT string) ([]string, error) {
	if len(roles) != 0 {
		return roles, nil
	}

	parsed, err := jwt.ParseSigned(userJWT)
	if err != nil {
		return nil, trace.Wrap(err, "parsing user JWT")
	}

	var claims struct {
		Roles []string `json:"roles"`
	}
	if err := parsed.UnsafeClaimsWithoutVerification(&claims); err != nil {
		return nil, trace.Wrap(err, "extracting claims")
	}
	return claims.Roles, nil
}
