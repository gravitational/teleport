package delegation

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	apiclient "github.com/gravitational/teleport/api/client"
	"github.com/gravitational/teleport/api/types"
	autoupdate "github.com/gravitational/teleport/lib/autoupdate/agent"
	"github.com/gravitational/teleport/lib/tbot/bot"
	"github.com/gravitational/teleport/lib/tbot/bot/connection"
	"github.com/gravitational/teleport/lib/tbot/bot/destination"
	"github.com/gravitational/teleport/lib/tbot/client"
	"github.com/gravitational/teleport/lib/tbot/identity"
	"github.com/gravitational/teleport/lib/tbot/internal"
	identitysvc "github.com/gravitational/teleport/lib/tbot/services/identity"
	"github.com/gravitational/teleport/lib/tbot/ssh"
	"github.com/gravitational/trace"
)

func ServiceBuilder(cfg *Config, opts ...ServiceOpt) bot.ServiceBuilder {
	return func(deps bot.ServiceDependencies) (bot.Service, error) {
		if err := cfg.CheckAndSetDefaults(); err != nil {
			return nil, trace.Wrap(err)
		}
		svc := &Service{
			cfg:               cfg,
			proxyPinger:       deps.ProxyPinger,
			identityGenerator: deps.IdentityGenerator,
			clientBuilder:     deps.ClientBuilder,
			botAuthClient:     deps.Client,
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

	insecure, fips bool
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

	path, err := s.generateUserSSHConfig(ctx, jwt)
	if err != nil {
		s.log.ErrorContext(ctx, "Failed to generate user SSH configuration", "error", err)
		http.Error(w, "Failed to generate user SSH configuration", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(struct {
		Path string `json:"path"`
	}{path})
}

func (s *Service) generateUserSSHConfig(ctx context.Context, userJWT string) (string, error) {
	// TODO: get a user certificate instead of this.
	id, err := s.identityGenerator.GenerateFacade(ctx,
		identity.WithLifetime(s.cfg.CredentialLifetime.TTL, s.cfg.CredentialLifetime.RenewalInterval),
	)
	if err != nil {
		return "", trace.Wrap(err, "generating identity")
	}

	client, err := s.clientBuilder.Build(ctx, id)
	if err != nil {
		return "", trace.Wrap(err, "building client")
	}

	clusterNames, err := internal.GetClusterNames(ctx, client, id.Get().ClusterName)
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

	// TODO: Also bad.
	if err := os.MkdirAll(dest.Path, os.ModePerm); err != nil {
		return "", trace.Wrap(err, "creating directory")
	}

	hostCAs, err := s.botAuthClient.GetCertAuthorities(ctx, types.HostCA, false)
	if err != nil {
		return "", trace.Wrap(err, "getting host CAs")
	}

	keyRing, err := internal.NewClientKeyRing(id.Get(), hostCAs)
	if err != nil {
		return "", trace.Wrap(err, "creating keyring")
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

	return filepath.Join(dest.Path, ssh.ConfigName), nil
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
