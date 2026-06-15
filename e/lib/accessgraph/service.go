package accessgraph

import (
	"context"
	"log/slog"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"

	"github.com/gravitational/teleport"
	accessgraphsecretsv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessgraph/v1"
	clusterconfigv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/clusterconfig/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/e/lib/licensefile"
	"github.com/gravitational/teleport/entitlements"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1alpha"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/devicetrust/assertserver"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/service"
	"github.com/gravitational/teleport/lib/service/servicecfg"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/services/readonly"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

const (
	maxGRPCMessageSize = 20 * 1024 * 1024 // 20MB
)

type ServiceConfig struct {
	// Logger is the logger to use.
	Logger *slog.Logger

	// Storage is the backend storage to use.
	Storage *local.AccessGraphSecretsService

	// Client is the access graph client.
	Client accessgraphv1.AccessGraphServiceClient

	// Authorizer is the authorizer to use.
	Authorizer authz.Authorizer

	// ClusterName is the name of the cluster.
	ClusterName string

	// DeviceAssertionServer is the device trust assertions service.
	DeviceAssertionServer func() (assertserver.Ceremony, error)

	// AuthPreferenceGetter a function to get the auth preference.
	AuthPreferenceGetter AuthPreferenceGetterFunc

	// UsageReporter is the usage reporter to use.
	UsageReporter usagereporter.UsageReporter

	// Modules define which features are enabled for the process.
	Modules modules.Modules
}

// AuthPreferenceGetterFunc is a function that returns the auth preference.
type AuthPreferenceGetterFunc func(ctx context.Context) (readonly.AuthPreference, error)

type Service struct {
	accessgraphv1.UnimplementedAccessGraphServiceServer
	accessgraphsecretsv1pb.UnimplementedSecretsScannerServiceServer

	// client is the access graph client.
	client accessgraphv1.AccessGraphServiceClient

	// authorizer is the authorizer to use.
	authorizer authz.Authorizer

	// log is the logger to use.
	log *slog.Logger

	clusterName    string
	secretsService *local.AccessGraphSecretsService

	deviceAssertionServer func() (assertserver.Ceremony, error)
	authPreferenceGetter  AuthPreferenceGetterFunc
	usageReporter         usagereporter.UsageReporter
	modules               modules.Modules
}

// NewService creates a new access graph service.
func NewService(cfg ServiceConfig) (*Service, error) {
	if err := cfg.checkAndSetDefaults(); err != nil {
		return nil, trace.Wrap(err)
	}

	return &Service{
		client:                cfg.Client,
		log:                   cfg.Logger,
		authorizer:            cfg.Authorizer,
		clusterName:           cfg.ClusterName,
		secretsService:        cfg.Storage,
		deviceAssertionServer: cfg.DeviceAssertionServer,
		authPreferenceGetter:  cfg.AuthPreferenceGetter,
		usageReporter:         cfg.UsageReporter,
		modules:               cfg.Modules,
	}, nil
}

func (c *ServiceConfig) checkAndSetDefaults() error {
	if c.Logger == nil {
		c.Logger = slog.Default()
	}

	if c.Client == nil {
		return trace.BadParameter("missing access graph client")
	}

	if c.Authorizer == nil {
		return trace.BadParameter("missing authorizer")
	}

	if c.ClusterName == "" {
		return trace.BadParameter("missing cluster name")
	}

	if c.Storage == nil {
		return trace.BadParameter("missing Storage")
	}

	if c.DeviceAssertionServer == nil {
		return trace.BadParameter("missing DeviceAssertionServer")
	}

	if c.AuthPreferenceGetter == nil {
		return trace.BadParameter("missing AuthPreferenceGetter")
	}

	if c.UsageReporter == nil {
		return trace.BadParameter("missing UsageReporter")
	}

	if c.Modules == nil {
		return trace.BadParameter("missing Modules")
	}

	return nil
}

func (s *Service) Query(ctx context.Context, request *accessgraphv1.QueryRequest) (*accessgraphv1.QueryResponse, error) {
	authzCtx, err := s.authorizer.Authorize(ctx)
	if err != nil {
		return nil, trace.Wrap(err)
	}

	if err := authzCtx.CheckAccessToKind(types.KindAccessGraph, types.VerbRead); err != nil {
		return nil, trace.WrapWithMessage(err, "not allowed to read the access graph")
	}

	resp, err := s.client.Query(
		ctx,
		request,
		grpc.MaxCallRecvMsgSize(maxGRPCMessageSize),
		grpc.MaxCallSendMsgSize(maxGRPCMessageSize),
	)
	return resp, trace.Wrap(err)
}

func (s *Service) GetFile(ctx context.Context, request *accessgraphv1.GetFileRequest) (*accessgraphv1.GetFileResponse, error) {
	// Do not perform access check as we only serve webassets.
	resp, err := s.client.GetFile(ctx, request,
		grpc.MaxCallRecvMsgSize(maxGRPCMessageSize),
		grpc.MaxCallSendMsgSize(maxGRPCMessageSize))
	return resp, trace.Wrap(err)
}

func (s *Service) EventsStream(_ accessgraphv1.AccessGraphService_EventsStreamServer) error {
	return trace.NotImplemented("EventsStream should not be called on the auth server")
}

func (s *Service) Register(_ context.Context, _ *accessgraphv1.RegisterRequest) (*accessgraphv1.RegisterResponse, error) {
	return nil, trace.NotImplemented("Register should not be called on the auth server")
}

func (s *Service) ReplaceCAs(_ context.Context, _ *accessgraphv1.ReplaceCAsRequest) (*accessgraphv1.ReplaceCAsResponse, error) {
	return nil, trace.NotImplemented("ReplaceCAs should not be called on the auth server")
}

// RegisterAccessGraphService registers the access graph sync service with the process.
func RegisterAccessGraphService(cfg *servicecfg.Config, process *service.TeleportProcess, license *licensefile.LicenseFile) error {
	if !cfg.AccessGraph.Enabled {
		return nil
	}
	ctx := process.ExitContext()
	cfg.Logger.DebugContext(ctx, "Access Graph integration enabled")

	registerFunc := func() error {
		// Need to check this here inside the service function, rather than on process creation,
		// since Cloud features are loaded dynamically. More detailed explanation in:
		// https://github.com/gravitational/teleport/blob/3af6d9c1a25836bb160589a27a7d168a19a4992b/lib/service/service.go#L1873
		accessGraphSettings, err := process.GetAuthServer().GetAccessGraphSettings(ctx)
		if err != nil {
			return trace.Wrap(err)
		}
		features := cfg.Modules.Features()
		demoModeEnabled := getDemoModeEnabled(accessGraphSettings, features)

		accessGraphEnabled := features.GetEntitlement(entitlements.AccessGraph).Enabled
		if !accessGraphEnabled && !demoModeEnabled {
			cfg.Logger.InfoContext(ctx, "Access Graph specified in config, but the license does not include Teleport Access Graph and demo mode not enabled. Access graph sync will not be enabled.")
			return nil
		}
		if accessGraphEnabled {
			cfg.Modules.EnableAccessGraph()
			cfg.Logger.InfoContext(ctx, "Starting access graph service")
		} else {
			cfg.Logger.InfoContext(ctx, "Starting access graph service in demo mode")
		}

		accessGraphAddr := cfg.AccessGraph.Addr
		if accessGraphAddr == "" {
			cfg.Logger.ErrorContext(ctx, "access graph endpoint not configured")
			return trace.NotFound("access graph endpoint not configured")
		}
		const accessGraphRetryPeriod = 5 * time.Second

		ctx := process.GracefulExitContext()

		conn, err := process.WaitForConnector(service.AuthIdentityEvent, cfg.Logger)
		if err != nil {
			return trace.Wrap(err)
		}

		log := cfg.Logger.With(teleport.ComponentKey, "accessgraph", "listen_address", accessGraphAddr)

		config := ServiceClientConfig{
			Addr:     accessGraphAddr,
			CA:       cfg.AccessGraph.CA,
			Insecure: cfg.AccessGraph.Insecure,
			AuditLog: AuditLogConfig(cfg.AccessGraph.AuditLog),
		}

		if err := registerAccessGraphMetrics(
			process.MetricsRegistry(),
			config.AuditLog.Enabled,
		); err != nil {
			return trace.Wrap(err, "registering access graph metrics")
		}

		// TODO(jakule): Very excessive retrying, but we need to make sure that
		// the access graph is initialized before we start serving requests.

		// Retry registration first: after it succeeds once, we do not need to re-attempt it.
		for {
			err := Register(ctx, &registrator{}, config, conn.ClientGetCertificate, process.GetAuthServer(), license, cfg.Modules)
			if err == nil {
				break
			}

			cfg.Logger.ErrorContext(ctx, "Access graph registration failed", "error", err)
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(accessGraphRetryPeriod):
				continue
			}
		}

		process.RegisterFunc("access-graph-ca-sync", func() error {
			clusterName := conn.ClusterName()
			// deliberately hitting the backend, since this will only result in
			// an occasional Get for a CA at init time, and making sure we can
			// reach TAG is critical enough that we ought to avoid depending on
			// a healthy cache for the watcher
			services := process.GetAuthServer().Services
			bk := process.GetBackend()

			for {
				err := backend.RunWhileLocked(
					ctx,
					backend.RunWhileLockedConfig{
						LockConfiguration: backend.LockConfiguration{
							LockNameComponents: []string{"accessGraphCASync"},
							Backend:            bk,
							TTL:                5 * time.Minute,
							RetryInterval:      time.Minute,
						},
						ReleaseCtxTimeout:   10 * time.Second,
						RefreshLockInterval: time.Minute,
					},
					func(ctx context.Context) error {
						// we have to recreate the client in case we hold on to
						// a client for the entirety of a CA rotation, because
						// right at the very end we will attempt to push a list
						// of CAs that doesn't include the issuer of the current
						// certificate, which is disallowed by TAG - recreating
						// the client has the effect of creating a new TLS
						// connection with credentials that are up to date
						// enough (at least, up to date enough)
						accessGraphConn, err := NewAccessGraphClient(ctx, config, conn.ClientGetCertificate)
						if err != nil {
							return trace.Wrap(err)
						}
						defer accessGraphConn.Close()
						accessGraphClient := accessgraphv1.NewAccessGraphServiceClient(accessGraphConn)

						err = watchAndPushCAs(ctx, log, clusterName, services, accessGraphClient)
						return trace.Wrap(err)
					},
				)
				if ctx.Err() != nil {
					return nil
				}
				cfg.Logger.ErrorContext(ctx, "Access graph CA sync failed.", "error", err)

				select {
				case <-time.After(accessGraphRetryPeriod):
					continue
				case <-ctx.Done():
					return nil
				}
			}
		})

		for {
			cfg.Logger.DebugContext(ctx, "Successfully registered with the access graph service")
			if err := initializeAndWatchAccessGraph(ctx,
				log,
				config,
				conn.ClientGetCertificate,
				process.GetAuthServer(), process.GetBackend()); err != nil {
				cfg.Logger.ErrorContext(ctx, "Access graph sync process failed", "error", err)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(accessGraphRetryPeriod):
					continue
				}
			}
		}
	}

	accessGraphSettings, err := process.GetAuthServer().GetAccessGraphSettings(ctx)
	if err != nil {
		return trace.Wrap(err)
	}
	features := cfg.Modules.Features()
	// if the license has access graph enabled, or they've already enabled demo mode, start the access graph service
	if features.GetEntitlement(entitlements.AccessGraph).Enabled || getDemoModeEnabled(accessGraphSettings, features) {
		// Register as non-critical service. We don't want to fail the startup if
		// access graph is not available.
		process.RegisterFunc("access-graph-service", registerFunc)

		return nil
	}

	// otherwise, start a watcher that will watch the AccessGraphSettings resource for updates to demo mode
	go func() {
		if err := waitForAccessGraphEntitlement(ctx, cfg.Logger, process.GetAuthServer(), cfg.Modules); err != nil {
			cfg.Logger.ErrorContext(ctx, "Error while waiting for AccessGraphSettings condition", "error", err)
			return
		}
		cfg.Logger.DebugContext(ctx, "AccessGraphSettings condition satisfied, registering access graph service")
		process.RegisterFunc("access-graph-service", registerFunc)
	}()

	return nil
}

