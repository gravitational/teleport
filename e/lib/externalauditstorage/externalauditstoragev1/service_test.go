package externalauditstoragev1

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/externalauditstorage/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	apievents "github.com/gravitational/teleport/api/types/events"
	"github.com/gravitational/teleport/api/types/externalauditstorage"
	conv "github.com/gravitational/teleport/api/types/externalauditstorage/convert/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
	"github.com/gravitational/teleport/lib/utils/clocki"
)

type testPack struct {
	clock           clocki.FakeClock
	mem             *memory.Memory
	s               *local.ExternalAuditStorageService
	integrationsSvc *local.IntegrationsService
}

func newTestPack(t *testing.T) *testPack {
	t.Helper()

	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	s := local.NewExternalAuditStorageService(mem)

	integrationsSvc, err := local.NewIntegrationsService(mem)
	require.NoError(t, err)

	oidcIntegration, err := types.NewIntegrationAWSOIDC(
		types.Metadata{Name: "aws-integration-1"},
		&types.AWSOIDCIntegrationSpecV1{
			RoleARN: "role1",
		},
	)
	require.NoError(t, err)
	_, err = integrationsSvc.CreateIntegration(context.Background(), oidcIntegration)
	require.NoError(t, err)

	return &testPack{
		clock:           clock,
		mem:             mem,
		s:               s,
		integrationsSvc: integrationsSvc,
	}
}

type fakeAuthorizer struct {
	checker *fakeChecker
}

func (f *fakeAuthorizer) Authorize(ctx context.Context) (*authz.Context, error) {
	return &authz.Context{
		Checker: f.checker,
	}, nil
}

type fakeChecker struct {
	services.AccessChecker
	allow  map[check]bool
	checks []check
}

func (f *fakeChecker) CheckAccessToRule(context services.RuleContext, namespace string, rule string, verb string) error {
	c := check{rule, verb}
	f.checks = append(f.checks, c)
	if f.allow[c] {
		return nil
	}
	return trace.AccessDenied("access to %s with verb %s is not allowed", rule, verb)
}

type check struct {
	rule, verb string
}

