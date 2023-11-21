package secreports

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/secreports/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/secreports"
	"github.com/gravitational/teleport/e/lib/secreports/limiter"
	"github.com/gravitational/teleport/e/lib/secreports/query"
	"github.com/gravitational/teleport/e/lib/secreports/reports"
	"github.com/gravitational/teleport/e/lib/secreports/scheduler"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

func TestService(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()

	memory, err := memory.New(memory.Config{
		Clock:   clock,
		Context: ctx,
	})
	require.NoError(t, err)

	store, err := local.NewSecReportsService(memory, clock)
	require.NoError(t, err)

	mockAthena := &athenaMock{
		runQueryFunc: func(ctx context.Context, queryTest string, days int) (*query.RunQueryResponse, error) {
			return &query.RunQueryResponse{
				ResultID: "1234",
			}, nil
		},
		getQueryResult: func(ctx context.Context, queryID, nextToken string, maxResults int32) (*query.GetQueryResultResponse, error) {
			return &query.GetQueryResultResponse{
				QueryID: "1234",
			}, nil
		},
	}
	svc := Service{
		backend: memory,
		log:     logrus.New(),
		authorizer: &mockAuthorizer{
			checker: &mockChecker{
				rules: []types.Rule{{Resources: []string{"security_report"}, Verbs: []string{"read", "list", "use"}}},
				roles: nil,
			},
		},
		semaphore: &mockSemaphore{},
		clock:     clock,
		storage:   store,
		athena:    mockAthena,
		emitter:   &mockEmitter{},
		reportStore: &mockReportStore{
			m: map[string]*pb.ReportResult{},
		},
		ParentCtx: context.Background(),
	}
	err = svc.initPrebuiltReports(ctx)
	require.NoError(t, err)

	t.Run("get security report ", func(t *testing.T) {
		report, err := svc.GetReport(ctx, &pb.GetReportRequest{Name: reports.PrivilegeAccessReport.Name})
		require.NoError(t, err)
		require.Equal(t, reports.PrivilegeAccessReport.Name, report.Spec.Name)
		require.Len(t, report.Spec.GetAuditQueries(), len(reports.PrivilegeAccessReport.Queries))
	})

	t.Run("get security report ", func(t *testing.T) {
		resp, err := svc.ListReports(ctx, &pb.ListReportsRequest{})
		require.NoError(t, err)
		require.Len(t, resp.Reports, 1)
	})

	t.Run("run security report", func(t *testing.T) {
		clock.Advance(time.Hour)
		_, err := svc.GetReportState(ctx, &pb.GetReportStateRequest{
			Name: reports.PrivilegeAccessReport.Name,
			Days: 7,
		})
		require.True(t, trace.IsNotFound(err))

		_, err = svc.RunReport(ctx, &pb.RunReportRequest{
			Name: reports.PrivilegeAccessReport.Name,
			Days: 7,
		})
		require.NoError(t, err)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			statusResp, err := svc.GetReportState(ctx, &pb.GetReportStateRequest{
				Name: reports.PrivilegeAccessReport.Name,
				Days: 7,
			})
			assert.NoError(t, err)
			assert.Equal(t, string(secreports.Ready), statusResp.Spec.State)
			assert.Equal(t, clock.Now().UTC().Format(time.RFC3339), statusResp.Spec.UpdatedAt)
		}, time.Second*2, time.Millisecond*100)
	})

	t.Run("run security report with error", func(t *testing.T) {
		clock.Advance(time.Hour)
		queryFunc := func(ctx context.Context, queryText string, days int) (*query.RunQueryResponse, error) {
			return nil, trace.BadParameter("failed to run query")
		}
		mustRunReportAndWaitForAllQueries(t, mockAthena, &svc, queryFunc)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			statusResp, err := svc.GetReportState(ctx, &pb.GetReportStateRequest{
				Name: reports.PrivilegeAccessReport.Name,
				Days: 7,
			})
			assert.NoError(t, err)
			assert.Equal(t, string(secreports.Failed), statusResp.Spec.State)
		}, time.Second*2, time.Millisecond*100)
	})

	t.Run("run report in sequence", func(t *testing.T) {
		clock.Advance(time.Hour)
		queryFunc := func(ctx context.Context, queryText string, days int) (*query.RunQueryResponse, error) {
			return &query.RunQueryResponse{
				ResultID: "1234",
			}, nil
		}
		mustRunReportAndWaitForAllQueries(t, mockAthena, &svc, queryFunc)

		timeFirstRun := clock.Now().UTC().Format(time.RFC3339)
		require.EventuallyWithT(t, func(t *assert.CollectT) {
			statusResp, err := svc.GetReportState(ctx, &pb.GetReportStateRequest{
				Name: reports.PrivilegeAccessReport.Name,
				Days: 7,
			})
			assert.NoError(t, err)
			assert.Equal(t, string(secreports.Ready), statusResp.Spec.State)
			assert.Equal(t, timeFirstRun, statusResp.Spec.UpdatedAt)

		}, time.Second*2, time.Millisecond*100)

		clock.Advance(time.Second * 10)
		_, err = svc.RunReport(ctx, &pb.RunReportRequest{
			Name: reports.PrivilegeAccessReport.Name,
			Days: 7,
		})
		require.NoError(t, err)

		statusResp, err := svc.GetReportState(ctx, &pb.GetReportStateRequest{
			Name: reports.PrivilegeAccessReport.Name,
			Days: 7,
		})
		require.NoError(t, err)
		require.Equal(t, string(secreports.Ready), statusResp.Spec.State)
		require.Equal(t, timeFirstRun, statusResp.Spec.UpdatedAt)

	})

	t.Run("try to run report on Running state", func(t *testing.T) {
		done := make(chan struct{})
		clock.Advance(time.Hour)
		mockAthena.runQueryFunc = func(ctx context.Context, queryText string, days int) (*query.RunQueryResponse, error) {
			select {
			case <-time.After(time.Second * 10):
				t.Fatal("timeout")
			case <-done:
			}
			return &query.RunQueryResponse{
				ResultID: "1234",
			}, nil
		}

		go func() {
			_, err = svc.RunReport(ctx, &pb.RunReportRequest{
				Name: reports.PrivilegeAccessReport.Name,
				Days: 7,
			})
			assert.NoError(t, err)
		}()

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			statusResp, err := svc.GetReportState(ctx, &pb.GetReportStateRequest{
				Name: reports.PrivilegeAccessReport.Name,
				Days: 7,
			})
			assert.NoError(t, err)
			assert.Equal(t, string(secreports.Running), statusResp.Spec.State)
			assert.Equal(t, clock.Now().UTC().Format(time.RFC3339), statusResp.Spec.UpdatedAt)
		}, time.Second*2, time.Millisecond*100)

		clock.Advance(time.Second * 10)
		close(done)

		require.EventuallyWithT(t, func(t *assert.CollectT) {
			statusResp, err := svc.GetReportState(ctx, &pb.GetReportStateRequest{
				Name: reports.PrivilegeAccessReport.Name,
				Days: 7,
			})
			assert.NoError(t, err)
			assert.Equal(t, string(secreports.Ready), statusResp.Spec.State)
			assert.Equal(t, clock.Now().UTC().Format(time.RFC3339), statusResp.Spec.UpdatedAt)
		}, time.Second*2, time.Millisecond*100)
	})

}

