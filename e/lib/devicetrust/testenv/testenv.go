package testenv

import (
	"context"
	"encoding/pem"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"

	"github.com/gravitational/teleport/api/client/proto"
	devicepb "github.com/gravitational/teleport/api/gen/proto/go/teleport/devicetrust/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/devicetrust/devicetrustv1"
	"github.com/gravitational/teleport/e/lib/devicetrust/storage"
	"github.com/gravitational/teleport/lib/auth"
	"github.com/gravitational/teleport/lib/backend"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/modules"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/utils"
)

// AugmentContextCertsFunc mimics the signature of
// [auth.Server.AugmentContextUserCertificates].
type AugmentContextCertsFunc func(ctx context.Context, authCtx *auth.Context, opts *auth.AugmentUserCertificateOpts) (*proto.Certs, error)

// E is an integrated test environment for device trust.
type E struct {
	DevicesClient devicepb.DeviceTrustServiceClient

	augmentCertsFunc AugmentContextCertsFunc
	authSpec         *types.AuthPreferenceSpecV2
	authorizer       auth.Authorizer
	emitter          apievents.Emitter
	closers          []func() error
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

// WithAuthorizer customizes the [E] authorizer.
func WithAuthorizer(a auth.Authorizer) Opt {
	return func(e *E) { e.authorizer = a }
}

// WithEmitter customizes the [E] event emitter.
func WithEmitter(em apievents.Emitter) Opt {
	return func(e *E) { e.emitter = em }
}

// MustNew creates a new [E] or panics.
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

	// A few device trust endpoints (like authn) indirectly check for Enterprise.
	// It's a bit silly in this module (since this is teleport.e), but those
	// checks come from OSS code.
	modules.SetTestModules(t, &modules.TestModules{
		TestBuildType: modules.BuildEnterprise,
	})

	return env
}

// New creates a new [E].
func New(opts ...Opt) (*E, error) {
	e := &E{
		augmentCertsFunc: fakeAugmentCertsFunc,
		// A non-nil spec is good enough for most tests.
		authSpec:   &types.AuthPreferenceSpecV2{},
		authorizer: &noopAuthorizer{},
		emitter:    &noopEmitter{},
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

	dtStorage, err := storage.New(func() backend.Backend { return mem })
	if err != nil {
		return nil, err
	}

	// Device service.
	dtV1, err := devicetrustv1.New(devicetrustv1.ServiceParams{
		AuthServer: &fakeAuthServer{
			augmentFunc: e.augmentCertsFunc,
			authSpec:    e.authSpec,
		},
		Authorizer: e.authorizer,
		Emitter:    e.emitter,
		Storage:    dtStorage,
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
		grpc.StreamInterceptor(utils.GRPCServerStreamErrorInterceptor),
		grpc.UnaryInterceptor(utils.GRPCServerUnaryErrorInterceptor),
	)
	e.closers = append(e.closers, func() error {
		s.GracefulStop()
		s.Stop()
		return nil
	})

	// Register service.
	devicepb.RegisterDeviceTrustServiceServer(s, dtV1)

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
		grpc.WithStreamInterceptor(utils.GRPCClientStreamErrorInterceptor),
		grpc.WithUnaryInterceptor(utils.GRPCClientUnaryErrorInterceptor),
	)
	if err != nil {
		return nil, err
	}
	e.closers = append(e.closers, cc.Close)
	e.DevicesClient = devicepb.NewDeviceTrustServiceClient(cc)

	ok = true
	return e, nil
}

func fakeAugmentCertsFunc(ctx context.Context, authCtx *auth.Context, opts *auth.AugmentUserCertificateOpts) (*proto.Certs, error) {
	return &proto.Certs{
		SSH: opts.SSHAuthorizedKey,
		TLS: pem.EncodeToMemory(&pem.Block{
			Type:  "CERTIFICATE",
			Bytes: []byte("<insert TLS cert here>"),
		}),
	}, nil
}

type fakeAuthServer struct {
	augmentFunc AugmentContextCertsFunc
	authSpec    *types.AuthPreferenceSpecV2
}

func (s *fakeAuthServer) AugmentContextUserCertificates(ctx context.Context, authCtx *auth.Context, opts *auth.AugmentUserCertificateOpts) (*proto.Certs, error) {
	return s.augmentFunc(ctx, authCtx, opts)
}

func (s *fakeAuthServer) GetAuthPreference(ctx context.Context) (types.AuthPreference, error) {
	return types.NewAuthPreference(*s.authSpec)
}

type noopAuthorizer struct{}

func (*noopAuthorizer) Authorize(ctx context.Context) (*auth.Context, error) {
	user, err := types.NewUser("llama")
	if err != nil {
		return nil, err
	}
	return &auth.Context{
		User:    user,
		Checker: &noopChecker{},
	}, nil
}

type noopChecker struct {
	services.AccessChecker
}

func (*noopChecker) CheckAccessToRule(ruleCtx services.RuleContext, namespace string, rule string, verb string, silent bool) error {
	return nil // Anything goes.
}

type noopEmitter struct{}

func (*noopEmitter) EmitAuditEvent(context.Context, apievents.AuditEvent) error {
	return nil
}
