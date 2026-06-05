package sessionsearchv1

import (
	"context"
	"net"
	"reflect"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/gravitational/trace"
	gopenai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/testing/protocmp"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/sessionsearch/v1"
	summarizerpb "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	sumopenai "github.com/gravitational/teleport/e/lib/auth/summarizer/openai"
	accessgraphv1 "github.com/gravitational/teleport/gen/proto/go/accessgraph/v1"
	"github.com/gravitational/teleport/lib/authz"
	libevent "github.com/gravitational/teleport/lib/events"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/tlsca"
)

type fakeAuthorizer struct {
	ctx *authz.Context
	err error
}

func (a *fakeAuthorizer) Authorize(context.Context) (*authz.Context, error) {
	return a.ctx, a.err
}

// fakeChecker implements services.AccessChecker with configurable behavior
// for the two methods used by this service.
type fakeChecker struct {
	services.AccessChecker

	// roles controls HasRole responses for builtin-role authorization checks.
	roles []string
	// guessErr is returned by GuessIfAccessIsPossible (used in MaybeAccessToKind).
	guessErr error
	// guessFn allows tests to make GuessIfAccessIsPossible resource-aware.
	guessFn func(ctx services.RuleContext, namespace, resource, verb string) error
	// checkFn is called by CheckAccessToRule for per-session RBAC decisions.
	// If nil, all checks are allowed.
	checkFn func(ruleCtx services.RuleContext, namespace, resource, verb string) error
}

func (c fakeChecker) HasRole(role string) bool {
	return slices.Contains(c.roles, role)
}

func (c fakeChecker) RoleNames() []string {
	return append([]string(nil), c.roles...)
}

func (c fakeChecker) GuessIfAccessIsPossible(ctx services.RuleContext, namespace, resource, verb string) error {
	if c.guessFn != nil {
		return c.guessFn(ctx, namespace, resource, verb)
	}
	return c.guessErr
}

func (c fakeChecker) CheckAccessToRule(ruleCtx services.RuleContext, namespace, resource, verb string) error {
	if c.checkFn != nil {
		return c.checkFn(ruleCtx, namespace, resource, verb)
	}
	return nil
}

// allowAll returns a fakeChecker that permits every access check.
func allowAll() fakeChecker { return fakeChecker{} }

// denySessionRule returns a checker that passes the coarse-grained
// MaybeAccessToKind gate but denies every per-session CheckAccessToRule call.
func denySessionRule() fakeChecker {
	return fakeChecker{
		checkFn: func(_ services.RuleContext, _, resource, _ string) error {
			if resource == types.KindSession {
				return trace.AccessDenied("session access denied")
			}
			return nil
		},
	}
}

// allowSessionAccessOnly returns a checker that only grants coarse-grained
// session list/read access, plus per-session checks.
func allowSessionAccessOnly() fakeChecker {
	return fakeChecker{
		guessFn: func(_ services.RuleContext, _, resource, verb string) error {
			if resource == types.KindSession && (verb == types.VerbList || verb == types.VerbRead) {
				return nil
			}
			return trace.AccessDenied("%s %s denied", resource, verb)
		},
	}
}

// denySessionAccessOnly returns a checker that allows session list but not read.
func denySessionAccessOnly() fakeChecker {
	return fakeChecker{
		guessFn: func(_ services.RuleContext, _, resource, verb string) error {
			if resource == types.KindSession && verb == types.VerbList {
				return nil
			}
			return trace.AccessDenied("%s %s denied", resource, verb)
		},
	}
}

// userFilterChecker permits sessions whose session_end_event.user matches
// allowedUser and denies all others.
func userFilterChecker(allowedUser string) fakeChecker {
	return fakeChecker{
		checkFn: func(ruleCtx services.RuleContext, _, resource, _ string) error {
			if resource != types.KindSession {
				return nil
			}
			sctx, ok := ruleCtx.(*services.Context)
			if !ok || sctx.Session == nil {
				return trace.AccessDenied("no session context")
			}
			se, ok := sctx.Session.(*apievents.SessionEnd)
			if !ok {
				return trace.AccessDenied("unexpected session event type")
			}
			if se.User != allowedUser {
				return trace.AccessDenied("user %q not allowed", se.User)
			}
			return nil
		},
	}
}

// fakeStream captures Send calls for a server-streaming RPC.
type fakeStream[T any] struct {
	ctx     context.Context
	sent    []*T
	sendErr error
}

func (s *fakeStream[T]) Context() context.Context     { return s.ctx }
func (s *fakeStream[T]) SetHeader(metadata.MD) error  { return nil }
func (s *fakeStream[T]) SendHeader(metadata.MD) error { return nil }
func (s *fakeStream[T]) SetTrailer(metadata.MD)       {}
func (s *fakeStream[T]) SendMsg(any) error            { return nil }
func (s *fakeStream[T]) RecvMsg(any) error            { return nil }

func (s *fakeStream[T]) Send(msg *T) error {
	if s.sendErr != nil {
		return s.sendErr
	}
	s.sent = append(s.sent, msg)
	return nil
}

// agPage describes a single page the fake access graph server will return.
type agPage struct {
	summaries []*accessgraphv1.SessionSummary
	hasMore   bool
	// nextToken is set as the CheckpointToken on every summary in the page so
	// that the service can use it as the cross-stream resume cursor.
	nextToken string
}