func TestUpsertSecurityReport(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	clock := clockwork.NewFakeClock()
	memory, err := memory.New(memory.Config{
		Clock:   clock,
		Context: ctx,
	})
	require.NoError(t, err)

	store, err := local.NewSecReportsService(memory, clock)
	require.NoError(t, err)
	svc := Service{
		backend:   memory,
		log:       logrus.New(),
		semaphore: &mockSemaphore{},
		storage:   store,
		clock:     clockwork.NewFakeClock(),
	}

	t.Run("get security report empty store", func(t *testing.T) {
		_, err := store.GetSecurityReport(ctx, reports.PrivilegeAccessReport.Name)
		require.True(t, trace.IsNotFound(err))
	})

	t.Run("upsert security report", func(t *testing.T) {
		err := svc.initPrebuiltReports(ctx)
		require.NoError(t, err)
		wantVersion := reports.PrivilegeAccessReport.Version
		assertReportVersion(t, store, reports.PrivilegeAccessReport.Name, wantVersion)
	})

	t.Run("updated report version unchanged", func(t *testing.T) {
		cpy := mustClone(t, reports.PrivilegeAccessReport)
		cpy.Version = "0.0.1"
		err := svc.maybeUpdateReport(ctx, cpy)
		require.NoError(t, err)
		wantVersion := reports.PrivilegeAccessReport.Version
		assertReportVersion(t, store, reports.PrivilegeAccessReport.Name, wantVersion)
	})

	t.Run("updated report new version", func(t *testing.T) {
		cpy := mustClone(t, reports.PrivilegeAccessReport)
		updatedVersion := "0.0.10"
		cpy.Version = updatedVersion
		err := svc.maybeUpdateReport(ctx, cpy)
		require.NoError(t, err)
		assertReportVersion(t, store, reports.PrivilegeAccessReport.Name, updatedVersion)
	})
}