func TestRBAC(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	sampleAthenaURI := "athena://db.table?topicArn=arn:aws:sns:eu-central-1:accnr:topicName&queryResultsS3=s3://testbucket/query-result/&workgroup=workgroup&locationS3=s3://testbucket/events-location&queueURL=https://sqs.eu-central-1.amazonaws.com/accnr/sqsname&largeEventsS3=s3://testbucket/largeevents"
	clusterAuditConfig, err := types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
		AuditEventsURI: []string{sampleAthenaURI},
		Region:         "us-west-2",
	})
	require.NoError(t, err)

	draftAuditConfig := pb.ExternalAuditStorage_builder{
		Header: headerv1.ResourceHeader_builder{
			Metadata: headerv1.Metadata_builder{
				Name: types.MetaNameExternalAuditStorageDraft,
			}.Build(),
		}.Build(),
		Spec: pb.ExternalAuditStorageSpec_builder{
			IntegrationName:        "aws-integration-1",
			Region:                 "us-west-2",
			PolicyName:             "test-policy",
			SessionRecordingsUri:   "s3://bucket/sess",
			AthenaWorkgroup:        "primary",
			GlueDatabase:           "teleport_db",
			GlueTable:              "teleport_table",
			AuditEventsLongTermUri: "s3://bucket/events",
			AthenaResultsUri:       "s3://bucket/results",
		}.Build(),
	}.Build()

	for _, tc := range []struct {
		desc         string
		f            func(*Service) error
		allow        map[check]bool
		expectChecks []check
		expectEvents []string
	}{
		{
			desc: "create draft",
			f: func(service *Service) error {
				_, err := service.CreateDraftExternalAuditStorage(ctx, pb.CreateDraftExternalAuditStorageRequest_builder{
					ExternalAuditStorage: draftAuditConfig,
				}.Build())
				return err
			},
			allow: map[check]bool{
				{types.KindExternalAuditStorage, types.VerbCreate}: true,
			},
			expectChecks: []check{
				{types.KindExternalAuditStorage, types.VerbCreate},
			},
		},
		{
			desc: "upsert draft",
			f: func(service *Service) error {
				_, err := service.UpsertDraftExternalAuditStorage(ctx, pb.UpsertDraftExternalAuditStorageRequest_builder{
					ExternalAuditStorage: draftAuditConfig,
				}.Build())
				return err
			},
			allow: map[check]bool{
				{types.KindExternalAuditStorage, types.VerbCreate}: true,
				{types.KindExternalAuditStorage, types.VerbUpdate}: true,
			},
			expectChecks: []check{
				{types.KindExternalAuditStorage, types.VerbCreate},
				{types.KindExternalAuditStorage, types.VerbUpdate},
			},
		},
		{
			desc: "get draft",
			f: func(service *Service) error {
				_, err := service.GetDraftExternalAuditStorage(ctx, &pb.GetDraftExternalAuditStorageRequest{})
				return err
			},
			allow: map[check]bool{
				{types.KindExternalAuditStorage, types.VerbRead}: true,
			},
			expectChecks: []check{
				{types.KindExternalAuditStorage, types.VerbRead},
			},
		},
		{
			desc: "promote to cluster",
			f: func(service *Service) error {
				_, err := service.PromoteToClusterExternalAuditStorage(ctx, &pb.PromoteToClusterExternalAuditStorageRequest{})
				return err
			},
			allow: map[check]bool{
				{types.KindExternalAuditStorage, types.VerbRead}:   true,
				{types.KindExternalAuditStorage, types.VerbCreate}: true,
				{types.KindExternalAuditStorage, types.VerbUpdate}: true,
			},
			expectChecks: []check{
				{types.KindExternalAuditStorage, types.VerbRead},
				{types.KindExternalAuditStorage, types.VerbCreate},
				{types.KindExternalAuditStorage, types.VerbUpdate},
			},
			expectEvents: []string{"external_audit_storage.enable"},
		},
		{
			desc: "get cluster",
			f: func(service *Service) error {
				_, err := service.GetClusterExternalAuditStorage(ctx, &pb.GetClusterExternalAuditStorageRequest{})
				return err
			},
			allow: map[check]bool{
				{types.KindExternalAuditStorage, types.VerbRead}: true,
			},
			expectChecks: []check{
				{types.KindExternalAuditStorage, types.VerbRead},
			},
		},
		{
			desc: "delete cluster",
			f: func(service *Service) error {
				_, err := service.DisableClusterExternalAuditStorage(ctx, &pb.DisableClusterExternalAuditStorageRequest{})
				return err
			},
			allow: map[check]bool{
				{types.KindExternalAuditStorage, types.VerbDelete}: true,
			},
			expectChecks: []check{
				{types.KindExternalAuditStorage, types.VerbDelete},
			},
			expectEvents: []string{"external_audit_storage.disable"},
		},
		{
			desc: "generate draft",
			f: func(service *Service) error {
				_, err := service.GenerateDraftExternalAuditStorage(ctx, pb.GenerateDraftExternalAuditStorageRequest_builder{
					IntegrationName: "aws-integration-1",
					Region:          "us-west-2",
				}.Build())
				return err
			},
			allow: map[check]bool{
				{types.KindExternalAuditStorage, types.VerbCreate}: true,
			},
			expectChecks: []check{
				{types.KindExternalAuditStorage, types.VerbCreate},
			},
		},
		{
			desc: "delete draft",
			f: func(service *Service) error {
				_, err := service.DeleteDraftExternalAuditStorage(ctx, &pb.DeleteDraftExternalAuditStorageRequest{})
				return err
			},
			allow: map[check]bool{
				{types.KindExternalAuditStorage, types.VerbDelete}: true,
			},
			expectChecks: []check{
				{types.KindExternalAuditStorage, types.VerbDelete},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			authorizer := &fakeAuthorizer{}
			emitter := &fakeEmitter{}
			cfg := &ServiceConfig{
				ExternalAuditStorage:     p.s,
				Authorizer:               authorizer,
				ClusterAuditConfigGetter: &staticAuditConfigGetter{clusterAuditConfig},
				IntegrationSvc:           p.integrationsSvc,
				OIDCTokenFn:              func(context.Context, string) (string, error) { return "token", nil },
				Emitter:                  emitter,
			}

			service, err := NewService(cfg)
			require.NoError(t, err)

			// First check with nothing allowed.
			authorizer.checker = &fakeChecker{}
			err = tc.f(service)
			require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)

			// Check with allowed rule/verbs from testcase.
			authorizer.checker = &fakeChecker{
				allow: tc.allow,
			}
			err = tc.f(service)
			require.NoError(t, err, trace.DebugReport(err))
			require.ElementsMatch(t, tc.expectChecks, authorizer.checker.checks)
			require.Equal(t, tc.expectEvents, emitter.events)
		})
	}
}

