package externalcloudauditv1

import (
	"context"
	"testing"

	"github.com/gravitational/trace"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	pb "github.com/gravitational/teleport/api/gen/proto/go/teleport/externalcloudaudit/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/externalcloudaudit"
	conv "github.com/gravitational/teleport/api/types/externalcloudaudit/convert/v1"
	"github.com/gravitational/teleport/lib/authz"
	"github.com/gravitational/teleport/lib/backend/memory"
	"github.com/gravitational/teleport/lib/services"
	"github.com/gravitational/teleport/lib/services/local"
)

type testPack struct {
	clock clockwork.FakeClock
	mem   *memory.Memory
	s     services.ExternalCloudAudits
}

func newTestPack(t *testing.T) *testPack {
	t.Helper()

	clock := clockwork.NewFakeClock()

	mem, err := memory.New(memory.Config{
		Clock: clock,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, mem.Close()) })

	s := local.NewExternalCloudAuditService(mem)

	return &testPack{
		clock: clock,
		mem:   mem,
		s:     s,
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

func (f *fakeChecker) CheckAccessToRule(context services.RuleContext, namespace string, rule string, verb string, silent bool) error {
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

	authorizer := &fakeAuthorizer{}
	sampleAthenaURI := "athena://db.table?topicArn=arn:aws:sns:eu-central-1:accnr:topicName&queryResultsS3=s3://testbucket/query-result/&workgroup=workgroup&locationS3=s3://testbucket/events-location&queueURL=https://sqs.eu-central-1.amazonaws.com/accnr/sqsname&largeEventsS3=s3://testbucket/largeevents"
	clusterAuditConfig, err := types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
		AuditEventsURI: []string{sampleAthenaURI},
	})
	require.NoError(t, err)

	cfg := &ServiceConfig{
		ExternalCloudAudit:       p.s,
		Authorizer:               authorizer,
		ClusterAuditConfigGetter: &staticAuditConfigGetter{clusterAuditConfig},
	}

	service, err := NewService(cfg)
	require.NoError(t, err)

	draftAuditConfig := &pb.ExternalCloudAudit{
		Header: &headerv1.ResourceHeader{
			Metadata: &headerv1.Metadata{
				Name: types.MetaNameExternalCloudAuditDraft,
			},
		},
		Spec: &pb.ExternalCloudAuditSpec{
			IntegrationName:        "aws-integration-1",
			Region:                 "us-west-2",
			PolicyName:             "test-policy",
			SessionsRecordingsUri:  "s3://bucket/sess",
			AthenaWorkgroup:        "primary",
			GlueDatabase:           "teleport_db",
			GlueTable:              "teleport_table",
			AuditEventsLongTermUri: "s3://bucket/events",
			AthenaResultsUri:       "s3://bucket/results",
		},
	}

	for _, tc := range []struct {
		desc         string
		f            func() error
		allow        map[check]bool
		expectChecks []check
	}{
		{
			desc: "upsert draft",
			f: func() error {
				_, err := service.UpsertDraftExternalCloudAudit(ctx, &pb.UpsertDraftExternalCloudAuditRequest{
					ExternalCloudAudit: draftAuditConfig,
				})
				return err
			},
			allow: map[check]bool{
				{types.KindExternalCloudAudit, types.VerbCreate}: true,
				{types.KindExternalCloudAudit, types.VerbUpdate}: true,
			},
			expectChecks: []check{
				{types.KindExternalCloudAudit, types.VerbCreate},
				{types.KindExternalCloudAudit, types.VerbUpdate},
			},
		},
		{
			desc: "get draft",
			f: func() error {
				_, err := service.GetDraftExternalCloudAudit(ctx, &pb.GetDraftExternalCloudAuditRequest{})
				return err
			},
			allow: map[check]bool{
				{types.KindExternalCloudAudit, types.VerbRead}: true,
			},
			expectChecks: []check{
				{types.KindExternalCloudAudit, types.VerbRead},
			},
		},
		{
			desc: "promote to cluster",
			f: func() error {
				_, err := service.PromoteToClusterExternalCloudAudit(ctx, &pb.PromoteToClusterExternalCloudAuditRequest{})
				return err
			},
			allow: map[check]bool{
				{types.KindExternalCloudAudit, types.VerbCreate}: true,
			},
			expectChecks: []check{
				{types.KindExternalCloudAudit, types.VerbCreate},
			},
		},
		{
			desc: "get cluster",
			f: func() error {
				_, err := service.GetClusterExternalCloudAudit(ctx, &pb.GetClusterExternalCloudAuditRequest{})
				return err
			},
			allow: map[check]bool{
				{types.KindExternalCloudAudit, types.VerbRead}: true,
			},
			expectChecks: []check{
				{types.KindExternalCloudAudit, types.VerbRead},
			},
		},
		{
			desc: "delete cluster",
			f: func() error {
				_, err := service.DisableClusterExternalCloudAudit(ctx, &pb.DisableClusterExternalCloudAuditRequest{})
				return err
			},
			allow: map[check]bool{
				{types.KindExternalCloudAudit, types.VerbDelete}: true,
			},
			expectChecks: []check{
				{types.KindExternalCloudAudit, types.VerbDelete},
			},
		},
		{
			desc: "generate draft",
			f: func() error {
				_, err := service.GenerateDraftExternalCloudAudit(ctx, &pb.GenerateDraftExternalCloudAuditRequest{
					IntegrationName: "test-integration",
					Region:          "us-west-2",
				})
				return err
			},
			allow: map[check]bool{
				{types.KindExternalCloudAudit, types.VerbCreate}: true,
			},
			expectChecks: []check{
				{types.KindExternalCloudAudit, types.VerbCreate},
			},
		},
		{
			desc: "delete draft",
			f: func() error {
				_, err := service.DeleteDraftExternalCloudAudit(ctx, &pb.DeleteDraftExternalCloudAuditRequest{})
				return err
			},
			allow: map[check]bool{
				{types.KindExternalCloudAudit, types.VerbDelete}: true,
			},
			expectChecks: []check{
				{types.KindExternalCloudAudit, types.VerbDelete},
			},
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			// First check with nothing allowed.
			authorizer.checker = &fakeChecker{}
			err := tc.f()
			require.True(t, trace.IsAccessDenied(err), "expected AccessDenied error, got %v", err)

			// Check with allowed rule/verbs from testcase.
			authorizer.checker = &fakeChecker{
				allow: tc.allow,
			}
			err = tc.f()
			require.NoError(t, err, trace.DebugReport(err))
			require.ElementsMatch(t, tc.expectChecks, authorizer.checker.checks)
		})
	}
}