var (
	days30                 = time.Hour * 24 * 30
	perReportRunQueryCount = int64(len(reportValidDaysRange))
)

func TestScheduleReportUpdate(t *testing.T) {
	s := newSuite(t)
	ctx := context.Background()

	t.Run("first run", func(t *testing.T) {
		next, err := s.sched.Next(ctx)
		require.NoError(t, err)
		require.Equal(t, s.clock.Now(), next)

		err = s.svc.schedulesReportsUpdate(ctx)
		require.NoError(t, err)

		require.Equal(t, perReportRunQueryCount, s.runQueryCallCount.Load())

		details := s.mustGetDetails(t)
		wantCurrent := 300
		require.Equal(t, uint64(wantCurrent), details.Current)
	})

	t.Run("second run 30% capacity", func(t *testing.T) {
		before := s.runQueryCallCount.Load()
		err := s.svc.schedulesReportsUpdate(ctx)
		require.NoError(t, err)
		require.Equal(t, before, s.runQueryCallCount.Load())

		details := s.mustGetDetails(t)
		p := float64(details.Current) / float64(details.Limit)
		dur := time.Duration(float64(days30) * p)
		s.clock.Advance(dur)

		err = s.svc.schedulesReportsUpdate(ctx)
		require.NoError(t, err)
		require.Equal(t, before+3, s.runQueryCallCount.Load())
	})

	t.Run("second run 100% capacity", func(t *testing.T) {
		s.updateCurrentLimiterUsage(t, 1000)

		before := s.runQueryCallCount.Load()
		err := s.svc.schedulesReportsUpdate(ctx)
		require.NoError(t, err)
		require.Equal(t, before, s.runQueryCallCount.Load())

		details := s.mustGetDetails(t)
		s.clock.Advance(details.End.Sub(s.clock.Now()))

		err = s.svc.schedulesReportsUpdate(ctx)
		require.NoError(t, err)
		require.Equal(t, before, s.runQueryCallCount.Load())

		s.clock.Advance(time.Minute)

		err = s.svc.schedulesReportsUpdate(ctx)
		require.NoError(t, err)
		require.Equal(t, before, s.runQueryCallCount.Load())

	})
}

type suite struct {
	svc               Service
	sched             *scheduler.Scheduler
	runQueryCallCount *atomic.Int64
	lim               *limiter.Limiter
	clock             clockwork.FakeClock
	costLimiterStore  services.CostLimiter
}

func (s *suite) updateCurrentLimiterUsage(t *testing.T, curr uint64) {
	ctx := context.Background()
	lim, err := s.costLimiterStore.GetCostLimiter(ctx, s.lim.Name)
	require.NoError(t, err)
	lim.Spec.BytesScanned = curr
	err = s.costLimiterStore.UpsertCostLimiter(context.Background(), lim)
	require.NoError(t, err)
}