func TestClusterAuditConfigCheck(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	authorizer := &fakeAuthorizer{&fakeChecker{
		allow: map[check]bool{
			{types.KindExternalAuditStorage, types.VerbCreate}: true,
			{types.KindExternalAuditStorage, types.VerbUpdate}: true,
			{types.KindExternalAuditStorage, types.VerbRead}:   true,
			{types.KindExternalAuditStorage, types.VerbDelete}: true,
		},
	}}
	sampleAthenaURI := "athena://db.table?topicArn=arn:aws:sns:eu-central-1:accnr:topicName&queryResultsS3=s3://testbucket/query-result/&workgroup=workgroup&locationS3=s3://testbucket/events-location&queueURL=https://sqs.eu-central-1.amazonaws.com/accnr/sqsname&largeEventsS3=s3://testbucket/largeevents"
	sampleFileURI := "file:///tmp/teleport-test/events"

	for _, tc := range []struct {
		desc               string
		auditURIs          []string
		clusterAuditRegion string
		easRegion          string
		expectErr          error
	}{
		{
			desc:               "only athena",
			auditURIs:          []string{sampleAthenaURI},
			clusterAuditRegion: "eu-central-1",
			easRegion:          "eu-central-1",
		},
		{
			desc:               "with athena",
			auditURIs:          []string{sampleFileURI, sampleAthenaURI},
			clusterAuditRegion: "eu-central-1",
			easRegion:          "eu-central-1",
		},
		{
			desc:               "without athena",
			auditURIs:          []string{sampleFileURI},
			clusterAuditRegion: "eu-central-1",
			easRegion:          "eu-central-1",
			expectErr:          externalAuditMissingAthenaError,
		},
		{
			desc:               "wrong region",
			auditURIs:          []string{sampleAthenaURI},
			clusterAuditRegion: "eu-central-1",
			easRegion:          "us-west-2",
			expectErr:          trace.BadParameter(`region "us-west-2" rejected: External Audit Storage must be configured in "eu-central-1"`),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			clusterAuditConfig, err := types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
				AuditEventsURI: tc.auditURIs,
				Region:         tc.clusterAuditRegion,
			})
			require.NoError(t, err)

			cfg := &ServiceConfig{
				ExternalAuditStorage:     p.s,
				Authorizer:               authorizer,
				ClusterAuditConfigGetter: &staticAuditConfigGetter{clusterAuditConfig},
				IntegrationSvc:           p.integrationsSvc,
				OIDCTokenFn:              func(context.Context, string) (string, error) { return "token", nil },
				Emitter:                  &fakeEmitter{},
			}
			service, err := NewService(cfg)
			require.NoError(t, err)

			_, err = service.GenerateDraftExternalAuditStorage(ctx, pb.GenerateDraftExternalAuditStorageRequest_builder{
				Region:          tc.easRegion,
				IntegrationName: "aws-integration-1",
			}.Build())
			assert.ErrorIs(t, err, tc.expectErr)

			// Clean up for Create test.
			if tc.expectErr == nil {
				_, err = service.DeleteDraftExternalAuditStorage(ctx, &pb.DeleteDraftExternalAuditStorageRequest{})
				require.NoError(t, err)
			}

			draft, err := externalauditstorage.GenerateDraftExternalAuditStorage("aws-integration-1", tc.easRegion)
			require.NoError(t, err)

			_, err = service.CreateDraftExternalAuditStorage(ctx, pb.CreateDraftExternalAuditStorageRequest_builder{
				ExternalAuditStorage: conv.ToProto(draft),
			}.Build())
			assert.ErrorIs(t, err, tc.expectErr)

			_, err = service.UpsertDraftExternalAuditStorage(ctx, pb.UpsertDraftExternalAuditStorageRequest_builder{
				ExternalAuditStorage: conv.ToProto(draft),
			}.Build())
			assert.ErrorIs(t, err, tc.expectErr)

			// Nothing to promote if we can't write the draft in the first place.
			if tc.expectErr == nil {
				_, err = service.PromoteToClusterExternalAuditStorage(ctx, &pb.PromoteToClusterExternalAuditStorageRequest{})
				assert.NoError(t, err)
			}
		})
	}
}

type staticAuditConfigGetter struct {
	clusterAuditConfig types.ClusterAuditConfig
}

func (s *staticAuditConfigGetter) GetClusterAuditConfig(ctx context.Context) (types.ClusterAuditConfig, error) {
	return s.clusterAuditConfig, nil
}

type fakeEmitter struct {
	events []string
}

func (f *fakeEmitter) EmitAuditEvent(ctx context.Context, e apievents.AuditEvent) error {
	f.events = append(f.events, e.GetType())
	return nil
}