// fakeAGServer implements accessgraphv1.SessionRecordingServiceServer.
// For each configured page it reads one request (SearchParams or FetchMore),
// sends individual summaries wrapped in SummaryAndCheckpoint, then sends the
// BatchComplete signal.
type fakeAGServer struct {
	accessgraphv1.UnimplementedSessionRecordingServiceServer
	pages   []agPage
	openErr error
	// receivedParams is set from the SearchParams payload of the first
	// received request and can be inspected after the call completes.
	receivedParams *accessgraphv1.SearchSessionSummariesParams
}

func (f *fakeAGServer) SearchSessionSummaries(
	stream grpc.BidiStreamingServer[accessgraphv1.SearchSessionSummariesRequest, accessgraphv1.SearchSessionSummariesResponse],
) error {
	if f.openErr != nil {
		return f.openErr
	}
	for _, page := range f.pages {
		req, err := stream.Recv()
		if err != nil {
			return err
		}
		if p := req.GetSearchParams(); p != nil && f.receivedParams == nil {
			f.receivedParams = p
		}
		for _, s := range page.summaries {
			if err := stream.Send(accessgraphv1.SearchSessionSummariesResponse_builder{
				Summary: accessgraphv1.SummaryAndCheckpoint_builder{
					Summary:         s,
					CheckpointToken: page.nextToken,
				}.Build(),
			}.Build()); err != nil {
				return err
			}
		}
		if err := stream.Send(accessgraphv1.SearchSessionSummariesResponse_builder{
			BatchComplete: accessgraphv1.SearchSessionSummariesResponse_BatchComplete_builder{
				HasMore: page.hasMore,
			}.Build(),
		}.Build()); err != nil {
			return err
		}
	}
	return nil
}

// startAGServer starts an in-process gRPC server backed by srv, registers a
// cleanup to stop it, and returns a getter that produces a connected client.
func startAGServer(t *testing.T, srv *fakeAGServer) func() (accessgraphv1.SessionRecordingServiceClient, error) {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() {
		lis.Close()
	})
	grpcSrv := grpc.NewServer()
	accessgraphv1.RegisterSessionRecordingServiceServer(grpcSrv, srv)
	go grpcSrv.Serve(lis) //nolint:errcheck // ignore err
	t.Cleanup(grpcSrv.Stop)

	conn, err := grpc.NewClient("passthrough:", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}))
	require.NoError(t, err)
	t.Cleanup(func() { conn.Close() })
	return func() (accessgraphv1.SessionRecordingServiceClient, error) {
		return accessgraphv1.NewSessionRecordingServiceClient(conn), nil
	}
}

// makeSessionSummary constructs an accessgraphv1.SessionSummary with a
// minimal session_end_event that can be parsed by events.FromEventFields.
// The user field in the event is set to login.
func makeSessionSummary(t *testing.T, id, login string) *accessgraphv1.SessionSummary {
	t.Helper()
	endEvent, err := structpb.NewStruct(map[string]any{
		libevent.EventType: libevent.SessionEndEvent, // "event": "session.end"
		"user":             login,
		"code":             "T2004I",
		"namespace":        "default",
	})
	require.NoError(t, err)
	return accessgraphv1.SessionSummary_builder{
		SessionId:       id,
		Kind:            "ssh",
		Username:        login,
		SessionEndEvent: endEvent,
	}.Build()
}

// makeAuthCtx creates an authz.Context for user "testuser" using the
// provided checker.
func makeAuthCtx(t *testing.T, checker services.AccessChecker) *authz.Context {
	t.Helper()
	user, err := types.NewUser("testuser")
	require.NoError(t, err)
	return &authz.Context{
		User:    user,
		Checker: checker,
		Identity: authz.LocalUser{
			Username: "testuser",
			Identity: tlsca.Identity{},
		},
	}
}

// baseRequest returns a valid SearchSessionSummariesRequest with sensible
// defaults that callers can override.
func baseRequest() *pb.SearchSessionSummariesRequest {
	from := time.Now().Add(-time.Hour)
	to := time.Now()
	return pb.SearchSessionSummariesRequest_builder{
		StartTime: timestamppb.New(from),
		EndTime:   timestamppb.New(to),
	}.Build()
}

// newService builds a Service backed by srv, using startAGServer to spin up
// an in-process gRPC server for the stream tests.
func newService(t *testing.T, srv *fakeAGServer, authorizer authz.Authorizer) *Service {
	t.Helper()
	svc, err := NewService(ServiceConfig{
		Authorizer:              authorizer,
		Cache:                   fakeCache{},
		AccessGraphClientGetter: startAGServer(t, srv),
		AvailabilityCache:       &fakeAvailabilityChecker{},
		IsLicensed:              func() bool { return true },
	})
	require.NoError(t, err)
	return svc
}

// collectSummaryIDs returns the IDs of all summary messages in order.
func collectSummaryIDs(responses []*pb.SearchSessionSummariesResponse) []string {
	var ids []string
	for _, r := range responses {
		if s := r.GetSummary(); s != nil {
			ids = append(ids, s.GetSessionId())
		}
	}
	return ids
}

// fakeCache is a minimal no-op implementation of services.Summarizer.
type fakeCache struct{ services.Summarizer }