func TestClusterAuditConfigCheck(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	p := newTestPack(t)

	authorizer := &fakeAuthorizer{&fakeChecker{
		allow: map[check]bool{
			{types.KindExternalCloudAudit, types.VerbCreate}: true,
			{types.KindExternalCloudAudit, types.VerbUpdate}: true,
		},
	}}
	sampleAthenaURI := "athena://db.table?topicArn=arn:aws:sns:eu-central-1:accnr:topicName&queryResultsS3=s3://testbucket/query-result/&workgroup=workgroup&locationS3=s3://testbucket/events-location&queueURL=https://sqs.eu-central-1.amazonaws.com/accnr/sqsname&largeEventsS3=s3://testbucket/largeevents"
	sampleFileURI := "file:///tmp/teleport-test/events"
	sampleExternalCloudAudit, err := externalcloudaudit.GenerateDraftExternalCloudAudit("test-integration", "us-west-2")
	require.NoError(t, err)

	for _, tc := range []struct {
		desc      string
		auditURIs []string
		expectErr error
	}{
		{
			desc:      "only athena",
			auditURIs: []string{sampleAthenaURI},
		},
		{
			desc:      "with athena",
			auditURIs: []string{sampleFileURI, sampleAthenaURI},
		},
		{
			desc:      "without athena",
			auditURIs: []string{sampleFileURI},
			expectErr: externalAuditMissingAthenaError,
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			clusterAuditConfig, err := types.NewClusterAuditConfig(types.ClusterAuditConfigSpecV2{
				AuditEventsURI: tc.auditURIs,
			})
			require.NoError(t, err)

			cfg := &ServiceConfig{
				ExternalCloudAudit:       p.s,
				Authorizer:               authorizer,
				ClusterAuditConfigGetter: &staticAuditConfigGetter{clusterAuditConfig},
			}
			service, err := NewService(cfg)
			require.NoError(t, err)

			_, err = service.GenerateDraftExternalCloudAudit(ctx, &pb.GenerateDraftExternalCloudAuditRequest{
				Region:          "us-west-2",
				IntegrationName: "test-integration",
			})
			assert.ErrorIs(t, err, tc.expectErr)

			_, err = service.UpsertDraftExternalCloudAudit(ctx, &pb.UpsertDraftExternalCloudAuditRequest{
				ExternalCloudAudit: conv.ToProto(sampleExternalCloudAudit),
			})
			assert.ErrorIs(t, err, tc.expectErr)

			_, err = service.PromoteToClusterExternalCloudAudit(ctx, &pb.PromoteToClusterExternalCloudAuditRequest{})
			assert.ErrorIs(t, err, tc.expectErr)
		})
	}
}

type staticAuditConfigGetter struct {
	clusterAuditConfig types.ClusterAuditConfig
}

func (s *staticAuditConfigGetter) GetClusterAuditConfig(ctx context.Context, opts ...services.MarshalOption) (types.ClusterAuditConfig, error) {
	return s.clusterAuditConfig, nil
}
