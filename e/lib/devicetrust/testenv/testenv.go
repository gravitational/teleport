package testenv

import (
	"context"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"testing"

	"github.com/gravitational/trace"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport/api/client/proto"
	devicetrustpublicv1pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/public/v1"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustpublicv1"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustv1"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/entitlements"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	osstestenv "github.com/gravitational/teleport/lib/devicetrust/testenv"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/modules/modulestest"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	usagereporter "github.com/gravitational/teleport/lib/usagereporter/teleport"
)

// DefaultUser is the default RPC caller, when not using a custom
// [WithAuthorizer].
const DefaultUser = "llama"

// AugmentContextCertsFunc mimics the signature of
// [auth.Server.AugmentContextUserCertificates].
type AugmentContextCertsFunc func(ctx context.Context, authCtx *authz.Context, opts *auth.AugmentUserCertificateOpts) (*proto.Certs, error)

// AugmentWebSessionCertsFunc mimics the signature of
// [auth.Server.AugmentWebSessionCertificates].
type AugmentWebSessionCertsFunc func(ctx context.Context, opts *auth.AugmentWebSessionCertificatesOpts) error

// AnonymizeAndSubmitFunc mimics the signature of
// [auth.Server.AnonymizeAndSubmit].
type AnonymizeAndSubmitFunc func(event ...usagereporter.Anonymizable)

// E is an integrated test environment for device trust.
type E struct {
	DevicesClient devicepb.DeviceTrustServiceClient
	// DevicesService is the underlying devicetrustv1.Service.
	// Most callers should test through DevicesClient instead.
	DevicesService *devicetrustv1.Service
	// PublicDevicesClient is the client for the public (unauthenticated) Device
	// Trust service.
	PublicDevicesClient devicetrustpublicv1pb.DeviceTrustServiceClient
	// PublicDevicesService is the underlying devicetrustpublicv1.Service.
	// Most callers should test through PublicDevicesClient instead.
	PublicDevicesService *devicetrustpublicv1.Service
	EnrollPairing        services.EnrollPairing
	AccessService        *local.AccessService
	IdentityService      *local.IdentityService

	augmentCertsFunc       AugmentContextCertsFunc
	augmentWebFunc         AugmentWebSessionCertsFunc
	authSpec               *types.AuthPreferenceSpecV2
	anonymizeAndSubmitFunc AnonymizeAndSubmitFunc
	authorizer             authz.Authorizer
	emitter                apievents.Emitter
	limiter                devicetrustv1.RateLimiter
	closers                []func() error
	modules                *modulestest.Modules
}

// Close tears down the test environment.
func (e *E) Close() error {
	var errs []error
	for i := len(e.closers) - 1; i >= 0; i-- {
		fn := e.closers[i]
		if err := fn(); err != nil {
			errs = append(errs, err)
		}
	}
	return trace.NewAggregate(errs...)
}

// Opt is a creation option for [E].
type Opt func(*E)

// WithModules customizes the [E] modules.
func WithModules(m *modulestest.Modules) Opt {
	return func(e *E) { e.modules = m }
}

// WithAugmentCertsFunc customizes the [E] augment certs function.
func WithAugmentCertsFunc(f AugmentContextCertsFunc) Opt {
	return func(e *E) { e.augmentCertsFunc = f }
}

// WithAugmentWebFunc customizes the [E] augment web function.
func WithAugmentWebFunc(f AugmentWebSessionCertsFunc) Opt {
	return func(e *E) { e.augmentWebFunc = f }
}

// WithAuthPreferenceSpec customizes the underlying [E] auth preference spec.
func WithAuthPreferenceSpec(spec types.AuthPreferenceSpecV2) Opt {
	return func(e *E) { e.authSpec = &spec }
}

// WithAnonymizeAndSubmitFunc customizes the underlying [E] anonymizeAndSubmit function.
func WithAnonymizeAndSubmitFunc(f AnonymizeAndSubmitFunc) Opt {
	return func(e *E) { e.anonymizeAndSubmitFunc = f }
}

// WithAuthorizer customizes the [E] authorizer.
func WithAuthorizer(a authz.Authorizer) Opt {
	return func(e *E) { e.authorizer = a }
}

// WithEmitter customizes the [E] event emitter.
func WithEmitter(em apievents.Emitter) Opt {
	return func(e *E) { e.emitter = em }
}