// fakeAvailabilityChecker implements AvailabilityChecker for tests.
// It always reports AVAILABLE unless overridden.
type fakeAvailabilityChecker struct {
	availability accessgraphv1.SessionSearchAvailability
	err          error
}

func (f *fakeAvailabilityChecker) Get(_ context.Context) (accessgraphv1.SessionSearchAvailability, error) {
	if f.err != nil {
		return accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_UNSPECIFIED, f.err
	}
	a := f.availability
	if a == accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_UNSPECIFIED {
		a = accessgraphv1.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_AVAILABLE
	}
	return a, nil
}

func TestNewService_Validation(t *testing.T) {
	t.Parallel()

	getter := startAGServer(t, &fakeAGServer{})
	authorizer := &fakeAuthorizer{}

	tests := []struct {
		name    string
		cfg     ServiceConfig
		wantErr string
	}{
		{
			name:    "missing authorizer",
			cfg:     ServiceConfig{Cache: fakeCache{}, AccessGraphClientGetter: getter},
			wantErr: "authorizer is required",
		},
		{
			name:    "missing cache",
			cfg:     ServiceConfig{Authorizer: authorizer, AccessGraphClientGetter: getter},
			wantErr: "backend is required",
		},
		{
			name:    "missing access graph client",
			cfg:     ServiceConfig{Authorizer: authorizer, Cache: fakeCache{}},
			wantErr: "access graph client getter is required",
		},
		{
			name:    "missing availability cache",
			cfg:     ServiceConfig{Authorizer: authorizer, Cache: fakeCache{}, AccessGraphClientGetter: getter},
			wantErr: "availability cache is required",
		},
		{
			name:    "missing is licensed",
			cfg:     ServiceConfig{Authorizer: authorizer, Cache: fakeCache{}, AccessGraphClientGetter: getter, AvailabilityCache: &fakeAvailabilityChecker{}},
			wantErr: "is licensed function is required",
		},
		{
			name: "all required fields present",
			cfg: ServiceConfig{
				Authorizer:              authorizer,
				Cache:                   fakeCache{},
				AccessGraphClientGetter: getter,
				AvailabilityCache:       &fakeAvailabilityChecker{},
				IsLicensed:              func() bool { return true },
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewService(tc.cfg)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateRequest(t *testing.T) {
	t.Parallel()

	now := time.Now()
	from := now.Add(-time.Hour)
	to := now

	tests := []struct {
		name    string
		req     *pb.SearchSessionSummariesRequest
		wantErr string
	}{
		{
			name:    "missing start_time",
			req:     pb.SearchSessionSummariesRequest_builder{EndTime: timestamppb.New(to)}.Build(),
			wantErr: "start_time is required",
		},
		{
			name:    "zero start_time",
			req:     pb.SearchSessionSummariesRequest_builder{StartTime: timestamppb.New(time.Time{}), EndTime: timestamppb.New(to)}.Build(),
			wantErr: "start_time is required",
		},
		{
			name:    "missing end_time",
			req:     pb.SearchSessionSummariesRequest_builder{StartTime: timestamppb.New(from)}.Build(),
			wantErr: "end_time is required",
		},
		{
			name:    "zero end_time",
			req:     pb.SearchSessionSummariesRequest_builder{StartTime: timestamppb.New(from), EndTime: timestamppb.New(time.Time{})}.Build(),
			wantErr: "end_time is required",
		},
		{
			name: "start after end",
			req: pb.SearchSessionSummariesRequest_builder{
				StartTime: timestamppb.New(to),
				EndTime:   timestamppb.New(from),
			}.Build(),
			wantErr: "must not be after",
		},
		{
			name: "max_results above limit",
			req: pb.SearchSessionSummariesRequest_builder{
				StartTime:  timestamppb.New(from),
				EndTime:    timestamppb.New(to),
				MaxResults: maxPageSize + 1,
			}.Build(),
			wantErr: "exceeds maximum",
		},
		{
			name: "search_queries above limit",
			req: pb.SearchSessionSummariesRequest_builder{
				StartTime:     timestamppb.New(from),
				EndTime:       timestamppb.New(to),
				SearchQueries: make([]string, maxSearchQueries+1),
			}.Build(),
			wantErr: "search_queries count",
		},
		{
			name: "valid minimal request",
			req:  pb.SearchSessionSummariesRequest_builder{StartTime: timestamppb.New(from), EndTime: timestamppb.New(to)}.Build(),
		},
		{
			name: "valid with max_results at boundary",
			req: pb.SearchSessionSummariesRequest_builder{
				StartTime:  timestamppb.New(from),
				EndTime:    timestamppb.New(to),
				MaxResults: maxPageSize,
			}.Build(),
		},
		{
			name: "valid with search_queries at boundary",
			req: pb.SearchSessionSummariesRequest_builder{
				StartTime:     timestamppb.New(from),
				EndTime:       timestamppb.New(to),
				SearchQueries: make([]string, maxSearchQueries),
			}.Build(),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validateRequest(tc.req)
			if tc.wantErr != "" {
				require.ErrorContains(t, err, tc.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestAuthorizeIsEnabled(t *testing.T) {
	t.Parallel()

	proxyUser, err := types.NewUser("proxy")
	require.NoError(t, err)

	tests := []struct {
		name    string
		authCtx *authz.Context
		assert  func(*testing.T, error)
	}{
		{
			name: "proxy builtin role allowed",
			authCtx: &authz.Context{
				User:             proxyUser,
				Checker:          fakeChecker{roles: []string{string(types.RoleProxy)}},
				Identity:         authz.BuiltinRole{Role: types.RoleProxy, Username: "proxy"},
				UnmappedIdentity: authz.BuiltinRole{Role: types.RoleProxy, Username: "proxy"},
			},
			assert: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "admin builtin role allowed",
			authCtx: &authz.Context{
				User:             proxyUser,
				Checker:          fakeChecker{roles: []string{string(types.RoleAdmin)}},
				Identity:         authz.BuiltinRole{Role: types.RoleAdmin, Username: "admin"},
				UnmappedIdentity: authz.BuiltinRole{Role: types.RoleAdmin, Username: "admin"},
			},
			assert: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "non proxy service denied",
			authCtx: &authz.Context{
				User:             proxyUser,
				Checker:          fakeChecker{roles: []string{string(types.RoleNode)}},
				Identity:         authz.BuiltinRole{Role: types.RoleNode, Username: "node"},
				UnmappedIdentity: authz.BuiltinRole{Role: types.RoleNode, Username: "node"},
			},
			assert: func(t *testing.T, err error) {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			},
		},
		{
			name:    "user with session access allowed",
			authCtx: makeAuthCtx(t, allowSessionAccessOnly()),
			assert: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name:    "user without session access denied",
			authCtx: makeAuthCtx(t, denySessionAccessOnly()),
			assert: func(t *testing.T, err error) {
				require.Error(t, err)
				require.True(t, trace.IsAccessDenied(err))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.assert(t, authorizeIsEnabled(tc.authCtx))
		})
	}
}

func TestSearchSessionSummaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		pages       []agPage
		agErr       error
		auth        authz.Authorizer
		req         *pb.SearchSessionSummariesRequest
		wantIDs     []string
		wantErr     func(*testing.T, error)
		checkStream func(*testing.T, []*pb.SearchSessionSummariesResponse)
	}{
		{
			name:    "unauthorized",
			auth:    &fakeAuthorizer{err: trace.AccessDenied("not authorized")},
			wantErr: func(t *testing.T, err error) { require.True(t, trace.IsAccessDenied(err)) },
		},
		{
			name:    "no session permission",
			auth:    &fakeAuthorizer{ctx: makeAuthCtx(t, fakeChecker{guessErr: trace.AccessDenied("no session perm")})},
			pages:   []agPage{{summaries: []*accessgraphv1.SessionSummary{makeSessionSummary(t, "s1", "alice")}}},
			wantErr: func(t *testing.T, err error) { require.True(t, trace.IsAccessDenied(err)) },
		},
		{
			name:    "single page all approved",
			auth:    &fakeAuthorizer{ctx: makeAuthCtx(t, allowAll())},
			pages:   []agPage{{summaries: []*accessgraphv1.SessionSummary{makeSessionSummary(t, "session-1", "alice"), makeSessionSummary(t, "session-2", "bob")}}},
			wantIDs: []string{"session-1", "session-2"},
		},
		{
			name:  "single page all denied",
			auth:  &fakeAuthorizer{ctx: makeAuthCtx(t, denySessionRule())},
			pages: []agPage{{summaries: []*accessgraphv1.SessionSummary{makeSessionSummary(t, "s1", "alice"), makeSessionSummary(t, "s2", "bob")}}},
		},
		{
			name:    "no session end event filtered",
			auth:    &fakeAuthorizer{ctx: makeAuthCtx(t, allowAll())},
			pages:   []agPage{{summaries: []*accessgraphv1.SessionSummary{accessgraphv1.SessionSummary_builder{SessionId: "no-event", Kind: "ssh"}.Build(), makeSessionSummary(t, "with-event", "alice")}}},
			wantIDs: []string{"with-event"},
		},
		{
			name:    "per-session RBAC filtering",
			auth:    &fakeAuthorizer{ctx: makeAuthCtx(t, userFilterChecker("alice"))},
			pages:   []agPage{{summaries: []*accessgraphv1.SessionSummary{makeSessionSummary(t, "alice-session", "alice"), makeSessionSummary(t, "bob-session", "bob"), makeSessionSummary(t, "alice-session-2", "alice")}}},
			wantIDs: []string{"alice-session", "alice-session-2"},
		},
		{
			name: "multi-page",
			auth: &fakeAuthorizer{ctx: makeAuthCtx(t, allowAll())},
			pages: []agPage{
				{summaries: []*accessgraphv1.SessionSummary{makeSessionSummary(t, "s1", "alice"), makeSessionSummary(t, "s2", "alice")}, hasMore: true},
				{summaries: []*accessgraphv1.SessionSummary{makeSessionSummary(t, "s3", "alice")}},
			},
			req:     func() *pb.SearchSessionSummariesRequest { r := baseRequest(); r.SetMaxResults(10); return r }(),
			wantIDs: []string{"s1", "s2", "s3"},
		},
		{
			name: "quota reached forwards cursor",
			auth: &fakeAuthorizer{ctx: makeAuthCtx(t, allowAll())},
			pages: []agPage{{
				summaries: []*accessgraphv1.SessionSummary{makeSessionSummary(t, "s1", "alice"), makeSessionSummary(t, "s2", "alice")},
				hasMore:   true,
				nextToken: "resume-cursor",
			}},
			req:     func() *pb.SearchSessionSummariesRequest { r := baseRequest(); r.SetMaxResults(2); return r }(),
			wantIDs: []string{"s1", "s2"},
			checkStream: func(t *testing.T, sent []*pb.SearchSessionSummariesResponse) {
				assert.Equal(t, "resume-cursor", sent[len(sent)-1].GetBatchComplete().GetNextBatchToken())
			},
		},
		{
			name:  "empty page",
			auth:  &fakeAuthorizer{ctx: makeAuthCtx(t, allowAll())},
			pages: []agPage{{hasMore: false}},
		},
		{
			name:    "AG stream open error",
			auth:    &fakeAuthorizer{ctx: makeAuthCtx(t, allowAll())},
			agErr:   trace.ConnectionProblem(nil, "ag unavailable"),
			wantErr: func(t *testing.T, err error) { require.ErrorContains(t, err, "ag unavailable") },
		},
		{
			name:    "invalid request",
			auth:    &fakeAuthorizer{ctx: makeAuthCtx(t, allowAll())},
			req:     &pb.SearchSessionSummariesRequest{},
			wantErr: func(t *testing.T, err error) { require.True(t, trace.IsBadParameter(err)) },
		},
		{
			name: "all denied multi-page",
			auth: &fakeAuthorizer{ctx: makeAuthCtx(t, denySessionRule())},
			pages: []agPage{
				{summaries: []*accessgraphv1.SessionSummary{makeSessionSummary(t, "s1", "bob")}, hasMore: true},
				{summaries: []*accessgraphv1.SessionSummary{makeSessionSummary(t, "s2", "bob")}},
			},
			req: func() *pb.SearchSessionSummariesRequest { r := baseRequest(); r.SetMaxResults(10); return r }(),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			req := tc.req
			if req == nil {
				req = baseRequest()
			}
			svc := newService(t, &fakeAGServer{pages: tc.pages, openErr: tc.agErr}, tc.auth)
			stream := &fakeStream[pb.SearchSessionSummariesResponse]{ctx: t.Context()}
			err := svc.SearchSessionSummaries(req, stream)
			if tc.wantErr != nil {
				require.Error(t, err)
				tc.wantErr(t, err)
				assert.Empty(t, stream.sent)
				return
			}
			require.NoError(t, err)
			assert.ElementsMatch(t, tc.wantIDs, collectSummaryIDs(stream.sent))
			if tc.checkStream != nil {
				tc.checkStream(t, stream.sent)
			}
		})
	}
}

func TestConvertResourceProperties(t *testing.T) {
	t.Parallel()

	hostname := "web-01"
	addr := "10.0.0.1:22"
	ns := "default"
	podName := "nginx-abc"
	dbName := "teleport"

	tests := []struct {
		name string
		src  *pb.ResourceProperties
		want *accessgraphv1.ResourceProperties
	}{
		{
			name: "nil input",
			src:  nil,
			want: nil,
		},
		{
			name: "SSH",
			src: pb.ResourceProperties_builder{
				Ssh: pb.SSHProperties_builder{
					ServerHostname: &hostname,
					ServerAddr:     &addr,
				}.Build(),
			}.Build(),
			want: accessgraphv1.ResourceProperties_builder{
				Ssh: accessgraphv1.SSHProperties_builder{
					ServerHostname: &hostname,
					ServerAddr:     &addr,
				}.Build(),
			}.Build(),
		},
		{
			name: "Kubernetes",
			src: pb.ResourceProperties_builder{
				Kubernetes: pb.KubernetesProperties_builder{
					PodNamespace: &ns,
					PodName:      &podName,
				}.Build(),
			}.Build(),
			want: accessgraphv1.ResourceProperties_builder{
				Kubernetes: accessgraphv1.KubernetesProperties_builder{
					PodNamespace: &ns,
					PodName:      &podName,
				}.Build(),
			}.Build(),
		},
		{
			name: "Database",
			src: pb.ResourceProperties_builder{
				Database: pb.DatabaseProperties_builder{
					DatabaseName: &dbName,
				}.Build(),
			}.Build(),
			want: accessgraphv1.ResourceProperties_builder{
				Database: accessgraphv1.DatabaseProperties_builder{
					DatabaseName: &dbName,
				}.Build(),
			}.Build(),
		},
		{
			name: "unknown variant returns nil",
			src:  &pb.ResourceProperties{},
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := convertResourceProperties(tc.src)
			diff := cmp.Diff(tc.want, got,
				protocmp.Transform(),
				cmpopts.EquateEmpty(),
			)
			assert.Empty(t, diff)
		})
	}
}

func TestConvertSummary(t *testing.T) {
	t.Parallel()

	labels := map[string]string{"env": "prod"}
	traits, err := structpb.NewStruct(map[string]any{"department": "eng"})
	require.NoError(t, err)
	sessionEndEvent, err := structpb.NewStruct(map[string]any{"event": "session.end"})
	require.NoError(t, err)

	src := accessgraphv1.SessionSummary_builder{
		SessionId:        "abc-123",
		Kind:             "ssh",
		Username:         "alice",
		UserRoles:        []string{"admin"},
		AccessRequestIds: []string{"req-1"},
		ResourceKind:     "node",
		ResourceId:       "node-id",
		ResourceName:     "web-01",
		ResourceLabels:   labels,
		ResourceProperties: accessgraphv1.ResourceProperties_builder{
			Ssh: &accessgraphv1.SSHProperties{},
		}.Build(),
		UserTraits: traits,
		// SessionEndEvent must NOT appear in the output.
		SessionEndEvent: sessionEndEvent,
	}.Build()

	got := convertSummary(src)

	assert.Equal(t, "abc-123", got.GetSessionId())
	assert.Equal(t, "ssh", got.GetKind())
	assert.Equal(t, "alice", got.GetUsername())
	assert.Equal(t, []string{"admin"}, got.GetUserRoles())
	assert.Equal(t, []string{"req-1"}, got.GetAccessRequestIds())
	assert.Equal(t, "node", got.GetResourceKind())
	assert.Equal(t, "node-id", got.GetResourceId())
	assert.Equal(t, "web-01", got.GetResourceName())
	assert.Equal(t, labels, got.GetResourceLabels())
	require.NotNil(t, got.GetResourceProperties(), "resource_properties should be converted")
	assert.NotNil(t, got.GetResourceProperties().GetSsh(), "SSH variant should be preserved")
}

// fakeCacheWithEmbeddings extends fakeCache to return a configurable
// RetrievalModel and InferenceSecret for embedding generation tests.
type fakeCacheWithEmbeddings struct {
	fakeCache
	model  *summarizerpb.RetrievalModel
	secret *summarizerpb.InferenceSecret
}

func (c *fakeCacheWithEmbeddings) GetRetrievalModel(_ context.Context) (*summarizerpb.RetrievalModel, error) {
	return c.model, nil
}

func (c *fakeCacheWithEmbeddings) GetInferenceSecret(_ context.Context, _ string) (*summarizerpb.InferenceSecret, error) {
	return c.secret, nil
}

// fakeOpenAIClientFactory creates fakeOpenAIEmbeddingClient instances.
type fakeOpenAIClientFactory struct{}

func (fakeOpenAIClientFactory) NewClient(_ ...option.RequestOption) sumopenai.Client {
	return &fakeOpenAIEmbeddingClient{}
}

// fakeOpenAIEmbeddingClient returns a fixed 3-dimensional embedding vector.
type fakeOpenAIEmbeddingClient struct{}

func (*fakeOpenAIEmbeddingClient) GenerateEmbeddings(
	_ context.Context, _ gopenai.EmbeddingNewParams, _ ...option.RequestOption,
) (*gopenai.CreateEmbeddingResponse, error) {
	return &gopenai.CreateEmbeddingResponse{
		Data:  []gopenai.Embedding{{Embedding: []float64{0.1, 0.2, 0.3}}},
		Usage: gopenai.CreateEmbeddingResponseUsage{TotalTokens: 3},
	}, nil
}

func (*fakeOpenAIEmbeddingClient) NewChatCompletion(
	_ context.Context, _ gopenai.ChatCompletionNewParams, _ ...option.RequestOption,
) (*gopenai.ChatCompletion, error) {
	panic("NewChatCompletion not expected in embedding tests")
}

// newEmbeddingsCache returns a fakeCacheWithEmbeddings pre-populated with a
// standard OpenAI retrieval model and a fake API key, used by embedding tests.
func newEmbeddingsCache() *fakeCacheWithEmbeddings {
	return &fakeCacheWithEmbeddings{
		model: summarizerpb.RetrievalModel_builder{
			Metadata: headerv1.Metadata_builder{Name: "my-model"}.Build(),
			Spec: summarizerpb.RetrievalModelSpec_builder{
				Openai: summarizerpb.OpenAIProvider_builder{
					OpenaiModelId:   "text-embedding-ada-002",
					ApiKeySecretRef: "my-secret",
				}.Build(),
			}.Build(),
		}.Build(),
		secret: summarizerpb.InferenceSecret_builder{
			Spec: summarizerpb.InferenceSecretSpec_builder{Value: "fake-api-key"}.Build(),
		}.Build(),
	}
}

func TestSearchSessionSummaries_EmbeddingsGenerated(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	agSrv := &fakeAGServer{
		pages: []agPage{
			{summaries: []*accessgraphv1.SessionSummary{makeSessionSummary(t, "s1", "alice")}, hasMore: false},
		},
	}
	cache := newEmbeddingsCache()

	svc, err := NewService(ServiceConfig{
		Authorizer:              &fakeAuthorizer{ctx: makeAuthCtx(t, allowAll())},
		Cache:                   cache,
		AccessGraphClientGetter: startAGServer(t, agSrv),
		AvailabilityCache:       &fakeAvailabilityChecker{},
		OpenAIClientFactory:     fakeOpenAIClientFactory{},
		IsLicensed:              func() bool { return true },
	})
	require.NoError(t, err)

	req := baseRequest()
	req.SetSearchQueries([]string{"find ssh sessions"})
	stream := &fakeStream[pb.SearchSessionSummariesResponse]{ctx: ctx}
	require.NoError(t, svc.SearchSessionSummaries(req, stream))

	// Verify the AG server received search params with the embedded query.
	require.NotNil(t, agSrv.receivedParams, "access graph should have received search params")
	require.Len(t, agSrv.receivedParams.GetSearchQueries(), 1)
	q := agSrv.receivedParams.GetSearchQueries()[0]
	assert.Equal(t, "find ssh sessions", q.GetText())
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, q.GetEmbeddings())
	assert.Equal(t, "my-model", q.GetModelName())

	// Verify the session was forwarded to the caller.
	assert.Equal(t, []string{"s1"}, collectSummaryIDs(stream.sent))
}

// trackingOpenAIClientFactory wraps fakeOpenAIClientFactory but counts
// how many times GenerateEmbeddings is called across all clients it creates.
type trackingOpenAIClientFactory struct {
	calls atomic.Int64
}

func (f *trackingOpenAIClientFactory) NewClient(_ ...option.RequestOption) sumopenai.Client {
	return &trackingOpenAIEmbeddingClient{factory: f}
}

type trackingOpenAIEmbeddingClient struct {
	factory *trackingOpenAIClientFactory
}

func (c *trackingOpenAIEmbeddingClient) GenerateEmbeddings(
	_ context.Context, _ gopenai.EmbeddingNewParams, _ ...option.RequestOption,
) (*gopenai.CreateEmbeddingResponse, error) {
	c.factory.calls.Add(1)
	return &gopenai.CreateEmbeddingResponse{
		Data:  []gopenai.Embedding{{Embedding: []float64{0.1, 0.2, 0.3}}},
		Usage: gopenai.CreateEmbeddingResponseUsage{TotalTokens: 3},
	}, nil
}

func (c *trackingOpenAIEmbeddingClient) NewChatCompletion(
	_ context.Context, _ gopenai.ChatCompletionNewParams, _ ...option.RequestOption,
) (*gopenai.ChatCompletion, error) {
	panic("NewChatCompletion not expected in search mode tests")
}

func newServiceWithTracking(t *testing.T, srv *fakeAGServer, factory *trackingOpenAIClientFactory) *Service {
	t.Helper()
	svc, err := NewService(ServiceConfig{
		Authorizer:              &fakeAuthorizer{ctx: makeAuthCtx(t, allowAll())},
		Cache:                   newEmbeddingsCache(),
		AccessGraphClientGetter: startAGServer(t, srv),
		AvailabilityCache:       &fakeAvailabilityChecker{},
		OpenAIClientFactory:     factory,
		IsLicensed:              func() bool { return true },
	})
	require.NoError(t, err)
	return svc
}

func TestSearchSessionSummaries_KeywordOnlySkipsEmbeddings(t *testing.T) {
	t.Parallel()
	ctx := t.Context()

	agSrv := &fakeAGServer{
		pages: []agPage{
			{summaries: []*accessgraphv1.SessionSummary{makeSessionSummary(t, "s1", "alice")}, hasMore: false},
		},
	}
	factory := &trackingOpenAIClientFactory{}
	svc := newServiceWithTracking(t, agSrv, factory)

	req := baseRequest()
	req.SetSearchQueries([]string{"find ssh sessions"})
	req.SetSearchMode(pb.SearchMode_SEARCH_MODE_KEYWORD_ONLY)
	stream := &fakeStream[pb.SearchSessionSummariesResponse]{ctx: ctx}
	require.NoError(t, svc.SearchSessionSummaries(req, stream))

	assert.Equal(t, int64(0), factory.calls.Load(), "KEYWORD_ONLY should not call the embedding provider")

	require.NotNil(t, agSrv.receivedParams)
	require.Len(t, agSrv.receivedParams.GetSearchQueries(), 1)
	assert.Equal(t, "find ssh sessions", agSrv.receivedParams.GetSearchQueries()[0].GetText())
	assert.Empty(t, agSrv.receivedParams.GetSearchQueries()[0].GetEmbeddings(), "KEYWORD_ONLY should send no embeddings")
	assert.Equal(t, accessgraphv1.SearchMode_SEARCH_MODE_KEYWORD_ONLY, agSrv.receivedParams.GetSearchMode())
}

func TestSearchSessionSummaries_SearchModeForwarded(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mode         pb.SearchMode
		expectedMode accessgraphv1.SearchMode
		wantEmbeds   bool
	}{
		{
			name:         "unspecified forwards as unspecified and generates embeddings",
			mode:         pb.SearchMode_SEARCH_MODE_UNSPECIFIED,
			expectedMode: accessgraphv1.SearchMode_SEARCH_MODE_UNSPECIFIED,
			wantEmbeds:   true,
		},
		{
			name:         "hybrid forwards as hybrid and generates embeddings",
			mode:         pb.SearchMode_SEARCH_MODE_HYBRID,
			expectedMode: accessgraphv1.SearchMode_SEARCH_MODE_HYBRID,
			wantEmbeds:   true,
		},
		{
			name:         "embedding-only forwards and generates embeddings",
			mode:         pb.SearchMode_SEARCH_MODE_EMBEDDING_ONLY,
			expectedMode: accessgraphv1.SearchMode_SEARCH_MODE_EMBEDDING_ONLY,
			wantEmbeds:   true,
		},
		{
			name:         "keyword-only forwards and skips embeddings",
			mode:         pb.SearchMode_SEARCH_MODE_KEYWORD_ONLY,
			expectedMode: accessgraphv1.SearchMode_SEARCH_MODE_KEYWORD_ONLY,
			wantEmbeds:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := t.Context()
			agSrv := &fakeAGServer{
				pages: []agPage{
					{summaries: []*accessgraphv1.SessionSummary{makeSessionSummary(t, "s1", "alice")}, hasMore: false},
				},
			}
			factory := &trackingOpenAIClientFactory{}
			svc := newServiceWithTracking(t, agSrv, factory)

			req := baseRequest()
			req.SetSearchQueries([]string{"lateral movement"})
			req.SetSearchMode(tc.mode)
			stream := &fakeStream[pb.SearchSessionSummariesResponse]{ctx: ctx}
			require.NoError(t, svc.SearchSessionSummaries(req, stream))

			require.NotNil(t, agSrv.receivedParams)
			assert.Equal(t, tc.expectedMode, agSrv.receivedParams.GetSearchMode())
			require.Len(t, agSrv.receivedParams.GetSearchQueries(), 1)
			assert.Equal(t, "lateral movement", agSrv.receivedParams.GetSearchQueries()[0].GetText())
			if tc.wantEmbeds {
				assert.Greater(t, factory.calls.Load(), int64(0), "mode %v should generate embeddings", tc.mode)
			} else {
				assert.Equal(t, int64(0), factory.calls.Load(), "mode %v should not generate embeddings", tc.mode)
			}
		})
	}
}

// TestServiceUnlicensed uses reflection to verify that every method declared by
// [pb.SessionSearchServiceServer] returns a "not licensed" response when
// [ServiceConfig.IsLicensed] returns false. This ensures that newly added
// endpoints cannot accidentally bypass the entitlement check.
func TestServiceUnlicensed(t *testing.T) {
	t.Parallel()

	adminCtx, err := authz.NewBuiltinRoleContext(types.RoleAdmin)
	require.NoError(t, err)

	svc, err := NewService(ServiceConfig{
		Authorizer: authz.AuthorizerFunc(func(context.Context) (*authz.Context, error) {
			return adminCtx, nil
		}),
		Cache: fakeCache{},
		AccessGraphClientGetter: func() (accessgraphv1.SessionRecordingServiceClient, error) {
			panic("access graph client must not be called when unlicensed")
		},
		AvailabilityCache: &fakeAvailabilityChecker{},
		IsLicensed:        func() bool { return false },
	})
	require.NoError(t, err)

	ctx := context.Background()
	svcType := reflect.TypeFor[*Service]()
	svcVal := reflect.ValueOf(svc)
	ctxIfaceType := reflect.TypeFor[context.Context]()

	// SearchSessionSummaries validates the request before checking the license,
	// so supply a valid but minimal request to ensure the license guard is reached.
	now := time.Now()
	validSearchReq := pb.SearchSessionSummariesRequest_builder{
		StartTime: timestamppb.New(now.Add(-time.Hour)),
		EndTime:   timestamppb.New(now),
	}.Build()

	tested := 0
	for i := range svcType.NumMethod() {
		m := svcType.Method(i)
		if !m.IsExported() {
			continue
		}
		if m.Type.NumIn() < 3 {
			continue
		}
		isUnary := m.Type.In(1).Implements(ctxIfaceType)

		var results []reflect.Value
		if isUnary {
			reqType := m.Type.In(2)
			results = svcVal.MethodByName(m.Name).Call([]reflect.Value{
				reflect.ValueOf(ctx),
				reflect.New(reqType.Elem()),
			})
		} else {
			stream := &fakeStream[pb.SearchSessionSummariesResponse]{ctx: ctx}
			results = svcVal.MethodByName(m.Name).Call([]reflect.Value{
				reflect.ValueOf(validSearchReq),
				reflect.ValueOf(stream),
			})
		}

		t.Run(m.Name, func(t *testing.T) {
			if m.Name == "IsEnabled" {
				// IsEnabled communicates "unlicensed" through the response body
				// (UNSPECIFIED availability) so callers can distinguish feature
				// absence from a transient error without treating either as fatal.
				require.True(t, results[1].IsNil(), "expected no error from IsEnabled, got: %v", results[1].Interface())
				resp, ok := results[0].Interface().(*pb.IsEnabledResponse)
				require.True(t, ok)
				require.Equal(t, pb.SessionSearchAvailability_SESSION_SEARCH_AVAILABILITY_UNSPECIFIED, resp.GetAvailability())
				return
			}

			errVal := results[0]
			require.False(t, errVal.IsNil(), "expected Unimplemented error from %s, got nil", m.Name)
			callErr := errVal.Interface().(error)
			st, ok := status.FromError(callErr)
			require.True(t, ok, "expected gRPC status error from %s, got: %v", m.Name, callErr)
			require.Equal(t, codes.Unimplemented, st.Code(),
				"expected Unimplemented from %s, got %v: %v", m.Name, st.Code(), callErr)
		})
		tested++
	}
	require.GreaterOrEqual(t, tested, 2, "reflection found fewer methods than expected — interface may have shrunk")
}