func (s *suite) mustGetDetails(t *testing.T) *limiter.Details {
	details, err := s.lim.GetDetails(context.Background())
	require.NoError(t, err)
	return details
}

func newSuite(t *testing.T) *suite {
	ctx := context.Background()
	clock := clockwork.NewFakeClockAt(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC))

	memory, err := memory.New(memory.Config{
		Clock:   clock,
		Context: ctx,
	})
	require.NoError(t, err)

	store, err := local.NewSecReportsService(memory, clock)
	require.NoError(t, err)

	queryDataScannedInBytes := int64(100)
	runQueryResp := &query.RunQueryResponse{
		ResultID:           "1234",
		DataScannedInBytes: queryDataScannedInBytes,
	}

	var runQueryCallCount atomic.Int64
	mockAthena := &athenaMock{
		runQueryFunc: func(ctx context.Context, queryTest string, days int) (*query.RunQueryResponse, error) {
			runQueryCallCount.Add(1)
			return runQueryResp, nil
		},
		getQueryResult: func(ctx context.Context, queryID, nextToken string, maxResults int32) (*query.GetQueryResultResponse, error) {
			return &query.GetQueryResultResponse{
				QueryID: "1234",
			}, nil
		},
	}

	lim, err := limiter.NewLimiter(limiter.Config{
		Store:              store,
		Semaphore:          &mockSemaphore{},
		Clock:              clock,
		RefillAfter:        days30,
		PreAllocationValue: 100,
		TotalLimit:         1000,
	})
	require.NoError(t, err)

	sched, err := scheduler.New(scheduler.Config{
		Limiter:     lim,
		Clock:       clock,
		MinInterval: time.Hour,
	})
	require.NoError(t, err)

	svc := Service{
		backend:   memory,
		Scheduler: sched,
		log:       logrus.New(),
		authorizer: &mockAuthorizer{
			checker: &mockChecker{
				rules: []types.Rule{{Resources: []string{"security_report"}, Verbs: []string{"read", "list", "use"}}},
				roles: nil,
			},
		},
		semaphore: &mockSemaphore{},
		clock:     clock,
		storage:   store,
		athena:    &queryLimiter{queryProvider: mockAthena, limiter: lim},
		emitter:   &mockEmitter{},
		reportStore: &mockReportStore{
			m: map[string]*pb.ReportResult{},
		},
		ParentCtx: context.Background(),
	}
	mustUpsertReport(t, store)

	return &suite{
		svc:               svc,
		sched:             sched,
		runQueryCallCount: &runQueryCallCount,
		lim:               lim,
		clock:             clock,
		costLimiterStore:  store,
	}
}

func mustRunReportAndWaitForAllQueries(t *testing.T, mockAthena *athenaMock, svc *Service, queryFn runQueryFuncType) {
	t.Helper()
	ctx := context.Background()
	var runQueryCount atomic.Int64

	mockAthena.runQueryFunc = func(ctx context.Context, queryText string, days int) (*query.RunQueryResponse, error) {
		defer runQueryCount.Add(1)
		return queryFn(ctx, queryText, days)

	}
	_, err := svc.RunReport(ctx, &pb.RunReportRequest{
		Name: reports.PrivilegeAccessReport.Name,
		Days: 7,
	})
	require.NoError(t, err)

	require.EventuallyWithT(t, func(t *assert.CollectT) {
		assert.Len(t, reports.PrivilegeAccessReport.Queries, int(runQueryCount.Load()))
	}, time.Second*3, time.Millisecond*100)

}

func mustUpsertReport(t *testing.T, store services.SecReports) {
	r := &reports.AuditReportType{
		Name: "test_report",
		Queries: []reports.AuditQueryType{
			{
				Name:  "test_query",
				Query: "select 1",
			},
		},
	}
	rep, err := reports.ToSecurityReportType(r)
	require.NoError(t, err)
	err = store.UpsertSecurityReport(context.Background(), rep)
	require.NoError(t, err)
}