// WithLimiter customizes the [E] rate limiter.
func WithLimiter(l devicetrustv1.RateLimiter) Opt {
	return func(e *E) { e.limiter = l }
}

// WithEnrollPairing customizes the [E] enroll pairing service.
func WithEnrollPairing(s services.EnrollPairing) Opt {
	return func(e *E) { e.EnrollPairing = s }
}

// NewUsingT creates a new [E] using t to report failures or register the
// appropriate cleanups.
func NewUsingT(t *testing.T, opts ...Opt) *E {
	env, err := New(opts...)
	if err != nil {
		t.Fatalf("Failed to create testenv.E: %v", err)
	}
	t.Cleanup(func() { _ = env.Close() })

	return env
}

// New creates a new [E].
func New(opts ...Opt) (*E, error) {
	e := &E{
		augmentCertsFunc: fakeAugmentCertsFunc,
		// A non-nil spec is good enough for most tests.
		authSpec:               &types.AuthPreferenceSpecV2{},
		anonymizeAndSubmitFunc: fakeAnonymizeAndSubmitFunc,
		authorizer:             &noopAuthorizer{},
		emitter:                &noopEmitter{},
		limiter:                &noopLimiter{},
		modules: &modulestest.Modules{
			TestBuildType: modules.BuildEnterprise,
			TestFeatures: modules.Features{
				Entitlements: map[entitlements.EntitlementKind]modules.EntitlementInfo{
					entitlements.DeviceTrust:            {Enabled: true},
					entitlements.MobileDeviceManagement: {Enabled: true},
				},
			},
		},
	}
	for _, opt := range opts {
		opt(e)
	}

	ok := false
	defer func() {
		if !ok {
			e.Close()
		}
	}()

	// Device backend and storage.
	mem, err := memory.New(memory.Config{})
	if err != nil {
		return nil, err
	}
	e.closers = append(e.closers, mem.Close)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelError + 1, // Silence logging for tests.
	}))

	e.AccessService = local.NewAccessService(mem)
	e.IdentityService, err = local.NewIdentityService(mem)
	if err != nil {
		return nil, err
	}
	dtStorage, err := storage.New(storage.Params{
		Logger:             logger,
		Backend:            mem,
		UsersService:       e.IdentityService,
		BCryptCostOverride: bcrypt.MinCost,
		Modules:            e.modules,
	})
	if err != nil {
		return nil, err
	}

	if e.EnrollPairing == nil {
		if e.EnrollPairing, err = local.NewEnrollPairingService(mem); err != nil {
			return nil, err
		}
	}

	// Device service.
	e.DevicesService, err = devicetrustv1.New(devicetrustv1.ServiceParams{
		Logger: logger,
		AuthServer: &fakeAuthServer{
			augmentFunc:            e.augmentCertsFunc,
			augmentWebFunc:         e.augmentWebFunc,
			authSpec:               e.authSpec,
			anonymizeAndSubmitFunc: e.anonymizeAndSubmitFunc,
		},
		Authorizer:          e.authorizer,
		CachedAccessService: e.AccessService,
		CachedUsersService:  e.IdentityService,
		Emitter:             e.emitter,
		Limiter:             e.limiter,
		Storage:             dtStorage,
		EnrollPairing:       e.EnrollPairing,
		Modules:             e.modules,
	})
	if err != nil {
		return nil, err
	}

	// Public (unauthenticated) device service.
	e.PublicDevicesService, err = devicetrustpublicv1.New(devicetrustpublicv1.ServiceParams{
		Logger:        logger,
		EnrollPairing: e.EnrollPairing,
		Storage:       dtStorage,
		Authorizer:    e.authorizer,
		CachedUsers:   e.IdentityService,
		Emitter:       e.emitter,
		Modules:       e.modules,
	})
	if err != nil {
		return nil, err
	}

	// gRPC server.
	const bufSize = 100 // arbitrary
	lis := bufconn.Listen(bufSize)
	e.closers = append(e.closers, lis.Close)

	s := grpc.NewServer(
		// Options below are similar to auth.GRPCServer.
		grpc.ChainStreamInterceptor(
			interceptors.GRPCServerStreamErrorInterceptor,
			clientSourceAddrStreamInterceptor, // for testing only
		),
		grpc.ChainUnaryInterceptor(
			interceptors.GRPCServerUnaryErrorInterceptor,
			clientSourceAddrUnaryInterceptor, // for testing only
		),
	)
	e.closers = append(e.closers, func() error {
		s.GracefulStop()
		s.Stop()
		return nil
	})

	// Register services.
	devicepb.RegisterDeviceTrustServiceServer(s, e.DevicesService)
	devicetrustpublicv1pb.RegisterDeviceTrustServiceServer(s, e.PublicDevicesService)

	// Start.
	go func() {
		// s.Stop may run before this goroutine is scheduled when a test finishes
		// quickly. Serve then returns ErrServerStopped, which is a clean shutdown
		// rather than a failure.
		if err := s.Serve(lis); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			// TODO(codingllama): Be more subtle?
			panic(fmt.Sprintf("Serve returned err = %v", err))
		}
	}()

	// gRPC client.
	cc, err := grpc.NewClient("passthrough:///bufconn",
		grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
			return lis.DialContext(ctx)
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithStreamInterceptor(interceptors.GRPCClientStreamErrorInterceptor),
		grpc.WithUnaryInterceptor(interceptors.GRPCClientUnaryErrorInterceptor),
	)
	if err != nil {
		return nil, err
	}
	e.closers = append(e.closers, cc.Close)
	e.DevicesClient = devicepb.NewDeviceTrustServiceClient(cc)
	e.PublicDevicesClient = devicetrustpublicv1pb.NewDeviceTrustServiceClient(cc)

	ok = true
	return e, nil
}

