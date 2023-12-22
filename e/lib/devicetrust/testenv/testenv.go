package testenv

import (
	"context"
	"encoding/pem"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gravitational/oxy/ratelimit"
	"github.com/gravitational/trace"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport/api/client/proto"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/utils/grpc/interceptors"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustv1"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
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

// AnonymizeAndSubmitFunc mimics the signature of
// [auth.Server.AnonymizeAndSubmit].
type AnonymizeAndSubmitFunc func(event ...usagereporter.Anonymizable)

// E is an integrated test environment for device trust.
type E struct {
	DevicesClient devicepb.DeviceTrustServiceClient
	// DevicesService is the underlying devicetrustv1.Service.
	// Most callers should test through DevicesClient instead.
	DevicesService  *devicetrustv1.Service
	IdentityService *local.IdentityService

	augmentCertsFunc       AugmentContextCertsFunc
	authSpec               *types.AuthPreferenceSpecV2
	anonymizeAndSubmitFunc AnonymizeAndSubmitFunc
	authorizer             authz.Authorizer
	emitter                apievents.Emitter
	limiter                devicetrustv1.RateLimiter
	closers                []func() error
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

// WithAugmentCertsFunc customizes the [E] augment certs function.
func WithAugmentCertsFunc(f AugmentContextCertsFunc) Opt {
	return func(e *E) { e.augmentCertsFunc = f }
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

// MustNew creates a new [E] or panics.
// Prefer [NewUsingT], as it configures [modules.TestModules] automatically.
func MustNew(opts ...Opt) *E {
	env, err := New(opts...)
	if err != nil {
		panic(err)
	}
	return env
}

// NewUsingT creates a new [E] using t to report failures or register the
// appropriate cleanups.
// Additionally, it also sets [modules.SetTestModules] to an Enterprise build
// type.
func NewUsingT(t *testing.T, opts ...Opt) *E {
	env, err := New(opts...)
	if err != nil {
		t.Fatalf("Failed to create testenv.E: %v", err)
	}
	t.Cleanup(func() { _ = env.Close() })

	// Set the build to Enterprise (required by a few OSS checks) and enable the
	// device trust feature.
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
		TestFeatures: modules.Features{
			DeviceTrust: modules.DeviceTrustFeature{
				Enabled: true,
			},
		},
	})

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

	e.IdentityService = local.NewIdentityService(mem)
	dtStorage, err := storage.New(storage.Params{
		Backend:      mem,
		UsersService: e.IdentityService,
	})
	if err != nil {
		return nil, err
	}

	logger := log.New()
	logger.SetLevel(log.PanicLevel) // Silence logging for tests.

	// Device service.
	e.DevicesService, err = devicetrustv1.New(devicetrustv1.ServiceParams{
		Logger: logger,
		AuthServer: &fakeAuthServer{
			augmentFunc:            e.augmentCertsFunc,
			authSpec:               e.authSpec,
			anonymizeAndSubmitFunc: e.anonymizeAndSubmitFunc,
		},
		Authorizer:         e.authorizer,
		CachedUsersService: e.IdentityService,
		Emitter:            e.emitter,
		Limiter:            e.limiter,
		Storage:            dtStorage,
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
		grpc.StreamInterceptor(interceptors.GRPCServerStreamErrorInterceptor),
		grpc.UnaryInterceptor(interceptors.GRPCServerUnaryErrorInterceptor),
	)
	e.closers = append(e.closers, func() error {
		s.GracefulStop()
		s.Stop()
		return nil
	})

	// Register service.
	devicepb.RegisterDeviceTrustServiceServer(s, e.DevicesService)

	// Start.
	go func() {
		if err := s.Serve(lis); err != nil {
			// TODO(codingllama): Be more subtle?
			panic(fmt.Sprintf("Serve returned err = %v", err))
		}
	}()

	// gRPC client.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cc, err := grpc.DialContext(ctx, "unused",
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
	authSpec               *types.AuthPreferenceSpecV2
	anonymizeAndSubmitFunc AnonymizeAndSubmitFunc
}

func (s *fakeAuthServer) AugmentContextUserCertificates(ctx context.Context, authCtx *authz.Context, opts *auth.AugmentUserCertificateOpts) (*proto.Certs, error) {
	return s.augmentFunc(ctx, authCtx, opts)
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
		User:                  user,
		Checker:               &NoopChecker{},
		AdminActionAuthorized: true,
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

func (*NoopChecker) CheckAccessToRule(ruleCtx services.RuleContext, namespace string, rule string, verb string, silent bool) error {
	return nil // Anything goes.
}

type noopEmitter struct{}

func (*noopEmitter) EmitAuditEvent(context.Context, apievents.AuditEvent) error {
	return nil
}

type noopLimiter struct{}

func (*noopLimiter) RegisterRequest(token string, customRate *ratelimit.RateSet) error {
	return nil
}