// waitForAccessGraphEntitlement blocks until the AccessGraph or AccessGraphDemoMode
// entitlement becomes active, then returns nil. Returns a non-nil error only
// when the process is shutting down or ctx is canceled.
func waitForAccessGraphEntitlement(ctx context.Context, log *slog.Logger, auth conditionWatcher, module modules.Modules) error {
	for {
		retry, err := watchAccessGraphEntitlementOnce(ctx, log, auth, module)
		if err != nil || !retry {
			return trace.Wrap(err)
		}
		select {
		case <-time.After(1 * time.Minute):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

type conditionWatcher interface {
	NewWatcher(ctx context.Context, watch types.Watch) (types.Watcher, error)
}

// watchAccessGraphEntitlementOnce runs a single watch iteration. It returns (true, nil) to
// signal that the caller should retry with a fresh watcher, and (false, nil)
// when the condition is met.
func watchAccessGraphEntitlementOnce(ctx context.Context, log *slog.Logger, auth conditionWatcher, module modules.Modules) (retry bool, err error) {
	watcher, err := auth.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{Kind: types.KindAccessGraphSettings}},
	})
	if err != nil {
		log.ErrorContext(ctx, "Unable to start AccessGraphSettings watcher, will retry.", "error", err)
		return true, nil
	}
	defer watcher.Close()

	const timerInterval = 5 * time.Minute
	timer := time.NewTimer(timerInterval)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-timer.C:
			// Fallback: if the AccessGraph entitlement was activated (e.g. via a
			// license update) but the corresponding AccessGraphSettings event was
			// missed, detect it here so we don't wait indefinitely.
			if module.Features().GetEntitlement(entitlements.AccessGraph).Enabled {
				return false, nil
			}
			timer.Reset(timerInterval)
		case event := <-watcher.Events():
			if event.Type != types.OpPut {
				continue
			}
			unwrapper, ok := event.Resource.(types.Resource153UnwrapperT[*clusterconfigv1.AccessGraphSettings])
			if !ok {
				log.ErrorContext(ctx, "Received unknown type in AccessGraphSettings watcher", "kind", event.Resource.GetKind())
				continue
			}
			// because entitlements can change during the lifetime of the service, we have to check
			// if they are entitled to use AccessGraphDemoMode before turning demo mode on
			if !getDemoModeEnabled(unwrapper.UnwrapT(), module.Features()) {
				continue
			}
			return false, nil
		case <-watcher.Done():
			if err := ctx.Err(); err != nil {
				return false, err
			}
			return true, nil
		}
	}
}