func fakeAugmentCertsFunc(ctx context.Context, authCtx *authz.Context, opts *auth.AugmentUserCertificateOpts) (*proto.Certs, error) {
	return &proto.Certs{
		SSH: opts.SSHAuthorizedKey,
		TLS: pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: []byte("<insert TLS cert here>"),
		}),
	}, nil
}

func fakeAnonymizeAndSubmitFunc(event ...usagereporter.Anonymizable) {}

type fakeAuthServer struct {
	augmentFunc            AugmentContextCertsFunc
	augmentWebFunc         AugmentWebSessionCertsFunc // may be nil
	authSpec               *types.AuthPreferenceSpecV2
	anonymizeAndSubmitFunc AnonymizeAndSubmitFunc
}

func (s *fakeAuthServer) AugmentContextUserCertificates(ctx context.Context, authCtx *authz.Context, opts *auth.AugmentUserCertificateOpts) (*proto.Certs, error) {
	return s.augmentFunc(ctx, authCtx, opts)
}

func (s *fakeAuthServer) AugmentWebSessionCertificates(ctx context.Context, opts *auth.AugmentWebSessionCertificatesOpts) error {
	if s.augmentWebFunc == nil {
		return nil
	}
	return s.augmentWebFunc(ctx, opts)
}

func (s *fakeAuthServer) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return types.NewAuthPreference(*s.authSpec)
}

func (s *fakeAuthServer) AnonymizeAndSubmit(event ...usagereporter.Anonymizable) {
	s.anonymizeAndSubmitFunc(event...)
}

type noopAuthorizer struct{}

func (*noopAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	user, err := types.NewUser(DefaultUser)
	if err != nil {
		return nil, err
	}
	return &authz.Context{
		User:                 user,
		Checker:              &NoopChecker{},
		AdminActionAuthState: authz.AdminActionAuthNotRequired,
	}, nil
}

// NoopChecker is a [services.AccessChecker] that does nothing for most methods
// and allows all access.
// It is the baseline for methods used by the [devicetrustv1.Service].
type NoopChecker struct {
	services.AccessChecker
}

func (*NoopChecker) HasRole(role string) bool {
	return false // Don't panic.
}

func (*NoopChecker) RoleNames() []string {
	return nil // Don't panic.
}

func (*NoopChecker) Roles() []types.Role {
	return nil // Don't panic.
}

func (*NoopChecker) CheckAccessToRule(ruleCtx services.RuleContext, namespace string, rule string, verb string) error {
	return nil // Anything goes.
}

type noopEmitter struct{}

func (*noopEmitter) EmitAuditEvent(context.Context, apievents.AuditEvent) error {
	return nil
}

type noopLimiter struct{}

func (*noopLimiter) RegisterRequest(token string) error {
	return nil
}

// NewSelfSignedSSHCert returns a self-signed SSH certificates in
// authorized-keys format, intended to be used as the user SSH certificate in
// authn tests.
var NewSelfSignedSSHCert = osstestenv.NewSelfSignedSSHCert
