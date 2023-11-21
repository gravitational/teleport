package secreports

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/e/lib/secreports/query"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/services"
)

type mockChecker struct {
	services.AccessChecker
	rules []types.Rule
	roles []string
}

func (f *mockChecker) CheckAccessToRule(context services.RuleContext, namespace string, kind string, verb string, silent bool) error {
	for _, r := range f.rules {
		if r.HasResource(kind) && r.HasVerb(verb) {
			return nil
		}
	}
	return trace.AccessDenied("access to %s with verb %s is not allowed", kind, verb)
}

// HasRole checks if the checker includes the role
func (f *mockChecker) HasRole(target string) bool {
	for _, role := range f.roles {
		if role == target {
			return true
		}
	}
	return false
}

type mockAuthorizer struct {
	checker *mockChecker
}

func (m mockAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	return &authz.Context{
		Checker: m.checker,
	}, nil
}

type mockSemaphore struct {
	lease types.SemaphoreLease
	types.Semaphores
}

func (m *mockSemaphore) AcquireSemaphore(ctx context.Context, params types.AcquireSemaphoreRequest) (*types.SemaphoreLease, error) {
	return &m.lease, nil
}

func (m *mockSemaphore) CancelSemaphoreLease(ctx context.Context, lease types.SemaphoreLease) error {
	return nil
}

type runQueryFuncType func(ctx context.Context, queryText string, days int) (*query.RunQueryResponse, error)

type athenaMock struct {
	runQueryFunc   runQueryFuncType
	getQueryResult func(ctx context.Context, queryID, nextToken string, maxResults int32) (*query.GetQueryResultResponse, error)
}

func (a *athenaMock) RunQuery(ctx context.Context, query string, days int) (*query.RunQueryResponse, error) {
	return a.runQueryFunc(ctx, query, days)
}

func (a *athenaMock) GetQueryResult(ctx context.Context, queryID, nextToken string, maxResults int32) (*query.GetQueryResultResponse, error) {
	return a.getQueryResult(ctx, queryID, nextToken, maxResults)
}

type mockReportStore struct {
	m   map[string]*pb.ReportResult
	mtx sync.Mutex
}

func (m *mockReportStore) SaveReportResult(ctx context.Context, name string, result *pb.ReportResult) error {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	m.m[name] = result
	return nil
}

func (m *mockReportStore) LoadReportResult(ctx context.Context, name string) (*pb.ReportResult, error) {
	m.mtx.Lock()
	defer m.mtx.Unlock()
	if v, ok := m.m[name]; ok {
		return v, nil
	}
	return nil, trace.NotFound("not found")
}

func assertReportVersion(t *testing.T, store services.SecReports, name, wantVersion string) {
	ctx := context.Background()
	r, err := store.GetSecurityReport(ctx, name)
	require.NoError(t, err)
	require.Equal(t, wantVersion, r.Spec.Version)
}

func mustClone[T any](t *testing.T, src T) T {
	data, err := json.Marshal(src)
	require.NoError(t, err)
	var dst T
	err = json.Unmarshal(data, &dst)
	require.NoError(t, err)
	return dst
}

type mockEmitter struct {
}

func (m mockEmitter) EmitAuditEvent(ctx context.Context, event apievents.AuditEvent) error {
	return nil
}