func getDemoModeEnabled(accessGraphSettings *clusterconfigv1.AccessGraphSettings, features modules.Features) bool {
	return accessGraphSettings.GetSpec().GetDemoMode() == clusterconfigv1.AccessGraphDemoMode_ACCESS_GRAPH_DEMO_MODE_ENABLED && features.GetEntitlement(entitlements.AccessGraphDemoMode).Enabled
}

func watchAndPushCAs(ctx context.Context, log *slog.Logger, clusterName string, services *auth.Services, client accessgraphv1.AccessGraphServiceClient) error {
	watcher, err := services.NewWatcher(ctx, types.Watch{
		Kinds: []types.WatchKind{{
			Kind: types.KindCertAuthority,
			Filter: types.CertAuthorityFilter{
				types.HostCA: clusterName,
			}.IntoMap(),
		}},
	})
	if err != nil {
		return trace.Wrap(err)
	}
	defer watcher.Close()

	select {
	case initEvent := <-watcher.Events():
		if initEvent.Type != types.OpInit {
			return trace.BadParameter("watcher yielded %[1]v (%[1]d) as first event, expected Init (this is a bug)", initEvent.Type)
		}
	case <-watcher.Done():
		return trace.Wrap(watcher.Error())
	case <-ctx.Done():
		return ctx.Err()
	}

	log.DebugContext(ctx, "CA watcher initialized.")

	{
		const loadKeysFalse = false
		hostCA, err := services.GetCertAuthority(ctx, types.CertAuthID{
			Type:       types.HostCA,
			DomainName: clusterName,
		}, loadKeysFalse)
		if err != nil {
			return trace.Wrap(err)
		}
		if err := pushCA(ctx, hostCA, client); err != nil {
			return trace.Wrap(err)
		}
	}

	for {
		select {
		case caEvent := <-watcher.Events():
			if caEvent.Type != types.OpPut {
				return trace.BadParameter("watcher yielded %[1]v (%[1]d) as event, expected Put (this is a bug)", caEvent.Type)
			}
			ca, ok := caEvent.Resource.(types.CertAuthority)
			if !ok {
				return trace.BadParameter("expected cert authority from watcher, got %T (this is a bug)", ca)
			}
			if caName := ca.GetClusterName(); caName != clusterName {
				return trace.BadParameter("expected cert authority for cluster %v, got %q (this is a bug)", clusterName, caName)
			}
			if caType := ca.GetType(); caType != types.HostCA {
				return trace.BadParameter("expected host cert authority, got %q (this is a bug)", caType)
			}

			log.DebugContext(ctx, "Got CA event, sending to TAG.")
			if err := pushCA(ctx, ca, client); err != nil {
				return trace.Wrap(err)
			}
		case <-watcher.Done():
			return trace.Wrap(watcher.Error())

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func pushCA(ctx context.Context, hostCA types.CertAuthority, client accessgraphv1.AccessGraphServiceClient) error {
	// this is roughly hostCA.GetTrustedTLSKeyPairs but it only gets the
	// certificates in PEM
	activeKeys := hostCA.GetActiveKeys().TLS
	additionalTrustedKeys := hostCA.GetAdditionalTrustedKeys().TLS
	caPEMs := make([][]byte, 0, len(activeKeys)+len(additionalTrustedKeys))
	for _, k := range activeKeys {
		caPEMs = append(caPEMs, k.Cert)
	}
	for _, k := range additionalTrustedKeys {
		caPEMs = append(caPEMs, k.Cert)
	}

	if len(caPEMs) < 1 {
		return trace.BadParameter("no TLS certs in host CA")
	}
	if _, err := client.ReplaceCAs(ctx, accessgraphv1.ReplaceCAsRequest_builder{HostCaPem: caPEMs}.Build()); err != nil {
		return trace.Wrap(err)
	}
	return nil
}
